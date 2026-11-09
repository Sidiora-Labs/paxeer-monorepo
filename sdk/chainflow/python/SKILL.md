---
name: chainflow-data-gateway-python-sdk
description: "Python SDK for Chainflow Data Gateway API. Use when writing Python code that calls Chainflow Data Gateway API with the chainflow-data-gateway package: installing it, constructing and authenticating the client, and calling API operations."
---

# Chainflow Data Gateway Python SDK

Use `chainflow-data-gateway` for Chainflow market data, TradingView UDF,
compatibility adapters, explorer observations and WebSocket streams. The client
base URL defaults to `https://data.feed.chainflowtrading.com`.

Paxeer integrations use one BFT network, chain ID 125 (`0x7d`), whose kernel
proposes blocks. A receipt is final once its block is committed. Public node
JSON-RPC uses the root path, for example
`https://api1.mainnet-beta.paxeer.network/`; configure that separately from this
SDK's data gateway. See the [public RPC reference](../../../docs/site/docs/reference/public-rpc.md).

## Install

For REST operations:

```sh
python3 -m pip install chainflow-data-gateway
```

For the WebSocket examples, install the optional dependency too:

```sh
python3 -m pip install 'chainflow-data-gateway[websockets]'
```

## Client setup and authentication

```python
import os

from chainflow_data_gateway import ChainflowDataGateway

client = ChainflowDataGateway(rest_api_key=os.environ["REST_API_KEY"])
```

The constructor reads these environment variables when the corresponding option
is omitted. It requires `rest_api_key` even for a client used only to mint tokens
or connect to a stream.

| Option | Environment | Sent to |
| --- | --- | --- |
| `rest_api_key` | `REST_API_KEY` | REST data operations, in `X-API-Key` |
| `ws_mint_key` | `WS_MINT_KEY` | `POST /auth/ws-token` only, in `X-API-Key` |
| `ws_token` | `WS_TOKEN` | WebSocket upgrades, in the `token` query parameter |

Keep both API keys server-side. Tokens are opaque credentials bound to one feed
or the `markets` or `explorer` scope, with a 300-second handshake lifetime. The
SDK does not mint or renew them automatically. Do not log tokens or connection
URLs containing them; Python WebSocket debug logging includes the connection URL.

Set `base_url` explicitly or use `SIDIORALABS_BASE_URL` to override the gateway.
`websocket_base_url` overrides the stream base; otherwise HTTP becomes WS and
HTTPS becomes WSS on the same host.

## Calling operations

```python
gateway = client.gateway.list_health()
price = client.price_feeds.list(market="btc")
print(gateway)
print(price)
```

Health returns text and checks gateway liveness only. It does not establish that
backing services or price data are ready. Look up exact method signatures,
parameter types and response caveats in [api.md](./api.md). HTTP 200 can carry an
application error, so inspect the returned price or UDF response as well as the
HTTP status. Call `client.close()` when finished, or use the client as a context
manager as below.

## WebSockets

Mint for the same scope that the connection will use, then explicitly install
the returned token. This complete example requires the `websockets` extra and
both API keys:

```python
import os

from chainflow_data_gateway import ChainflowDataGateway

with ChainflowDataGateway(
    rest_api_key=os.environ["REST_API_KEY"],
    ws_mint_key=os.environ["WS_MINT_KEY"],
) as client:
    credential = client.gateway.create_web_socket_token(symbol="btc")
    client.ws_token = credential.token
    with client.web_sockets.connect_price_feed(market="btc") as connection:
        message = connection.recv()
        print(message)
```

Use `symbol="markets"` with `connect_markets()` and `symbol="explorer"` with
`connect_explorer()` or `connect_explorer_alias()`. An established connection is
not closed solely because its token expires; mint and install a fresh token
before a later upgrade after expiry. Use separate clients for concurrent scopes.
`AsyncChainflowDataGateway` provides the same resources with `await` for REST
calls and `async with` for connections.

## Error handling

HTTP 4xx and 5xx responses raise `APIStatusError` subclasses. The exception
exposes `status_code`, `body`, `request` and `response`; headers are available
through `response.headers`. Connection failures raise `APIConnectionError`, and
timeouts raise `APITimeoutError`. WebSocket failures use the connection library's
exceptions as well as the SDK's connection and queue errors.

```python
from chainflow_data_gateway import APIStatusError

try:
    with ChainflowDataGateway(rest_api_key=os.environ["REST_API_KEY"]) as client:
        price = client.price_feeds.list(market="btc")
except APIStatusError as err:
    print(err.status_code)
    raise
```

## Requirements

- Python 3.11 or newer
- The `websockets` extra for stream connections

## Reference files

- [README.md](./README.md) — client options, retries, timeouts and logging.
- [api.md](./api.md) — operation catalogue, routes, request and response types.
- [Rust SDK](../rust/README.md) — the asynchronous Rust client for the same gateway.
