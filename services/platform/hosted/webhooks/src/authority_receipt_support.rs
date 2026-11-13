//! Retained authority evidence traversing the production producer, HTTPS client
//! and strict decoder. The authority binary serializer export is an external wiring request.
use crate::encoding::{fixed_hex, hex_decode, hex_encode};
use crate::error::WebhookError;
use crate::hosted::repair_support::{must, tls_material, Peer};
use crate::http::Reply;
use crate::trusted::AuthorityReceiptVerifier;
use layerx_platform_authority::{
    authorized_batch_by_activity, parse_replica_evidence, BatchEvidence, BatchIdentityEvidence,
};
use serde_json::{json, Value};
use std::collections::VecDeque;
use std::sync::{Arc, Mutex};

pub fn authority_path() {
    let maintained: Value = must(serde_json::from_str(include_str!(
        "../../gateway/tests/fixtures/maintained-authority.json"
    )));
    let historical: Value = must(serde_json::from_str(include_str!(
        "../../authority/tests/fixtures/real-program-deploy-receipt.json"
    )));
    for maintained_case in [true, false] {
        authority_case(&maintained, &historical, maintained_case);
    }
}

fn authority_evidence(
    maintained: &Value,
    historical: &Value,
    maintained_case: bool,
) -> (
    Vec<u8>,
    BatchEvidence,
    layerx_platform_gateway::SequencerAuthorization,
) {
    if maintained_case {
        let receipt = must(hex_decode(
            maintained["receipt_hex"]
                .as_str()
                .unwrap_or_else(|| panic!("receipt")),
        ));
        let pin = must(layerx_platform_gateway::configured_sequencer(
            maintained["sequencer_id"]
                .as_str()
                .unwrap_or_else(|| panic!("sequencer")),
            maintained["sequencer_public_key"]
                .as_str()
                .unwrap_or_else(|| panic!("key")),
            maintained["first_batch"]
                .as_str()
                .unwrap_or_else(|| panic!("first")),
            maintained["last_batch"]
                .as_str()
                .unwrap_or_else(|| panic!("last")),
        ));
        // Exercise the retained producer identity field without changing signed evidence.
        let evidence = must(parse_replica_evidence(
            &must(serde_json::to_vec(&json!({
                "authority_replica_id":"01".repeat(32), "sequencer_public_key":maintained["sequencer_public_key"],
                "batch_evidence":maintained["authority"]["batch_evidence"],
            }))),
            [1; 32],
            pin.public_key(),
        ));
        (receipt, evidence, pin)
    } else {
        let field = |name: &str| {
            must(hex_decode(
                historical[name]
                    .as_str()
                    .unwrap_or_else(|| panic!("retained field")),
            ))
        };
        let receipt = field("receipt_hex");
        let header = field("header_hex");
        let pin = layerx_platform_gateway::SequencerAuthorization::new(
            must(field("sequencer_id_hex").try_into()),
            must(field("sequencer_public_key_hex").try_into()),
            1,
            u64::MAX,
        );
        assert_eq!(historical["proof_index"], 0);
        assert_eq!(historical["proof_count"], 1);
        assert_eq!(historical["proof_siblings"], json!([]));
        // Canonical proof of the retained single-leaf tree: version, index,
        // leaf count and zero siblings, without changing its signed receipt.
        let proof = vec![1, 0, 0, 0, 0, 0, 0, 0, 1, 0];
        let evidence = BatchEvidence {
            header,
            header_signature: must(field("header_signature_hex").try_into()),
            receipt_proof: proof,
            batch_identity: BatchIdentityEvidence::Historical,
        };
        (receipt, evidence, pin)
    }
}

