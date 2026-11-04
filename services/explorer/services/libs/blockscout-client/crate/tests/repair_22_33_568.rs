use blockscout_client::{
    apis::{addresses_api::*, health_api::HealthError, tokens_api::*, Error},
    models, Api, ApiClient, Configuration,
};
use serde_json::{json, Value};
use std::{collections::BTreeMap, fmt::Debug, path::Path, time::Duration};
use tokio::{
    io::{AsyncReadExt, AsyncWriteExt},
    net::TcpListener,
    task::JoinHandle,
};

mod support;
use support::OwnedApiClient;

const ADDRESS: &str = "0xc0De20A37E2dAC848F81A93BD85FE4ACDdE7C0DE";
const TOKEN: &str = "0xD4416b13d2b3a9aBae7AcD5D6C2BbDBE25686401";
const REQUEST_DEADLINE: Duration = Duration::from_millis(300);
const TEST_DEADLINE: Duration = Duration::from_secs(5);

fn recording(name: &str) -> Value {
    let path = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("tests/recorded/eth_blockscout_com")
        .join(name);
    let envelope: Value = serde_json::from_slice(&std::fs::read(path).unwrap()).unwrap();
    assert_eq!(envelope["request"]["method"], "GET");
    assert_eq!(envelope["response"]["status"], 200);
    serde_json::from_str(envelope["response"]["body"].as_str().unwrap()).unwrap()
}

fn client(origin: &str, deadline: Duration) -> ApiClient {
    let http = reqwest::Client::builder()
        .no_proxy()
        .connect_timeout(deadline)
        .timeout(deadline)
        .build()
        .unwrap();
    ApiClient::new(
        Configuration::builder()
            .base_path(origin)
            .request_timeout(deadline)
            .client(reqwest_middleware::ClientBuilder::new(http).build())
            .build(),
    )
}

enum Reply {
    Body(u16, String),
    StallBeforeHeaders,
    StallInBody,
}

struct Exchange {
    path: String,
    query: BTreeMap<String, String>,
    reply: Reply,
}

impl Exchange {
    fn body(path: impl Into<String>, status: u16, body: String) -> Self {
        Self {
            path: path.into(),
            query: BTreeMap::new(),
            reply: Reply::Body(status, body),
        }
    }
}

// This peer serves committed compatibility/branch fixtures. The component under
// test is the production ApiClient, request transport and model codecs.
struct TranscriptServer {
    origin: String,
    task: Option<JoinHandle<()>>,
}

impl TranscriptServer {
    async fn start(script: Vec<Exchange>) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let origin = format!("http://{}", listener.local_addr().unwrap());
        let task = tokio::spawn(async move {
            tokio::time::timeout(TEST_DEADLINE, async move {
                for exchange in script {
                    let (mut stream, _) = listener.accept().await.unwrap();
                    let mut request = Vec::new();
                    while !request.ends_with(b"\r\n\r\n") {
                        assert!(request.len() < 8192, "oversized request headers");
                        request.push(stream.read_u8().await.unwrap());
                    }
                    let headers = std::str::from_utf8(&request).unwrap();
                    let parts: Vec<_> = headers.lines().next().unwrap().split_whitespace().collect();
                    assert_eq!(parts.len(), 3);
                    assert_eq!(parts[0], "GET");
                    assert_eq!(parts[2], "HTTP/1.1");
                    let target = url::Url::parse(&format!("http://localhost{}", parts[1])).unwrap();
                    assert_eq!(target.path(), exchange.path);
                    let pairs: Vec<_> = target.query_pairs().into_owned().collect();
                    let query: BTreeMap<_, _> = pairs.iter().cloned().collect();
                    assert_eq!(pairs.len(), query.len(), "no duplicated query keys");
                    assert_eq!(query, exchange.query);
                    match exchange.reply {
                        Reply::Body(status, body) => {
                            let response = format!(
                                "HTTP/1.1 {status} Fixture\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()
                            );
                            stream.write_all(response.as_bytes()).await.unwrap();
                            stream.shutdown().await.unwrap();
                        }
                        Reply::StallBeforeHeaders => std::future::pending::<()>().await,
                        Reply::StallInBody => {
                            stream.write_all(b"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: 1024\r\nConnection: close\r\n\r\n{").await.unwrap();
                            std::future::pending::<()>().await;
                        }
                    }
                }
            }).await.expect("HTTP transcript server exceeded its deadline");
        });
        Self {
            origin,
            task: Some(task),
        }
    }

    async fn finish(mut self) {
        self.task
            .take()
            .unwrap()
            .await
            .expect("HTTP transcript peer failed");
    }

    async fn stop(mut self) {
        let task = self.task.take().unwrap();
        task.abort();
        assert!(task.await.unwrap_err().is_cancelled());
    }
}

