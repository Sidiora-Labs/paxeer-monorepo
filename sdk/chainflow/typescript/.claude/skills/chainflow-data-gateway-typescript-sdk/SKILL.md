---
name: chainflow-data-gateway-typescript-sdk
description: "TypeScript SDK for Chainflow Data Gateway API. Use when writing TypeScript code that calls Chainflow Data Gateway API with the @sidioralabs/chainflow-data-gateway package: installing it, constructing and authenticating the client, and calling API operations."
---

# Chainflow Data Gateway TypeScript SDK

Generated TypeScript client for Chainflow Data Gateway API, published as `@sidioralabs/chainflow-data-gateway`. Use the generated client instead of hand-writing HTTP requests.

## Install

```sh
npm install @sidioralabs/chainflow-data-gateway
```

## Client setup and authentication

```ts
import ChainflowDataGateway from '@sidioralabs/chainflow-data-gateway';

const client = new ChainflowDataGateway({
  restAPIKey: process.env['REST_API_KEY'],
  wsMintKey: null,
  wsToken: null,
});
```

The constructor requires `restAPIKey` or `REST_API_KEY`. HTTP key options
share the `X-API-Key` header: `wsMintKey` overwrites `restAPIKey` if both are
configured. Explicitly set `wsMintKey: null` and `wsToken: null` for ordinary
REST calls. A configured token is added to REST query strings as well as
WebSocket upgrades. Credentials default from these environment variables:

- `restAPIKey` (env: `REST_API_KEY`) — REST key; not the WS minting key.
- `wsMintKey` (env: `WS_MINT_KEY`) — WS minting key; overrides the shared HTTP authentication header when set.
- `wsToken` (env: `WS_TOKEN`) — Opaque 300-second, scope-bound token from POST /auth/ws-token. Checked at handshake, not forcibly expired mid-session.

## Calling operations

```ts
import ChainflowDataGateway from '@sidioralabs/chainflow-data-gateway';

const client = new ChainflowDataGateway({
  restAPIKey: process.env['REST_API_KEY'],
  wsMintKey: null,
  wsToken: null,
});

const response = await client.gateway.listHealth();

console.log(response);
```

Method names, parameter shapes, and response types are generated from the API description — do not guess them. Look up the exact call signature in [api.md](../../../api.md) before writing a call.

## WebSockets

Use this server-side Node.js example with a REST client configured as in
Client setup and authentication and a separate `WS_MINT_KEY`. The request
header overrides the REST key for token minting. Minting does not install the returned token automatically.

```ts
const mintKey = process.env['WS_MINT_KEY'];
if (!mintKey) throw new Error('WS_MINT_KEY is required');

const credential = await client.gateway.createWebSocketToken(
  { symbol: 'btc' },
  { headers: { 'X-API-Key': mintKey } },
);
const streamClient = client.withOptions({
  wsMintKey: null,
  wsToken: credential.token,
});
const connection = streamClient.webSockets.connectPriceFeed('btc');

try {
  for await (const event of connection) {
    if (event.type === 'error') throw new Error('WebSocket connection failed');
    if (event.type === 'message') console.log(event.message);
  }
} finally {
  connection.close();
}
```

The feed uses `/ws/btc` and the token must have scope `btc`. Use scope `markets`
with `connectMarkets()` or `explorer` with `connectExplorer()` or
`connectExplorerAlias()`. Tokens last 300 seconds for a handshake; expiry alone
does not close an established connection. Mint a new token before a later
handshake after expiry. Automatic reconnection is opt-in through `reconnect`
options and a synchronous `onReconnecting` callback; arrange fresh credentials
before another upgrade.

The Node.js wrapper uses `ws`, includes the configured REST `X-API-Key` header
and appends `wsToken` to the query. Keep this client and both API keys on the
server. Browser clients should obtain a scoped token from a trusted backend
and use a browser WebSocket transport without long-lived API keys. Never log
token responses, connection URLs or complete connection events; events may
contain transport errors. Use a separate stream client so its token does not
also become a query parameter on REST calls.

## Error handling

Non-success responses throw generated API errors. Error objects expose status, headers, response body, and request metadata where the target runtime supports it.

```ts
import { APIError } from '@sidioralabs/chainflow-data-gateway';

try {
  const response = await client.gateway.listHealth();
} catch (err) {
  if (err instanceof APIError) {
    console.log(err.status, err.name);
  }
  throw new Error('Gateway request failed');
}
```

Handle connection errors separately. Log only reviewed metadata; error bodies,
headers and URLs may contain credentials. HTTP 200 data responses may still
contain application errors.

## Requirements

- Node.js 20+ with `fetch` and the `ws` transport for the default entry point.

## Reference files

- [README.md](../../../README.md) — full feature tour: client options, request options, retries and timeouts, logging.
- [api.md](../../../api.md) — complete catalogue of every operation with request and response types.
