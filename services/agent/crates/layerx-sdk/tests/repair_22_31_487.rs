use layerx_agent_api::budget::{BudgetId, BudgetRecord, BudgetTarget};
use layerx_agent_api::error::RequestId;
use layerx_agent_api::identity::{AgentDid, TenantId};
use layerx_agent_api::prepare::CanonicalBytes;
use layerx_agent_api::proof::{ProofBundle, ProofBundleTarget};
use layerx_agent_api::read::{Freshness, ReadRequest, RelativeTo, VerifiedRead};
use layerx_agent_api::verify::{ApiSuccess, Rung, VerificationStatus};
use layerx_client::sequencer::ReceiptKeyEntry;
use layerx_sdk::agent_envelope::{
    decode_proof_bundle_response, decode_response, encode_envelope, EnvelopeCredential,
};
use layerx_sdk::rpc_budget::decode_budget_state_response;
use layerx_sdk::Operation;
use serde_json::{json, Value};

type Failure = Box<dyn std::error::Error>;

const READ_SCHEMA: &str = include_str!("../../../schema/agent-api/read.kvx");
const RECORDED: &str = include_str!("../src/repair_22_31_227_fixture/committed.json");

fn bytes(value: &Value) -> Vec<u8> {
    let text = value.as_str().expect("recorded hex");
    let text = text.strip_prefix("0x").expect("recorded hex prefix");
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(std::str::from_utf8(pair).expect("hex text"), 16)
                .expect("recorded byte")
        })
        .collect()
}

fn fixed(value: &Value) -> [u8; 32] {
    bytes(value).try_into().expect("recorded bytes32")
}

#[test]
fn recorded_api_read_decodes_through_sdk_without_a_retired_alias() -> Result<(), Failure> {
    assert!(READ_SCHEMA
        .contains("required = [\"value\",\"achieved_verification_level\",\"freshness\"]"));
    let recording: Value = serde_json::from_str(RECORDED)?;
    let signed = bytes(&recording["receiptJson"]["receipt"]);
    let receipt = layerx_wire::receipt::decode(&signed).expect("recorded canonical receipt");
    let digest = layerx_wire::hash::receipt_digest(
        &layerx_wire::receipt::encode_unsigned(&receipt).expect("recorded unsigned receipt"),
    )
    .expect("recorded receipt digest");
    let key = &recording["pinnedKey"];
    let height = key["firstHeight"].as_u64().expect("recorded height");
    let read = VerifiedRead::new(
        ProofBundle {
            format: "layerx-receipt-proof-v2".to_owned(),
            verification_level: Rung::Receipted,
            raw_transaction: CanonicalBytes::new(bytes(&recording["rawTransaction"]))
                .expect("recorded transaction"),
            signed_receipt: CanonicalBytes::new(signed).expect("recorded receipt bytes"),
            receipt_digest: digest,
            receipt_key: ReceiptKeyEntry {
                key_id: fixed(&key["keyId"]),
                public_key: fixed(&key["publicKey"]),
                first_height: height,
                last_height: key["lastHeight"].as_u64().expect("recorded last height"),
            },
            block_header: None,
            receipt_inclusion: None,
            commit: None,
            validator_set: None,
            account_proof: None,
        },
        Rung::Receipted,
        Freshness {
            chain_head: height,
            value_height: height,
            relative_to: RelativeTo::Block {
                height,
                hash: fixed(&recording["receiptJson"]["blockHash"]),
            },
        },
    );
    let wire = read.encode_json().expect("API read encoding");
    assert!(wire.contains("\"achieved_verification_level\":\"receipted\""));
    assert!(!wire.contains("achieved_rung"));
    let response = ApiSuccess {
        request_id: RequestId(487),
        value: serde_json::from_str(&wire)?,
        verification_status: VerificationStatus::Achieved(Rung::Receipted),
    };
    let request = ReadRequest {
        selector: ProofBundleTarget::Receipt(fixed(&recording["receiptJson"]["txHash"]))
            .encode()
            .expect("recorded receipt selector"),
        requested_rung: Rung::Receipted,
    };
    assert_eq!(
        decode_proof_bundle_response(&request, &response),
        Some(read)
    );
    for (pointer, value) in [
        ("/freshness/chain_head", json!((height - 1).to_string())),
        (
            "/freshness/relative_to/height",
            json!((height + 1).to_string()),
        ),
        (
            "/freshness/relative_to/hash",
            json!(format!("0x{}", "00".repeat(32))),
        ),
        (
            "/freshness/relative_to/hash",
            json!(recording["receiptJson"]["blockHash"]
                .as_str()
                .expect("recorded block hash")
                .strip_prefix("0x")
                .expect("hash prefix")),
        ),
    ] {
        let mut malformed = response.clone();
        *malformed
            .value
            .pointer_mut(pointer)
            .expect("freshness field") = value;
        assert!(decode_proof_bundle_response(&request, &malformed).is_none());
    }
    for keep_current in [false, true] {
        let mut legacy = response.clone();
        legacy.value["achieved_rung"] = json!("receipted");
        if !keep_current {
            legacy
                .value
                .as_object_mut()
                .expect("read object")
                .remove("achieved_verification_level");
        }
        assert!(decode_proof_bundle_response(&request, &legacy).is_none());
    }
    Ok(())
}