impl Drop for TranscriptServer {
    fn drop(&mut self) {
        if let Some(task) = &self.task {
            task.abort();
        }
    }
}

#[test]
fn shared_server_lifetime_is_owned_by_each_test_runtime() {
    fn runtime() -> tokio::runtime::Runtime {
        tokio::runtime::Builder::new_current_thread()
            .enable_all()
            .build()
            .unwrap()
    }
    let first_runtime = runtime();
    let second_runtime = runtime();
    let first = first_runtime.block_on(OwnedApiClient::start()).unwrap();
    let second = second_runtime.block_on(OwnedApiClient::start()).unwrap();
    first_runtime.block_on(async { drop(first) });
    drop(first_runtime);
    second_runtime.block_on(async {
        let response = second.health_api().health().await.unwrap();
        assert_eq!(response.status.as_u16(), 200);
        assert!(response.try_parse_as_success_model().unwrap().healthy);
        let response = second
            .addresses_api()
            .get_address(
                GetAddressParams::builder()
                    .address_hash(ADDRESS.to_owned())
                    .build(),
            )
            .await
            .unwrap();
        assert_eq!(response.try_parse_as_success_model().unwrap().hash, ADDRESS);
        drop(second);
    });
}

#[tokio::test]
async fn startup_failure_propagates_directory_diagnostic() {
    let missing = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("tests/recorded/eth_blockscout_com/absent-recording-directory");
    let result = tokio::time::timeout(TEST_DEADLINE, OwnedApiClient::start_from(&missing))
        .await
        .expect("startup failure must be bounded");
    let error = match result {
        Ok(_) => panic!("missing fixture directory unexpectedly started"),
        Err(error) => error,
    };
    assert!(error.to_string().contains("recording directory"));
    assert!(error.to_string().contains("absent-recording-directory"));
    assert_eq!(
        error.downcast_ref::<std::io::Error>().unwrap().kind(),
        std::io::ErrorKind::NotFound
    );
}

