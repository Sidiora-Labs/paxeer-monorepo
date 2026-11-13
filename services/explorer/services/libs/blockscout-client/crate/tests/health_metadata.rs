//! Task 22.31.278 repair record; baseline 4f4ac8be692fba2dae558bc6fabf0ca9f9885fa5.
//! Finding health-metadata: health_metadata_recording_preserves_blocks and
//! health_metadata_survives_health_api_http prove closure only when both pass.
//! The recording's fixture-only metadata.latest_block sibling has been removed;
//! the retained blocks payload has not been regenerated from a running local controller.
//! Prior candidate 1695a10e1fca298cd65efc6927a4cca3ff1d181f failed both tests
//! in the supplied verification evidence (exit 101); this continuation does not
//! repeat that unchanged gate or claim an additional baseline run.
//! Continuation baseline: 2b502183a86af3a2498748de6572b1bde20e4768.
//! HealthCheckResponseMetadata preserves blocks and node/cache/db numbers.
//! Attempt 5 adds skip_serializing_if to its legacy latest_block: Option<LatestBlock>
//! so an absent sibling stays absent when serialized. Both proving tests retain
//! the assertion that this fixture-only sibling must be absent.
//! Wiring request, domain 14: align services/explorer/services/libs/blockscout-client/swaggers/overrides/002_health.yaml with metadata.blocks.healthy/latest_block (db/cache/node.number), correcting cache.mber, then regenerate crate/src/models without the legacy metadata.latest_block field.
//! Wiring request, domain 14: capture services/explorer/backend's HealthController /api/health response on a controlled backend and replace the recording; no backend or Elixir runtime is available here.
//! The Swagger override is outside the allowed task scope; the metadata model is in scope.
//! These wiring requests remain open; this record does not claim a fresh capture.
//! Attempt 4 baseline: 2e10cd3d8dd2a35c0a89321517a0d24b11d0f890, including
//! prerequisite 22.31.445 (f51be99a9382ff23d7dae38ccbe11028ec6bdef4).
//! Retain both proving tests and qualify this continuation through hf-build/hf-done.

use blockscout_client::{models::HealthCheckResponse, Api, ApiClient, Configuration};
use serde_json::Value;
use std::time::Duration;
use tokio::{
    io::{AsyncReadExt, AsyncWriteExt},
    net::TcpListener,
};

const HEALTH_RECORDING: &str = include_str!("recorded/eth_blockscout_com/_api_health.json");

fn assert_blocks_preserved(wire: &Value, typed: &HealthCheckResponse) {
    assert_eq!(wire["healthy"], true);
    let metadata = wire["metadata"].as_object().expect("wire metadata object");
    assert!(!metadata.contains_key("latest_block"));
    let blocks = metadata.get("blocks").expect("wire metadata.blocks");
    assert_eq!(blocks["healthy"], true);
    for source in ["db", "cache", "node"] {
        let number = blocks["latest_block"][source]["number"]
            .as_str()
            .expect("wire block number must be a string");
        assert!(!number.is_empty());
        assert!(number.chars().all(|c| c.is_ascii_digit()));
    }

    assert_eq!(typed.healthy, true);
    let decoded = serde_json::to_value(typed).expect("serialize generated health model");
    assert_eq!(
        decoded["metadata"]["blocks"], *blocks,
        "generated HealthCheckResponse must retain blocks health and every block source"
    );
    assert!(decoded["metadata"].get("latest_block").is_none());
}

#[test]
fn health_metadata_recording_preserves_blocks() {
    let recording: Value = serde_json::from_str(HEALTH_RECORDING).expect("health recording JSON");
    assert_eq!(recording["request"]["method"], "GET");
    assert_eq!(recording["request"]["urlPath"], "/api/health");
    assert_eq!(recording["response"]["status"], 200);
    let body = recording["response"]["body"]
        .as_str()
        .expect("recorded response body");
    let wire: Value = serde_json::from_str(body).expect("recorded health JSON");
    for source in ["db", "cache", "node"] {
        assert_eq!(
            wire["metadata"]["blocks"]["latest_block"][source]["number"],
            "23116935"
        );
    }
    let typed: HealthCheckResponse =
        serde_json::from_str(body).expect("decode using the real generated health model");
    assert_blocks_preserved(&wire, &typed);
}

#[tokio::test]
async fn health_metadata_survives_health_api_http() {
    let recording: Value = serde_json::from_str(HEALTH_RECORDING).expect("health recording JSON");
    let body = recording["response"]["body"]
        .as_str()
        .expect("recorded response body")
        .to_owned();
    let listener = TcpListener::bind("127.0.0.1:0")
        .await
        .expect("bind health transcript server");
    let origin = format!("http://{}", listener.local_addr().expect("server address"));
    let server = tokio::spawn(async move {
        tokio::time::timeout(Duration::from_secs(30), async move {
            let (mut stream, _) = listener.accept().await.expect("accept HealthApi request");
            let mut request = Vec::new();
            while !request.ends_with(b"\r\n\r\n") {
                assert!(request.len() < 8192, "oversized HTTP request headers");
                request.push(stream.read_u8().await.expect("read HTTP request"));
            }
            let request = std::str::from_utf8(&request).expect("HTTP request headers");
            assert_eq!(request.lines().next(), Some("GET /api/health HTTP/1.1"));
            let response = format!(
                "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
                body.len(), body
            );
            stream
                .write_all(response.as_bytes())
                .await
                .expect("write recorded health response");
            stream.shutdown().await.expect("close health response");
        })
        .await
        .expect("health transcript server timeout");
    });
    let http = reqwest::Client::builder()
        .no_proxy()
        .timeout(Duration::from_secs(30))
        .build()
        .expect("HTTP client");
    let client = ApiClient::new(
        Configuration::builder()
            .base_path(origin)
            .client(reqwest_middleware::ClientBuilder::new(http).build())
            .build(),
    );
    let response = client
        .health_api()
        .health()
        .await
        .expect("HealthApi request to the loopback transcript server");
    server.await.expect("health transcript server task");
    assert_eq!(response.status.as_u16(), 200);
    assert_eq!(
        response.content,
        recording["response"]["body"].as_str().unwrap()
    );
    let wire: Value = serde_json::from_str(&response.content).expect("HTTP health JSON");
    let typed = response
        .try_as_success()
        .expect("typed HTTP health success");
    assert_blocks_preserved(&wire, typed);
    let parsed = response
        .try_parse_as_success_model()
        .expect("explicit generated health decoder");
    assert_blocks_preserved(&wire, &parsed);
}
