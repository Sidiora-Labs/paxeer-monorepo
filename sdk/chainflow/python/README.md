# Chainflow Data Gateway

This library provides synchronous and asynchronous access to the Chainflow
Data Gateway from Python.

The generated client targets `https://data.feed.chainflowtrading.com`, the
REST and WebSocket data gateway for price feeds, markets, explorer observations
and local CoinGecko/CMC-compatible adapters.

Paxeer mainnet-beta is one BFT `paxd` network. Its kernel is part of the network
and proposes blocks; the EVM chain ID is 125 (`0x7d`) and a receipt is final
when its block commits. Explorer snapshots from this SDK are observations, not
an additional finality mechanism. Sub-chains are tenant domains in the database.

Chain JSON-RPC uses public node roots such as
`https://api1.mainnet-beta.paxeer.network`, `api2` through `api14` on the same
domain, and `https://api-mainnet-beta.paxeer.network`. Those endpoints cannot
serve as this SDK's REST base URL. Hosted `px_` methods are declared separately
in the [hosted gateway contract](../../../services/platform/hosted/gateway/openrpc.json).

The full API of this library can be found in [api.md](./api.md).

<br />

## Contents

- [Installation](#installation)
- [Usage](#usage)
- [API Reference](./api.md)
- [Async](#async)
- [WebSockets](#websockets)
- [Authentication](#authentication)
- [Errors](#errors)
- [Client Options](#client-options)
- [Retries and Timeouts](#retries-and-timeouts)
- [Helpers](#helpers)
- [Logging](#logging)
- [Requirements](#requirements)

<br />

## Installation

```sh
python -m pip install "chainflow-data-gateway[websockets]"
```

To install this checked-out version instead, run from the monorepo root:

```sh
python -m pip install "./sdk/chainflow/python[websockets]"
```

The `websockets` extra supplies the optional transport (`websockets>=13,<16`).
For REST-only use, omit `[websockets]`.

<br />

## Usage

```python
import os

from chainflow_data_gateway import ChainflowDataGateway

client = ChainflowDataGateway(
    rest_api_key=os.environ["REST_API_KEY"],
)

gateway = client.gateway.list_health()

print(gateway)
```

The examples in the following sections assume a `client` configured as shown above.

See the [API reference](./api.md) for every available operation.

<br />

## Async

Use `AsyncChainflowDataGateway` for asynchronous REST calls with `await`.
The client reads configured credentials from the environment. Public health
needs no credential. Async WebSocket connections use `async with` and
`async for`; their token-refresh callback is synchronous, so arrange token
acquisition outside the event loop as needed.

```python
import asyncio

from chainflow_data_gateway import AsyncChainflowDataGateway


async def main() -> None:
    async with AsyncChainflowDataGateway() as client:
        gateway = await client.gateway.list_health()
        print(gateway)


asyncio.run(main())
```

<br />

## WebSockets

Install the `websockets` extra above. This server-side example requires
`WS_MINT_KEY`. Mint a token for the same lowercase feed used in the connection
(`btc`), and send only that short-lived token on the WebSocket upgrade. Keep
long-lived keys on the server.

```python
import os

from chainflow_data_gateway import ChainflowDataGateway
from chainflow_data_gateway.types.websocket_reconnection import (
    ReconnectingEvent,
    ReconnectingOverrides,
)

with ChainflowDataGateway(
    ws_mint_key=os.environ["WS_MINT_KEY"],
) as client:
    credential = client.gateway.create_web_socket_token(symbol="btc")

    def refresh_token(_event: ReconnectingEvent) -> ReconnectingOverrides:
        fresh = client.gateway.create_web_socket_token(symbol="btc")
        return {"extra_query": {"token": fresh.token}}

    with client.web_sockets.connect_price_feed(
        market="btc",
        extra_query={"token": credential.token},
        on_reconnecting=refresh_token,
    ) as connection:
        for message in connection:
            print(message)
```

This opens `wss://data.feed.chainflowtrading.com/ws/btc`. The token lifetime is
300 seconds; authentication happens at upgrade, so expiry alone does not close
an established socket. Each reconnect needs a valid token. The callback above
mints a fresh one for every supported reconnect attempt. Iteration drives the
reconnect handling; a direct `recv()` call receives one message and does not
run that retry loop. Connection setup and unrecoverable closes still raise.

Use `symbol="markets"` with `connect_markets()` for `/ws/markets`, or
`symbol="explorer"` with `connect_explorer()` for `/ws/explorer`. Feed and scope
must match. A token obtained from your trusted backend can also be set through
`ws_token` on the client; `extra_query` takes precedence for a handshake.
Never print or cache token responses, tokens or connection URLs.

<br />

## Authentication

The constructor permits REST-key-only, mint-key-only, token-only, mixed and
unauthenticated clients. It reads `REST_API_KEY`, `WS_MINT_KEY` and `WS_TOKEN`
if the corresponding constructor arguments are omitted. Each protected
operation requires its own credential: REST data reads use the REST header,
token minting uses the minting header, and WebSocket upgrades use the token
query. `/health` checks Nginx only and sends no auth header by default; it
does not prove upstream health.

| Option | Type | Default | Description |
| --- | --- | --- | --- |
| `rest_api_key` | `str \| None` | - | REST key; not the WS minting key. Defaults to REST_API_KEY. |
| `ws_mint_key` | `str \| None` | - | Separate WS key used ONLY to mint symbol-bound credentials. Defaults to WS_MINT_KEY. |
| `ws_token` | `str \| None` | - | Opaque 300-second, scope-bound token from POST /auth/ws-token. Checked at handshake, not forcibly expired mid-session. Defaults to WS_TOKEN. |

Declared schemes:

- `RestApiKey` API key in header `X-API-Key`
- `WsMintKey` API key in header `X-API-Key`
- `WsToken` API key in query `token`

<br />

## Errors

Non-success responses throw generated API errors. Error objects expose status, headers, response body, and request metadata where the target runtime supports it.

```python
from chainflow_data_gateway import APIStatusError

try:
    gateway = client.gateway.list_health()
except APIStatusError as err:
    print(err.status_code)
    raise
```

`APIConnectionError` covers connection failures and `APITimeoutError` covers
timeouts. Status errors expose `response`, `request`, `body` and `status_code`.
Review fields before logging: credentials can appear in URLs and raw bodies.
HTTP 200 feed or UDF responses can still contain application errors.

Documented error statuses: `400`, `401`, `403`, `404`, `405`, `413`, `415`, `426`, `429`, `500`, `502`, `503`, `504`.

<br />

## Client Options

Configure the generated client by setting any of these options when you create it.

```python
from chainflow_data_gateway import ChainflowDataGateway

client = ChainflowDataGateway(
    timeout=60.0,
    max_retries=2,
)
```

| Option | Type | Default | Description |
| --- | --- | --- | --- |
| `rest_api_key` | `str \| None` | `os.environ.get("REST_API_KEY")` | REST key; not the WS minting key. |
| `ws_mint_key` | `str \| None` | `os.environ.get("WS_MINT_KEY")` | Separate WS key used ONLY to mint symbol-bound credentials. |
| `ws_token` | `str \| None` | `os.environ.get("WS_TOKEN")` | Opaque 300-second, scope-bound token from POST /auth/ws-token. Checked at handshake, not forcibly expired mid-session. |
| `base_url` | `str \| httpx.URL \| None` | `SIDIORALABS_BASE_URL`, otherwise the production data gateway | Trusted REST base URL. |
| `websocket_base_url` | `str \| httpx.URL \| None` | Derived from `base_url` | `https` becomes `wss`, `http` becomes `ws`. |
| `timeout` | `float \| Timeout \| None` | `httpx.Timeout(60, connect=5)` | Per-request timeout; override with seconds or an `httpx.Timeout`. |
| `max_retries` | `int` | `2` | Number of retries for temporary failures. |
| `default_headers` | `Mapping[str, str] \| None` | - | Headers sent with every request. |
| `default_query` | `Mapping[str, object] \| None` | - | Query parameters sent with every request. |

<br />

## Retries and Timeouts

Generated clients support request timeouts and retry temporary failures such as network errors, 408, 409, 429, and 5xx responses. Tune the retry and timeout client options shown above, or override them per request.
HTTP timeouts apply per attempt, not to the combined duration of all retries.
The client uses a parsed `Retry-After` delay only when it is greater than zero
and at most 60 seconds; otherwise it uses exponential backoff with jitter.
WebSocket retries use the separate connection options and callback described above.

<br />

## Helpers

- Use `client.<resource>.with_raw_response.<method>(...)` to get the SDK response
  wrapper; inspect its headers or call `.parse()` for the typed value.
- Use `client.<resource>.with_streaming_response.<method>(...)` inside a context
  manager to consume a response body without buffering it.

<br />

## Logging

- Set the `SIDIORALABS_LOG` environment variable to `info` or `debug` to enable HTTP logging.
- Logs use the standard `logging` module under `chainflow_data_gateway` and `httpx`.
  The SDK redacts the `token` query parameter in WebSocket connection logs and
  recognized credential fields in connection-option logs. Received WebSocket
  message bytes are logged at debug level. Review message content and logging
  from transport libraries separately; never log tokens, API keys or mint responses.
- `SIDIORALABS_CUSTOM_HEADERS` reads newline-separated `Header: value` entries.
  Avoid supplying authentication there; operation-specific credentials should
  come from the constructor fields.

<br />

## Requirements

- Python 3.11 or newer
