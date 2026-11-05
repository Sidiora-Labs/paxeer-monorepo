//! Doc-13 offline consumer proof. Upstream recordings and explicitly labeled
//! source-derived controller cases traverse loopback HTTP and the real client.
//! This does not execute the Elixir producer or certify a running deployment.

use blockscout_client::{apis::health_api::HealthError, Api, ApiClient, Configuration, Error};
use serde_json::Value;
use std::time::Duration;
use tokio::{
    io::{AsyncReadExt, AsyncWriteExt},
    net::TcpListener,
    time::timeout,
};

const RECORDED: &str =
    include_str!("../blockscout-client/crate/tests/recorded/eth_blockscout_com/_api_health.json");
const LIFECYCLE: &str =
    include_str!("../blockscout-client/crate/tests/health/repair_22_31_269.json");

fn assert_preserved(produced: &Value, consumed: &Value) {
    if let Value::Object(fields) = produced {
        let actual = consumed
            .as_object()
            .expect("typed consumer retained object");
        for (key, expected) in fields {
            assert_preserved(
                expected,
                actual.get(key).expect("typed consumer retained field"),
            );
        }
    } else {
        assert_eq!(produced, consumed);
    }
}

async fn check_health(status: u16, body: String) {
    let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
    let address = listener.local_addr().unwrap();
    let wire = body.clone();
    let server = tokio::spawn(async move {
        timeout(Duration::from_secs(5), async move {
            let (mut socket, _) = listener.accept().await.unwrap();
            let mut request = Vec::new();
            while !request.ends_with(b"\r\n\r\n") {
                assert!(request.len() < 16384);
                let mut byte = [0];
                socket.read_exact(&mut byte).await.unwrap();
                request.push(byte[0]);
            }
            assert!(request.starts_with(b"GET /api/health HTTP/1.1\r\n"));
            let headers = format!(
                "HTTP/1.1 {status} Replay\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
                wire.len()
            );
            socket.write_all(headers.as_bytes()).await.unwrap();
            socket.write_all(wire.as_bytes()).await.unwrap();
            socket.shutdown().await.unwrap();
        })
        .await
        .expect("health replay must finish");
    });
    let http = reqwest::Client::builder()
        .no_proxy()
        .timeout(Duration::from_secs(5))
        .build()
        .unwrap();
    let client = ApiClient::new(
        Configuration::builder()
            .base_path(format!("http://{address}"))
            .client(reqwest_middleware::ClientBuilder::new(http).build())
            .build(),
    );
    let produced: Value = serde_json::from_str(&body).unwrap();
    let result = client.health_api().health().await;
    server.await.unwrap();
    match status {
        200 => {
            let response = result.expect("healthy HTTP response");
            assert_eq!(response.status.as_u16(), status);
            assert_eq!(response.content, body);
            let borrowed = response.try_as_success().expect("typed health success");
            let parsed = response.try_parse_as_success_model().unwrap();
            assert_eq!(borrowed, &parsed);
            assert!(parsed.healthy);
            assert_preserved(&produced, &serde_json::to_value(parsed).unwrap());
        }
        500 => match result {
            Err(Error::ResponseError(response)) => {
                assert_eq!(response.status.as_u16(), status);
                assert_eq!(response.content, body);
                match response.entity.expect("typed health error") {
                    HealthError::Status500(model) => {
                        assert!(!model.healthy);
                        assert_eq!(model.metadata.blocks.error.as_ref(), Some(&model.error));
                        assert_eq!(serde_json::to_value(model).unwrap(), produced);
                    }
                    other => panic!("unhealthy response lost its typed error: {other:?}"),
                }
            }
            other => panic!("HTTP 500 must remain an error: {other:?}"),
        },
        other => panic!("unexpected health fixture status {other}"),
    }
}

#[tokio::test]
async fn doc_13_recorded_health_uses_mounted_route_and_both_consumers() {
    let recording: Value = serde_json::from_str(RECORDED).unwrap();
    assert_eq!(recording["request"]["method"], "GET");
    assert_eq!(recording["request"]["urlPath"], "/api/health");
    assert_eq!(recording["response"]["status"], 200);
    check_health(200, recording["response"]["body"].as_str().unwrap().into()).await;
}

#[tokio::test]
async fn doc_13_controller_lifecycle_preserves_metadata_and_errors() {
    let cases: Vec<Value> = serde_json::from_str(LIFECYCLE).unwrap();
    assert_eq!(cases.len(), 5);
    for (case, expected) in
        cases
            .iter()
            .zip(["healthy", "no_blocks", "stale", "rollup", "media_worker"])
    {
        assert_eq!(case["case"], expected);
        let status = u16::try_from(case["status"].as_u64().unwrap()).unwrap();
        assert_eq!(
            status,
            if matches!(expected, "no_blocks" | "stale") {
                500
            } else {
                200
            }
        );
        check_health(status, serde_json::to_string(&case["body"]).unwrap()).await;
    }
}

#[tokio::test]
async fn doc_13_legacy_health_refuses_before_transport() {
    // An origin that cannot be used makes a transport attempt fail differently.
    let client = ApiClient::new(Configuration::default());
    assert!(matches!(
        client.health_api().health_v1().await,
        Err(Error::UnsupportedCapability(_))
    ));
}
