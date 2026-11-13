# Chainflow Data Gateway

Asynchronous Rust client for the Chainflow Data Gateway REST API and WebSocket
streams. The default gateway is `https://data.feed.chainflowtrading.com`.

For Paxeer integrations, the network is one BFT chain with chain ID 125 (`0x7d`);
its kernel proposes blocks and receipts are final when their block is committed.
Node JSON-RPC uses the public node root path, for example
`https://api1.mainnet-beta.paxeer.network/`. Configure node access separately from
this market-data client; see [public RPC](../../../docs/site/docs/reference/public-rpc.md).

See [api.md](./api.md) for operations, [reference.md](./reference.md) for runtime
options, and the [Python guide](../python/SKILL.md) and
[Python API reference](../python/api.md) for the same gateway's Python client.

## Installation

Add this to your `Cargo.toml`:

```toml
[dependencies]
chainflow-data-gateway = "0.1.0" # x-release-please-version
tokio = { version = "1", features = ["rt-multi-thread", "macros"] }
```

Or install via cargo:

```sh
cargo add chainflow-data-gateway
cargo add tokio --features rt-multi-thread,macros
```

## Usage

The client is asynchronous, with `reqwest` as the default HTTP backend
(swappable — see "Bring your own HTTP client" below):

```rust,no_run
use chainflow_data_gateway::*;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let client = ChainflowDataGatewayClient::builder()
        .rest_api_key(std::env::var("REST_API_KEY")?)
        .build()?;

    let response = client.gateway().list_health().send().await?;

    println!("{:?}", response);

    Ok(())
}
```

REST operations return request builders; set optional parameters fluently and
finish with `.send().await`. WebSocket builders finish with `.open().await`.
Gateway health checks liveness only, not the readiness of backing data services.
Some data operations return application errors within HTTP 200 responses; inspect
the response variant as well as the HTTP result. The [Python API reference](../python/api.md)
also records the gateway's text UDF time, history and display/raw ticker caveats.

Set only the credentials needed for the operations you call. The builder stores
explicitly supplied credentials; `from_env` reads them from the environment:

```rust,no_run
use chainflow_data_gateway::ChainflowDataGatewayClient;

async fn run() -> Result<(), Box<dyn std::error::Error>> {
    let client = ChainflowDataGatewayClient::builder()
        .rest_api_key("…")
        .ws_mint_key("…")
        .ws_token("…")
        .build()?;

    // Or, reading credentials from the environment:
    let client = ChainflowDataGatewayClient::from_env()?;
    let _ = client;
    Ok(())
}
```

## Authentication

| Builder option | Environment read by `from_env` | Use |
| --- | --- | --- |
| `rest_api_key` | `REST_API_KEY` | REST data reads, in `X-API-Key` |
| `ws_mint_key` | `WS_MINT_KEY` | `POST /auth/ws-token` only, in `X-API-Key` |
| `ws_token` | `WS_TOKEN` | WebSocket upgrades, in the `token` query parameter |

`from_env` also reads `SIDIORALABS_BASE_URL`. For an explicit builder, use
`.base_url(...)`. Missing credentials are left unset, so construction alone does
not prove an operation is authorized. Each operation selects its own credential;
configuring the three options together does not combine their privileges.

Keep REST and mint keys server-side. WS tokens are opaque, scope-bound
credentials with a 300-second handshake lifetime. The client does not perform
OAuth exchanges, cache tokens, refresh them or automatically mint them. Call the
mint operation and pass the result to a client used for that stream. After a
token expires, mint a new one before the next upgrade; expiry alone does not
close an established connection.

With the default `reqwest` and `websocket` features, this example mints a `btc`
token and opens that feed. The two clients reuse the same HTTP transport and
connection pool:

```rust,no_run
use chainflow_data_gateway::{ChainflowDataGatewayClient, Market, WsScope, WsTokenRequest};
use chainflow_data_gateway::transport::ReqwestTransport;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let transport = ReqwestTransport::new()?;
    let mint_client = ChainflowDataGatewayClient::builder()
        .transport(transport.clone())
        .ws_mint_key(std::env::var("WS_MINT_KEY")?)
        .build()?;
    let credential = mint_client.gateway()
        .create_web_socket_token(WsTokenRequest { symbol: WsScope::Btc })
        .send().await?;
    let stream_client = ChainflowDataGatewayClient::builder()
        .transport(transport)
        .ws_token(credential.token)
        .build()?;
    let mut connection = stream_client.web_sockets()
        .connect_price_feed(Market::Btc).open().await?;
    if let Some(message) = connection.recv().await {
        println!("{}", message?);
    }
    connection.close().await?;
    Ok(())
}
```

