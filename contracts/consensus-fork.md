# Consensus fork contract

Paxeer mainnet-beta is one BFT network, operated by `paxd`, with EVM chain ID
125 (`0x7d`). The kernel belongs to the network and proposes blocks. A receipt
is final when its block is committed. The node's execution, receipt and
sequencing contracts are documented in [sequencer.md](sequencer.md).

This page describes the consensus implementation under [`consensus/`](../consensus).
Its Go import prefix is
`github.com/Sidiora-Labs/Paxeer-X-Network/consensus`; upstream Tendermint names
in packages and protocol messages are retained identifiers.

## Go API

The following source files define the producer and consumer boundaries:

| Surface | Authoritative declaration and behavior |
| --- | --- |
| Application | [`abci/types/application.go`](../consensus/abci/types/application.go): `Application`, implemented by the node application, defines initialization, validation, proposal processing, finalization, commitment and snapshots. See [node ABCI](sequencer.md#node-abci). |
| Blocks, validators and events | [`types/`](../consensus/types): Go block, validator, vote, transaction and event types; use their validation and conversion methods when crossing the wire boundary. |
| Node construction | [`node/`](../consensus/node): node setup and lifecycle; configuration is defined by [`config/config.go`](../consensus/config/config.go). |
| Sequenced proposals | [`internal/state/execution.go`](../consensus/internal/state/execution.go): `BlockExecutor.SetSequencerSource` installs the source used by `CreateProposalBlock` at sequenced heights. It admits transactions, seals their ordered slice and includes the sequence batch. |
| Consensus RPC | [`internal/rpc/core/routes.go`](../consensus/internal/rpc/core/routes.go): `RPCService` and `NewRoutesMap`; request and response types live in [`rpc/coretypes/responses.go`](../consensus/rpc/coretypes/responses.go). |
| RPC client | [`rpc/client/http/http.go`](../consensus/rpc/client/http/http.go): the HTTP client sends the declared consensus methods; it is separate from an Ethereum JSON-RPC client. |
| Autobahn data | [`autobahn/types/`](../consensus/autobahn/types): application proposals, quorum certificates and global block types used by the Autobahn implementation. |

Go paths containing `internal` identify implementation owners and are subject
to Go's internal-package import restrictions. They are not an additional public
network interface.

## RPC routes

Public Ethereum JSON-RPC is served at the root path of the node hosts
`api1.mainnet-beta.paxeer.network` through
`api14.mainnet-beta.paxeer.network`, and
`api-mainnet-beta.paxeer.network`, over HTTPS. `eth_chainId` returns `0x7d`.
The node namespaces are `debug`, `echo`, `eth`, `net`, `pax`, `px`, `test` and `txpool`.
The `px` namespace answers from node state; `px_register` and
`px_requestFunds` are operations of the hosted identity and faucet
services declared in [the gateway OpenRPC contract](../services/platform/hosted/gateway/openrpc.json).
`node.hyperpaxeer.com` is a library mirror, never an RPC endpoint.

The consensus RPC listener is configured separately in `config.toml` under
`[rpc]`. `DefaultRPCConfig` binds `tcp://127.0.0.1:26657`; configure `laddr`
explicitly for another deployment. JSON-RPC requests are POSTed to this
listener's root path. Named methods also have URI handlers. The WebSocket
handler is `/websocket`; `subscribe`, `unsubscribe` and `unsubscribe_all`
are WebSocket-only methods.

[`NewRoutesMap`](../consensus/internal/rpc/core/routes.go) registers:

| Group | Methods |
| --- | --- |
| Events | `events`, `subscribe`, `unsubscribe`, `unsubscribe_all` |
| Node information | `health`, `status`, `lag_status`, `net_info`, `genesis`, `genesis_chunked` |
| Blocks | `blockchain`, `header`, `header_by_hash`, `block`, `block_by_hash`, `block_results`, `commit`, `block_search` |
| Transactions | `check_tx`, `tx`, `tx_search`, `unconfirmed_txs`, `num_unconfirmed_txs`, `broadcast_tx` |
| Consensus | `validators`, `dump_consensus_state`, `consensus_state`, `consensus_params` |
| Application and evidence | `abci_query`, `abci_info`, `broadcast_evidence` |

The compatibility routes `broadcast_tx_commit`, `broadcast_tx_sync` and
`broadcast_tx_async` remain registered but are deprecated. In particular,
`broadcast_tx` reports admission through CheckTx; a successful response is not
proof of block commitment. `unsafe_flush_mempool` is registered only when the
service implements `RPCUnsafe` and `RouteOptions.Unsafe` is true.

JSON-RPC error codes are declared in
[`rpc/jsonrpc/types/types.go`](../consensus/rpc/jsonrpc/types/types.go):

| Code | Meaning |
| --- | --- |
| -32700 | Parse error |
| -32600 | Invalid request |
| -32601 | Method not found |
| -32602 | Invalid parameters |
| -32603 | Internal error |
| -32604 | Excessive lag |

These transport codes differ from application transaction result codes.
Callers must inspect the returned error or transaction result instead of
treating HTTP success as execution success.

## RPC Autobahn

When Autobahn is installed, the consensus implementation serves committed
global blocks through the internal `GigaRouter` type in
[`giga_router.go`](../consensus/internal/p2p/giga_router.go). This is an internal
consensus adapter. The pre-merge RPC router is retired.

[`blocks.go`](../consensus/internal/rpc/core/blocks.go),
[`consensus.go`](../consensus/internal/rpc/core/consensus.go) and
[`responses.go`](../consensus/rpc/coretypes/responses.go) define the differences:

- `block` and `block_by_hash` translate retained global blocks into consensus
  results. Requested heights are checked against the application head and the
  earliest retained block. Pruned heights report `ErrHeightNotAvailable`;
  heights above the head report `ErrHeightExceedsChainHead`.
- `block_results` reads persisted `FinalizeBlock` responses.
- `commit` returns `ErrCommitNotAvailable`, whose message is
  `commit is not available under autobahn consensus`.
- `dump_consensus_state` and `consensus_state` return
  `ErrConsensusStateNotAvailable`, with message
  `consensus round state is not available under autobahn consensus`.
- `validators` uses genesis validators with pagination. Proposer priority is
  zero because Autobahn uses its own committee leader election.

The absence of a Tendermint commit response under Autobahn does not change the
network's block finality rule. Consumers must use the response shapes supported
by the active consensus implementation.

Recovery is owned by [`data.State`](../consensus/internal/autobahn/data/state.go)
and its `DataWAL`, which pairs global-block and full-commit-QC persisters.
Startup reconciles their retained ranges after interrupted persistence or
pruning; blocks without any retained QCs are treated as corrupt and require
state sync. `PushAppHash` waits for the block and its QC to become durable before
application votes can be issued. The adapter also requires a state store for
persisted finalization responses. Preserve these stores alongside application
state when restarting a node; the receipt WAL described in
[sequencer WAL](sequencer.md#sequencer-wal) is a different log.

## Autobahn events

After `FinalizeBlock` results are persisted and `Commit` succeeds,
[`executeBlock`](../consensus/internal/p2p/giga_router.go) publishes
`NewBlockHeader` and one `Tx` event per transaction when an event bus is
installed. The transaction event includes height, index, bytes and execution
result. Event consumers should use the declared event types in
[`types/events.go`](../consensus/types/events.go).

## Proto

The schemas under [`proto/tendermint/`](../consensus/proto/tendermint) retain
the `tendermint.*` protobuf namespaces. They define blocks, votes, validator
updates, evidence, consensus gossip, state synchronization, ABCI and remote
validator signing. The private-validator service declares `GetPubKey`,
`SignVote` and `SignProposal` in
[`privval/service.proto`](../consensus/proto/tendermint/privval/service.proto).
Autobahn's own messages are declared in
[`autobahn.proto`](../consensus/internal/autobahn/autobahn.proto) and
[`giga/api.proto`](../consensus/internal/p2p/giga/api.proto).

Generated `*.wireguard.go` schemas participate in decoding through
[`protoutils`](../consensus/internal/protoutils). Its wire scanner applies the
declared nested-message and repeated-field rules before unmarshalling. Use the
schema and decoder belonging to the message producer; do not assume upstream
ABCI definitions cover this fork's Go application API.

## OpenAPI

[`rpc/openapi/openapi.yaml`](../consensus/rpc/openapi/openapi.yaml) describes
consensus URI routes, root JSON-RPC envelopes and error responses. The root
handler accepts a request or a batch of up to ten requests; notifications do
not produce response objects. Its upstream descriptive text and example
servers do not define Paxeer's public deployment or override the loopback
default in `DefaultRPCConfig`. Use the [RPC routes](#rpc-routes) above for
endpoint selection, and the running implementation for conditional Autobahn
behavior.

## Metrics

[`InstrumentationConfig`](../consensus/config/config.go) controls the consensus
Prometheus listener. `prometheus` defaults to false, `prometheus-listen-addr`
to `:26660`, and `namespace` to `tendermint`; the scrape path is `/metrics`.

Instrument definitions and their generated constructors live beside each
component, including [`consensus metrics`](../consensus/internal/consensus/metrics.go),
[`mempool metrics`](../consensus/internal/mempool/metrics.go),
[`state metrics`](../consensus/internal/state/metrics.go) and
[`P2P metrics`](../consensus/internal/p2p/metrics.go). With the default namespace,
consensus instruments include `tendermint_consensus_height` and
`tendermint_consensus_rounds`. The prefix follows configuration; instrument
availability follows the active component. Node execution instruments are
covered by [node metrics](sequencer.md#node-metrics).

## Version pins

[`version/version.go`](../consensus/version/version.go) declares:

| Constant | Value |
| --- | --- |
| `TMVersionDefault` | `0.35.0-unreleased` |
| `ABCISemVer`, `ABCIVersion` | `0.17.0` |
| `P2PProtocol` | `8` |
| `BlockProtocol` | `11` |

`RequestInfo` carries the compiled version information into the application
handshake. These are protocol/library versions, not the EVM chain ID.

## CLI

The fork retains consensus CLI commands under
[`cmd/tendermint/commands/`](../consensus/cmd/tendermint/commands).
The merged network is run with `paxd`; use its registered commands described
in [node CLI](sequencer.md#node-cli). Consensus maintenance commands that reset
state or validator signing data require an operator's explicit intent and are
not a normal node startup step.

## Env

[`libs/cli/setup.go`](../consensus/libs/cli/setup.go) enables Viper environment
lookup with the prefix supplied to `PrepareBaseCmd`. Dots and hyphens in keys
become underscores. The standalone e2e node reads `TMHOME` in
[`test/e2e/node/main.go`](../consensus/test/e2e/node/main.go); this test harness
setting is not a universal production home-directory setting.

## Make targets

[`consensus/Makefile`](../consensus/Makefile) retains `build`, `install`,
`build_privval_server`, `format`, `metrics` and test targets. Targets are
invoked from the consensus directory. Its `all` target includes build, tests
and installation; it is broader than a compile check. Select the owning
task's declared gate when qualifying a change.
