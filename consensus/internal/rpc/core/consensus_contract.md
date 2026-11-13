# Consensus diagnostic contract

This contract describes the internal consensus diagnostic interface maintained
with [the OpenAPI document](../../../rpc/openapi/openapi.yaml). Its producers are
[`Environment`](env.go) and [`NewRoutesMap`](routes.go); request and result types
live in [`coretypes`](../../../rpc/coretypes/). The interface distinguishes
Autobahn from the classic consensus path.
A legacy environment can still serve CometBFT data when its corresponding
stores, consensus state, reactor and evidence pool are configured.

## RPC boundary

[`DefaultRPCConfig`](../../../config/config.go) selects a loopback TCP listener
on port 26657. CORS allowed origins are empty by default. Operators
configure `[rpc].laddr` and the CORS lists explicitly when exposing diagnostics.
The OpenAPI server entry is this local diagnostic listener.

The public network has EVM chain ID 125 (`0x7d`). Its public JSON-RPC uses the
root path on the configured network endpoint.
The [public endpoint and namespace contract](../../../../docs/site/docs/reference/public-rpc.md#method-contracts)
maintains the node inventory. The public node namespaces are `debug`, `echo`,
`eth`, `net`, `pax`, `test` and `txpool`, subject to node configuration. Hosted
`px_` methods, including `px_register` and `px_requestFunds`, are declared in
[the gateway OpenRPC contract](../../../../services/platform/hosted/gateway/openrpc.json).
Public node availability does not establish hosted gateway availability.
The diagnostic OpenAPI schema does not describe `eth_` or `px_` methods.

## Names and encodings

Methods such as `block`, `header`, `block_results`, `commit`, `consensus_state`,
`dump_consensus_state` and `broadcast_evidence` are registered in `NewRoutesMap`.
URI GET uses `/<method>` and returns a bare result or `RPCError`. Root POST
accepts JSON-RPC 2.0 and returns `jsonrpc`, the request `id`, and `result` or
`error`. Batches return arrays; notifications have no response body. WebSocket
subscriptions use the internal `/websocket` endpoint.

Heights and 64-bit integer result fields use decimal strings where indicated
by their Go JSON tags and the OpenAPI schemas. Hashes use hexadecimal strings;
byte payloads such as transactions and ABCI values use base64. Consult each
schema for its specific encoding and nullability. `block` returns `block_id`
and `block`; `header` wraps its value under `header`; `block_results` exposes
`height`, `txs_results`, `total_gas_used` and execution events. Nonpositive,
future or pruned requested heights return errors. An unknown block/header hash
returns a null object field. In the classic path, a valid height with no block
metadata also returns a null block or header field.

## Autobahn refusals

| Route                  | In-process sentinel                       | JSON-RPC error `data`                                             |
| ---------------------- | ----------------------------------------- | ----------------------------------------------------------------- |
| `commit`               | `coretypes.ErrCommitNotAvailable`         | `commit is not available under autobahn consensus`                |
| `consensus_state`      | `coretypes.ErrConsensusStateNotAvailable` | `consensus round state is not available under autobahn consensus` |
| `dump_consensus_state` | `coretypes.ErrConsensusStateNotAvailable` | `consensus round state is not available under autobahn consensus` |
| `broadcast_evidence`   | `coretypes.ErrEvidenceNotAccepted`        | `evidence is not accepted under autobahn consensus`               |

After parameter decoding, these handlers refuse before accessing CometBFT
components. Even valid CometBFT evidence is refused. In-process callers can use
`errors.Is`. Across HTTP the numeric `RPCError.code` is `-32603`, `message` is
`Internal error`, and `data` is the exact diagnostic string above. HTTP status
is 200. URI GET returns the bare error object; root POST returns an `error`
envelope with the echoed request id and no `result`. The wire does not carry Go
sentinel identity. Malformed parameters can fail before the handler is called.

The legacy success schemas apply only to configured CometBFT components:
`commit` reads the BlockStore, `consensus_state` requires consensus state,
`dump_consensus_state` also requires the reactor, and `broadcast_evidence`
validates evidence before submitting it to the evidence pool. At the current
classic height, a matching seen commit has `canonical` set to false. A commit
loaded through `LoadBlockCommit` has `canonical` set to true. A missing classic
commit may be null. These conditions do not
provide a mode switch on an Autobahn node.

## Commitment sources

Autobahn's `GigaRouter` serves retained finalized block metadata through `block`
and `header`. Its translated blocks carry an empty `LastCommit` placeholder with no
signatures and do not supply Tendermint signed precommit proofs. `block_results` serves persisted ABCI
FinalizeBlock execution results, not a consensus certificate. Autobahn commit
QCs are internal consensus state; this diagnostic schema exposes no replacement
commitment-proof route. A light client must not treat these metadata or result
routes as an independently verifiable commitment source. Tendermint commitments
are available from `commit` only in the configured legacy mode described above.

## State recovery

`GigaRouter` uses [`data.State`](../../autobahn/data/state.go) for retained
global blocks and the block-hash index, rather than the CometBFT BlockStore.
The JSON `persistent_state_dir` setting in
[`AutobahnFileConfig`](../../../config/autobahn.go) supplies
`Consensus.PersistentStateDir`; relative paths resolve against the node home.
Normal configurations require a nonblank directory. In-memory operation is
available only through `unsafe_test_only_disable_persistence`, which is mutually
exclusive with a configured directory and provides no restart recovery.
The data WAL stores `globalblocks` and `fullcommitqcs` alongside the consensus
persistence. Startup reconciles their retained ranges, restores certificates
and blocks, and rebuilds the hash index. The retained block range is served only
after reconciliation and validation.

Execution is a separate durable path: `GigaRouter` saves each FinalizeBlock
response to the configured `StateStore` before application Commit.
`Environment.BlockResults` reads that store at the requested application height.
Reopening the same persistent store preserves these responses; an application
reporting that height and an active router are still needed to resolve requests.
Opening a new in-memory router does not restore its old block/hash history.
The application, consensus/data WALs and result store must all be retained for
node recovery; preserving only one is not a complete node backup. Application
`RetainHeight` prunes both retained blocks and execution responses, so recovery
does not promise archival access to pruned heights.
