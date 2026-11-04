//! Task 22.33.427 / audit 22.30.146 repair record.
//!
//! Finding -> proving test:
//! unmounted-config-discovery -> unmounted_config_discovery
//! configuration-debug-secrets -> configuration_debug_secrets
//! wrong-default-origin -> wrong_default_origin, rest_and_root_node_origins_are_distinct
//! unbounded-http -> unbounded_http_deadlines, unbounded_http_body_deadline,
//!   unbounded_http_response_limit, unbounded_http_preserves_error_evidence
//! unmounted-v1-health -> unmounted_v1_health, current_health_producer_envelopes
//! deep-object-array-panic -> deep_object_arrays_nested_objects_and_null
//! success-decode-observability -> success_decode_observability,
//!   transaction_declared_statuses, transaction_success_models_and_error_evidence
//!
//! The real production ApiClient, reqwest transport and codecs run against local
//! HTTP peers. Health fixtures follow API.HealthController.health and
//! blocks_indexing_status; the committed /api/health transcript also covers the
//! older extra metadata.latest_block field. Recorded transaction bodies are
//! replayed unchanged. Error and timeout peers deliberately exercise bad input.
//! No restart finding applies: this client has no durable state.
//! Baseline verification was not run: the ordered task instructions require
//! implementation before qualification and prohibit verify_cmd before hf-done.
//! No pre-change failing test result is claimed. Verified revision, command,
//! exit and private log are supplied by the runner's hf-done ledger.
//!
//! Wiring request (domain 14, explorer): update services/explorer/services/libs/blockscout-client/swaggers/blockscout-api-v2.yaml and swaggers/blockscout-api-final.yaml to mark RPC discovery and v1 health unsupported on the local backend and require deployment REST/root node origins.
//! Wiring request (domain 14, explorer): update services/explorer/services/libs/blockscout-client/swaggers/overrides/002_health.yaml with the /api/health healthy/500 envelopes.
//! Wiring request (domain 14, explorer): update services/explorer/services/libs/blockscout-client/rust-template-handlebars/reqwest-trait/api.handlebars, api_mod.handlebars and configuration.handlebars to preserve bounded transport, explicit origins, redacted Debug, scalar-array encoding and strict typed success decoding on regeneration.

use blockscout_client::{
    apis::{configuration::*, health_api::HealthError, parse_deep_object, transactions_api::*},
    Api, ApiClient, Configuration, Error,
};
use serde_json::{json, Value};
use std::{collections::BTreeMap, time::Duration};
use tokio::{
    io::{AsyncReadExt, AsyncWriteExt},
    net::{TcpListener, TcpStream},
    task::JoinHandle,
};

struct Peer {
    origin: String,
    task: JoinHandle<String>,
}

impl Drop for Peer {
    fn drop(&mut self) {
        self.task.abort();
    }
}

impl Peer {
    fn api(&self) -> ApiClient {
        ApiClient::new(Configuration::builder().base_path(&self.origin).build())
    }

    async fn request(&mut self) -> String {
        tokio::time::timeout(Duration::from_secs(3), &mut self.task)
            .await
            .expect("HTTP peer did not complete")
            .expect("HTTP peer failed")
    }
}

async fn read_request(stream: &mut TcpStream) -> String {
    let mut request = Vec::new();
    let mut byte = [0];
    while !request.ends_with(b"\r\n\r\n") {
        assert!(request.len() < 16 * 1024);
        stream.read_exact(&mut byte).await.unwrap();
        request.push(byte[0]);
    }
    let headers = String::from_utf8(request.clone()).unwrap();
    let length = headers
        .lines()
        .find_map(|line| {
            let (name, value) = line.split_once(':')?;
            name.eq_ignore_ascii_case("content-length")
                .then(|| value.trim().parse::<usize>().unwrap())
        })
        .unwrap_or(0);
    let mut body = vec![0; length];
    stream.read_exact(&mut body).await.unwrap();
    request.extend(body);
    String::from_utf8(request).unwrap()
}

