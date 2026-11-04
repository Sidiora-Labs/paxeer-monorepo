# EthereumJSONRPC

EthereumJSONRPC is the explorer's Elixir library for single and batch JSON-RPC
calls, ABI encoding and decoding, and block, transaction, receipt, log and
balance retrieval. Transport behaviours support HTTP, IPC and WebSocket
subscriptions; client variants adapt node-specific responses. HTTP request
coordination, throttling and endpoint availability tracking are implemented
in [lib/ethereum_jsonrpc](lib/ethereum_jsonrpc).

Paxeer mainnet-beta is one BFT network, EVM chain ID 125 (`0x7d`). Its kernel
is part of the network and proposes blocks; receipts are final when their
blocks commit. Public HTTP JSON-RPC uses the root path on
`api1.mainnet-beta.paxeer.network` through
`api14.mainnet-beta.paxeer.network`, and
`api-mainnet-beta.paxeer.network`, for example
`https://api1.mainnet-beta.paxeer.network/`. The explorer's
[account REST API](../block_scout_web/API.md) is served by the explorer origin.

## Configuration

[config/runtime.exs](../../config/runtime.exs) configures the transport, HTTP
headers, timeouts and request guards. For production, it loads
[config/runtime/prod.exs](../../config/runtime/prod.exs), which selects
[explorer/config/prod/paxeer_x.exs](../explorer/config/prod/paxeer_x.exs).
Set the Paxeer chain identity and variant for the backend:

```sh
export CHAIN_TYPE=paxeer_x
export ETHEREUM_JSONRPC_VARIANT=paxeer_x
export ETHEREUM_JSONRPC_TRANSPORT=http
export ETHEREUM_JSONRPC_HTTP_URLS=https://api1.mainnet-beta.paxeer.network/,https://api2.mainnet-beta.paxeer.network/
```

The selected profile sets `config :explorer, json_rpc_named_arguments: [...]` with
`transport: EthereumJSONRPC.HTTP`, `variant: EthereumJSONRPC.PaxeerX`, and
these `transport_options`:

| Option | Environment input |
| --- | --- |
| `urls` | `ETHEREUM_JSONRPC_HTTP_URLS` or `ETHEREUM_JSONRPC_HTTP_URL` |
| `trace_urls` | `ETHEREUM_JSONRPC_TRACE_URLS` or `ETHEREUM_JSONRPC_TRACE_URL` |
| `eth_call_urls` | `ETHEREUM_JSONRPC_ETH_CALL_URLS` or `ETHEREUM_JSONRPC_ETH_CALL_URL` |
| `fallback_urls` | `ETHEREUM_JSONRPC_FALLBACK_HTTP_URLS` or `ETHEREUM_JSONRPC_FALLBACK_HTTP_URL` |
| `fallback_trace_urls` | `ETHEREUM_JSONRPC_FALLBACK_TRACE_URLS` or `ETHEREUM_JSONRPC_FALLBACK_TRACE_URL` |
| `fallback_eth_call_urls` | `ETHEREUM_JSONRPC_FALLBACK_ETH_CALL_URLS` or `ETHEREUM_JSONRPC_FALLBACK_ETH_CALL_URL` |

Lists are comma-separated, without spaces; a nonempty plural variable takes
precedence over the singular one. Unset trace and call lists reuse the HTTP
list. Unset fallback HTTP URLs reuse the primary HTTP list; unset specialized
fallback lists reuse fallback HTTP. Configure endpoints with the tracing and
archive capabilities the indexer needs. The method mapping sends `eth_call`
to the call list and `debug_traceTransaction` and `debug_traceBlockByNumber`
to the trace list.

The HTTP adapter is `EthereumJSONRPC.HTTP.Tesla` and its options are built by
`ConfigHelper.http_options(1)`. `ETHEREUM_JSONRPC_HTTP_TIMEOUT` is in seconds;
the helper converts it to milliseconds (default 60 seconds for this path).
Headers come from `ETHEREUM_JSONRPC_HTTP_HEADERS` (a JSON object) merged over
the default JSON content type. `ETHEREUM_JSONRPC_HTTP_GZIP_ENABLED` requests
gzip-compressed responses with `Accept-Encoding: gzip` (default false).
Put URL lists and the HTTP adapter in
`json_rpc_named_arguments[:transport_options]`; top-level `url`, `trace_url`
and `http` settings do not configure this call path.

Subscriptions use the separate `subscribe_named_arguments` entry with
`transport: EthereumJSONRPC.WebSocket`, `web_socket:
EthereumJSONRPC.WebSocket.WebSocketClient`, `url` from
`ETHEREUM_JSONRPC_WS_URL`, and `fallback_url` from
`ETHEREUM_JSONRPC_FALLBACK_WS_URL`. Set those to paired WebSocket endpoints
provided by your node deployment. For a disposable local Paxeer node, use
`ETHEREUM_JSONRPC_HTTP_URLS=http://127.0.0.1:8545/` and
`ETHEREUM_JSONRPC_WS_URL=ws://127.0.0.1:8546/`; both listeners must serve the
same chain-125 node. These are example local endpoints that you must supply.
Keep request transport as `http` when using
subscriptions: runtime transport selection accepts only `http` or `ipc`.
WebSocket URLs in HTTP URL lists are rejected. IPC uses `IPC_PATH`.

