---
order: 1
parent:
  title: RPC
  order: 6
---

# RPC spec

This page documents the consensus JSON-RPC service implemented in this tree.
[`internal/rpc/core/routes.go`](../../internal/rpc/core/routes.go) registers the
methods; [`requests.go`](../../rpc/coretypes/requests.go) and
[`responses.go`](../../rpc/coretypes/responses.go) define their wire fields.
These local consensus methods are distinct from the public Ethereum-compatible
node API.

Paxeer mainnet-beta is one BFT network, with the kernel participating in block
proposal. Its EVM chain ID is 125 (`0x7d`), and a receipt is final once its block
is committed. The public JSON-RPC endpoints are HTTPS at the **root path** of
`api1.mainnet-beta.paxeer.network` through `api14.mainnet-beta.paxeer.network` and
`api-mainnet-beta.paxeer.network`. They expose the node namespaces `debug`,
`echo`, `eth`, `net`, `pax`, `test` and `txpool`. For example:

```sh
curl -sS 'https://api1.mainnet-beta.paxeer.network/' \
  -H 'Content-Type: application/json' \
  --data-binary '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

The chain ID result is `"0x7d"`. Hosted `px_` methods have their own declared
contract in
[`services/platform/hosted/gateway/openrpc.json`](../../../services/platform/hosted/gateway/openrpc.json).
Sub-chains are tenant domains in the database. They do not require a new
consensus listener per domain.

See [Peers](../p2p/peer.md), [classic consensus messages](../p2p/messages/consensus.md)
and [state-sync messages](../p2p/messages/state-sync.md) for the underlying peer
protocols.

## Support

The local listener defaults to `tcp://127.0.0.1:26657` via `[rpc] laddr` in
[`config/config.go`](../../config/config.go). The examples below use ordinary
HTTP on that listener. HTTPS requires both `tls-cert-file` and `tls-key-file` to
be configured; changing the URL scheme alone does not enable TLS.

| Transport | Local consensus service |
| --- | --- |
| JSON-RPC 2.0 over HTTP | POST to `http://localhost:26657/` |
| HTTP URI requests | GET a registered method path, such as `/status` |
| WebSocket | `ws://localhost:26657/websocket`, unless `experimental-disable-websocket` is enabled |

`subscribe`, `unsubscribe` and `unsubscribe_all` are WebSocket-only. The root
handler accepts batches of at most ten requests. Requests without an ID are
notifications and are ignored. An HTTP URI request has response ID `-1`; a
JSON-RPC request receives its own ID back.

Use named parameter objects. Request `Int64` fields accept either decimal JSON
numbers or decimal strings. Boolean fields use JSON booleans. Consensus hashes
and ABCI query `data` use hex strings **without** a `0x` prefix in JSON; transaction
bytes (`tx`) use base64. Do not copy Ethereum quantity encoding into consensus
RPC parameters. Result integer encodings vary by field; the response types,
rather than a rule that every number is a string, define the format.

For shell requests, single-quote literal JSON and quote URLs containing `&`.
Use `curl --get --data-urlencode` for URI string parameters that need escaping.
Examples referring to a height, hash, query path or transaction require data
available to the selected node; syntactic validity does not imply that the
application accepts a particular query or transaction.

### Consensus mode and errors

The same route names are registered for classic consensus and Autobahn. Under
Autobahn, block and header queries read finalized global blocks, and
`block_results` reads stored `FinalizeBlock` responses. Methods that require
classic commits or round state return explicit errors:

| Method | Autobahn error data |
| --- | --- |
| `commit` | `commit is not available under autobahn consensus` |
| `consensus_state`, `dump_consensus_state` | `consensus round state is not available under autobahn consensus` |
| `broadcast_evidence` | `evidence is not accepted under autobahn consensus` |

