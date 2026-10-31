use super::{artifacts, validate_program_lifecycle};

#[test]
fn retired_program_routes_answer_not_found() {
    use std::collections::BTreeMap;
    use std::sync::{Arc, Mutex};
    use std::time::Duration;
    use zeroize::Zeroizing;

    let mut nonce = [0; 16];
    getrandom::fill(&mut nonce).expect("test directory nonce");
    let root = std::env::temp_dir().join(format!("layerx-retired-routes-{}", super::hex(&nonce)));
    std::fs::create_dir(&root).expect("test directory");
    let keys = root.join("receipt-keys.json");
    std::fs::write(
        &keys,
        serde_json::json!([{
            "keyId": "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE=",
            "publicKey": "SAdaWX5yGhVuLgeZ3lzAxTJNxufq8c3UYlCGjsUyFd0=",
            "firstHeight": 1,
            "lastHeight": u64::MAX,
        }])
        .to_string(),
    )
    .expect("receipt keys");
    let tls = rustls::ServerConfig::builder_with_provider(Arc::new(
        rustls::crypto::ring::default_provider(),
    ))
    .with_safe_default_protocol_versions()
    .expect("TLS protocol versions")
    .with_no_client_auth()
    .with_cert_resolver(Arc::new(rustls::server::ResolvesServerCertUsingSni::new()));
    let config = super::Config {
        listen: "127.0.0.1:0".parse().expect("listen address"),
        tls: Arc::new(tls),
        gateway_token: Zeroizing::new("gateway-plane-token".to_owned()),
        registry_token: Zeroizing::new("registry-plane-token".to_owned()),
        webhook_token: Zeroizing::new("webhook-plane-token".to_owned()),
        lni_socket: root.join("lni.sock"),
        lni_deadline: Duration::from_secs(1),
        node: super::NodeEndpoint { port: 1 },
        chain_receipts: super::chain_receipt::ChainReceipts::from_config(
            "http://127.0.0.1:1",
            &keys,
        )
        .expect("chain receipts"),
        node_token: Zeroizing::new("node-token".to_owned()),
        state_dir: root.join("state"),
        protocol_network_id: 7_332,
        network_name: "layerx-retired-routes-test".to_owned(),
        registry: super::module_registry().expect("module registry"),
        receipt_wait: Duration::from_secs(1),
        gate: super::ConnectionGate::new(super::LNI_CONNECTIONS),
        sessions: super::SessionPool::new(),
        key_locks: Mutex::new(BTreeMap::new()),
    };
    let id = "01".repeat(32);
    let retired = [
        ("POST", "/v1/activities".to_owned()),
        ("POST", "/v1/programs/call".to_owned()),
        ("POST", "/v1/programs/read".to_owned()),
        ("POST", "/v1/programs/simulate".to_owned()),
        ("POST", "/v1/programs/deploy".to_owned()),
        ("POST", "/v1/programs/upgrade".to_owned()),
        ("POST", "/v1/programs/wind-down".to_owned()),
        ("GET", format!("/v1/programs/activities/{id}")),
        ("GET", "/v1/programs/activities/invalid".to_owned()),
        ("GET", format!("/v1/programs/receipts/by-idempotency/{id}")),
        ("GET", "/v1/programs/receipts/by-idempotency/key".to_owned()),
        ("GET", "/v1/programs/account-state/changes".to_owned()),
        (
            "GET",
            "/v1/programs/account-state/changes?after_sequence=0".to_owned(),
        ),
        ("GET", format!("/v1/programs/{id}/account-state?at=1")),
        (
            "GET",
            format!("/v1/programs/{id}/account-state?at=1&profile=2"),
        ),
        ("GET", format!("/v1/programs/{id}/state-proof?at=1")),
    ];
    for token in [
        config.gateway_token.as_str(),
        config.registry_token.as_str(),
        config.webhook_token.as_str(),
    ] {
        for (method, path) in &retired {
            let request = super::Request {
                method: (*method).to_owned(),
                path: path.clone(),
                headers: BTreeMap::from([
                    ("authorization".to_owned(), format!("Bearer {token}")),
                    (
                        "content-type".to_owned(),
                        "application/octet-stream".to_owned(),
                    ),
                    ("idempotency-key".to_owned(), "retired-route".to_owned()),
                ]),
                body: Vec::new(),
            };
            let response = super::route(&config, &request);
            assert_eq!(response.status, 404, "{method} {path}: {}", response.body);
            let body: serde_json::Value =
                serde_json::from_str(&response.body).expect("refusal JSON");
            assert_eq!(body["error"]["code"], "not_found", "{method} {path}");
            assert_eq!(response.retry_after, None, "{method} {path}");
        }
    }
    let request = super::Request {
        method: "GET".to_owned(),
        path: "/v1/receipts/invalid".to_owned(),
        headers: BTreeMap::from([(
            "authorization".to_owned(),
            format!("Bearer {}", config.gateway_token.as_str()),
        )]),
        body: Vec::new(),
    };
    let response = super::route(&config, &request);
    assert_eq!(response.status, 400, "receipt route: {}", response.body);
    let body: serde_json::Value =
        serde_json::from_str(&response.body).expect("receipt refusal JSON");
    assert_eq!(body["error"]["code"], "invalid_activity_id");
    assert!(super::relay_path_allowed(
        "/v1/protocol/account-state/head",
        None
    ));
    assert!(super::relay_path_allowed(
        &format!("/v1/receipts/{id}/account-state"),
        None,
    ));
    assert!(!config.state_dir.exists(), "retired routes wrote a journal");
    std::fs::remove_dir_all(root).expect("remove test directory");
}

