//! Offline proof for docs-client-generation. The production ApiClient and codecs
//! consume committed upstream samples and labeled source-derived examples over
//! loopback HTTP; this does not execute or certify the Elixir deployment.

use blockscout_client::{
    apis::{feed::FeedItem, health_api::HealthError, transactions_api::GetTxsParams},
    Api, ApiClient, Configuration, Error,
};
use serde_json::{json, Value};
use std::time::Duration;
use tokio::{
    io::{AsyncReadExt, AsyncWriteExt},
    net::TcpListener,
    task::JoinHandle,
    time::timeout,
};

const README: &str = include_str!("../blockscout-client/README.md");
const SAMPLES: &str =
    include_str!("../blockscout-client/crate/tests/fixtures/repair_22_33_430.json");
const HEALTH: &str =
    include_str!("../blockscout-client/crate/tests/recorded/eth_blockscout_com/_api_health.json");
const TRANSACTIONS: &str = include_str!(
    "../blockscout-client/crate/tests/recorded/eth_blockscout_com/_api_v2_transactions.json"
);

fn upstream_body(recording: &str) -> String {
    let transcript: Value = serde_json::from_str(recording).unwrap();
    assert_eq!(transcript["response"]["status"], 200);
    transcript["response"]["body"].as_str().unwrap().into()
}

async fn replay(path: &str, status: u16, body: String) -> (ApiClient, JoinHandle<()>) {
    let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
    let origin = format!("http://{}", listener.local_addr().unwrap());
    let path = path.to_owned();
    let server = tokio::spawn(async move {
        timeout(Duration::from_secs(5), async move {
            let (mut socket, _) = listener.accept().await.unwrap();
            let mut request = Vec::new();
            while !request.ends_with(b"\r\n\r\n") {
                assert!(request.len() < 16384, "unexpected oversized HTTP request");
                let mut byte = [0];
                socket.read_exact(&mut byte).await.unwrap();
                request.push(byte[0]);
            }
            let request = std::str::from_utf8(&request).unwrap();
            assert_eq!(request.lines().next(), Some(format!("GET {path} HTTP/1.1").as_str()));
            let headers = format!(
                "HTTP/1.1 {status} Fixture\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
                body.len()
            );
            socket.write_all(headers.as_bytes()).await.unwrap();
            socket.write_all(body.as_bytes()).await.unwrap();
            socket.shutdown().await.unwrap();
        })
        .await
        .expect("loopback fixture must complete");
    });
    let http = reqwest::Client::builder()
        .no_proxy()
        .timeout(Duration::from_secs(5))
        .build()
        .unwrap();
    let config = Configuration::builder()
        .base_path(origin)
        .client(reqwest_middleware::ClientBuilder::new(http).build())
        .build();
    (ApiClient::new(config), server)
}

#[test]
fn docs_client_generation_local_authority_and_validation() {
    // Keep the actionable workflow tied to the real generator input and scripts.
    let package: Value =
        serde_json::from_str(include_str!("../blockscout-client/package.json")).unwrap();
    assert!(package["scripts"]["generate"]
        .as_str()
        .unwrap()
        .contains("-i /local/swaggers/blockscout-api-final.yaml"));
    assert_eq!(
        package["scripts"]["generate-from-remote"],
        "yarn getV2 && yarn merge && yarn generate && yarn format"
    );
    for required in [
        "GET /api/v2/openapi",
        "swaggers/paxeer-local-openapi.json",
        "paths[`/api${path}`] = item",
        "spec.servers = [{ url: origin.origin }]",
        "swaggers/blockscout-api-v2.yaml",
        "yarn merge",
        "yarn generate",
        "**upstream samples**",
        "actual-producer validation",
        "native type `127`",
        "Error::ResponseError",
        "Error::Decode",
        "HTTP 500",
        "node_rpc_origin",
        "Configuration::builder().base_path",
        "try_parse_as_success_model",
    ] {
        assert!(
            README.contains(required),
            "missing documented contract: {required}"
        );
    }
    assert!(
        README
            .find("--output swaggers/paxeer-local-openapi.json")
            .unwrap()
            < README.find("\nyarn merge\n").unwrap()
    );
    assert!(README.contains("writes every response with status 200"));
    assert!(README.contains("no environment-based local-origin switch"));
}

#[tokio::test]
async fn docs_client_generation_explicit_rest_origin() {
    for origin in [
        "",
        "http://127.0.0.1:1/api",
        "http://127.0.0.1:1/api/v2",
        "http://127.0.0.1:1?key=value",
        "http://user:password@127.0.0.1:1",
    ] {
        let api = ApiClient::new(Configuration::builder().base_path(origin).build());
        assert!(
            matches!(
                api.health_api().health().await,
                Err(Error::Configuration(_))
            ),
            "invalid origin must fail before transport: {origin}"
        );
    }
    let api = ApiClient::new(Configuration::default());
    assert!(matches!(
        api.health_api().health().await,
        Err(Error::Configuration(_))
    ));
    let (api, server) = replay("/api/health", 200, upstream_body(HEALTH)).await;
    let response = api.health_api().health().await.unwrap();
    server.await.unwrap();
    assert_eq!(response.status, reqwest::StatusCode::OK);
    let cached = response.try_as_success().expect("typed health");
    let parsed = response.try_parse_as_success_model().unwrap();
    assert_eq!(cached, &parsed);
    assert_eq!(parsed.healthy, Some(true));
    let blocks = parsed.metadata.unwrap().blocks.unwrap();
    assert!(blocks.healthy);
    assert_eq!(
        blocks.latest_block.db.unwrap().number.as_deref(),
        Some("23116935")
    );
    assert_eq!(
        blocks.latest_block.cache.unwrap().number.as_deref(),
        Some("23116935")
    );
    assert_eq!(blocks.latest_block.node.unwrap().number, "23116935");
}

