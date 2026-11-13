# Chainflow Data Gateway

This library provides access to the Chainflow Data Gateway REST API from Go.
It includes price feeds, market summaries, TradingView data, explorer observations
and local CoinGecko/CMC-compatible adapters.

The client defaults to `https://data.feed.chainflowtrading.com/` for the
Chainflow REST data gateway. Paxeer mainnet-beta is one BFT network running
`paxd`, with the kernel inside the network proposing blocks and EVM chain ID
125 (`0x7d`). A transaction receipt is final when its block commits. Gateway
explorer snapshots are observations, not a separate finality source.

For chain JSON-RPC, use the root of a public node such as
`https://api1.mainnet-beta.paxeer.network` (also `api2` through `api14` on the
same domain, and `https://api-mainnet-beta.paxeer.network`). Those node URLs
are not substitutes for this client's data gateway base URL. Hosted `px_`
methods have their own [gateway contract](../../../services/platform/hosted/gateway/openrpc.json).
Sub-chains are tenant domains in the database, not additional chains to deploy.

The full API of this library can be found in [api.md](./api.md).

<br />

## Contents

- [Installation](#installation)
- [Usage](#usage)
- [API Reference](./api.md)
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

The checked-in [go.mod](./go.mod) declares `module chainflow-data-gateway`.
That hostname-free name is the import path, not a downloadable public module
address. Consume this checkout with a local `replace`; do not invent a published
GitHub module path.

From a directory outside the monorepo, create a consumer module (use your own
module name and the absolute path to your checkout):

```sh
CHAINFLOW_SDK=/absolute/path/to/paxeer-monorepo/sdk/chainflow/go
mkdir chainflow-example
cd chainflow-example
go mod init example.com/chainflow-example
go mod edit -replace="chainflow-data-gateway=$CHAINFLOW_SDK"
go get chainflow-data-gateway@v0.0.0
```

The replacement supplies the SDK locally; Go downloads its declared dependencies.
Keep `sdk "chainflow-data-gateway"` and `"chainflow-data-gateway/option"` in
imports. The local `v0.0.0` requirement does not claim a published SDK release.
Save the usage example as `main.go` in the consumer module. A local replacement
must also be available to other developers or CI using that consumer. After saving
`main.go`, resolve dependencies and compile the new consumer:

```sh
go mod tidy
go build ./...
```

The regression fixture in `testdata/repair_m1306_consumer` is a separate consumer
module with the same requirement, imports and local replacement. The SDK
workspace links it for the focused tests without adding consumer dependencies
to the SDK module.

<br />

## Usage

```go
package main

import (
	"context"
	"fmt"
	"os"

	sdk "chainflow-data-gateway"
	"chainflow-data-gateway/option"
)

func main() {
	client := sdk.NewClient(
		option.WithRestAPIKey(os.Getenv("REST_API_KEY")),
		option.WithQueryDel("token"),
	)

	gateway, err := client.Gateway.ListHealth(context.Background())
	if err != nil {
		panic(err)
	}

	fmt.Println(*gateway)
}
```

The examples in the following sections assume a `client` configured as shown above.

See the [API reference](./api.md) for every available operation.

<br />

## Authentication

The constructor reads `REST_API_KEY`, `WS_MINT_KEY` and `WS_TOKEN` when set;
explicit options are applied afterward. Keep long-lived REST and minting keys
on the server. `/health` is a liveness check of Nginx only. It does not prove
that upstream services or the database are healthy.

| Option | Type | Default | Description |
| --- | --- | --- | --- |
| `option.WithRestAPIKey` | `func(string) option.RequestOption` | - | REST key; not the WS minting key. Defaults to REST_API_KEY. |
| `option.WithWsMintKey` | `func(string) option.RequestOption` | - | WS minting key; sets the shared authentication header for the client or request. Defaults to WS_MINT_KEY. |
| `option.WithWsToken` | `func(string) option.RequestOption` | - | Opaque 300-second, scope-bound token from POST /auth/ws-token. Checked at handshake, not forcibly expired mid-session. Defaults to WS_TOKEN. |

Both key options set the same `X-API-Key` header; the last applied option wins.
If both key environment variables are set, the minting key overwrites the REST
key in the defaults. Explicitly select `WithRestAPIKey` for REST calls as in
Usage, and apply `WithWsMintKey` on the token-mint operation as shown in the
[API reference](./api.md#mint-a-scoped-websocket-token). `WithWsToken` adds a
query parameter to every request on which it is applied: leave `WS_TOKEN`
unset for REST clients, or remove that query using `option.WithQueryDel("token")`.
The Go client has no generated WebSocket connection manager; use a WebSocket
transport for the handshake with the minted token and matching scope.

Declared schemes:

- `RestApiKey` API key in header `X-API-Key`
- `WsMintKey` API key in header `X-API-Key`
- `WsToken` API key in query `token`

<br />

## Errors

Non-success responses return generated API errors. Error objects expose status, headers, response body, and request metadata where the target runtime supports it.

```go
_, err := client.Gateway.ListHealth(context.Background())
if err != nil {
	var apiErr *sdk.Error
	if errors.As(err, &apiErr) {
		fmt.Println(apiErr.StatusCode)
	}
	return
}

// imports: "context", "errors", "fmt", sdk "chainflow-data-gateway"
```

Handle transport errors separately from `*sdk.Error`. Avoid logging whole errors,
request dumps or raw responses: they can contain credentials or token-bearing URLs.
An HTTP 200 response may still contain a feed or UDF application error; consult
the operation caveats in [api.md](./api.md).

Documented error statuses: `400`, `401`, `403`, `404`, `405`, `413`, `415`, `429`, `500`, `502`, `503`, `504`.

<br />

## Client Options

Configure the generated client by setting any of these options when you create it.

```go
client := sdk.NewClient(
	option.WithBaseURL("https://data.feed.chainflowtrading.com/"),
	option.WithMaxRetries(2),
	option.WithRequestTimeout(60*time.Second),
)

// imports: sdk "chainflow-data-gateway", "chainflow-data-gateway/option", "time"
```

| Option | Type | Default | Description |
| --- | --- | --- | --- |
| `option.WithRestAPIKey` | `func(string) option.RequestOption` | `os.Getenv("REST_API_KEY")` | REST key; not the WS minting key. |
| `option.WithWsMintKey` | `func(string) option.RequestOption` | `os.Getenv("WS_MINT_KEY")` | WS minting key; sets the shared authentication header for the client or request. |
| `option.WithWsToken` | `func(string) option.RequestOption` | `os.Getenv("WS_TOKEN")` | Opaque 300-second, scope-bound token from POST /auth/ws-token. Checked at handshake, not forcibly expired mid-session. |
| `option.WithEnvironmentProductionHttpsGatewayUseWssForWebSocketUpgrades` | `func() option.RequestOption` | - | Select the production_https_gateway_use_wss_for_web_socket_upgrades API environment. |
| `option.WithBaseURL` | `func(string) option.RequestOption` | `SIDIORALABS_BASE_URL`, otherwise the production data gateway | Override with a trusted gateway URL. |
| `option.WithRequestTimeout` | `func(time.Duration) option.RequestOption` | - | Maximum time to wait for each request attempt. |
| `option.WithMaxRetries` | `func(int) option.RequestOption` | `2` | Number of retries for temporary failures. |
| `option.WithHTTPClient` | `func(option.HTTPClient) option.RequestOption` | - | Custom HTTP client or transport implementation. |

<br />

## Request Options

| Option | Type | Default | Description |
| --- | --- | --- | --- |
| `option.WithHeader` | `func(string, string) option.RequestOption` | - | Set a per-request header. |
| `option.WithQuery` | `func(string, string) option.RequestOption` | - | Set a per-request query parameter. |
| `option.WithRequestBody` | `func(string, any) option.RequestOption` | - | Override the serialized request body and content type. |
| `option.WithResponseInto` | `func(**http.Response) option.RequestOption` | - | Capture the raw HTTP response. |
| `option.WithResponseBodyInto` | `func(any) option.RequestOption` | - | Override the response deserialization target. |

<br />

## Retries and Timeouts

The client defaults to two retries and retries temporary failures such as network errors, 408, 409, 429, and 5xx responses. Retry delays honor `Retry-After` headers when present. Tune the retry and timeout options above, or override them per request.
`WithRequestTimeout` bounds each attempt; use a context deadline to bound the
entire operation including retries. With the standard HTTP transport, the
default client sets a ten-minute response-header timeout, not an overall
response-body deadline.

<br />

## Helpers

- Pass `option.WithResponseInto(&raw)` to capture the underlying `*http.Response` for a request.
- Use the generated `F`, `Null`, `Raw`, `Int`, `String`, `Float`, `Bool`, and `FileParam` helpers when setting optional params.

<br />

## Logging

- Use `option.WithMiddleware(...)` for tracing. Redact `X-API-Key`, the `token`
  query parameter and token response bodies before logging. Never print a `WsToken`.
- `SIDIORALABS_CUSTOM_HEADERS` supplies newline-separated `Header: value` entries;
  avoid overriding authentication headers unintentionally.

<br />

## Requirements

- Go 1.25 or newer