fn fixture(name: &str) -> Vec<u8> {
    let path = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../../../sdk/platform/conformance/fixtures")
        .join(format!("native-program-{name}-v3.json"));
    let source = std::fs::read_to_string(&path)
        .unwrap_or_else(|error| panic!("{}: {error}", path.display()));
    let document: serde_json::Value =
        serde_json::from_str(&source).unwrap_or_else(|error| panic!("fixture JSON: {error}"));
    let payload = document["payload_hex"]
        .as_str()
        .unwrap_or_else(|| panic!("fixture omits payload_hex"));
    artifacts::canonical_hex(payload, super::MAX_ACTIVITY_BYTES)
        .unwrap_or_else(|error| panic!("fixture hexadecimal: {error}"))
}

#[test]
fn native_c_lifecycle_vectors_are_admitted() {
    for (name, ordinal) in [
        ("deploy", 1),
        ("upgrade", 2),
        ("wind-down-route", 7),
        ("wind-down-deprecate", 7),
        ("wind-down-tombstone", 7),
        ("wind-down-exit", 7),
    ] {
        let payload = fixture(name);
        assert!(
            validate_program_lifecycle(ordinal, &payload).is_ok(),
            "{name}"
        );
        let mut trailing = payload;
        trailing.push(0);
        assert!(
            validate_program_lifecycle(ordinal, &trailing).is_err(),
            "{name} trailing byte"
        );
    }
}

#[test]
fn lifecycle_code_hash_substitution_refuses_before_submit() {
    for (name, ordinal) in [("deploy", 1), ("upgrade", 2)] {
        let mut payload = fixture(name);
        payload[68] ^= 1;
        let refused = validate_program_lifecycle(ordinal, &payload)
            .err()
            .unwrap_or_else(|| panic!("{name} substituted hash was accepted"));
        assert_eq!(refused.status, 400);
        assert!(refused.body.contains("program_payload_hash_mismatch"));
        payload = fixture(name);
        payload[35] = 1;
        let refused = validate_program_lifecycle(ordinal, &payload)
            .err()
            .unwrap_or_else(|| panic!("{name} reserved field was accepted"));
        assert_eq!(refused.status, 400);
        assert!(refused.body.contains("malformed_program_"));
    }
}

#[test]
fn idempotency_receipts_require_signed_activity_and_sequencer_bindings() {
    let document: serde_json::Value = serde_json::from_str(include_str!(
        "../../../../../sdk/platform/conformance/fixtures/receipt-programs-positive-v3.json"
    ))
    .unwrap_or_else(|error| panic!("receipt fixture JSON: {error}"));
    let receipt = document["canonical_receipt_hex"]
        .as_str()
        .unwrap_or_else(|| panic!("fixture omits receipt"));
    let signer = document["authorized_batch"]["sequencer_public_key_hex"]
        .as_str()
        .and_then(super::parse_hex32)
        .unwrap_or_else(|| panic!("fixture omits sequencer key"));
    let bytes = artifacts::canonical_hex(receipt, super::MAX_ACTIVITY_BYTES)
        .unwrap_or_else(|error| panic!("receipt hexadecimal: {error}"));
    let decoded = layerx_wire::receipt::decode(&bytes)
        .unwrap_or_else(|error| panic!("receipt decoding: {error:?}"));
    let activity_id = super::hex(
        &decoded
            .protocol()
            .unwrap_or_else(|| panic!("fixture omits protocol receipt"))
            .activity_id(),
    );
    let response = serde_json::json!({"activity_id":activity_id,"receipt":receipt}).to_string();
    assert!(super::verify_idempotency_receipt(&response, signer, Some(&activity_id)).is_ok());
    assert!(super::verify_idempotency_receipt(&response, signer, Some(&"00".repeat(32))).is_err());
    let mut wrong_signer = signer;
    wrong_signer[0] ^= 1;
    assert!(
        super::verify_idempotency_receipt(&response, wrong_signer, Some(&activity_id)).is_err()
    );
    let mut corrupt = bytes;
    let last = corrupt
        .last_mut()
        .unwrap_or_else(|| panic!("empty receipt"));
    *last ^= 1;
    let response =
        serde_json::json!({"activity_id":activity_id,"receipt":super::hex(&corrupt)}).to_string();
    assert!(super::verify_idempotency_receipt(&response, signer, Some(&activity_id)).is_err());
}
