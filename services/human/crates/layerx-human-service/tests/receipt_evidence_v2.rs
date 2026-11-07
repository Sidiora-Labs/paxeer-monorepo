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
        .unwrap_or_else(|poisoned| poisoned.into_inner());
    let mut fixture = Fixture::open()?;
    fixture
        .peer_state
        .lock()
        .map_err(|_| "peer state")?
        .receipt_rung = Some(rung);
    let plan = fixture.plan()?;
    let body = fixture.submission(&plan)?;
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
    let key = receipt_key();
    let mut evidence_paths = Vec::new();
    for stage in stages {
        let evidence_id = stage["evidence"][0]["evidence_id"]
            .as_str()
            .ok_or("evidence id")?;
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
        let parsed = PortableReceiptV2::from_json(&bytes)?;
        let verified = PortableReceiptV2::verify_json(&bytes, &[key])?;
        assert_eq!(parsed.to_json()?, bytes);
        assert_eq!(
            stage["tx_hash"],
            format!("0x{}", cluster::hex(&verified.activity_id()))
        );
        assert_eq!(verified.receipt_key(), &key);
        let inclusion_height = if rung == Rung::Committed {
            HEAD + stage["nonce"].as_u64().ok_or("leg nonce")? - NONCE + 1
        } else {
            HEAD
        };
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
