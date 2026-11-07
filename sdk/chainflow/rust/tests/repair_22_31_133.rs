use std::io::{BufRead, BufReader, Read};
use std::net::TcpStream;
use std::time::Duration;

use chainflow_data_gateway::transport_reqwest::ReqwestTransport;
use chainflow_data_gateway::{ChainflowDataGatewayClient, Error, Market, WsScope, WsTokenRequest};
use http::{Method, Request};
use tokio::net::TcpListener;
use tokio::time::timeout;

const REST_KEY: &str = "rest-key-133";
const MINT_KEY: &str = "mint-key-133";
const SCOPED_TOKEN: &str = "scoped-token-133";
const DEADLINE: Duration = Duration::from_secs(5);

#[derive(Clone, Copy)]
enum Operation {
    Rest,
    Mint,
    Price,
    Markets,
    Explorer,
    Alias,
}

impl Operation {
    fn method(self) -> Method {
        match self {
            Self::Mint => Method::POST,
            _ => Method::GET,
        }
    }

    fn path(self) -> &'static str {
        match self {
            Self::Rest => "/api/btc/time",
            Self::Mint => "/auth/ws-token",
            Self::Price => "/ws/btc",
            Self::Markets => "/ws/markets",
            Self::Explorer => "/ws/explorer",
            Self::Alias => "/explorer",
        }
    }
}

fn read_request(stream: TcpStream) -> Request<Vec<u8>> {
    stream.set_nonblocking(false).expect("blocking peer socket");
    stream.set_read_timeout(Some(DEADLINE)).expect("peer read deadline");
    let mut reader = BufReader::new(stream);
    let mut line = String::new();
    assert!(reader.read_line(&mut line).expect("request line") > 0);
    let parts: Vec<_> = line.split_whitespace().collect();
    assert_eq!(parts.len(), 3);
    assert_eq!(parts[2], "HTTP/1.1");
    let mut request = Request::builder().method(parts[0]).uri(parts[1]);
    loop {
        line.clear();
        assert!(reader.read_line(&mut line).expect("request header") > 0);
        if line == "\r\n" {
            break;
        }
        let (name, value) = line.trim_end_matches("\r\n").split_once(':').expect("header pair");
        request = request.header(name, value.trim());
    }
    let mut request = request.body(Vec::new()).expect("wire request");
    assert!(!request.headers().contains_key("transfer-encoding"));
    let length = request.headers().get("content-length").map_or(0, |value| {
        value
            .to_str()
            .expect("content length text")
            .parse::<usize>()
            .expect("content length")
    });
    request.body_mut().resize(length, 0);
    reader.read_exact(request.body_mut()).expect("request body");
    request
}

async fn capture(
    operation: Operation,
    rest: Option<&str>,
    mint: Option<&str>,
    token: Option<&str>,
) -> Request<Vec<u8>> {
    let listener = TcpListener::bind("127.0.0.1:0").await.expect("loopback listener");
    let transport = ReqwestTransport::from_client(
        reqwest::Client::builder()
            .no_proxy()
            .redirect(reqwest::redirect::Policy::none())
            .build()
            .expect("HTTP client"),
    );
    let mut builder = ChainflowDataGatewayClient::builder()
        .base_url(format!("http://{}", listener.local_addr().expect("peer address")))
        .transport(transport)
        .timeout(DEADLINE)
        .max_retries(0);
    if let Some(key) = rest {
        builder = builder.rest_api_key(key);
    }
    if let Some(key) = mint {
        builder = builder.ws_mint_key(key);
    }
    if let Some(token) = token {
        builder = builder.ws_token(token);
    }
    let client = builder.build().expect("SDK client configuration");
    let peer = async move {
        let (stream, _) = timeout(DEADLINE, listener.accept())
            .await
            .expect("peer accept deadline")
            .expect("peer accept");
        tokio::task::spawn_blocking(move || read_request(stream.into_std().expect("peer stream")))
            .await
            .expect("peer request capture")
    };
    let send = async {
        match operation {
            Operation::Rest => {
                let result = client.trading_view().list_feed_time(Market::Btc).send().await;
                assert!(matches!(result, Err(Error::Transport(_))), "expected peer disconnect");
            }
            Operation::Mint => {
                let result = client
                    .gateway()
                    .create_web_socket_token(WsTokenRequest { symbol: WsScope::Btc })
                    .send()
                    .await;
                assert!(matches!(result, Err(Error::Transport(_))), "expected peer disconnect");
            }
            operation => {
                let resource = client.web_sockets();
                let result = match operation {
                    Operation::Price => resource.connect_price_feed(Market::Btc).open().await,
                    Operation::Markets => resource.connect_markets().open().await,
                    Operation::Explorer => resource.connect_explorer().open().await,
                    Operation::Alias => resource.connect_explorer_alias().open().await,
                    _ => unreachable!(),
                };
                assert!(
                    matches!(result, Err(Error::WebSocket { response: None, .. })),
                    "expected peer disconnect"
                );
            }
        }
    };
    let (request, result) = tokio::join!(peer, timeout(Duration::from_secs(15), send));
    result.expect("SDK operation deadline");
    request
}

