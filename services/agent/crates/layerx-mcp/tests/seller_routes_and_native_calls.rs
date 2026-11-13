use layerx_mcp_test_support::{native_alias, option_assertion, seller_exchange};
use serde_json::json;

#[test]
fn seller_routes_and_node_transport_complete_exact_and_metered() {
    seller_exchange::seller_routes_and_node_transport_complete_exact_and_metered();
}

#[test]
fn node_account_failures_never_reach_settlement() {
    seller_exchange::node_account_failures_never_reach_settlement();
}

#[test]
fn native_unsigned_call_prepare_disclose_sign_and_submit_binding() {
    native_alias::native_unsigned_call_prepare_disclose_sign_and_submit_binding();
}

#[test]
fn native_call_rejects_every_changed_response_and_intent_field() {
    native_alias::native_call_rejects_every_changed_response_and_intent_field();
}

#[test]
fn native_legacy_owner_contract_remains_refused() {
    native_alias::native_legacy_owner_contract_remains_refused();
}

#[test]
fn native_catalogue_advertises_v2_send_purpose_and_preserves_preparation_v1() {
    let schema = option_assertion::required(
        layerx_mcp::catalogue::input_schema("activity.prepare"),
        "prepare schema",
    );
    let variants = option_assertion::required(schema["oneOf"].as_array(), "prepare variants");
    let send = option_assertion::required(
        variants
            .iter()
            .find(|schema| schema["properties"]["variant"]["enum"] == json!(["native_send_v1"])),
        "native Send schema",
    );
    let signed = &send["properties"]["purpose"];
    assert_eq!(signed["required"], json!(["purpose", "signature"]));
    assert_eq!(signed["additionalProperties"], false);
    assert!(signed["properties"].get("owner_public_key").is_none());
    let purpose = &signed["properties"]["purpose"];
    let properties = &purpose["properties"];
    assert_eq!(
        properties["version"]["enum"],
        json!([layerx_agent_api::identity::NativeSendPurposeV2::VERSION.to_string()])
    );
    assert_eq!(properties["transaction_type"]["enum"], json!(["2"]));
    assert_eq!(properties["chain_id"]["enum"], json!(["125"]));
    assert_eq!(properties["evm_sender"]["minLength"], 40);
    assert_eq!(properties["evm_sender"]["maxLength"], 40);
    assert_eq!(properties["consent_public_key"]["minLength"], 64);
    for field in [
        "consent_public_key",
        "evm_sender",
        "transaction_type",
        "chain_id",
    ] {
        assert!(
            option_assertion::required(purpose["required"].as_array(), "required fields")
                .contains(&json!(field))
        );
    }
    for retired in ["owner_public_key", "protocol_version", "network_id"] {
        assert!(properties.get(retired).is_none());
    }
    assert_eq!(purpose["additionalProperties"], false);
    for variant in ["native_v1", "native_effect_v1"] {
        let legacy = option_assertion::required(
            variants
                .iter()
                .find(|schema| schema["properties"]["variant"]["enum"] == json!([variant])),
            "preparation V1 schema",
        );
        assert_eq!(
            legacy["properties"]["purpose"]["properties"]["purpose"]["properties"]["version"]
                ["enum"],
            json!(["1"])
        );
        assert_eq!(
            legacy["properties"]["purpose"]["required"],
            json!(["purpose", "owner_public_key", "signature"])
        );
    }
}