fn recorded_budget_response() -> Value {
    json!({
        "request_id": "487",
        "verification_status": {"state": "achieved", "rung": "committed"},
        "value": {
            "authority": {
                "tenant": "identity-ops", "agent_did": "did:layerx:identity-ops",
                "authority_ref": "08".repeat(32),
                "protocol_authority": format!("02{}", "08".repeat(32)),
            },
            "value": {
                "record": {
                    "enforcement": "ProtocolBudget", "budget_id": "09".repeat(32),
                    "owner": "11".repeat(32), "budget_account": "22".repeat(32),
                    "asset_id": "33".repeat(32), "purpose_hash": "44".repeat(32),
                    "per_period_limit": "1000", "configured_period_limit": "900",
                    "carry_cap": "100", "spent_this_period": "350", "carried": "100",
                    "period_length_ms": "60000", "period_start_ms": "1000",
                    "expiry_ms": "90000", "revocation_counter": "7", "rollover_policy": "2",
                    "closed": false, "revoked": false,
                    "delegates": ["66".repeat(32), "77".repeat(32)],
                    "source_account": "55".repeat(32), "head": "42", "activity_id": null,
                    "achieved_verification_level": "committed",
                },
                "balance": "650", "proven_head": "42", "activity_id": null,
            },
        },
    })
}

#[test]
fn budget_envelope_preserves_the_current_wire_name_and_refuses_the_old_one() -> Result<(), Failure>
{
    let request = BudgetTarget {
        tenant: TenantId::new("identity-ops").expect("recorded tenant"),
        agent_did: AgentDid::new("did:layerx:identity-ops").expect("recorded agent"),
        budget_id: BudgetId::new("09".repeat(32)).expect("recorded budget"),
    };
    let credential =
        EnvelopeCredential::new("identity-ops", [1; 32], [2; 32], 1).expect("session credential");
    let envelope = encode_envelope(
        Operation::BudgetState,
        RequestId(487),
        &json!({"tenant": request.tenant.as_str(), "agent_did": request.agent_did.as_str(),
            "budget_id": request.budget_id.as_str()}),
        Some(&credential),
        None,
    )
    .expect("SDK budget envelope");
    let encoded = serde_json::to_string(&envelope)?;
    assert!(!encoded.contains("achieved_rung"));
    assert_eq!(envelope["operation"], "budget.state");
    let recorded = recorded_budget_response();
    let encoded = serde_json::to_string(&recorded)?;
    assert!(encoded.contains("\"achieved_verification_level\":\"committed\""));
    assert!(!encoded.contains("achieved_rung"));
    let success = decode_response(200, &serde_json::from_str(&encoded)?)
        .expect("SDK success envelope")
        .expect("budget success");
    let decoded = decode_budget_state_response(&request, &success).expect("SDK protocol budget");
    let BudgetRecord::Protocol(view) = decoded.value.record.record else {
        panic!("protocol budget expected");
    };
    assert_eq!(view.achieved_rung, Rung::Committed);
    assert_eq!(view.budget_id, [9; 32]);
    assert_eq!(decoded.value.proven_head, 42);
    assert_eq!(decoded.value.balance.0, 650);
    for keep_current in [false, true] {
        let mut legacy = success.clone();
        let record = legacy.value["value"]["record"]
            .as_object_mut()
            .expect("budget object");
        record.insert("achieved_rung".to_owned(), json!("committed"));
        if !keep_current {
            record.remove("achieved_verification_level");
        }
        assert!(decode_budget_state_response(&request, &legacy).is_err());
    }
    Ok(())
}