The effective production named arguments are below. `ConfigHelper` is loaded
by the backend configuration; the URL and timeout helpers read the variables
above. Production consumers use
`Application.fetch_env!(:explorer, :json_rpc_named_arguments)` and
`Application.fetch_env!(:explorer, :subscribe_named_arguments)`.

<!-- named-arguments:start -->
```elixir
json_rpc_named_arguments = [
  transport: EthereumJSONRPC.HTTP,
  transport_options: [
    http: EthereumJSONRPC.HTTP.Tesla,
    urls: ConfigHelper.parse_urls_list(:http),
    trace_urls: ConfigHelper.parse_urls_list(:trace),
    eth_call_urls: ConfigHelper.parse_urls_list(:eth_call),
    fallback_urls: ConfigHelper.parse_urls_list(:fallback_http),
    fallback_trace_urls: ConfigHelper.parse_urls_list(:fallback_trace),
    fallback_eth_call_urls: ConfigHelper.parse_urls_list(:fallback_eth_call),
    method_to_url: [
      eth_call: :eth_call,
      debug_traceTransaction: :trace,
      debug_traceBlockByNumber: :trace
    ],
    http_options: ConfigHelper.http_options(1)
  ],
  variant: EthereumJSONRPC.PaxeerX
]

subscribe_named_arguments = [
  transport: EthereumJSONRPC.WebSocket,
  transport_options: [
    web_socket: EthereumJSONRPC.WebSocket.WebSocketClient,
    url: ConfigHelper.parse_url_env_var("ETHEREUM_JSONRPC_WS_URL"),
    fallback_url: ConfigHelper.parse_url_env_var("ETHEREUM_JSONRPC_FALLBACK_WS_URL")
  ],
  variant: EthereumJSONRPC.PaxeerX
]
```
<!-- named-arguments:end -->

`EthereumJSONRPC.fetch_chain_id(json_rpc_named_arguments)` must return
`{:ok, 125}` (`eth_chainId` returns `"0x7d"` on the wire). Check the same
identity over WebSocket before consuming subscriptions. The realtime indexer
supervisor converts subscription `transport_options` into an
`EthereumJSONRPC.WebSocket` struct, attaches
`EthereumJSONRPC.WebSocket.WebSocketClient.Options` with the supervised client
name, and starts `EthereumJSONRPC.WebSocket.Supervisor`. A standalone caller
must do that setup too before calling `EthereumJSONRPC.subscribe/3`; the raw
configuration keyword list alone is not a running WebSocket connection.
For a request on that connection, use `EthereumJSONRPC.WebSocket.json_rpc/2`
with an `EthereumJSONRPC.request/1` request and the initialized WebSocket
struct; decode its `eth_chainId` result with
`EthereumJSONRPC.quantity_to_integer/1` and require 125.

### Runtime timeouts and guards

The following keys are configured in `config/runtime.exs` unless the consumer
is `transport_options[:http_options]`, which comes from `ConfigHelper`.
Timeout environment variables state their units; duration strings accept
values such as `20s` and `1m`.

| Environment input | Consumer key | Default |
| --- | --- | --- |
| `ETHEREUM_JSONRPC_HTTP_TIMEOUT` | `http_options[:recv_timeout]` and `[:timeout]` | 60 seconds (60_000 milliseconds) |
| `ETHEREUM_JSONRPC_DEBUG_TRACE_TRANSACTION_TIMEOUT` | `EthereumJSONRPC.Geth[:debug_trace_timeout]` | `5s` |
| `ETHEREUM_JSONRPC_WAIT_PER_TIMEOUT` | `EthereumJSONRPC.RequestCoordinator[:wait_per_timeout]` | `20s` |
| `ETHEREUM_JSONRPC_WS_RETRY_INTERVAL` | `EthereumJSONRPC.WebSocket.RetryWorker[:retry_interval]` | `1m` |
| `ETHEREUM_JSONRPC_THROTTLE_MAX_INFLIGHT` | `EthereumJSONRPC.HTTP.Throttle[:max_inflight]` | `32` |
| `ETHEREUM_JSONRPC_THROTTLE_RPS` | `EthereumJSONRPC.HTTP.Throttle[:rps]` | `200` |
| `ETHEREUM_JSONRPC_THROTTLE_TRACE_MAX_INFLIGHT` | `EthereumJSONRPC.HTTP.Throttle[:trace_max_inflight]` | `4` |
| `ETHEREUM_JSONRPC_THROTTLE_ARCHIVE_MAX_INFLIGHT` | `EthereumJSONRPC.HTTP.Throttle[:archive_max_inflight]` | `8` |
| `ETHEREUM_JSONRPC_THROTTLE_COALESCE_ENABLED` | `EthereumJSONRPC.HTTP.Throttle[:coalesce?]` | `true` |