These errors are currently mapped to JSON-RPC code `-32603`, with the message in
`error.data`. They do not indicate that a committed Autobahn block lacks
finality. For example, an Autobahn node answers a `commit` request with:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32603,
    "message": "Internal error",
    "data": "commit is not available under autobahn consensus"
  }
}
```

The JSON-RPC error codes in
[`types.go`](../../rpc/jsonrpc/types/types.go) are:

| Code | Meaning |
| --- | --- |
| -32700 | Parse error |
| -32600 | Invalid request, including mapped height/page/request validation errors |
| -32601 | Method not found or unavailable on the selected transport |
| -32602 | Invalid params |
| -32603 | Internal error; also the current mapping for other service errors |
| -32604 | Lag is too high |

A service error is an `error` object, not an empty string. Application-level
failure can instead appear as a nonzero `result.code` or `result.response.code`.
Inspect both layers. A pruned height and a height beyond the head are distinct
service errors; do not treat every `-32603` as an unknown block. For
`block_by_hash` and `header_by_hash`, an unknown hash returns a result with a null
block or header rather than a fabricated historical record.

## Timestamps

RPC timestamps use RFC3339Nano. Trailing fractional-second zeros are removed:
`1985-04-12T23:20:50.5200000Z` is returned as
`1985-04-12T23:20:50.52Z`.

## Info Routes

### Health

`health` takes no parameters and returns `{}` when the service is responsive.
This is a heartbeat, not a claim that the node has caught up.

```sh
curl -sS 'http://localhost:26657/health'
curl -sS 'http://localhost:26657/' -H 'Content-Type: application/json' \
  --data-binary '{"jsonrpc":"2.0","id":1,"method":"health","params":{}}'
```

The POST response is:

```json
{"jsonrpc":"2.0","id":1,"result":{}}
```

### Status

`status` takes no parameters. It returns `node_info`, `application_info`,
`sync_info`, `validator_info` and `light_client_info`. `node_info` describes
identity, the network string and protocol versions. `sync_info` includes the
latest and earliest retained heights, hashes and times, peer height and
`catching_up`.

`sync_info.last_committed_block_height` is the consensus-finalized height;
`latest_block_height` is the application-applied height. Under classic consensus
they coincide. Under Autobahn the committed height can briefly lead the applied
height while the application catches up within the same node.

```sh
curl -sS 'http://localhost:26657/status'
curl -sS 'http://localhost:26657/' -H 'Content-Type: application/json' \
  --data-binary '{"jsonrpc":"2.0","id":1,"method":"status","params":{}}'
```

The consensus `node_info.network` value comes from genesis; it is not the
Ethereum hex chain ID returned by `eth_chainId`.

### NetInfo

`net_info` takes no parameters. The result includes `listening`, `listeners`,
`n_peers`, `peers` (with `node_id` and `url`) and `peer_connections` (with
`node_id`, `state` and `score`).

```sh
curl -sS 'http://localhost:26657/net_info'
```

### Blockchain

`blockchain` accepts `minHeight` and `maxHeight` and returns `last_height` and
`block_metas` in descending height order, at most 20 entries. Zero or omitted
bounds use defaults; the range is bounded by retained data and the current
head. Autobahn may omit pruned entries within the requested range.

```sh
curl -sS 'http://localhost:26657/blockchain?minHeight=1&maxHeight=2'
curl -sS 'http://localhost:26657/' -H 'Content-Type: application/json' \
  --data-binary '{"jsonrpc":"2.0","id":1,"method":"blockchain","params":{"minHeight":"1","maxHeight":"2"}}'
```

### Header

`header` accepts optional `height`; omission selects the latest available height.
The result wraps the header in `header`. Explicit heights must be positive,
retained and no greater than the available head.

```sh
curl -sS 'http://localhost:26657/header'
curl -sS 'http://localhost:26657/' -H 'Content-Type: application/json' \
  --data-binary '{"jsonrpc":"2.0","id":1,"method":"header","params":{"height":"1"}}'
