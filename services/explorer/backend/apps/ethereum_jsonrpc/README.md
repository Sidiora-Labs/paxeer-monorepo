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

Set the Paxeer chain identity and variant for the backend:

```sh
export CHAIN_TYPE=paxeer_x
export ETHEREUM_JSONRPC_VARIANT=paxeer_x
export ETHEREUM_JSONRPC_TRANSPORT=http
export ETHEREUM_JSONRPC_HTTP_URLS=https://api1.mainnet-beta.paxeer.network/,https://api2.mainnet-beta.paxeer.network/
```

The production path is [config/runtime/prod.exs](../../config/runtime/prod.exs)
loading [explorer/config/prod/paxeer_x.exs](../explorer/config/prod/paxeer_x.exs).
It sets `config :explorer, json_rpc_named_arguments: [...]` with
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
the default JSON content type. The obsolete top-level `url`, `trace_url` and
`http` configuration example does not configure this production call path.

Subscriptions use the separate `subscribe_named_arguments` entry with
`transport: EthereumJSONRPC.WebSocket`, `web_socket:
EthereumJSONRPC.WebSocket.WebSocketClient`, `url` from
`ETHEREUM_JSONRPC_WS_URL`, and `fallback_url` from
`ETHEREUM_JSONRPC_FALLBACK_WS_URL`. Set those to WebSocket endpoints provided
by your node deployment. Keep request transport as `http` when using
subscriptions: runtime transport selection accepts only `http` or `ipc`.
WebSocket URLs in HTTP URL lists are rejected. IPC uses `IPC_PATH`.

For an explicit library call, pass named arguments directly:

```elixir
json_rpc_named_arguments = [
  transport: EthereumJSONRPC.HTTP,
  transport_options: [
    http: EthereumJSONRPC.HTTP.Tesla,
    http_options: [recv_timeout: 60_000, timeout: 60_000, pool: :ethereum_jsonrpc],
    urls: ["http://localhost:8545"]
  ],
  variant: EthereumJSONRPC.PaxeerX
]

case EthereumJSONRPC.fetch_chain_id(json_rpc_named_arguments) do
  {:ok, 125} -> :ok
  {:ok, other} -> {:error, {:unexpected_chain_id, other}}
  {:error, reason} -> {:error, reason}
end
```

Here `localhost:8545` means a locally supplied Paxeer node. The helper returns
the decoded integer chain ID. Production consumers read the configured
`:explorer` named arguments instead of building a second configuration.

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
- `fetch_syncing_status/1` maps an errored `eth_syncing` call to
  `{:ok, :synced}`. This includes transport errors, so it is not an endpoint
  health or consensus catch-up check.
- EVM balances and transaction values use 18 decimals. Bank `uhpx` uses six;
  bank-to-EVM conversion multiplies by `10^12`. Imported EVM balances need no
  rescaling. `PAXEER_X_NATIVE_COIN_DECIMALS` defaults to 18.

HTTP concurrency and rate settings are under
`EthereumJSONRPC.HTTP.Throttle` in [runtime.exs](../../config/runtime.exs):
`ETHEREUM_JSONRPC_THROTTLE_MAX_INFLIGHT` (32),
`ETHEREUM_JSONRPC_THROTTLE_RPS` (200),
`ETHEREUM_JSONRPC_THROTTLE_TRACE_MAX_INFLIGHT` (4),
`ETHEREUM_JSONRPC_THROTTLE_ARCHIVE_MAX_INFLIGHT` (8), and
`ETHEREUM_JSONRPC_THROTTLE_COALESCE_ENABLED` (true).

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

These are transport-isolated unit tests. The legacy Geth and Nethermind
`HTTPWebSocket` case modules embed external provider addresses in their
source, including the WebSocket case modules; setting production URL variables
does not redirect them to a local node. Do not use those cases as a local
Paxeer integration check. For integration, use the real configured transport
against a controlled Paxeer node, check chain ID 125, and exercise the
requested methods and subscriptions with that node's actual capabilities.

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
