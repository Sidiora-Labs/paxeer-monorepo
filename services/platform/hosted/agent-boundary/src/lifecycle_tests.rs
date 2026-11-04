#[test]
fn retired_program_routes_answer_not_found() {
    use std::collections::BTreeMap;
    use std::sync::Arc;
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
        chain_receipts: super::chain_receipt::ChainReceipts::from_config(
            "http://127.0.0.1:1",
            &keys,
        )
        .expect("chain receipts"),
        network_name: "layerx-retired-routes-test".to_owned(),
    };
    let id = "01".repeat(32);
    let retired = [
        ("GET", "/v1/protocol/account-state/head".to_owned()),
        ("GET", format!("/v1/receipts/{id}/account-state")),
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
    assert!(
        !root.join("state").exists(),
        "retired routes wrote a journal"
    );
    std::fs::remove_dir_all(root).expect("remove test directory");
}
