#[path = "support/production_fixture_plan.rs"]
mod fixture_plan;
#[path = "support/production_fixture.rs"]
mod production_fixture;

use base64::{engine::general_purpose::STANDARD, Engine as _};
use layerx_portable::PortableReceiptV2;
use layerx_types::rung::Rung;
use production_fixture::*;
use serde_json::json;

fn exercise_receipt_evidence(rung: Rung) -> Result<()> {
    let verification = match rung {
        Rung::Receipted => "receipt-verified",
        Rung::Committed => "committed",
        Rung::Submitted => return Err("submitted has no verified receipt".into()),
    };
    let _guard = ENVIRONMENT
        .lock()
        .unwrap_or_else(std::sync::PoisonError::into_inner);
    let mut fixture = Fixture::open()?;
    let pending_nonce = fixture.start_bank_node()?;
    let node = fixture
        .peer_state
        .lock()
        .map_err(|_| "peer state")?
        .bank_node
        .clone()
        .ok_or("bank node")?;
    let keys = node.receipt_keys();
    fixture
        .peer_state
        .lock()
        .map_err(|_| "peer state")?
        .receipt_rung = Some(rung);
    let plan = fixture_plan::plan(&fixture)?;
    assert_eq!(plan["legs"][0]["nonce"], pending_nonce);
    let body = fixture_plan::submission(&fixture, &plan)?;
    let submitted = fixture.request("POST", "/v1/intents/submit", &body, "receipt-v2-submit")?;
    let journey_id = submitted["journey_id"].as_str().ok_or("journey id")?;
    let journey = fixture.request(
        "GET",
        &format!("/v1/journeys/{journey_id}"),
        &json!({}),
        "receipt-v2-journey",
    )?;
    let stages = journey["stages"].as_array().ok_or("journey stages")?;
    assert_eq!(
        stages.len(),
        plan["legs"].as_array().ok_or("planned legs")?.len()
    );
    assert!(!stages.is_empty());
    let published_key = keys.first().ok_or("governed receipt key")?;
    let key = layerx_proof::inclusion::SequencerAuthorization::new(
        published_key.key_id,
        published_key.public_key,
        published_key.first_height,
        published_key.last_height,
    );
    let mut evidence_paths = Vec::new();
    for stage in stages {
        let evidence_id = stage["evidence"][0]["evidence_id"]
            .as_str()
            .ok_or("evidence id")?;
        let hash = unhex(stage["tx_hash"].as_str().ok_or("transaction hash")?)
            .try_into()
            .map_err(|_| "transaction hash width")?;
        let receipt = node.receipt(hash, &keys, rung);
        let raw = fixture
            .peer_state
            .lock()
            .map_err(|_| "peer state")?
            .submitted
            .get(&hash)
            .cloned()
            .ok_or("submitted transaction")?;
        let inclusion_height = if rung == Rung::Committed {
            receipt.covering_block().ok_or("covering block")?.0
        } else {
            receipt.height()
        };
        let expected = PortableReceiptV2::export_with_transaction(
            receipt.canonical_bytes(),
            &key,
            inclusion_height,
            &raw,
        )?
        .to_json()?;
        let committed = node.receipt(hash, &keys, Rung::Committed);
        assert_eq!(committed.canonical_bytes(), receipt.canonical_bytes());
        assert_eq!(committed.rung(), Rung::Committed);
        let path = format!("/v1/evidence/{evidence_id}");
        let evidence = fixture.request("GET", &path, &json!({}), "receipt-v2-evidence")?;
        assert_eq!(evidence["evidence_id"], evidence_id);
        assert_eq!(evidence["class"], "layerx-receipt");
        assert_eq!(
            evidence["content_type"],
            "application/vnd.layerx.receipt-proof-v2+json"
        );
        assert_eq!(evidence["verification"], verification);
        assert_eq!(evidence["rung"], rung.name());
        let bytes = STANDARD.decode(evidence["bytes_base64"].as_str().ok_or("receipt bytes")?)?;
        assert_eq!(bytes, expected);
        let parsed = PortableReceiptV2::from_json(&bytes)?;
        let verified = PortableReceiptV2::verify_json(&bytes, &[key])?;
        assert_eq!(parsed.to_json()?, bytes);
        assert_eq!(
            stage["tx_hash"],
            format!("0x{}", cluster::hex(&verified.activity_id()))
        );
        assert_eq!(verified.receipt_key(), &key);
        assert_eq!(
            receipt.execution().nonce,
            stage["nonce"].as_u64().ok_or("leg nonce")?
        );
        assert_eq!(evidence["inclusion_height"], inclusion_height);
        assert_eq!(verified.inclusion_height(), inclusion_height);
        {
            let peer = fixture.peer_state.lock().map_err(|_| "peer state")?;
            let raw = peer
                .submitted
                .get(&verified.activity_id())
                .ok_or("submitted transaction")?;
            assert_eq!(verified.raw_transaction(), Some(raw.as_slice()));
        }
        evidence_paths.push((path, evidence));
    }
    fixture.reopen()?;
    for (path, evidence) in evidence_paths {
        assert_eq!(
            fixture.request("GET", &path, &json!({}), "receipt-v2-reopened")?,
            evidence
        );
    }
    let peer = fixture.peer_state.lock().map_err(|_| "peer state")?;
    assert!(peer
        .rpc_served
        .iter()
        .any(|method| method == "pax_sequencerReceipt"));
    assert!(peer
        .rpc_served
        .iter()
        .any(|method| method == "px_getReceiptKeys"));
    assert_eq!(peer.submitted.len(), stages.len());
    Ok(())
}

#[test]
fn evidence_get_returns_receipt_proof_v2() -> Result<()> {
    exercise_receipt_evidence(Rung::Receipted)
}

#[test]
fn evidence_get_marks_committed_receipts() -> Result<()> {
    exercise_receipt_evidence(Rung::Committed)
}

pub mod require_value {
    include!("support/require_value.rs");
}
