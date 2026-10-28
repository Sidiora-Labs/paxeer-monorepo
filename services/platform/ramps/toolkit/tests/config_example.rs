use serde_json::Value;

type LayerxConfig = layerx_ramp_toolkit::clients::LayerxConfig;

#[test]
fn config_example_layerx_section_parses() {
    let example: Value = serde_json::from_str(include_str!("../../config.example.json"))
        .expect("ramp example must be valid JSON");
    let config = serde_json::from_value::<LayerxConfig>(
        example
            .get("layerx")
            .expect("LayerX section must exist")
            .clone(),
    )
    .expect("LayerX example must deserialize into the production LayerxConfig");

    assert_eq!(config.chain_id, 125);
    assert!(config.rpc_endpoint.starts_with("https://"));
    assert!(!config.rpc_trust_anchor_der.as_os_str().is_empty());
    assert!(!config.operator_address.is_empty());
    assert!(!config.receipt_keys.is_empty());
    for key in &config.receipt_keys {
        assert!(!key.key_id.is_empty());
        assert!(!key.public_key.is_empty());
        assert!(key.first_height <= key.last_height);
    }
    assert!(config.max_fee_per_gas >= config.max_priority_fee_per_gas);
    assert!(config.gas_limit_cap > 0);
    assert!(config.signer_endpoint.starts_with("https://"));
    assert!(!config.signer_token_file.as_os_str().is_empty());
    assert_eq!(config.actor_did, "did:layerx:market-maker-eu-1");
}

#[test]
fn config_example_has_no_paxeer_rpc_agreement() {
    let example: Value = serde_json::from_str(include_str!("../../config.example.json"))
        .expect("ramp example must be valid JSON");
    let paxeer = example
        .get("paxeer")
        .and_then(Value::as_object)
        .expect("Paxeer section must be an object");

    for retired in [
        "rpc_endpoints",
        "rpc_trust_anchor_der",
        "rpc_chain_id",
        "rpc_minimum_agreement",
        "required_confirmations",
        "delayed_after_polls",
    ] {
        assert!(
            !paxeer.contains_key(retired),
            "Paxeer example must omit retired field {retired}"
        );
    }
    assert_eq!(paxeer.get("poll_cadence_seconds"), Some(&Value::from(5)));
}