async fn peer(expected: &str, status: u16, body: &str) -> Peer {
    let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
    let origin = format!("http://{}", listener.local_addr().unwrap());
    let expected = expected.to_owned();
    let body = body.to_owned();
    let task = tokio::spawn(async move {
        let (mut stream, _) = listener.accept().await.unwrap();
        let request = read_request(&mut stream).await;
        assert_eq!(request.lines().next().unwrap(), expected);
        let response = format!("HTTP/1.1 {status} Fixture\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len());
        stream.write_all(response.as_bytes()).await.unwrap();
        request
    });
    Peer { origin, task }
}

fn health_body(healthy: bool) -> String {
    // HealthController.blocks_indexing_status and health(false), including the
    // node/cache/db split. The unhealthy branch is the producer's 5002 case.
    let error = if healthy {
        Value::Null
    } else {
        json!({"code": 5002, "message": "There are no blocks in the DB."})
    };
    let mut blocks = json!({"healthy": healthy, "latest_block": {
        "db": {"number": "100", "timestamp": "2025-08-11 09:32:59Z"},
        "cache": {"number": "102", "timestamp": "2025-08-11 09:32:59Z"},
        "node": {"number": "110"}
    }});
    if !healthy {
        blocks["error"] = error.clone();
    }
    json!({"healthy": healthy, "error": error, "metadata": {"blocks": blocks}}).to_string()
}

fn recorded_body(transcript: &str) -> String {
    let transcript: Value = serde_json::from_str(transcript).unwrap();
    assert_eq!(transcript["response"]["status"], 200);
    transcript["response"]["body"].as_str().unwrap().to_owned()
}

#[tokio::test]
async fn unmounted_config_discovery() {
    let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
    let api = ApiClient::new(
        Configuration::builder()
            .base_path(format!("http://{}", listener.local_addr().unwrap()))
            .build(),
    );
    assert!(matches!(api.config_api().get_json_rpc_url().await,
        Err(Error::UnsupportedCapability(message)) if message.contains("node_rpc_origin")));
    assert!(
        tokio::time::timeout(Duration::from_millis(30), listener.accept())
            .await
            .is_err()
    );
}

#[test]
fn configuration_debug_secrets() {
    let secrets = [
        "basic-user-secret",
        "basic-pass-secret",
        "oauth-secret",
        "bearer-secret",
        "apikey-secret",
        "prefix-secret",
        "agent-secret",
        "node-secret",
        "url-secret",
    ];
    let config = Configuration::builder()
        .base_path("https://url-secret:password@explorer.example/?token=url-secret")
        .node_rpc_origin("https://node-secret:password@node.example/")
        .user_agent(secrets[6])
        .basic_auth((secrets[0].into(), Some(secrets[1].into())))
        .oauth_access_token(secrets[2])
        .bearer_access_token(secrets[3])
        .api_key(ApiKey {
            prefix: Some(secrets[5].into()),
            key: secrets[4].into(),
        })
        .build();
    for debug in [
        format!("{config:?}"),
        format!("{config:#?}"),
        format!("{:?}", config.api_key.as_ref().unwrap()),
        format!("{:#?}", config.api_key.as_ref().unwrap()),
    ] {
        assert!(debug.contains("REDACTED"));
        for secret in secrets {
            assert!(!debug.contains(secret), "credential leaked: {secret}");
        }
        assert!(!debug.contains("password"));
    }
    assert!(format!("{config:?}").contains("explorer.example"));
}

#[tokio::test]
async fn wrong_default_origin() {
    for config in [Configuration::default(), Configuration::new()] {
        assert!(config.base_path.is_empty());
        let api = ApiClient::new(config);
        assert!(matches!(
            api.health_api().health().await,
            Err(Error::Configuration(_))
        ));
        assert!(matches!(
            api.transactions_api()
                .get_txs(GetTxsParams::builder().build())
                .await,
            Err(Error::Configuration(_))
        ));
    }
    for origin in [
        "https://example.com/rpc",
        "ftp://example.com",
        "https://user:password@example.com",
        "https://example.com?token=secret",
        "https://example.com/#fragment",
    ] {
        let api = ApiClient::new(Configuration::builder().base_path(origin).build());
        assert!(matches!(
            api.health_api().health().await,
            Err(Error::Configuration(_))
        ));
    }
    let mut server = peer("GET /api/health HTTP/1.1", 200, &health_body(true)).await;
    let config = Configuration::builder()
        .base_path(format!("{}/", server.origin))
        .build();
    assert_eq!(config.request_timeout, DEFAULT_REQUEST_TIMEOUT);
    assert_eq!(config.max_response_bytes, DEFAULT_MAX_RESPONSE_BYTES);
    assert_eq!(DEFAULT_CONNECT_TIMEOUT, Duration::from_secs(5));
    assert!(ApiClient::new(config)
        .health_api()
        .health()
        .await
        .unwrap()
        .try_as_success()
        .unwrap()
        .healthy
        .unwrap());
    server.request().await;
}

#[tokio::test]
async fn rest_and_root_node_origins_are_distinct() {
    let mut rest = peer("GET /api/health HTTP/1.1", 200, &health_body(true)).await;
    let mut node = peer(
        "POST / HTTP/1.1",
        200,
        r#"{"jsonrpc":"2.0","id":1,"result":"0x7d"}"#,
    )
    .await;
    let config = Configuration::builder()
        .base_path(&rest.origin)
        .node_rpc_origin(&node.origin)
        .build();
    assert_ne!(config.base_path, config.node_origin().unwrap());
    let rpc: Value = config
        .client
        .post(config.node_origin().unwrap())
        .timeout(config.request_timeout)
        .json(&json!({"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}))
        .send()
        .await
        .unwrap()
        .json()
        .await
        .unwrap();
    assert_eq!(rpc["result"], "0x7d");
    assert_eq!(
        serde_json::from_str::<Value>(node.request().await.split("\r\n\r\n").nth(1).unwrap())
            .unwrap()["method"],
        "eth_chainId"
    );
    let api = ApiClient::new(config);
    assert!(api
        .health_api()
        .health()
        .await
        .unwrap()
        .try_as_success()
        .is_some());
    rest.request().await;
    assert!(Configuration::default().node_origin().is_err());
    let config = Configuration::builder()
        .base_path(&rest.origin)
        .node_rpc_origin(format!("{}/rpc", node.origin))
        .build();
    assert!(config.node_origin().is_err());
}

#[tokio::test]
async fn unmounted_v1_health() {
    let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
    let api = ApiClient::new(
        Configuration::builder()
            .base_path(format!("http://{}", listener.local_addr().unwrap()))
            .build(),
    );
    assert!(
        matches!(api.health_api().health_v1().await, Err(Error::UnsupportedCapability(message)) if message.contains("/api/health"))
    );
    assert!(
        tokio::time::timeout(Duration::from_millis(30), listener.accept())
            .await
            .is_err()
    );
}

#[tokio::test]
async fn current_health_producer_envelopes() {
    for body in [
        health_body(true),
        recorded_body(include_str!("recorded/eth_blockscout_com/_api_health.json")),
        r#"{"healthy":true,"data":{}}"#.into(),
    ] {
        let mut server = peer("GET /api/health HTTP/1.1", 200, &body).await;
        let response = server.api().health_api().health().await.unwrap();
        assert_eq!(response.status, 200);
        assert_eq!(response.content, body);
        let model = response.try_as_success().unwrap();
        assert_eq!(model.healthy, Some(true));
        if let Some(metadata) = &model.metadata {
            assert!(metadata.blocks.as_ref().unwrap().healthy);
        }
        server.request().await;
    }
    let body = health_body(false);
    let mut server = peer("GET /api/health HTTP/1.1", 500, &body).await;
    match server.api().health_api().health().await.unwrap_err() {
        Error::ResponseError(response) => {
            assert_eq!(response.status, 500);
            assert_eq!(response.content, body);
            match response.entity.unwrap() {
                HealthError::Status500(health) => {
                    assert_eq!(health.healthy, Some(false));
                    assert_eq!(health.error.unwrap().code, 5002);
                    let blocks = health.metadata.unwrap().blocks.unwrap();
                    assert!(!blocks.healthy);
                    assert_eq!(blocks.latest_block.node.unwrap().number, "110");
                    assert_eq!(blocks.error.unwrap().code, 5002);
                }
                other => panic!("lost typed unhealthy response: {other:?}"),
            }
        }
        other => panic!("lost health error evidence: {other:?}"),
    }
    server.request().await;
}

async fn stalled_peer(after_headers: bool) -> Peer {
    let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
    let origin = format!("http://{}", listener.local_addr().unwrap());
    let task = tokio::spawn(async move {
        let (mut stream, _) = listener.accept().await.unwrap();
        let request = read_request(&mut stream).await;
        assert_eq!(request.lines().next().unwrap(), "GET /api/health HTTP/1.1");
        if after_headers {
            stream
                .write_all(b"HTTP/1.1 200 OK\r\nContent-Length: 1000\r\n\r\n{")
                .await
                .unwrap();
        }
        // Wait for the real client to close the timed-out connection.
        let mut byte = [0];
        assert_eq!(stream.read(&mut byte).await.unwrap(), 0);
        request
    });
    Peer { origin, task }
}

async fn deadline(after_headers: bool) {
    let mut server = stalled_peer(after_headers).await;
    // The custom client has no default timeout: the API must enforce its own.
    let client = reqwest_middleware::ClientBuilder::new(reqwest::Client::new()).build();
    let api = ApiClient::new(
        Configuration::builder()
            .base_path(&server.origin)
            .client(client)
            .request_timeout(Duration::from_millis(80))
            .build(),
    );
    let error = tokio::time::timeout(Duration::from_secs(2), api.health_api().health())
        .await
        .expect("API exceeded configured deadline")
        .unwrap_err();
    match error {
        Error::Reqwest(error) => assert!(error.is_timeout()),
        Error::ReqwestMiddleware(reqwest_middleware::Error::Reqwest(error)) => {
            assert!(error.is_timeout())
        }
        other => panic!("expected timeout: {other:?}"),
    }
    server.request().await;
}

#[tokio::test]
async fn unbounded_http_deadlines() {
    deadline(false).await;
}

#[tokio::test]
async fn unbounded_http_body_deadline() {
    deadline(true).await;
}

#[tokio::test]
async fn unbounded_http_response_limit() {
    for status in [200, 500] {
        let body = "oversized-evidence".repeat(1024);
        let mut server = peer("GET /api/health HTTP/1.1", status, &body).await;
        let api = ApiClient::new(
            Configuration::builder()
                .base_path(&server.origin)
                .max_response_bytes(19)
                .build(),
        );
        match api.health_api().health().await.unwrap_err() {
            Error::ResponseTooLarge {
                status: actual,
                content,
                limit,
            } => {
                assert_eq!(actual, status);
                assert_eq!(limit, 19);
                assert_eq!(content, body.as_bytes()[..19]);
            }
            other => panic!("body limit not applied: {other:?}"),
        }
        server.request().await;
    }
    let body = health_body(true);
    let mut server = peer("GET /api/health HTTP/1.1", 200, &body).await;
    let api = ApiClient::new(
        Configuration::builder()
            .base_path(&server.origin)
            .max_response_bytes(body.len())
            .build(),
    );
    assert_eq!(api.health_api().health().await.unwrap().content, body);
    server.request().await;
}

#[tokio::test]
async fn unbounded_http_chunked_response_limit() {
    let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
    let origin = format!("http://{}", listener.local_addr().unwrap());
    let task = tokio::spawn(async move {
        let (mut stream, _) = listener.accept().await.unwrap();
        let request = read_request(&mut stream).await;
        assert_eq!(
            request.lines().next().unwrap(),
            "GET /api/v2/stats HTTP/1.1"
        );
        stream.write_all(b"HTTP/1.1 500 Error\r\nTransfer-Encoding: chunked\r\nConnection: close\r\n\r\n4\r\nABCD\r\n4\r\nEFGH\r\n4\r\nIJKL\r\n0\r\n\r\n").await.unwrap();
        request
    });
    let mut server = Peer { origin, task };
    let api = ApiClient::new(
        Configuration::builder()
            .base_path(&server.origin)
            .max_response_bytes(7)
            .build(),
    );
    match api.stats_api().get_stats().await.unwrap_err() {
        Error::ResponseTooLarge {
            status,
            content,
            limit,
        } => {
            assert_eq!(status, 500);
            assert_eq!(limit, 7);
            assert_eq!(content, b"ABCDEFG");
        }
        other => panic!("chunked stats response unbounded: {other:?}"),
    }
    server.request().await;
}

#[tokio::test]
async fn unbounded_http_preserves_error_evidence() {
    let body = r#"{"error":"upstream unavailable"}"#;
    let mut server = peer("GET /api/v2/stats HTTP/1.1", 503, body).await;
    let api = ApiClient::new(
        Configuration::builder()
            .base_path(&server.origin)
            .max_response_bytes(body.len())
            .build(),
    );
    match api.stats_api().get_stats().await.unwrap_err() {
        Error::ResponseError(response) => {
            assert_eq!(response.status, 503);
            assert_eq!(response.content, body);
            assert!(response.entity.is_some());
        }
        other => panic!("error evidence lost: {other:?}"),
    }
    server.request().await;
}

#[tokio::test]
async fn deep_object_arrays_nested_objects_and_null() {
    let params = parse_deep_object(
        "filter",
        &json!({
            "names": ["a & b", "", null], "nested": {"values": [1, true, {"key": "value"}, [false]]}, "null": null
        }),
    );
    let expected: BTreeMap<String, String> = [
        ("filter[names][0]", "a & b"),
        ("filter[names][1]", ""),
        ("filter[names][2]", "null"),
        ("filter[nested][values][0]", "1"),
        ("filter[nested][values][1]", "true"),
        ("filter[nested][values][2][key]", "value"),
        ("filter[nested][values][3][0]", "false"),
        ("filter[null]", "null"),
    ]
    .into_iter()
    .map(|(key, value)| (key.into(), value.into()))
    .collect();
    assert_eq!(params.len(), expected.len());
    assert_eq!(params.iter().cloned().collect::<BTreeMap<_, _>>(), expected);
    for value in [
        json!(null),
        json!("scalar"),
        json!(17),
        json!(false),
        json!([1, null]),
    ] {
        assert!(!parse_deep_object("root", &value).is_empty());
    }
    let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
    let origin = format!("http://{}", listener.local_addr().unwrap());
    let task = tokio::spawn(async move {
        let (mut stream, _) = listener.accept().await.unwrap();
        let request = read_request(&mut stream).await;
        let target = request.split_whitespace().nth(1).unwrap();
        let url = url::Url::parse(&format!("http://localhost{target}")).unwrap();
        assert_eq!(
            url.query_pairs().into_owned().collect::<BTreeMap<_, _>>(),
            expected
        );
        stream
            .write_all(b"HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\n{}")
            .await
            .unwrap();
        request
    });
    let mut server = Peer { origin, task };
    assert_eq!(
        reqwest::Client::new()
            .get(&server.origin)
            .query(&params)
            .send()
            .await
            .unwrap()
            .status(),
        200
    );
    server.request().await;
}

#[tokio::test]
async fn success_decode_observability() {
    for body in [
        "{malformed",
        "{}",
        r#"{"items":{},"next_page_params":null}"#,
        r#"{"items":[{}],"next_page_params":null}"#,
    ] {
        let mut server = peer("GET /api/v2/transactions HTTP/1.1", 200, body).await;
        let error = server
            .api()
            .transactions_api()
            .get_txs(GetTxsParams::builder().build())
            .await
            .unwrap_err();
        // Human-facing diagnostics must not dump response values.
        assert!(!error.to_string().contains(body));
        assert!(std::error::Error::source(&error).is_some());
        match error {
            Error::Decode {
                route,
                status,
                content,
                ..
            } => {
                assert_eq!(route, "/api/v2/transactions");
                assert_eq!(status, 200);
                assert_eq!(content, body);
            }
            other => panic!("invalid success accepted or evidence lost: {other:?}"),
        }
        server.request().await;
    }
}

#[tokio::test]
async fn transaction_declared_statuses() {
    let body = r#"{"items":[],"next_page_params":null}"#;
    for status in [201, 202, 204, 206, 302, 304, 400, 500] {
        let mut server = peer("GET /api/v2/transactions HTTP/1.1", status, body).await;
        match server
            .api()
            .transactions_api()
            .get_txs(GetTxsParams::builder().build())
            .await
            .unwrap_err()
        {
            Error::ResponseError(response) => {
                assert_eq!(response.status, status);
                // HTTP forbids bodies on 204/304; retain the actual received body.
                if ![204, 304].contains(&status) {
                    assert_eq!(response.content, body);
                }
            }
            other => panic!("undeclared status accepted: {status}: {other:?}"),
        }
        server.request().await;
    }
}

#[tokio::test]
async fn transaction_success_models_and_error_evidence() {
    let body = recorded_body(include_str!(
        "recorded/eth_blockscout_com/_api_v2_transactions.json"
    ));
    let mut server = peer("GET /api/v2/transactions HTTP/1.1", 200, &body).await;
    let response = server
        .api()
        .transactions_api()
        .get_txs(GetTxsParams::builder().build())
        .await
        .unwrap();
    assert_eq!(response.content, body);
    assert!(!response.try_as_success().unwrap().items.is_empty());
    assert_eq!(
        response.try_parse_as_success_model().unwrap().items.len(),
        response.try_as_success().unwrap().items.len()
    );
    server.request().await;
    for body in [r#"{"message":"not found"}"#, "upstream error (not JSON)"] {
        let mut server = peer("GET /api/v2/transactions HTTP/1.1", 502, body).await;
        match server
            .api()
            .transactions_api()
            .get_txs(GetTxsParams::builder().build())
            .await
            .unwrap_err()
        {
            Error::ResponseError(response) => {
                assert_eq!(response.status, 502);
                assert_eq!(response.content, body);
                assert_eq!(response.entity.is_some(), body.starts_with('{'));
            }
            other => panic!("error evidence lost: {other:?}"),
        }
        server.request().await;
    }
}

#[tokio::test]
async fn every_transaction_operation_rejects_incompatible_success() {
    macro_rules! rejects {
        ($route:expr, $method:ident, $params:expr) => {{
            for body in ["not-json", "{}"] {
                let mut server = peer(concat!("GET ", $route, " HTTP/1.1"), 200, body).await;
                let result = server.api().transactions_api().$method($params).await;
                assert!(matches!(result, Err(Error::Decode { status, content, .. }) if status == 200 && content == body));
                server.request().await;
            }
        }};
    }
    rejects!(
        "/api/v2/transactions/0x123/internal-transactions",
        get_transaction_internal_txs,
        GetTransactionInternalTxsParams::builder()
            .transaction_hash("0x123".into())
            .build()
    );
    rejects!(
        "/api/v2/transactions/0x123/logs",
        get_transaction_logs,
        GetTransactionLogsParams::builder()
            .transaction_hash("0x123".into())
            .build()
    );
    rejects!(
        "/api/v2/transactions/0x123/raw-trace",
        get_transaction_raw_trace,
        GetTransactionRawTraceParams::builder()
            .transaction_hash("0x123".into())
            .build()
    );
    rejects!(
        "/api/v2/transactions/0x123/state-changes",
        get_transaction_state_changes,
        GetTransactionStateChangesParams::builder()
            .transaction_hash("0x123".into())
            .build()
    );
    rejects!(
        "/api/v2/transactions/0x123/summary",
        get_transaction_summary,
        GetTransactionSummaryParams::builder()
            .transaction_hash("0x123".into())
            .build()
    );
    rejects!(
        "/api/v2/transactions/0x123/token-transfers",
        get_transaction_token_transfers,
        GetTransactionTokenTransfersParams::builder()
            .transaction_hash("0x123".into())
            .build()
    );
    rejects!(
        "/api/v2/transactions/0x123",
        get_tx,
        GetTxParams::builder()
            .transaction_hash("0x123".into())
            .build()
    );
}

#[tokio::test]
async fn every_transaction_operation_accepts_declared_success() {
    macro_rules! accepts {
        ($route:expr, $method:ident, $params:expr, $body:expr) => {{
            let body = $body;
            let mut server = peer(concat!("GET ", $route, " HTTP/1.1"), 200, &body).await;
            let response = server
                .api()
                .transactions_api()
                .$method($params)
                .await
                .unwrap();
            assert_eq!(response.status, 200);
            assert_eq!(response.content, body);
            assert!(response.try_as_success().is_some());
            assert!(response.try_parse_as_success_model().is_ok());
            server.request().await;
        }};
    }
    accepts!("/api/v2/transactions/0x123/internal-transactions", get_transaction_internal_txs, GetTransactionInternalTxsParams::builder().transaction_hash("0x123".into()).build(), recorded_body(include_str!("recorded/eth_blockscout_com/_api_v2_transactions_0x4dd7e3f4_internal-transactions.json")));
    accepts!(
        "/api/v2/transactions/0x123/logs",
        get_transaction_logs,
        GetTransactionLogsParams::builder()
            .transaction_hash("0x123".into())
            .build(),
        r#"{"items":[],"next_page_params":null}"#
    );
    accepts!(
        "/api/v2/transactions/0x123/raw-trace",
        get_transaction_raw_trace,
        GetTransactionRawTraceParams::builder()
            .transaction_hash("0x123".into())
            .build(),
        "[]"
    );
    accepts!(
        "/api/v2/transactions/0x123/state-changes",
        get_transaction_state_changes,
        GetTransactionStateChangesParams::builder()
            .transaction_hash("0x123".into())
            .build(),
        r#"{"items":[],"next_page_params":null}"#
    );
    accepts!(
        "/api/v2/transactions/0x123/summary",
        get_transaction_summary,
        GetTransactionSummaryParams::builder()
            .transaction_hash("0x123".into())
            .build(),
        r#"{"success":true,"data":{"summaries":[]}}"#
    );
    accepts!(
        "/api/v2/transactions/0x123/token-transfers",
        get_transaction_token_transfers,
        GetTransactionTokenTransfersParams::builder()
            .transaction_hash("0x123".into())
            .build(),
        r#"{"items":[],"next_page_params":null}"#
    );
    accepts!(
        "/api/v2/transactions/0x123",
        get_tx,
        GetTxParams::builder()
            .transaction_hash("0x123".into())
            .build(),
        recorded_body(include_str!(
            "recorded/eth_blockscout_com/_api_v2_transactions_0x4dd7e3f4.json"
        ))
    );
}

#[tokio::test]
async fn transport_bounds_cannot_be_disabled_with_zero() {
    for config in [
        Configuration::builder()
            .base_path("http://127.0.0.1:1")
            .max_response_bytes(0)
            .build(),
        Configuration::builder()
            .base_path("http://127.0.0.1:1")
            .request_timeout(Duration::ZERO)
            .build(),
    ] {
        assert!(matches!(
            ApiClient::new(config).health_api().health().await,
            Err(Error::Configuration(_))
        ));
    }
}
