---
name: chainflow-data-gateway-go-sdk
description: "Use the Go Chainflow Data Gateway SDK from a local checkout: installation, client authentication, REST operations and scoped WebSocket token minting."
---

# Chainflow Data Gateway Go SDK

Generated Go REST client for the Chainflow data gateway. Its module name is
`chainflow-data-gateway`; this checkout does not provide a hostname-qualified
module path for public installation. Use the local replacement below.

## Install

The checked-in [go.mod](../../../go.mod) declares `module chainflow-data-gateway`.
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

## Client setup and authentication

`NewClient` reads `REST_API_KEY`, `WS_MINT_KEY`, `WS_TOKEN` and
`SIDIORALABS_BASE_URL`. Explicit options follow those defaults. The default
base URL is `https://data.feed.chainflowtrading.com/`.

- `option.WithRestAPIKey` — REST `X-API-Key`.
- `option.WithWsMintKey` — separate `X-API-Key` for `POST /auth/ws-token`.
- `option.WithWsToken` — opaque, scope-bound handshake query token, valid for
  300 seconds. An established socket is not closed just because it expires.

Both key options write the same header, so the last option wins. If both key
environment variables are set, explicitly select the REST key for ordinary
calls and apply the minting key on the token-mint request. Keep both keys
server-side. Leave `WS_TOKEN` unset for REST clients; its default adds `token`
to every request. `option.WithQueryDel("token")` removes it for an operation.

## Calling operations

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
	health, err := client.Gateway.ListHealth(context.Background())
	if err != nil {
		panic(err)
	}
	fmt.Println(*health)
}
```

`/health` checks Nginx only. Read the exact operation signature and response
caveats in [api.md](../../../api.md), including the
[token-mint example](../../../api.md#mint-a-scoped-websocket-token). Do not print a
`WsToken`, its raw JSON, or a handshake URL. The Go SDK mints tokens but has
no generated WebSocket connection manager.

The data gateway provides market data and explorer observations. Paxeer is one
BFT `paxd` network with the kernel proposing blocks within it, chain ID 125
(`0x7d`), and finality at block commit. Chain JSON-RPC uses node roots such as
`https://api1.mainnet-beta.paxeer.network`; do not replace the SDK's REST base
URL with a node URL. See the [README](../../../README.md) for network context.

## Error handling

API failures return `*sdk.Error`; transport errors remain ordinary errors.
Use `errors.As` and log only reviewed metadata:

```go
_, err := client.Markets.ListHealth(context.Background())
if err != nil {
	var apiErr *sdk.Error
	if errors.As(err, &apiErr) {
		fmt.Println(apiErr.StatusCode)
	}
	return
}

// Additional import: "errors". Use inside a function with client in scope.
```

Whole errors, request dumps and response bodies can expose credentials.
HTTP 200 feed and UDF responses can contain application-level errors.

## Requirements

- Go 1.22 or newer.

## Reference files

- [README.md](../../../README.md) — installation, credentials, options, retries and logging.
- [api.md](../../../api.md) — every generated REST operation and its types.