Feed connections use `/ws/{market}` with a matching lowercase feed scope.
`connect_markets()` uses `/ws/markets` and `WsScope::Markets`;
`connect_explorer()` uses `/ws/explorer` and `WsScope::Explorer`.
`connect_explorer_alias()` upgrades `/explorer` with the same explorer scope.
These connections send and receive `serde_json::Value` messages. Use separate
clients for different scopes or replacement tokens; cloning a client retains
its supplied credential values. Set the same base URL on mint and stream clients
when overriding the default gateway.

## Error handling

Fallible operations return [`Error`](src/error.rs). Match on the
result of `send().await` to distinguish API errors (with status and decoded
body) from transport and decoding failures:

```rust,no_run
use chainflow_data_gateway::Error;

fn report<T>(result: Result<T, Error>) {
    match result {
        Ok(value) => { /* … */ }
        Err(Error::Api(api)) => eprintln!("status {}", api.status),
        Err(other) => eprintln!("request failed: {other}"),
    }
}
```

`Error::Api` carries the response `status`, `headers`, the server-assigned
`request_id` when present, the decoded `body`, and `raw` text for a non-JSON body.
Error bodies are capped too: `truncated` indicates the payload was cut short
by the size limit or a read failure. WebSocket handshake rejections use
`Error::WebSocket { response, .. }`, with HTTP details when available.
Avoid logging bodies or headers that may contain credentials.

## Bring your own HTTP client

Every HTTP request flows through the `transport::Transport` trait — one
`execute` method over `http::Request<SdkBody>`. The default backend is
`reqwest`; to tune it (proxies, connection pools, TLS settings), pass a
configured client. Add a direct dependency matching the SDK's reqwest version
and choose its TLS backend, for example
`cargo add reqwest@0.13 --no-default-features --features rustls`:

```rust,no_run
use chainflow_data_gateway::ChainflowDataGatewayClient;
use chainflow_data_gateway::transport::ReqwestTransport;

fn configure() -> Result<(), Box<dyn std::error::Error>> {
    let http_client = reqwest::Client::builder()
        .redirect(reqwest::redirect::Policy::none())
        .build()?;
    let client = ChainflowDataGatewayClient::builder()
        .transport(ReqwestTransport::from_client(http_client))
        .build()?;
    Ok(())
}
```

To replace reqwest entirely, implement `Transport` for your backend and
drop the default features:

```sh
cargo add chainflow-data-gateway --no-default-features --features tokio
```

Re-enabling the `reqwest` feature on top of `--no-default-features` also
requires a TLS backend — add `rustls` or `native-tls` (the default feature
set includes one), or the crate fails to compile with a clear error.

`SdkBody` implements `http_body::Body`, so hyper and tower stacks consume
requests without conversion glue.
A complete hyper-backed transport ships
in [`examples/byo_transport_hyper.rs`](./examples/byo_transport_hyper.rs).
Retries, deadlines, auth, and decoding stay in the SDK runtime, so a custom
transport only moves one request and classifies its failures. With `tokio`
disabled, also supply a real `transport::Sleep` implementation through
`.sleeper(...)`; otherwise `.build()` returns `Error::Config`. WebSocket upgrades
use `tokio-tungstenite` directly and do not pass through the HTTP transport.

## Testing with MockTransport

The `mock` cargo feature ships an in-process transport with an ordered
response queue and full request capture, so tests drive the real client
without a network:

```sh
cargo add --dev chainflow-data-gateway --features mock
```

```rust,ignore
use chainflow_data_gateway::ChainflowDataGatewayClient;
use chainflow_data_gateway::transport::{InstantSleep, MockTransport};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let mock = MockTransport::new();
    mock.enqueue(200, "ok");

    let client = ChainflowDataGatewayClient::builder()
        .transport(mock.clone())
        .sleeper(InstantSleep)
        .build()?;
    let response = client.gateway().list_health().send().await?;
    assert_eq!(response.as_ref(), b"ok");

    let requests = mock.requests();
    assert_eq!(requests.len(), 1);
    assert_eq!(requests[0].method, http::Method::GET);
    assert_eq!(requests[0].uri.path(), "/health");
    Ok(())
}
```

Responses are returned strictly in the order enqueued; executing a request
with an empty queue panics, so an unexpected extra request fails loudly.