#[tokio::test]
async fn docs_client_generation_native_evm_and_empty_pages() {
    let examples: Value = serde_json::from_str(SAMPLES).unwrap();
    let transactions: Value = serde_json::from_str(&upstream_body(TRANSACTIONS)).unwrap();
    let evm = transactions["items"][0].clone();
    assert!(evm.is_object());
    let native = examples["native"].clone();
    let items = vec![native.clone(), evm.clone()];
    let (api, server) = replay(
        "/api/v2/main-page/transactions",
        200,
        json!(items).to_string(),
    )
    .await;
    let response = api.main_page_api().get_main_page_txs().await.unwrap();
    server.await.unwrap();
    assert_eq!(response.status, reqwest::StatusCode::OK);
    let parsed = response.try_parse_as_success_model().unwrap();
    for typed in [response.try_as_success().unwrap(), &parsed] {
        assert_eq!(typed.len(), 2);
        match &typed[0] {
            FeedItem::Native(update) => {
                assert_eq!(serde_json::to_value(update).unwrap(), native);
                assert_eq!(update.price_update.observation_count, 1);
                assert_eq!(
                    update.price_update.observations[0].index_price,
                    "4273590000000000000000"
                );
                assert_eq!(
                    update.price_update.observations[0].symbol.as_deref(),
                    Some("ETH/USD")
                );
            }
            other => panic!("native submission lost its discriminator: {other:?}"),
        }
        match &typed[1] {
            FeedItem::Evm(tx) => assert_eq!(tx.hash, evm["hash"].as_str().unwrap()),
            other => panic!("EVM transaction misclassified: {other:?}"),
        }
    }
    for items in [items, vec![]] {
        let body = json!({"items":items,"next_page_params":null});
        let (api, server) = replay("/api/v2/transactions", 200, body.to_string()).await;
        let response = api
            .transactions_api()
            .get_txs(GetTxsParams::builder().build())
            .await
            .unwrap();
        server.await.unwrap();
        let parsed = response.try_parse_as_success_model().unwrap();
        for typed in [response.try_as_success().unwrap(), &parsed] {
            assert_eq!(typed.items.len(), items.len());
            assert!(typed.next_page_params.is_null());
            if !items.is_empty() {
                assert!(matches!(typed.items[0], FeedItem::Native(_)));
                assert!(matches!(typed.items[1], FeedItem::Evm(_)));
            }
        }
        assert!(response.next_page_cursor().unwrap().is_none());
    }
}

#[tokio::test]
async fn docs_client_generation_error_status_and_raw_body() {
    let examples: Value = serde_json::from_str(SAMPLES).unwrap();
    let body = examples["unhealthy"].to_string();
    let (api, server) = replay("/api/health", 500, body.clone()).await;
    let error = api.health_api().health().await.unwrap_err();
    server.await.unwrap();
    match error {
        Error::ResponseError(response) => {
            assert_eq!(response.status, reqwest::StatusCode::INTERNAL_SERVER_ERROR);
            assert_eq!(response.content, body);
            match response.entity.unwrap() {
                HealthError::Status500(health) => {
                    assert_eq!(health.healthy, Some(false));
                    assert_eq!(health.error.as_ref().unwrap().code, 5002);
                    assert_eq!(
                        health.error.as_ref().unwrap().message,
                        "There are no blocks in the DB."
                    );
                    let blocks = health.metadata.as_ref().unwrap().blocks.as_ref().unwrap();
                    assert!(!blocks.healthy);
                    assert_eq!(blocks.error, health.error);
                    assert_eq!(blocks.latest_block.node.as_ref().unwrap().number, "110");
                }
                other => panic!("unhealthy body must retain its type: {other:?}"),
            }
        }
        other => panic!("HTTP 500 must retain transport evidence: {other:?}"),
    }
    let body = r#"{"healthy":true,"metadata":{"blocks":{"healthy":true,"latest_block":{"node":{"number":[]}}}}}"#;
    let (api, server) = replay("/api/health", 200, body.into()).await;
    let error = api.health_api().health().await.unwrap_err();
    server.await.unwrap();
    match error {
        Error::Decode {
            route,
            status,
            content,
            ..
        } => {
            assert_eq!(route, "/api/health");
            assert_eq!(status, reqwest::StatusCode::OK);
            assert_eq!(content, body);
        }
        other => panic!("malformed success must be observable: {other:?}"),
    }
}
