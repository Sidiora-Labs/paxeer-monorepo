include!("support/required.rs");

use layerx_network_gateway::{decode_network, ConsensusHead, Evidence, GatewayError};
use serde_json::{json, Value};

fn recorded_network_answer() -> Value {
    serde_json::from_str(
        r#"{
            "network_id": "paxeer-degraded",
            "paxeer": {"chain_id": "0x7d", "latest_block": "0x10"},
            "consensus": {
                "latest_height": "34742403",
                "latest_block_hash": "A1A1A1A1A1A1A1A1A1A1A1A1A1A1A1A1A1A1A1A1A1A1A1A1A1A1A1A1A1A1A1A1",
                "latest_app_hash": "B2B2B2B2B2B2B2B2B2B2B2B2B2B2B2B2B2B2B2B2B2B2B2B2B2B2B2B2B2B2B2B2",
                "catching_up": false
            }
        }"#,
    )
    .required_with("task 8.5.3 network result")
}

#[test]
fn network_decodes_consensus_head() {
    let answer = recorded_network_answer();
    let head = decode_network(&answer).required_with("task 8.5.3 consensus head decodes");
    assert_eq!(head.network_id, "paxeer-degraded");
    assert_eq!(head.chain_id, 125);
    assert_eq!(head.latest_block, 16);
    assert_eq!(head.evidence, Evidence::GatewayReported);
    assert_eq!(
        head.consensus,
        Some(ConsensusHead {
            latest_height: 34_742_403,
            latest_block_hash: [0xa1; 32],
            latest_app_hash: [0xb2; 32],
            catching_up: false,
        })
    );

    for (height, catching_up) in [
        (900, false),
        (9_007_199_254_740_993, true),
        (u64::MAX, true),
    ] {
        let mut answer = answer.clone();
        answer["paxeer"]["latest_block"] = json!(height);
        answer["consensus"]["latest_height"] = json!(height.to_string());
        answer["consensus"]["catching_up"] = json!(catching_up);
        let head = decode_network(&answer).required_with("exact heights and sync state decode");
        assert_eq!(head.chain_id, 125);
        assert_eq!(head.latest_block, height);
        assert_eq!(
            head.consensus,
            Some(ConsensusHead {
                latest_height: height,
                latest_block_hash: [0xa1; 32],
                latest_app_hash: [0xb2; 32],
                catching_up,
            })
        );
    }

    for (field, invalid) in [
        ("latest_height", Value::Null),
        ("latest_height", json!(-1)),
        ("latest_height", json!("18446744073709551616")),
        ("latest_block_hash", json!("not-a-hash")),
        ("latest_block_hash", json!("A1".repeat(31))),
        ("latest_app_hash", Value::Null),
        ("latest_app_hash", json!("B2".repeat(33))),
        ("catching_up", json!("false")),
    ] {
        let mut invalid_answer = answer.clone();
        invalid_answer["consensus"][field] = invalid;
        assert_eq!(
            decode_network(&invalid_answer),
            Err(GatewayError::MalformedAnswer),
            "malformed {field} must be refused"
        );
    }
    for field in [
        "latest_height",
        "latest_block_hash",
        "latest_app_hash",
        "catching_up",
    ] {
        let mut incomplete = answer.clone();
        incomplete["consensus"]
            .as_object_mut()
            .required()
            .remove(field);
        assert_eq!(
            decode_network(&incomplete),
            Err(GatewayError::MalformedAnswer)
        );
    }
}

#[test]
fn network_answers_without_consensus() {
    let answer: Value = serde_json::from_str(
        r#"{
            "network_id": "paxeer-degraded",
            "paxeer": {"chain_id": "0x7d", "latest_block": "0x10"},
            "consensus": null,
            "reason": "consensus_unavailable"
        }"#,
    )
    .required_with("task 8.5.3 unavailable consensus result");
    let head = decode_network(&answer).required_with("Paxeer still answers without consensus");
    assert_eq!(head.network_id, "paxeer-degraded");
    assert_eq!(head.chain_id, 125);
    assert_eq!(head.latest_block, 16);
    assert_eq!(head.consensus, None);
    assert_eq!(head.evidence, Evidence::GatewayReported);

    for invalid in [json!(false), json!([]), json!("unavailable"), json!({})] {
        let mut invalid_answer = answer.clone();
        invalid_answer["consensus"] = invalid;
        assert_eq!(
            decode_network(&invalid_answer),
            Err(GatewayError::MalformedAnswer)
        );
    }
    let mut incomplete = answer.clone();
    incomplete.as_object_mut().required().remove("consensus");
    assert_eq!(
        decode_network(&incomplete),
        Err(GatewayError::MalformedAnswer)
    );
    incomplete = answer.clone();
    incomplete.as_object_mut().required().remove("reason");
    assert_eq!(
        decode_network(&incomplete),
        Err(GatewayError::MalformedAnswer)
    );
    for field in ["chain_id", "latest_block"] {
        let mut invalid_answer = answer.clone();
        invalid_answer["paxeer"][field] = Value::Null;
        assert_eq!(
            decode_network(&invalid_answer),
            Err(GatewayError::MalformedAnswer)
        );
    }
}