A complete, network-free round-trip ships in [`examples/mock_transport.rs`](./examples/mock_transport.rs).

## Tracing

The `tracing` feature (off by default — enabling it is the only way this
crate pulls in the `tracing` dependency) instruments the request runtime
with `debug`-level events: each request attempt, each retry decision and the
delay it chose, the terminal outcome, and response decode failures. The runtime
uses the credentials supplied to the client; it has no automatic credential
acquisition or renewal lifecycle to trace.

```sh
cargo add chainflow-data-gateway --features tracing
```

This crate emits events only and never installs a subscriber — your binary
does that, e.g. with `tracing-subscriber`:

```sh
RUST_LOG=chainflow_data_gateway=debug cargo run
```

Credentials never reach an event: URLs are logged with their query string
redacted, header values and names are never logged, and request/response
bodies are never logged (a failed decode reports the status and an error
class only). Event targets, messages, and field names are diagnostics rather
than API, and may change in any release.

## Cargo features

| Feature | Default | Enables |
| --- | --- | --- |
| `reqwest` | yes | The bundled `reqwest` HTTP backend. Implies `tokio`. |
| `tokio` | yes | The tokio-backed timer used for retry backoff and deadlines. |
| `rustls` | yes | TLS for the bundled backend via `rustls`. |
| `native-tls` | no | TLS for the bundled backend via the platform's own stack. |
| `websocket` | yes | WebSocket connections via `tokio-tungstenite` with rustls/WebPKI roots; implies `tokio`. |
| `mock` | no | `MockTransport` and `InstantSleep` for network-free tests. |
| `tracing` | no | `debug`-level request instrumentation via the `tracing` facade. |

Supported HTTP configurations include the defaults (`reqwest` with `rustls`),
or `--no-default-features --features reqwest,native-tls`. Enabling `reqwest`
requires at least one of `rustls` or `native-tls`; enabling both is allowed,
while enabling neither triggers a compile-time error. The TLS flags configure
the HTTP backend; the WebSocket dependency independently uses rustls.

For a custom HTTP transport, `--no-default-features --features tokio` omits
reqwest and uses the Tokio timer. Supply `.transport(...)` before `.build()`.
With no features, supply both a transport and a sleeper. Add `websocket` when
stream connections are needed; it also enables Tokio. Custom transport TLS
configuration belongs to that transport.

## Minimum supported Rust version

The crate declares Rust 1.85 in `Cargo.toml` (`edition = "2024"`). Check that
manifest when selecting a toolchain for the SDK version you use.

## Platform notes

The default HTTP TLS backend is rustls. Its native build dependencies may need
platform build tools such as CMake and, on Windows, NASM. To select the platform
TLS stack for HTTP (Schannel or Secure Transport where applicable):

```sh
cargo add chainflow-data-gateway --no-default-features --features reqwest,native-tls
```

## Reusing the client

Reuse a `ChainflowDataGatewayClient` for a stable credential set. `Clone` is
cheap: clones share an `Arc` containing the transport, timer, supplied
credentials, base URL, limits and default headers. With the bundled HTTP
transport, they share its connection pool. Cloning does not acquire or renew a
token, and changing an environment variable does not update existing clients.

To replace a WS token, explicitly mint a new one and build a client with
`.ws_token(...)`. Reuse a clone of the existing `ReqwestTransport` as in the
example above to retain its HTTP pool. Rebuilding with the default transport
for every request creates a new pool.

## Untrusted input

Response bodies are untrusted, so the runtime bounds them by default:

- Buffered response bodies are capped at 64 MiB (`max_response_size`), which
  bounds memory against a hostile or misconfigured server. Streaming responses
  are consumed incrementally and are exempt.
- Each attempt carries a 60-second deadline unless one is set explicitly; it
  covers request-body upload through response headers, and every retry gets a
  fresh one. Reading a *buffered* response body is bounded the same way, so a
  server that sends headers and then stalls cannot hang the call. The default
  skips streamed and multipart uploads and the body of a binary download, and
  streaming responses are always exempt.
- The default `ReqwestTransport::new()` disables redirects to avoid forwarding
  credentials to another host. `ReqwestTransport::from_client(...)` preserves
  the supplied client's redirect policy; configure it explicitly as above.
- Header values are never logged, URLs are logged with their query string
  redacted, and request/response bodies are never logged.

## API reference

See [api.md](./api.md) for Rust resources and operations and
[the Python operation catalogue](../python/api.md) for gateway routes and response
caveats shared by both clients.