```

### HeaderByHash

`header_by_hash` accepts `hash` and returns `header`. Use the hash of a block
available on this node; the illustrative hash below may return a null header.

```sh
curl -sS 'http://localhost:26657/' -H 'Content-Type: application/json' \
  --data-binary '{"jsonrpc":"2.0","id":1,"method":"header_by_hash","params":{"hash":"D70952032620CC4E2737EB8AC379806359D8E0B17B0488F627997A0B043ABDED"}}'
```

### Block

`block` accepts optional `height` and returns `block_id` and `block`. Omission
selects the latest available block. Under Autobahn the block comes from finalized
global block data, not the classic block store.

```sh
curl -sS 'http://localhost:26657/block'
curl -sS 'http://localhost:26657/' -H 'Content-Type: application/json' \
  --data-binary '{"jsonrpc":"2.0","id":1,"method":"block","params":{"height":"1"}}'
```

### BlockByHash

`block_by_hash` accepts `hash` and returns the same wrapper as `block`. Unknown
hashes have `block: null`. Autobahn's hash lookup uses a temporary in-memory
index, so it is not a promise of archival hash lookup across restarts.

```sh
curl -sS 'http://localhost:26657/' -H 'Content-Type: application/json' \
  --data-binary '{"jsonrpc":"2.0","id":1,"method":"block_by_hash","params":{"hash":"D70952032620CC4E2737EB8AC379806359D8E0B17B0488F627997A0B043ABDED"}}'
```

### BlockResults

`block_results` accepts optional `height`. Its result contains `height`,
`txs_results`, `total_gas_used`, `finalize_block_events`, `validator_updates` and
`consensus_param_updates`. The events come from `FinalizeBlock`; the response
does not have the old begin-block/end-block event fields. If a stored response
is unavailable the method returns an error rather than inventing empty results.

```sh
curl -sS 'http://localhost:26657/block_results'
curl -sS 'http://localhost:26657/' -H 'Content-Type: application/json' \
  --data-binary '{"jsonrpc":"2.0","id":1,"method":"block_results","params":{"height":"1"}}'
```

### Commit

`commit` accepts optional `height`. With classic consensus it returns
`signed_header` and `canonical`. The latest height may use the locally seen
commit with `canonical: false`; a canonical commit comes from the next block.
Under Autobahn this method returns the explicit error described in
[Consensus mode and errors](#consensus-mode-and-errors).

```sh
curl -sS 'http://localhost:26657/commit'
```

### Validators

`validators` accepts optional `height`, `page` and `per_page`. Results contain
`block_height`, `validators`, `count` and `total`. Pagination starts at page 1;
the default page size is 30 and normal safe mode caps it at 100. Under Autobahn
it returns the fixed genesis committee for the validated retained height, with
proposer priorities zero. Classic mode loads the height's validator set from
state.

```sh
curl -sS 'http://localhost:26657/validators?page=1&per_page=20'
curl -sS 'http://localhost:26657/' -H 'Content-Type: application/json' \
  --data-binary '{"jsonrpc":"2.0","id":1,"method":"validators","params":{"page":1,"per_page":20}}'
```

### Genesis

`genesis` takes no parameters and returns `genesis`, the node's genesis document.
For a document spanning more than one genesis chunk, use `genesis_chunked`.

```sh
curl -sS 'http://localhost:26657/genesis'
```

### GenesisChunked

`genesis_chunked` takes `chunk`, a zero-based integer index. The result has
`chunk` and `total` encoded as decimal strings, and `data` containing base64
bytes. Decode and concatenate the chunks in index order to recover the genesis
JSON document. These are RPC document chunks, not state-sync snapshot chunks.

```sh
curl -sS 'http://localhost:26657/genesis_chunked?chunk=0'
curl -sS 'http://localhost:26657/' -H 'Content-Type: application/json' \
  --data-binary '{"jsonrpc":"2.0","id":1,"method":"genesis_chunked","params":{"chunk":0}}'