#[tokio::test]
async fn production_codecs_and_http_preserve_identity_amounts_nullability_and_pages() {
    let address: models::Address = serde_json::from_value(recording(
        "_api_v2_addresses_0xc0de20a37e2dac848f81a93bd85fe4acdde7c0de.json",
    ))
    .unwrap();
    let token: models::TokenInfo = serde_json::from_value(recording(
        "_api_v2_tokens_0xD4416b13d2b3a9aBae7AcD5D6C2BbDBE25686401.json",
    ))
    .unwrap();
    let original: models::GetTokenHolders200Response = serde_json::from_value(recording(
        "_api_v2_tokens_0xD4416b13d2b3a9aBae7AcD5D6C2BbDBE25686401_holders.json",
    ))
    .unwrap();
    assert_eq!(original.items.len(), 50);
    let mut first = original.clone();
    let second_items = first.items.split_off(25);
    let last = first.items.last().unwrap();
    first.next_page_params = json!({
        "address_hash": last.address_hash.as_ref().unwrap().hash,
        "value": last.value, "items_count": 25
    });
    let second = models::GetTokenHolders200Response {
        items: second_items,
        next_page_params: Value::Null,
    };
    let path = format!("/api/v2/tokens/{TOKEN}/holders");
    let mut continuation = Exchange::body(&path, 200, serde_json::to_string(&second).unwrap());
    continuation.query = first
        .next_page_params
        .as_object()
        .unwrap()
        .iter()
        .map(|(key, value)| {
            (
                key.clone(),
                value
                    .as_str()
                    .map(str::to_owned)
                    .unwrap_or_else(|| value.to_string()),
            )
        })
        .collect();
    let server = TranscriptServer::start(vec![
        Exchange::body(
            format!("/api/v2/addresses/{ADDRESS}"),
            200,
            serde_json::to_string(&address).unwrap(),
        ),
        Exchange::body(
            format!("/api/v2/tokens/{TOKEN}"),
            200,
            serde_json::to_string(&token).unwrap(),
        ),
        Exchange::body(&path, 200, serde_json::to_string(&first).unwrap()),
        continuation,
    ])
    .await;
    let client = client(&server.origin, TEST_DEADLINE);
    let response = client
        .addresses_api()
        .get_address(
            GetAddressParams::builder()
                .address_hash(ADDRESS.to_owned())
                .build(),
        )
        .await
        .unwrap();
    assert_eq!(response.status.as_u16(), 200);
    let decoded = response.try_parse_as_success_model().unwrap();
    assert_eq!(decoded, address);
    assert_eq!(decoded.hash, ADDRESS);
    assert_eq!(decoded.coin_balance.as_deref(), Some("55237013765083938"));
    assert!(decoded.creation_transaction_hash.is_none());
    assert!(decoded.creator_address_hash.is_none());
    assert!(decoded.token.is_none());
    let response = client
        .tokens_api()
        .get_token(
            GetTokenParams::builder()
                .address_hash(TOKEN.to_owned())
                .build(),
        )
        .await
        .unwrap();
    assert_eq!(response.status.as_u16(), 200);
    let decoded = response.try_parse_as_success_model().unwrap();
    assert_eq!(decoded, token);
    assert_eq!(decoded.address_hash.as_deref(), Some(TOKEN));
    assert_eq!(decoded.r#type.as_deref(), Some("ERC-1155"));
    assert!(
        decoded.decimals.is_none()
            && decoded.total_supply.is_none()
            && decoded.exchange_rate.is_none()
    );
    let response = client
        .tokens_api()
        .get_token_holders(
            GetTokenHoldersParams::builder()
                .address_hash(TOKEN.to_owned())
                .build(),
        )
        .await
        .unwrap();
    assert_eq!(response.status.as_u16(), 200);
    let cursor = response
        .next_page_cursor()
        .unwrap()
        .expect("nonterminal first page");
    let mut holders = response.try_parse_as_success_model().unwrap().items;
    assert_eq!(holders.len(), 25);
    drop(client);
    let resumed = client_for_resume(&server.origin);
    let response = resumed
        .tokens_api()
        .get_token_holders(
            GetTokenHoldersParams::builder()
                .address_hash(TOKEN.to_owned())
                .cursor(cursor)
                .build(),
        )
        .await
        .unwrap();
    assert_eq!(response.status.as_u16(), 200);
    assert!(response.next_page_cursor().unwrap().is_none());
    let page = response.try_parse_as_success_model().unwrap();
    assert_eq!(page.items.len(), 25);
    holders.extend(page.items);
    assert_eq!(holders, original.items);
    let identities: std::collections::BTreeSet<_> = holders
        .iter()
        .map(|holder| {
            (
                holder.address_hash.as_ref().unwrap().hash.clone(),
                holder.token_id.clone(),
            )
        })
        .collect();
    assert_eq!(identities.len(), 50);
    assert_eq!(
        holders[0].address_hash.as_ref().unwrap().hash,
        "0xffFfffff7A5CB56bBccb9dFAFF17Bb2B26a3d71C"
    );
    assert_eq!(
        holders[0].token_id.as_deref(),
        Some("84266618887154161292994929131093917163100278909472630836549921586931270424257")
    );
    assert_eq!(holders[0].value, "1");
    server.finish().await;
}

fn client_for_resume(origin: &str) -> ApiClient {
    client(origin, TEST_DEADLINE)
}

fn refusal_cases() -> Value {
    serde_json::from_str(include_str!("fixtures/repair_22_33_568.json")).unwrap()
}

#[tokio::test]
async fn invalid_hash_unknown_resource_and_restricted_address_preserve_refusals() {
    let fixtures = refusal_cases();
    for case in fixtures["refusals"].as_array().unwrap().iter().take(3) {
        let path = case["path"].as_str().unwrap();
        let status = case["status"].as_u64().unwrap() as u16;
        let body = serde_json::to_string(&case["body"]).unwrap();
        let server =
            TranscriptServer::start(vec![Exchange::body(path, status, body.clone())]).await;
        let client = client(&server.origin, TEST_DEADLINE);
        let hash = path.rsplit('/').next().unwrap().to_owned();
        if case["name"] == "unknown-token" {
            match client
                .tokens_api()
                .get_token(GetTokenParams::builder().address_hash(hash).build())
                .await
            {
                Err(Error::ResponseError(response)) => {
                    assert_eq!(response.status.as_u16(), 404);
                    assert_eq!(response.content, body);
                    match response.entity.unwrap() {
                        GetTokenError::UnknownValue(payload) => assert_eq!(payload, case["body"]),
                        other => panic!("unexpected refusal variant: {other:?}"),
                    }
                }
                other => panic!("unknown token must be refused: {other:?}"),
            }
        } else {
            match client
                .addresses_api()
                .get_address(GetAddressParams::builder().address_hash(hash).build())
                .await
            {
                Err(Error::ResponseError(response)) => {
                    assert_eq!(response.status.as_u16(), status);
                    assert_eq!(response.content, body);
                    match response.entity.unwrap() {
                        GetAddressError::UnknownValue(payload) => assert_eq!(payload, case["body"]),
                        other => panic!("unexpected refusal variant: {other:?}"),
                    }
                }
                other => panic!("{} must be refused: {other:?}", case["name"]),
            }
        }
        server.finish().await;
    }
}

#[tokio::test]
async fn unhealthy_indexing_retains_http_status_and_typed_health_error() {
    let fixtures = refusal_cases();
    let case = &fixtures["refusals"][3];
    let producer: models::UnhealthyHealthCheckResponse =
        serde_json::from_value(case["body"].clone()).unwrap();
    let body = serde_json::to_string(&producer).unwrap();
    let server =
        TranscriptServer::start(vec![Exchange::body("/api/health", 500, body.clone())]).await;
    match client(&server.origin, TEST_DEADLINE)
        .health_api()
        .health()
        .await
    {
        Err(Error::ResponseError(response)) => {
            assert_eq!(response.status.as_u16(), 500);
            assert_eq!(response.content, body);
            match response.entity.unwrap() {
                HealthError::Status500(health) => {
                    assert_eq!(*health, producer);
                    assert!(!health.healthy);
                    assert!(!health.metadata.blocks.healthy);
                    assert_eq!(health.error.code, 5002);
                    assert_eq!(health.error.message, "There are no blocks in the DB.");
                    assert_eq!(health.metadata.blocks.error.as_ref(), Some(&health.error));
                }
                other => panic!("unhealthy indexer must retain its typed refusal: {other:?}"),
            }
        }
        other => panic!("unhealthy indexing must never be success: {other:?}"),
    }
    server.finish().await;
}

#[tokio::test]
async fn malformed_success_is_a_diagnostic_decode_failure() {
    let server =
        TranscriptServer::start(vec![Exchange::body("/api/health", 200, "{".to_owned())]).await;
    match client(&server.origin, TEST_DEADLINE)
        .health_api()
        .health()
        .await
    {
        Err(Error::Decode {
            route,
            status,
            content,
            source,
        }) => {
            assert_eq!(route, "/api/health");
            assert_eq!(status.as_u16(), 200);
            assert_eq!(content, "{");
            assert!(source.is_eof());
        }
        other => panic!("malformed success must retain decode diagnostics: {other:?}"),
    }
    server.finish().await;
}

fn assert_transport_error<T: Debug>(result: Result<T, Error<HealthError>>, timeout: bool) {
    let error = match result {
        Err(Error::Reqwest(error)) => error,
        Err(Error::ReqwestMiddleware(reqwest_middleware::Error::Reqwest(error))) => error,
        other => panic!("expected bounded HTTP transport failure: {other:?}"),
    };
    if timeout {
        assert!(
            error.is_timeout(),
            "deadline must cause transport failure: {error}"
        );
    } else {
        assert!(
            error.is_connect(),
            "server shutdown must refuse connection: {error}"
        );
    }
}

#[tokio::test]
async fn request_deadline_bounds_stalled_headers_and_body() {
    for reply in [Reply::StallBeforeHeaders, Reply::StallInBody] {
        let server = TranscriptServer::start(vec![Exchange {
            path: "/api/health".to_owned(),
            query: BTreeMap::new(),
            reply,
        }])
        .await;
        let client = client(&server.origin, REQUEST_DEADLINE);
        let result = tokio::time::timeout(TEST_DEADLINE, client.health_api().health())
            .await
            .expect("production request deadline must fire before the test watchdog");
        assert_transport_error(result, true);
        server.stop().await;
    }
}

#[tokio::test]
async fn server_shutdown_produces_bounded_connection_diagnostic() {
    let server = TranscriptServer::start(vec![Exchange {
        path: "/api/health".to_owned(),
        query: BTreeMap::new(),
        reply: Reply::StallBeforeHeaders,
    }])
    .await;
    let client = client(&server.origin, REQUEST_DEADLINE);
    server.stop().await;
    let result = tokio::time::timeout(TEST_DEADLINE, client.health_api().health())
        .await
        .expect("closed server request must be bounded");
    assert_transport_error(result, false);
}

#[test]
fn upstream_recordings_have_explicit_provenance_and_schema_version() {
    let provenance: Value = serde_json::from_str(include_str!(
        "fixtures/upstream_compatibility_provenance.json"
    ))
    .unwrap();
    assert_eq!(provenance["origin"], "https://eth.blockscout.com");
    assert_eq!(provenance["schema_version"], "BlockScout API OpenAPI 1.0.0");
    assert_eq!(provenance["consumer_version"], env!("CARGO_PKG_VERSION"));
    assert_eq!(
        provenance["upstream_server_version"],
        "not captured by the original recorder"
    );
    let root = Path::new(env!("CARGO_MANIFEST_DIR")).join("tests/recorded/eth_blockscout_com");
    let mut actual: Vec<_> = std::fs::read_dir(root)
        .unwrap()
        .map(|entry| entry.unwrap().path())
        .filter(|path| path.extension().and_then(|extension| extension.to_str()) == Some("json"))
        .map(|path| path.file_name().unwrap().to_str().unwrap().to_owned())
        .collect();
    actual.sort();
    let expected: Vec<_> = provenance["recordings"]
        .as_array()
        .unwrap()
        .iter()
        .map(|name| name.as_str().unwrap().to_owned())
        .collect();
    assert_eq!(actual, expected);
    assert_eq!(actual.len(), 28);
}