These guards bound concurrent HTTP work and request rate, with separate trace
and archive limits and optional request coalescing. Endpoint availability
checking is enabled by runtime configuration. Historical balance reads are
disabled when both trace URL variables are absent, or when
`ETHEREUM_JSONRPC_DISABLE_ARCHIVE_BALANCES=true`; the
`ETHEREUM_JSONRPC_ARCHIVE_BALANCES_WINDOW` default is 200 blocks. Receipt
batching is opt-in via `ETHEREUM_JSONRPC_RECEIPTS_BY_BLOCK` (false), with
`ETHEREUM_JSONRPC_MAX_RECEIPTS_BY_BLOCK` defaulting to 1000.

### Paxeer variant

[EthereumJSONRPC.PaxeerX](lib/ethereum_jsonrpc/paxeer_x.ex) delegates tracing,
pending transactions and beneficiaries to the Geth implementation and handles
the Paxeer wire differences:

- Cosmos-originated transaction type `0xffffffff` maps to stored type `0x7f`
  by default. `PAXEER_X_COSMOS_TRANSACTION_TYPE` configures the stored type.
- `receiptsRoot` is the previous block's CometBFT `LastResultsHash`, not a
  receipt trie checksum or a unique block identifier.
- `fetch_block_by_new_head/2` fetches the announced block body, checks number,
  hash, parent hash and receipts root, and refetches by number on mismatch.
- `fetch_syncing_status/1` maps `eth_syncing: false` to `{:ok, :synced}`,
  decodes progress heights, and preserves RPC and transport errors. An
  unexpected successful payload returns `{:error, {:unexpected_syncing_result, payload}}`.
- EVM balances and transaction values use 18 decimals. Bank `uhpx` uses six;
  bank-to-EVM conversion multiplies by `10^12`. Imported EVM balances need no
  rescaling. `PAXEER_X_NATIVE_COIN_DECIMALS` defaults to 18.

## Testing

The existing unit harness defaults to `EthereumJSONRPC.Case.Nethermind.Mox`
and `EthereumJSONRPC.WebSocket.Case.Mox`. From this application directory,
the explicit Nethermind selection is:

```sh
ETHEREUM_JSONRPC_CASE=EthereumJSONRPC.Case.Nethermind.Mox \
ETHEREUM_JSONRPC_WEB_SOCKET_CASE=EthereumJSONRPC.WebSocket.Case.Mox \
mix test --exclude no_nethermind
```

The existing Geth unit selection is:

```sh
ETHEREUM_JSONRPC_CASE=EthereumJSONRPC.Case.Geth.Mox \
ETHEREUM_JSONRPC_WEB_SOCKET_CASE=EthereumJSONRPC.WebSocket.Case.Mox \
mix test --exclude no_geth
```

These are transport-isolated unit profiles. Other-client profiles are optional
compatibility checks, not the Paxeer production variant. For integration,
configure a disposable node explicitly; production URL variables do not
configure the test case modules. In test configuration, for example:

```elixir
config :ethereum_jsonrpc, EthereumJSONRPC.Case.Geth.HTTPWebSocket,
  http_url: "http://127.0.0.1:8545/",
  websocket_url: "ws://127.0.0.1:8546/",
  profile: :paxeer_x

config :ethereum_jsonrpc, EthereumJSONRPC.WebSocket.Case.Geth,
  http_url: "http://127.0.0.1:8545/",
  web_socket_url: "ws://127.0.0.1:8546/",
  profile: :paxeer_x
```

The corresponding Nethermind modules accept the same endpoint keys. The
HTTPWebSocket profiles default to Paxeer chain 125; another chain requires
`profile: :geth_compatibility` or `:nethermind_compatibility`, respectively,
and its explicit positive `chain_id`. The standalone WebSocket case modules
require `profile: :paxeer_x`, or `profile: :compatibility` plus an explicit
positive `chain_id`. Both transports validate chain identity; the standalone
WebSocket cases also compare the same block over HTTP and WebSocket. Keep
HTTP and WebSocket endpoints on the same node host and supply no embedded
credentials.

The documentation repair gate needs no supplied node or credentials. From the
repository root, the bounded task runner uses `hf-build` then `hf-done`; the
latter runs this focused test through the builder:

```sh
services/explorer/deploy/tools/mix-in-builder.sh test apps/ethereum_jsonrpc/test/repair/repair_22_33_445_test.exs
```

The test executes the named-argument example, checks it against the active
production profile and runtime defaults, and uses a loopback HTTP/WebSocket
peer replaying committed Paxeer chain data to deliver a head and fetch its
block through the real transports and Paxeer decoder.

## Installation

The OTP application can be used by another application in this backend
umbrella by adding it to `mix.exs`:

```elixir
def deps do
  [
    {:ethereum_jsonrpc, in_umbrella: true}
  ]
end
```
