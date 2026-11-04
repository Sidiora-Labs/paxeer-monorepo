---
name: chainflow-data-gateway-python-sdk
description: "Python SDK for Chainflow Data Gateway API. Use when writing Python code that calls Chainflow Data Gateway API with the chainflow-data-gateway package: installing it, constructing and authenticating the client, and calling API operations."
---

# Chainflow Data Gateway Python SDK

Generated sync and async Python client for the Chainflow data gateway, packaged
as `chainflow-data-gateway`. It defaults to
`https://data.feed.chainflowtrading.com`. Use the generated operation signatures
instead of guessing request fields.

Paxeer is one BFT `paxd` network, with the kernel proposing blocks within it,
EVM chain ID 125 (`0x7d`) and finality at block commit. This data gateway
provides market data and explorer observations. Chain JSON-RPC uses public node
roots such as `https://api1.mainnet-beta.paxeer.network`; do not substitute that
URL for the data gateway. See the [README](../../../README.md) for context.

## Install

```sh
python -m pip install "chainflow-data-gateway[websockets]"
```

To install this checked-out version instead, run from the monorepo root:

```sh
python -m pip install "./sdk/chainflow/python[websockets]"
```

The `websockets` extra supplies the optional transport (`websockets>=13,<16`).
For REST-only use, omit `[websockets]`.

## Client setup and authentication

```python
import os

from chainflow_data_gateway import ChainflowDataGateway

client = ChainflowDataGateway(
    rest_api_key=os.environ["REST_API_KEY"],
)
```

The constructor requires `rest_api_key` or `REST_API_KEY` even for WebSocket
use. Credential fields default from their corresponding environment variables;
request authentication selects the scheme for each operation:

- `rest_api_key` (env: `REST_API_KEY`) — REST key; not the WS minting key.
- `ws_mint_key` (env: `WS_MINT_KEY`) — Separate WS key used ONLY to mint symbol-bound credentials.
- `ws_token` (env: `WS_TOKEN`) — Opaque 300-second, scope-bound token from POST /auth/ws-token. Checked at handshake, not forcibly expired mid-session.

## Calling operations

```python
import os

from chainflow_data_gateway import ChainflowDataGateway

client = ChainflowDataGateway(
    rest_api_key=os.environ["REST_API_KEY"],
)

gateway = client.gateway.list_health()

print(gateway)
```

Method names, parameter shapes, and response types are generated from the API description — do not guess them. Look up the exact call signature in [api.md](../../../api.md) before writing a call.

## WebSockets

Install the `websockets` extra above. This server-side example requires both
`REST_API_KEY` and the separate `WS_MINT_KEY`. Mint a token for the same
lowercase feed used in the connection (`btc`), and send only that short-lived
token on the WebSocket upgrade. Keep long-lived keys on the server.

```python
import os

from chainflow_data_gateway import ChainflowDataGateway
from chainflow_data_gateway.types.websocket_reconnection import (
    ReconnectingEvent,
    ReconnectingOverrides,
)

with ChainflowDataGateway(
    rest_api_key=os.environ["REST_API_KEY"],
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

## Error handling

Non-success responses throw generated API errors. Error objects expose status, headers, response body, and request metadata where the target runtime supports it.

```python
from chainflow_data_gateway import APIStatusError

try:
    gateway = client.gateway.list_health()
except APIStatusError as err:
    print(err.status_code)
    raise
```

Transport failures raise `APIConnectionError` or `APITimeoutError`. HTTP 200
feed and UDF results can contain application errors. Avoid logging whole errors,
raw bodies or URLs. WebSocket debug logs include the token-bearing URL: disable
credential-bearing debug output unless redaction filters are installed.
`/health` tests Nginx liveness only, not the auth helper or data services.

## Requirements

- Python 3.9 or newer

## Reference files

- [README.md](../../../README.md) — full feature tour: client options, retries and timeouts, logging.
- [api.md](../../../api.md) — complete catalogue of every operation with request and response types.