```

### ConsensusParams

`consensus_params` accepts optional `height` and returns `block_height` and
`consensus_params`. Classic mode loads the height's parameters from state.
Autobahn returns the genesis parameters for a validated retained height.
Synchrony and timeout defaults are populated by the service.

```sh
curl -sS 'http://localhost:26657/consensus_params'
```

### UnconfirmedTxs

`unconfirmed_txs` accepts `page` and `per_page`, with the same pagination defaults
as `validators`. There is no `limit` request field. The result has `n_txs` (the
returned page's count), `total`, `total_bytes` and `txs` (base64 transaction
bytes). The first three fields are decimal strings. The mempool can change
between requests, so pagination is not a stable snapshot across calls.

```sh
curl -sS 'http://localhost:26657/unconfirmed_txs?page=1&per_page=20'
curl -sS 'http://localhost:26657/' -H 'Content-Type: application/json' \
  --data-binary '{"jsonrpc":"2.0","id":1,"method":"unconfirmed_txs","params":{"page":1,"per_page":20}}'
```

### NumUnconfirmedTxs

`num_unconfirmed_txs` takes no parameters. It returns `n_txs`, `total` and
`total_bytes` for the whole mempool; it does not return transaction contents.

```sh
curl -sS 'http://localhost:26657/num_unconfirmed_txs'
```

### Tx

`tx` accepts a hex `hash` and boolean `prove`. It requires the KV event sink.
Results contain `hash`, `height`, `index`, `tx_result`, `tx` and an optional
inclusion `proof`. Proof construction also requires the corresponding block to
remain available. A missing indexed transaction produces an error.

```sh
curl -sS 'http://localhost:26657/' -H 'Content-Type: application/json' \
  --data-binary '{"jsonrpc":"2.0","id":1,"method":"tx","params":{"hash":"D70952032620CC4E2737EB8AC379806359D8E0B17B0488F627997A0B043ABDED","prove":false}}'
```

## Transaction Routes

All transaction submission methods take `tx`, the base64 representation of actual
application transaction bytes in JSON. Obtain a signed transaction from the
application's transaction tooling; base64 does not sign or validate it. A
successful mempool admission is not yet a committed receipt.

### BroadCastTxSync

`broadcast_tx` is the current synchronous admission method;
`broadcast_tx_sync` is a deprecated alias. It waits for mempool validation and
returns numeric `code`, hex `data`, `log`, `codespace` and hex `hash`. It does not
wait for execution in a block. Under Autobahn admission goes through its mempool;
classic mode uses `CheckTx`.

Given a file `signed-tx.json` containing `{"tx":"..."}` with the actual base64
transaction substituted, construct and send a request with:

```sh
jq '{jsonrpc:"2.0",id:1,method:"broadcast_tx",params:.}' signed-tx.json |
  curl -sS 'http://localhost:26657/' -H 'Content-Type: application/json' \
    --data-binary @-
```

### BroadCastTxAsync

`broadcast_tx_async` is deprecated. It schedules admission and returns the
transaction hash immediately; it does not wait for validation or execution.
The result uses the `ResultBroadcastTx` shape, but its zero-valued admission
fields do not certify success.

### BroadcastTxCommit

`broadcast_tx_commit` is deprecated but still registered. It waits for admission
and, on acceptance, polls the indexed transaction until execution is available
or the configured timeout expires. It requires the KV event sink for commit
confirmation. Its result includes `check_tx`, `tx_result`, `hash` and `height`.
A nonzero `check_tx.code` returns without waiting for a commit.

### CheckTx

`check_tx` validates a transaction through the application without adding it to
the mempool. It returns the application's check result directly under `result`,
including `code`, `data`, `log`, `gasWanted`, `gasEstimated`, `priority` and
`codespace`. The gas and priority values are decimal strings; `data` is base64
and `code` is numeric. Use the request construction above with method `check_tx`
and actual transaction bytes.

## ABCI Routes

### ABCIInfo

`abci_info` takes no parameters. It returns the application's ABCI `Info`
response under `response`, including its version and last applied block data.

```sh
curl -sS 'http://localhost:26657/abci_info'
curl -sS 'http://localhost:26657/' -H 'Content-Type: application/json' \
  --data-binary '{"jsonrpc":"2.0","id":1,"method":"abci_info","params":{}}'
