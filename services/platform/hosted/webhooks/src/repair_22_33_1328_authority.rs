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
        let (receipt, evidence, pin) = if maintained_case {
            let receipt = must(hex_decode(
                maintained["receipt_hex"].as_str().expect("receipt"),
            ));
            let pin = must(layerx_platform_gateway::configured_sequencer(
                maintained["sequencer_id"].as_str().expect("sequencer"),
                maintained["sequencer_public_key"].as_str().expect("key"),
                maintained["first_batch"].as_str().expect("first"),
                maintained["last_batch"].as_str().expect("last"),
            ));
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
                    historical[name].as_str().expect("retained field"),
                ))
            };
            let receipt = field("receipt_hex");
            let header = field("header_hex");
            let decoded = must(layerx_wire::receipt::decode_batch_header(&header));
            let pin = layerx_platform_gateway::SequencerAuthorization::new(
                decoded.sequencer_id(),
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
        };
        let activity = must(layerx_wire::receipt::decode(&receipt))
            .protocol()
            .expect("protocol")
            .activity_id();
        let facts = must(authorized_batch_by_activity(
            activity, &receipt, &evidence, &pin,
        ));
        let network =
            must(layerx_wire::receipt::decode_batch_header(&evidence.header)).network_id();
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
        let tls = tls_material();
        let id = hex_encode(&activity);
        let receipt_path = format!("/internal/v1/receipts/{id}");
        let component_body = json!({"activity_id":id,"receipt":hex_encode(&receipt)}).to_string();
        let component = Peer::http(&tls, move |request| {
            assert_eq!(request.path, receipt_path);
            assert_eq!(request.header("authorization"), Some("Bearer component"));
            Reply::json(200, component_body.clone())
        });
        let authority_path = format!("/internal/v1/activities/{id}/authority");
        let retained_bodies = Arc::clone(&bodies);
        let authority = Peer::http(&tls, move |request| {
            assert_eq!(request.path, authority_path);
            assert_eq!(request.header("authorization"), Some("Bearer authority"));
            Reply::json(
                200,
                must(retained_bodies.lock())
                    .pop_front()
                    .expect("one contract response per verification")
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
        let verified = must(verifier.verify(&id));
        assert_eq!(verified.activity_id(), must(fixed_hex::<32>(&id)));
        assert_eq!(verified.receipt(), receipt);
        let operation: Value = must(serde_json::from_slice(verified.response()));
        assert_eq!(operation["global_sequence"], facts.global_sequence);
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
}