fn assert_key(request: &Request<Vec<u8>>, operation: Operation, key: Option<&str>) {
    assert_eq!(request.method(), operation.method());
    assert_eq!(request.uri().path(), operation.path());
    let keys: Vec<_> = request.headers().get_all("x-api-key").iter().collect();
    match key {
        Some(key) => {
            assert_eq!(keys.len(), 1, "operation must send exactly its selected API key");
            assert_eq!(keys[0], key);
        }
        None => assert!(keys.is_empty(), "operation leaked an API key"),
    }
}

#[tokio::test]
async fn repair_133_rest_credential_selection() {
    let request = capture(Operation::Rest, Some(REST_KEY), Some(MINT_KEY), Some(SCOPED_TOKEN)).await;
    assert_key(&request, Operation::Rest, Some(REST_KEY));
    assert_eq!(request.uri().query(), None, "REST read leaked a query credential");
}

#[tokio::test]
async fn repair_133_mint_credential_selection() {
    let request = capture(Operation::Mint, Some(REST_KEY), Some(MINT_KEY), Some(SCOPED_TOKEN)).await;
    assert_key(&request, Operation::Mint, Some(MINT_KEY));
    assert_eq!(request.uri().query(), None, "mint operation leaked a query credential");
    let body: serde_json::Value = serde_json::from_slice(request.body()).expect("SDK mint JSON");
    assert_eq!(body, serde_json::json!({ "symbol": "btc" }));
}

#[tokio::test]
async fn repair_133_upgrade_credential_selection() {
    for operation in [
        Operation::Price,
        Operation::Markets,
        Operation::Explorer,
        Operation::Alias,
    ] {
        let request = capture(operation, Some(REST_KEY), Some(MINT_KEY), Some("scope+token/133?&=")).await;
        assert_key(&request, operation, None);
        assert_eq!(request.uri().query(), Some("token=scope%2Btoken%2F133%3F%26%3D"));
        assert!(
            request.headers()["upgrade"]
                .to_str()
                .expect("upgrade header")
                .eq_ignore_ascii_case("websocket")
        );
        assert!(
            !request.headers()["sec-websocket-key"].is_empty(),
            "real WebSocket handshake"
        );
    }
}

#[tokio::test]
async fn repair_133_no_credential_substitution() {
    for operation in [
        Operation::Rest,
        Operation::Mint,
        Operation::Price,
        Operation::Markets,
        Operation::Explorer,
        Operation::Alias,
    ] {
        let (rest, mint, token) = match operation {
            Operation::Rest => (None, Some(MINT_KEY), Some(SCOPED_TOKEN)),
            Operation::Mint => (Some(REST_KEY), None, Some(SCOPED_TOKEN)),
            _ => (Some(REST_KEY), Some(MINT_KEY), None),
        };
        let request = capture(operation, rest, mint, token).await;
        assert_key(&request, operation, None);
        assert_eq!(
            request.uri().query(),
            None,
            "operation substituted an unrelated credential"
        );
    }
}