```

### ABCIQuery

`abci_query` forwards these parameters to the application:

| Parameter | Type | Meaning |
| --- | --- | --- |
| path | string | Application-defined query route |
| data | hex string | Query bytes, separate from the path |
| height | decimal integer or string | Application query height; zero uses application semantics |
| prove | boolean | Request application state proof operations |

The service does not interpret application paths or verify the returned proof
for the caller. This is a state-query proof request, not a transaction inclusion
proof. The following examples encode the same query: replace `a/b/c` and
`DEADBEEF` with a route and query bytes supported by the application.

```sh
curl -sS --get 'http://localhost:26657/abci_query' \
  --data-urlencode 'path="a/b/c"' \
  --data-urlencode 'data=DEADBEEF' \
  --data-urlencode 'height=1' \
  --data-urlencode 'prove=true'

curl -sS 'http://localhost:26657/' -H 'Content-Type: application/json' \
  --data-binary '{"jsonrpc":"2.0","id":1,"method":"abci_query","params":{"path":"a/b/c","data":"DEADBEEF","height":"1","prove":true}}'
```

The result wraps `ResponseQuery` in `response`. It contains numeric `code`,
`log`, `info`, string-encoded `index` and `height`, base64 `key` and `value`,
`proofOps` (null when absent), and `codespace`. The nested ABCI response uses
protobuf JSON field names, including camel-case `proofOps`. Application failures can therefore be a
successful JSON-RPC response with nonzero `response.code`; inspect that field.

## Evidence Routes

### BroadcastEvidence

`broadcast_evidence` accepts `evidence` as a registered typed JSON evidence
object, not an arbitrary string containing encoded JSON. The decoder and
registered evidence types are in
[`internal/jsontypes`](../../internal/jsontypes) and
[`types/evidence.go`](../../types/evidence.go). Classic mode validates the
object and adds it to the evidence pool, returning a base64 `hash` on success.
Autobahn rejects classic evidence with the explicit error described above.

Given `evidence.json` containing a real typed evidence object for a classic node,
the request can be constructed without shell-escaping its nested JSON:

```sh
jq '{jsonrpc:"2.0",id:1,method:"broadcast_evidence",params:{evidence:.}}' evidence.json |
  curl -sS 'http://localhost:26657/' -H 'Content-Type: application/json' \
    --data-binary @-
```

## Additional registered routes

The registration map also exposes the following methods. Request and response
fields are defined by the linked core types at the top of this page.

| Method | Parameters | Result or restriction |
| --- | --- | --- |
| `lag_status` | None | `current_height`, `max_peer_height`, `lag`; excessive lag uses error code -32604 |
| `tx_search` | `query`, `prove`, `page`, `per_page`, `order_by` | Indexed `txs` and `total_count`; requires KV event sink |
| `block_search` | `query`, `page`, `per_page`, `order_by` | Indexed `blocks` and `total_count`; requires KV event sink |
| `consensus_state` | None | Classic `round_state`; unavailable under Autobahn |
| `dump_consensus_state` | None | Classic `round_state` and `peers`; unavailable under Autobahn |
| `events` | `filter`, `maxItems`, `after`, `before`, `waitTime` | Cursor-based event log, when enabled |
| `subscribe` | `query` | WebSocket-only event subscription |
| `unsubscribe` | `query` | WebSocket-only subscription cancellation |
| `unsubscribe_all` | None | WebSocket-only cancellation of all this connection's subscriptions |

`unsafe_flush_mempool` is only registered when unsafe routes are enabled and the
service implements `RPCUnsafe`. Search ordering accepts `asc`, `desc` or empty
(default descending). Event-log retention and availability depend on the local
RPC configuration; they are not a durable history guarantee.
