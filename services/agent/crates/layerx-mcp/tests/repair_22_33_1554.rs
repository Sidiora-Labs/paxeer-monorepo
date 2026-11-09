//! Task 22.33.1554, finding `payment_binding_fixture_retired`:
//! `payment_binding_fixture_retired` proves the shared positive fixture and every receipt-key refusal.
//! Baseline: no prior gate ledger or repair target existed. The implementation-first,
//! wrapper-only order precludes the contradictory additional unchanged-tree verify run;
//! no baseline test failure is claimed. The audit records the original positive parse failure.
//! No out-of-path wiring is required. hf-`build/hf-done` retain the qualified revision and logs.

use layerx_mcp_test_support::option_assertion;
use layerx_mcp_test_support::result_assertion;

use layerx_mcp_test_support::payment_binding;

use layerx_agentd::enrolment::binding_shape::BindingDocument;
use layerx_mcp::binding::{Binding, BindingError};
use layerx_mcp::boundary::AgentSurface;
use serde_json::{json, Value};

fn valid_baseline() -> Value {
    let valid = payment_binding::document();
    let encoded =
        result_assertion::required(serde_json::to_vec(&valid), "encode producer document");
    let produced: BindingDocument =
        result_assertion::required(serde_json::from_slice(&encoded), "producer roundtrip");
    let binding = result_assertion::required(
        Binding::parse(result_assertion::required(
            std::str::from_utf8(&encoded),
            "JSON transport encoding",
        )),
        "current producer fixture parses before any negative assertion",
    );
    assert_eq!(produced.core_height, 7);
    assert_eq!(binding.core_height(), produced.core_height);
    assert!(valid.get("core_sequence").is_none());
    assert_eq!(binding.agent_endpoint(), "127.0.0.1:9500");
    assert_eq!(binding.agent_endpoint(), produced.agent.endpoint);
    result_assertion::required(
        AgentSurface::new(
            binding.agent_endpoint(),
            "b".repeat(48),
            &produced.agent.probe_program,
            binding.deadline(),
        ),
        "producer endpoint is accepted by the real daemon transport consumer",
    );
    valid
}

#[test]
fn payment_binding_fixture_retired() {
    let valid = valid_baseline();
    assert!(valid["web"].get("sequencer_public_key").is_none());
    let mut missing = valid.clone();
    option_assertion::required(missing["web"].as_object_mut(), "web block")
        .remove("receipt_key_index");
    let error = layerx_mcp_test_support::refusal_assertion::refused(
        Binding::parse(&missing.to_string()),
        "index is required",
    );
    assert!(error.detail().contains("receipt_key_index"));

    for keep_index in [false, true] {
        let mut retired = valid_baseline();
        if !keep_index {
            option_assertion::required(retired["web"].as_object_mut(), "web block")
                .remove("receipt_key_index");
        }
        retired["web"]["sequencer_public_key"] = json!("5e".repeat(32));
        let error = layerx_mcp_test_support::refusal_assertion::refused(
            Binding::parse(&retired.to_string()),
            "retired key is refused",
        );
        assert!(error.detail().contains("sequencer_public_key"));
    }

    for index in [
        json!([]),
        json!("5e".repeat(32)),
        json!([{"keyId": "00".repeat(32), "publicKey": "5e".repeat(32), "firstHeight": 1, "lastHeight": 100}]),
    ] {
        let mut invalid = valid_baseline();
        invalid["web"]["receipt_key_index"] = index;
        let error = layerx_mcp_test_support::refusal_assertion::refused(
            Binding::parse(&invalid.to_string()),
            "malformed key index refused",
        );
        assert!(error.detail().contains("receipt_key_index"));
    }

    let mut uncovered = valid_baseline();
    uncovered["web"]["receipt_key_index"][0]["lastHeight"] = json!(6);
    let error = layerx_mcp_test_support::refusal_assertion::refused(
        Binding::parse(&uncovered.to_string()),
        "key must cover configured heights",
    );
    assert!(error.detail().contains("receipt_key_index"));
    assert!(error.detail().contains("cover"));
}

#[test]
fn payment_binding_core_sequence_remains_retired() {
    for keep_height in [false, true] {
        let mut retired = valid_baseline();
        if !keep_height {
            option_assertion::required(retired.as_object_mut(), "binding object")
                .remove("core_height");
        }
        retired["core_sequence"] = json!(7);
        assert_eq!(
            layerx_mcp_test_support::refusal_assertion::refused(
                Binding::parse(&retired.to_string()),
                "retired height key is refused"
            ),
            BindingError::Refused("mcp.binding_retired_key".to_owned()),
        );
    }
}