fn authority_case(maintained: &Value, historical: &Value, maintained_case: bool) {
    let (receipt, evidence, pin) = authority_evidence(maintained, historical, maintained_case);
    let activity = must(layerx_proof::receipt::verify_sequencer_signature(
        &receipt,
        layerx_platform_gateway::ReceiptKeys::HistoricalKernel(pin.public_key()),
    ))
    .protocol()
    .unwrap_or_else(|| panic!("protocol"))
    .activity_id();
    let facts = must(authorized_batch_by_activity(
        activity, &receipt, &evidence, &pin,
    ));
    let verified_header = must(layerx_proof::inclusion::verify_header(
        &evidence.header,
        &evidence.header_signature,
        &pin,
    ));
    let network = verified_header.header().network_id();
    // Serialize the actual verified producer facts, retaining the full HTTP
    // contract (receipt and numeric network included), never a shape-only object.
    let mut response = json!({
        "activity_id":hex_encode(&facts.activity_id), "receipt":hex_encode(&receipt),
        "batch_id":hex_encode(&facts.batch_id), "asset":hex_encode(&facts.asset),
        "previous_state_root":hex_encode(&facts.previous_state_root), "resulting_state_root":hex_encode(&facts.resulting_state_root),
        "sequencer_public_key":hex_encode(&facts.sequencer_public_key),
        "network_id":"layerx-testnet", "protocol_network_id":network, "wire_version":"3",
    });
    if maintained_case {
        response["batch_evidence"] = maintained["authority"]["batch_evidence"].clone();
    }
    let mut null = response.clone();
    null["batch_evidence"] = Value::Null;
    let mut unknown = response.clone();
    unknown["unexpected"] = json!(true);
    let mut foreign = response.clone();
    foreign["protocol_network_id"] = json!(network + 1);
    let mut bad_receipt = response.clone();
    bad_receipt["receipt"] = json!("00");
    let bodies = Arc::new(Mutex::new(VecDeque::from([
        response.clone(),
        null,
        unknown,
        foreign,
        bad_receipt,
    ])));
    verify_case(
        &receipt,
        pin,
        network,
        activity,
        facts.global_sequence,
        &response,
        &bodies,
    );
}

fn verify_case(
    receipt: &[u8],
    pin: layerx_platform_gateway::SequencerAuthorization,
    network: u32,
    activity: [u8; 32],
    global_sequence: u64,
    response: &Value,
    bodies: &Arc<Mutex<VecDeque<Value>>>,
) {
    let tls = tls_material();
    let id = hex_encode(&activity);
    let receipt_path = format!("/internal/v1/receipts/{id}");
    let component_body = json!({"activity_id":id,"receipt":hex_encode(receipt)}).to_string();
    let component = Peer::http(&tls, move |request| {
        assert_eq!(request.path, receipt_path);
        assert_eq!(request.header("authorization"), Some("Bearer component"));
        Reply::json(200, component_body.clone())
    });
    let authority_path = format!("/internal/v1/activities/{id}/authority");
    let retained_bodies = Arc::clone(bodies);
    let authority = Peer::http(&tls, move |request| {
        assert_eq!(request.path, authority_path);
        assert_eq!(request.header("authorization"), Some("Bearer authority"));
        Reply::json(
            200,
            must(retained_bodies.lock())
                .pop_front()
                .unwrap_or_else(|| panic!("one contract response per verification"))
                .to_string(),
        )
    });
    let verifier = must(AuthorityReceiptVerifier::new(
        &component.url,
        &authority.url,
        "component".into(),
        "authority".into(),
        &tls.ca,
        tls.identity.clone(),
        pin,
        "layerx-testnet".into(),
        "3".into(),
        network,
    ));
    let verified_operation = must(verifier.verify(&id));
    assert_eq!(verified_operation.activity_id(), must(fixed_hex::<32>(&id)));
    assert_eq!(verified_operation.receipt(), receipt);
    let operation: Value = must(serde_json::from_slice(verified_operation.response()));
    assert_eq!(operation["global_sequence"], global_sequence);
    assert_eq!(operation["batch_id"], response["batch_id"]);
    for category in [
        "null evidence",
        "unknown field",
        "foreign protocol network",
        "receipt mismatch",
    ] {
        assert!(
            matches!(
                verifier.verify(&id),
                Err(WebhookError::VerificationRequired)
            ),
            "{category} must fail closed"
        );
    }
    assert!(must(bodies.lock()).is_empty());
}
