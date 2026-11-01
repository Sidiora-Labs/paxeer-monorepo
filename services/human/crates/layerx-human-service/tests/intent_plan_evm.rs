#[path = "support/production_fixture.rs"]
mod production_fixture;

use production_fixture::*;
use serde_json::json;
use std::time::Duration;

use sha2::{Digest, Sha256};

fn transaction_hash(text: &str) -> std::result::Result<[u8; 32], String> {
    let digits = text
        .strip_prefix("0x")
        .ok_or_else(|| "transaction hash: MissingPrefix".to_owned())?;
    if digits.len() != 64 {
        return Err("transaction hash: WrongLength".to_owned());
    }
    if !digits.bytes().all(|byte| byte.is_ascii_hexdigit()) {
        return Err("transaction hash: NotHex".to_owned());
    }
    let mut bytes = [0; 32];
    for (index, byte) in bytes.iter_mut().enumerate() {
        *byte = u8::from_str_radix(&digits[index * 2..index * 2 + 2], 16)
            .map_err(|_| "transaction hash: NotHex".to_owned())?;
    }
    Ok(bytes)
}

#[test]
fn intent_plan_answers_eip1559_legs() -> Result<()> {
    assert_eq!(<[u8; 32]>::from(Sha256::digest(b"alxp")), ASSET);
    let _guard = ENVIRONMENT
        .lock()
        .unwrap_or_else(|poisoned| poisoned.into_inner());
    let fixture = Fixture::open()?;
    let plan = fixture.plan()?;
    let legs = plan["legs"].as_array().ok_or("legs")?;
    assert!(!legs.is_empty());
    let first = legs[0]["nonce"].as_u64().ok_or("pending nonce")?;
    assert_eq!(first, NONCE);
    for (index, leg) in legs.iter().enumerate() {
        assert_eq!(leg["nonce"], first + index as u64);
        assert_eq!(leg["value"], "0");
        assert_eq!(leg["to"].as_str().ok_or("to")?.len(), 42);
        assert!(leg["calldata"].as_str().ok_or("calldata")?.len() >= 10);
        let maximum: u64 = leg["max_fee_per_gas"].as_str().ok_or("max fee")?.parse()?;
        let priority: u64 = leg["max_priority_fee_per_gas"]
            .as_str()
            .ok_or("priority fee")?
            .parse()?;
        assert!(maximum > 0);
        assert!(priority <= maximum);
        assert!(leg.get("tx_hash").is_none());
    }
    assert_eq!(plan, fixture.plan()?);
    Ok(())
}

