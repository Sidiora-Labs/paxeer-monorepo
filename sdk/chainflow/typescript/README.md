# Chainflow Data Gateway

This library provides convenient access to the Chainflow Data Gateway REST API from TypeScript or JavaScript.

The default data gateway is `https://data.feed.chainflowtrading.com`. It
provides price feeds, market summaries, TradingView data and explorer
observations. Paxeer mainnet-beta is one BFT `paxd` network with EVM chain ID
125 (`0x7d`); receipts become final when their block commits. Configure node
JSON-RPC separately using the root URLs in the [public RPC reference](../../../docs/site/docs/reference/public-rpc.md).

The full API of this library can be found in [api.md](./api.md).

<br />

## Contents

- [Installation](#installation)
- [Usage](#usage)
- [API Reference](./api.md)
- [WebSockets](#websockets)
- [Authentication](#authentication)
- [Errors](#errors)
- [Client Options](#client-options)
- [Request Options](#request-options)
- [Retries and Timeouts](#retries-and-timeouts)
- [Helpers](#helpers)
- [Logging](#logging)
- [Requirements](#requirements)

<br />

## Installation

```sh
npm install @sidioralabs/chainflow-data-gateway
```

<br />

## Usage

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

The examples in the following sections assume a `client` configured as shown above.

See the [API reference](./api.md) for every available operation.

<br />

## WebSockets

Use this server-side Node.js example with a REST client configured as in
Usage and a separate `WS_MINT_KEY`. The request header overrides the REST key
for token minting. Minting does not install the returned token automatically.

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

<br />

## Authentication

The constructor requires `restAPIKey` or `REST_API_KEY`. It also reads
`WS_MINT_KEY` and `WS_TOKEN` unless those options are explicitly set to `null`.
HTTP authentication applies both key options to `X-API-Key`; `wsMintKey` wins
when both are configured. Use `wsMintKey: null` for a REST client, and override
`X-API-Key` on the token-mint request as above. A configured `wsToken` adds a
`token` query parameter to HTTP requests as well as WebSocket upgrades.

| Option | Type | Default | Description |
| --- | --- | --- | --- |
| `restAPIKey` | `string \| AuthTokenProvider` | - | REST key; not the WS minting key. Defaults to REST_API_KEY. |
| `wsMintKey` | `string \| AuthTokenProvider \| null` | - | WS minting key; overrides the shared HTTP authentication header when set. Defaults to WS_MINT_KEY. |
| `wsToken` | `string \| AuthTokenProvider \| null` | - | Opaque 300-second, scope-bound token from POST /auth/ws-token. Checked at handshake, not forcibly expired mid-session. Defaults to WS_TOKEN. |

Declared schemes:

- `RestApiKey` API key in header `X-API-Key`
- `WsMintKey` API key in header `X-API-Key`
- `WsToken` API key in query `token`

<br />

## Errors

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

Transport failures use `APIConnectionError` or `APIConnectionTimeoutError`.
Review error fields before logging; raw bodies, URLs and headers may contain
credentials. HTTP 200 feed and UDF responses can contain application errors.

Documented error statuses: `400`, `401`, `403`, `404`, `405`, `413`, `415`, `426`, `429`, `500`, `502`, `503`, `504`.

<br />

## Client Options

Configure the generated client by setting any of these options when you create it.

```ts
import ChainflowDataGateway from '@sidioralabs/chainflow-data-gateway';

const client = new ChainflowDataGateway({
  timeout: 60000,
  maxRetries: 2,
  logLevel: 'warn',
});
```

| Option | Type | Default | Description |
| --- | --- | --- | --- |
| `restAPIKey` | `string \| AuthTokenProvider` | `process.env["REST_API_KEY"]` | REST key; not the WS minting key. |
| `wsMintKey` | `string \| AuthTokenProvider \| null` | `process.env["WS_MINT_KEY"]` | WS minting key; overrides the shared HTTP authentication header when set. |
| `wsToken` | `string \| AuthTokenProvider \| null` | `process.env["WS_TOKEN"]` | Opaque 300-second, scope-bound token from POST /auth/ws-token. Checked at handshake, not forcibly expired mid-session. |
| `baseURL` | `string \| null` | `process.env["SIDIORALABS_BASE_URL"]`, otherwise the data gateway | Override the base URL. `null` selects the default data gateway. |
| `timeout` | `number` | `60000` | Maximum time in milliseconds to wait for a response before aborting a request. |
| `maxRetries` | `number` | `2` | Number of retries for temporary failures. |
| `defaultHeaders` | `HeadersLike` | - | Headers sent with every request. |
| `defaultQuery` | `Record<string, string \| undefined>` | - | Query parameters sent with every request. |
| `fetchOptions` | `RequestInit` | - | Additional fetch options sent with every request. |
| `fetch` | `Fetch` | - | Custom fetch implementation. |
| `logLevel` | `"off" \| "error" \| "warn" \| "info" \| "debug"` | `process.env["SIDIORALABS_LOG"]`, otherwise `"warn"` | Controls request and retry debug logging. |
| `logger` | `Logger` | `console` | Custom logger implementation. |

<br />

## Request Options

| Option | Type | Default | Description |
| --- | --- | --- | --- |
| `headers` | `HeadersLike` | - | Per-request headers. |
| `query` | `Record<string, unknown>` | - | Per-request query parameters. |
| `body` | `unknown` | - | Override the generated request body. |
| `timeout` | `number` | - | Per-request timeout in milliseconds. |
| `maxRetries` | `number` | - | Per-request retry count. |
| `signal` | `AbortSignal` | - | Abort an in-flight request. |
| `fetchOptions` | `RequestInit` | - | Per-request fetch options. |
| `idempotencyKey` | `string` | - | Request metadata; this client does not configure an idempotency header. Set a required header explicitly in `headers`. |

<br />

## Retries and Timeouts

Generated clients support request timeouts and retry temporary failures such as network errors, 408, 409, 429, and 5xx responses. Retry delays honor `Retry-After` headers when present. Tune the retry and timeout client options shown above, or override them per request. HTTP timeouts apply to each attempt; retries can extend the total call duration. WebSocket reconnection uses separate connection options.

<br />

## Helpers

- Use `.withResponse()` on any request to inspect both parsed data and the raw `Response` object.
- Every operation returns an `APIPromise`, so you can `await` it directly or chain `.withResponse()`.

<br />

## Logging

- Set `logLevel: "debug"` to log request URLs, options, response status, response headers, and retry attempts.
- Pass a custom `logger` to route logs into your own observability pipeline.
- Set `logLevel: "off"` to disable logging regardless of `SIDIORALABS_LOG`.
- The built-in HTTP logger redacts credential headers, URL userinfo and the
  `token` query value. Request bodies and custom loggers still need review;
  never log mint responses or WebSocket connection URLs.

<br />

## Requirements

- Node.js 20+ with `fetch` for the default entry point and its `ws` transport.
- Browser WebSockets require a browser transport and backend-issued tokens.