#[test]
fn intent_submit_records_each_leg_tx_hash() -> Result<()> {
    let _guard = ENVIRONMENT
        .lock()
        .unwrap_or_else(|poisoned| poisoned.into_inner());
    let mut fixture = Fixture::open()?;
    let plan = fixture.plan()?;
    let body = fixture.submission(&plan)?;
    let submitted = fixture.request("POST", "/v1/intents/submit", &body, "intent-submit-evm")?;
    assert_eq!(submitted["plan_digest"], plan["plan_digest"]);
    let path = format!(
        "/v1/journeys/{}",
        submitted["journey_id"].as_str().ok_or("journey id")?
    );
    let journey = fixture.request("GET", &path, &json!({}), "intent-get-evm")?;
    let record = fixture.journey_record(submitted["journey_id"].as_str().ok_or("journey id")?)?;
    let legs = journey["stages"].as_array().ok_or("journey stages")?;
    assert_eq!(journey["kind"], "move");
    assert_eq!(
        journey["state_copy_key"],
        format!(
            "journey.state.{}",
            journey["state"].as_str().ok_or("journey state")?
        )
    );
    assert!(journey.get("legs").is_none());
    assert!(!serde_json::to_string(&journey)?.contains("raw_transaction"));
    assert_eq!(journey["started_at"], record["started_at"]);
    assert_eq!(journey["updated_at"], record["updated_at"]);
    assert_eq!(
        legs.len(),
        plan["legs"].as_array().ok_or("plan legs")?.len()
    );
    assert!(!legs.is_empty());
    let node =
        layerx_agentd::node_rpc::NodeRpc::connect(&layerx_agentd::node_rpc::NodeRpcConfig {
            url: fixture.rpc_server.as_ref().ok_or("RPC peer")?.url.clone(),
            chain_id: 125,
            deadline: Duration::from_secs(5),
        })?;
    let mut transaction_hashes = std::collections::BTreeSet::new();
    for (index, leg) in legs.iter().enumerate() {
        let hash = leg["tx_hash"].as_str().ok_or("transaction hash")?;
        let hash = transaction_hash(hash)?;
        assert_ne!(hash, [0; 32]);
        assert!(
            transaction_hashes.insert(hash),
            "each leg must have its own signed transaction"
        );
        assert_eq!(leg["nonce"], plan["legs"][index]["nonce"]);
        assert!(leg.get("raw_transaction").is_none());
        assert_eq!(record["legs"][index]["tx_hash"], leg["tx_hash"]);
        assert_eq!(record["legs"][index]["nonce"], leg["nonce"]);
        assert_eq!(
            leg["stage_id"],
            format!(
                "stg_{}_{index}",
                submitted["journey_id"].as_str().ok_or("journey id")?
            )
        );
        assert_eq!(
            leg["evidence"][0]["evidence_id"],
            format!("evd_{}", cluster::hex(&hash))
        );
        assert_eq!(journey["evidence"][index], leg["evidence"][0]);
        assert_eq!(leg["copy_key"], "journey.stage.submitted");
        assert_eq!(leg["evidence"][0]["class"], "submission-record");
        assert_eq!(leg["evidence"][0]["verification"], "unverified");
        let raw: Vec<u8> =
            serde_json::from_value(record["legs"][index]["raw_transaction"].clone())?;
        assert_eq!(layerx_crypto::evm_transaction::transaction_hash(&raw), hash);
        let (call, signature) = layerx_crypto::evm_transaction::decode_signed(&raw)
            .map_err(|error| format!("signed transaction: {error:?}"))?;
        let planned = &plan["legs"][index];
        assert_eq!(call.nonce, planned["nonce"].as_u64().ok_or("leg nonce")?);
        assert_eq!(
            call.chain_id,
            planned["chain_id"].as_u64().ok_or("chain id")?
        );
        assert_eq!(
            call.gas_limit,
            planned["gas_limit"].as_u64().ok_or("gas limit")?
        );
        assert_eq!(call.max_fee_per_gas.to_string(), planned["max_fee_per_gas"]);
        assert_eq!(
            call.max_priority_fee_per_gas.to_string(),
            planned["max_priority_fee_per_gas"]
        );
        assert_eq!(format!("0x{}", cluster::hex(&call.to)), planned["to"]);
        assert_eq!(
            format!("0x{}", cluster::hex(&call.data)),
            planned["calldata"]
        );
        let peer = fixture.peer_state.lock().map_err(|_| "peer state")?;
        let actor = fixture.document["actor"].as_str().ok_or("actor")?;
        assert_eq!(
            layerx_crypto::evm_transaction::recover_sender(&call, &signature)
                .map_err(|error| format!("transaction sender: {error:?}"))?,
            peer.identities.get(actor).ok_or("owner identity")?.1
        );
        assert_eq!(peer.submitted.get(&hash), Some(&raw));
        drop(peer);
        assert_eq!(record["legs"][index]["submitted"], true);
        let receipt = node
            .sequencer_receipt(&hash)?
            .ok_or("submitted transaction receipt")?;
        let observed = match receipt {
            layerx_client::sequencer::SequencerReceipt::Submitted { tx_hash, .. } => tx_hash,
            layerx_client::sequencer::SequencerReceipt::Receipted(receipt)
            | layerx_client::sequencer::SequencerReceipt::Committed { receipt, .. } => {
                receipt.tx_hash
            }
        };
        assert_eq!(observed, hash);
    }
    let page = fixture.request("GET", "/v1/journeys", &json!({}), "intent-list-evm")?;
    let listed = page["journeys"]
        .as_array()
        .ok_or("journey page")?
        .iter()
        .find(|entry| entry["journey_id"] == journey["journey_id"])
        .ok_or("listed intent journey")?;
    assert_eq!(listed, &journey);
    assert!(!serde_json::to_string(&page)?.contains("raw_transaction"));
    let submitted_before_restart = fixture
        .peer_state
        .lock()
        .map_err(|_| "peer state")?
        .submitted
        .clone();
    assert_eq!(submitted_before_restart.len(), legs.len());
    fixture.reopen()?;
    let recovered = fixture.request("GET", &path, &json!({}), "intent-recovered-evm")?;
    let recovered_record =
        fixture.journey_record(submitted["journey_id"].as_str().ok_or("journey id")?)?;
    let before =
        time::seconds_from_rfc3339(journey["updated_at"].as_str().ok_or("updated timestamp")?)
            .ok_or("updated timestamp format")?;
    let after = time::seconds_from_rfc3339(
        recovered["updated_at"]
            .as_str()
            .ok_or("recovered timestamp")?,
    )
    .ok_or("recovered timestamp format")?;
    assert!(after >= before);
    assert_eq!(recovered["updated_at"], recovered_record["updated_at"]);
    let mut expected_recovered = journey.clone();
    expected_recovered["updated_at"] = recovered_record["updated_at"].clone();
    assert_eq!(expected_recovered, recovered);
    assert_eq!(
        submitted,
        fixture.request("POST", "/v1/intents/submit", &body, "intent-submit-evm")?
    );
    assert_eq!(
        fixture
            .peer_state
            .lock()
            .map_err(|_| "peer state")?
            .submitted,
        submitted_before_restart
    );
    Ok(())
}
