---
order: 1
title: Overview and basic concepts
---

## Outline

- [ABCI++ vs. ABCI](#abci-vs-abci)
- [Methods overview](#methods-overview)
  - [Consensus methods](#consensusblock-execution-methods)
  - [Mempool methods](#mempool-methods)
  - [Info methods](#info-methods)
  - [State-sync methods](#state-sync-methods)
- [Next-block execution vs. same-block execution](#next-block-execution-vs-same-block-execution)
  - [Tendermint timeouts](#tendermint-timeouts-in-same-block-execution)
- [Determinism](#determinism)
- [Errors](#errors)
- [Events](#events)
- [Evidence](#evidence)

# Overview and basic concepts

This page describes the application's boundary with the embedded consensus engine
in this fork. The `_002_draft` filename is retained for existing links; the
upstream ABCI++ draft is not a conformance specification for this runtime.
The authoritative API is the Go
[`Application` interface](../../abci/types/application.go), together with its
[plain Go types](../../abci/types/plain_types.go),
[extended transaction types](../../abci/types/types.go), and
[protobuf response types](../../proto/tendermint/abci/types.proto).
See [Client and Server](./abci++_client_server_002_draft.md) for integration and
startup details.

Paxeer mainnet-beta is one BFT network, run by `paxd`, with EVM chain ID 125
(`0x7d`). The kernel is part of that network and proposes blocks. A receipt is
final once its block is committed. Sub-chains are tenant domains represented by
database namespaces, rather than independently deployed consensus engines.

## ABCI++ vs. ABCI

[&uparrow; Back to Outline](#outline)

ABCI separates consensus from the deterministic application state machine.
The engine initiates calls and the application returns results. This fork uses
in-process Go calls, including `ProcessProposal`, `FinalizeBlock`, and `Commit`.
`FinalizeBlock` delivers a decided block; it replaces the engine-facing sequence
of `BeginBlock`, per-transaction `DeliverTx`, and `EndBlock` calls. Those older
response types still occur in the source, but are not methods on `Application`.

The upstream draft's `PrepareProposal`, `ExtendVote`, and `VerifyVoteExtension`
methods are absent from this interface. Do not implement proposal preparation,
vote extensions, or a same-block header protocol on the assumption that this
runtime calls those hooks. The sibling [Methods draft](./abci++_methods_002_draft.md),
[Application Requirements draft](./abci++_app_requirements_002_draft.md), and
[Expected Behavior draft](./abci++_tmint_expected_behavior_002_draft.md) retain
upstream design material; use the code and the mode distinctions below to
establish which behavior applies here.

## Methods overview

[&uparrow; Back to Outline](#outline)

The interface groups operations by purpose. These groups are not four network
connections: the node embeds one Go application. Calls are synchronous at the
interface boundary, but different callers may run concurrently.

### Consensus/block execution methods

- **`InitChain`** initializes application state from genesis when starting a
  fresh chain. Restart recovery uses committed height and state rather than
  unconditionally initializing genesis again.
- **`ProcessProposal`** inspects a proposed block in the classic consensus path.
  Its request includes `Txs`, `ProposedLastCommit`, `ByzantineValidators`, `Hash`,
  and `Header`. Return `ResponseProcessProposal_ACCEPT` or
  `ResponseProcessProposal_REJECT`. The classic
  [`BlockExecutor.ProcessProposal`](../../internal/state/execution.go) consumes
  the status; `UNKNOWN` causes a panic. Rejection can prevent progress, so
  proposal validation must agree across validators.
- **`FinalizeBlock`** executes the decided block and returns
  `ResponseFinalizeBlock`: block `Events`, ordered `TxResults`,
  `ValidatorUpdates`, `ConsensusParamUpdates`, and `AppHash`. Each transaction
  result is an `ExecTxResult`; return one result for each input transaction in
  the same order. Its request carries `DecidedLastCommit` in place of
  `ProposedLastCommit`.
- **`Commit`** persists application state. `ResponseCommit` carries
  `RetainHeight` for pruning; the application hash comes from
  `ResponseFinalizeBlock.AppHash`, not from `ResponseCommit`.

Execution order depends on the configured consensus mode:

| Path | Application calls and applicability |
| --- | --- |
| Classic consensus | Proposal validation calls `ProcessProposal`. After a decision, `BlockExecutor.ApplyBlock` calls `FinalizeBlock`, saves its response, validates updates, calls `Commit`, updates the mempool, and saves consensus state. |
| Autobahn | A non-empty `autobahn-config-file` selects this path in `node/node.go`. `GigaRouter.runExecute` reads ordered global blocks and `executeBlock` calls `FinalizeBlock`, saves its response, calls `Commit`, then records the application hash. This execution loop does not call `ProcessProposal`. |

The Autobahn implementation is in
[`internal/p2p/giga_router.go`](../../internal/p2p/giga_router.go). Its
`RequestFinalizeBlock.Hash` is the Autobahn header hash. It supplies an empty
`DecidedLastCommit`; this does not mean validators failed to vote. Classic
Tendermint commit, header, and evidence assumptions must not be inferred from
that request in Autobahn mode. See the [integration lifecycle](./abci++_client_server_002_draft.md#execution-and-recovery)
for restart behavior.

### Mempool methods

**`CheckTx`** takes `RequestCheckTxV2` and returns `*ResponseCheckTxV2`. It validates
transactions for local admission, without committing application state.
`ResponseCheckTxV2` embeds `*ResponseCheckTx` and adds Go-only EVM fields such as
`IsEVM`, `EVMNonce`, `EVMHash`, sender addresses, and required balance.
A successful check is not a promise of inclusion or successful execution.

In the [classic mempool](../../internal/mempool/mempool.go), a nonzero response
code rejects admission. After a block is committed, optional rechecking applies
to remaining transactions with `CheckTxTypeV2Recheck`, not to transactions that
were included in that block. `BlockExecutor.Commit` passes the
`ConsensusParams.ABCI.RecheckTx` setting to this update.

**`GetTxPriorityHint`**, **`EvmNonce`**, and **`EvmBalance`** provide transaction
priority and account-state information. They are also part of `Application` and
must be implemented with the signatures and types declared there. The classic
mempool is installed only outside Autobahn mode; Autobahn has its own transaction
producer and execution path.

### Info methods

- **`Info`** reports application version, committed height, and committed
  application hash. Startup and recovery depend on accurate
  `LastBlockHeight` and `LastBlockAppHash` values.
- **`Query`** reads application state and returns a `ResponseQuery`, including
  an application code and optional proof information.
- **`GetValidators`** returns the application's validator updates directly as a
  Go slice. Autobahn execution uses it when selecting an application-facing
  proposer address.

### State-sync methods

The interface retains four snapshot methods:

- **`ListSnapshots`** lists available snapshot metadata, not the full contents.
- **`OfferSnapshot`** asks the receiving application whether it accepts a
  snapshot and its trusted application hash.
- **`LoadSnapshotChunk`** retrieves a chunk for a peer.
- **`ApplySnapshotChunk`** applies a received chunk and can request retries or
  reject the snapshot.

A `Snapshot` contains `Height`, `Format`, `Chunks`, `Hash`, and `Metadata`.
In the [classic state-sync implementation](../../internal/statesync/syncer.go),
the engine obtains trusted state, offers a snapshot, restores chunks, and checks
`Info` against the expected application hash and height. The existence of these
methods does not establish that classic state sync is an Autobahn bootstrap
protocol. See also the inherited [state-sync protocol](../p2p/messages/state-sync.md)
for classic message concepts.

### Other methods

The current `Application` interface has no `Echo` or `Flush` method. They are not
required connection-management calls for this in-process integration.

## Next-block execution vs. same-block execution

[&uparrow; Back to Outline](#outline)

The earlier draft proposed returning execution commitments from
`PrepareProposal` and selecting a same-block protocol through a `same_block`
parameter. Neither hook nor parameter is part of the current application
contract. Do not use that draft recipe to configure the network.

In the classic path, the engine obtains the application hash and transaction
results from `FinalizeBlock` after the block is decided. These become the
application and results commitments for subsequent block validation.
`ResponseProcessProposal` has execution-related fields in its Go definition,
but `BlockExecutor.ProcessProposal` uses its status only; filling those fields
does not replace the `FinalizeBlock` response or change the header protocol.

The node application can perform optimistic work during proposal processing.
[`BaseApp`](../../../sdk/baseapp/abci.go) and the
[node handlers](../../../node/app.go) maintain proposal and delivery state and
reconcile work at finalization. Speculative execution must not commit a losing
proposal. Autobahn orders and executes global blocks through its own code path;
it is not the draft's same-block execution mechanism.

### Tendermint timeouts in same-block execution

This heading is retained for existing links. The draft's proposed application
callback for changing proposal timeouts is not implemented in `Application`.
Classic consensus timing and Autobahn timing are separate configuration
contracts; do not derive either from speculative `PrepareProposal` behavior.
For Autobahn, [`AutobahnFileConfig`](../../config/autobahn.go) declares
`block_interval` and `view_timeout` in the JSON configuration selected by
`autobahn-config-file`.

## Determinism

[&uparrow; Back to Outline](#outline)

Given the same committed state and ordered block input, validators must compute
the same state transition and consensus-relevant results. Local admission,
queries, and speculative execution must not introduce independent changes to
committed state. Use block input for time-dependent logic rather than wall-clock
time, and avoid uncontrolled randomness, map iteration order, floating-point
variation, local files, or external service responses in replicated execution.

For classic result commitments,
[`TxResultsHash`](../../internal/state/execution.go) uses
[`MarshalTxResults`](../../abci/types/types.go), which retains only `Code`,
`Data`, `GasWanted`, and `GasUsed` from each `ExecTxResult`. Logs, informational
strings, events, codespace, and EVM metadata are not included in that results
hash. This exclusion does not make it safe for those fields to affect a later
state transition differently on different nodes. Application state hashes,
validator updates, and consensus parameter updates must also agree wherever
consumed by the consensus path.

## Errors

[&uparrow; Back to Outline](#outline)

Application response codes and Go call errors are different channels.
`CodeTypeOK` is `0`; a nonzero code in `ResponseQuery`, `ResponseCheckTx`, or
`ExecTxResult` reports an application failure. `Codespace` qualifies a code by
application namespace. These are ABCI application codes, not JSON-RPC error
codes.

### `CheckTx`

`CheckTx` returns a response pointer without a Go `error`. The local proxy's
`CheckTxSafe` recovers panics and rejects nil responses. For an EVM response it
also requires a nonzero `EVMHash` and a nonempty `PaxSenderAddress`. The classic
mempool rejects nonzero application codes; failed rechecks remove remaining
transactions.

### `TxResult` (as part of `FinalizeBlock`)

`ResponseFinalizeBlock.TxResults` contains `ExecTxResult` values, one per
transaction. `TxResult` is the wrapper used for indexing a transaction with its
height, index, bytes, and execution result. A nonzero execution code records a
failed transaction already included in the decided block; it does not undo the
consensus decision.

### `Query`

A nonzero `ResponseQuery.Code` is returned to the querying client. Callers must
also handle the separate Go `error` returned by `Query`.

`Info`, `InitChain`, `ProcessProposal`, `FinalizeBlock`, `Commit`, and snapshot
operations can return Go errors. The proxy forwards these to callers. The
classic executor propagates finalization and commit failures; the Autobahn
execution loop likewise returns them. There is no blanket API requirement that
all such methods crash the application, and a failed block-level call must not
be converted into fabricated success results.

## Events

[&uparrow; Back to Outline](#outline)

`ResponseFinalizeBlock` carries block events, and each `ExecTxResult` can carry
transaction events. The current `ResponseCheckTx` has no `Events` field.
Events provide metadata for indexing, subscriptions, and queries; they are not
part of the classic `LastResultsHash` commitment.

The [protobuf declarations](../../proto/tendermint/abci/types.proto) define:

```protobuf
message Event {
  string type = 1;
  repeated EventAttribute attributes = 2;
}

message EventAttribute {
  bytes key = 1;
  bytes value = 2;
  bool index = 3;
}
```

An event can share a type with other events. Attributes are byte slices in Go,
not protobuf strings; use appropriate text encodings when an indexer or consumer
expects text. The `index` flag requests indexing and is non-deterministic.
Index availability and emitted events do not replace verification of committed
application state. Classic event publication is in `FireEvents` in the block
executor; Autobahn publishes block-header and transaction events in
`GigaRouter.executeBlock`.

## Evidence

[&uparrow; Back to Outline](#outline)

In the classic path, the engine verifies and includes evidence of validator
misbehavior and converts it to `ByzantineValidators` in the proposal and
finalization requests. The application is responsible for applying its
punishment policy. The current enum is `MisbehaviorType`, with `UNKNOWN = 0`,
`DUPLICATE_VOTE = 1`, and `LIGHT_CLIENT_ATTACK = 2`. Each `Misbehavior` also
contains the validator, height, time, and total voting power.

Autobahn's `executeBlock` does not populate `ByzantineValidators`. Do not infer
classic evidence delivery, light-client trust rules, or punishment coverage for
Autobahn from the shared request type. The inherited
[data structures](../core/data_structures.md) and
[accountability material](../light-client/accountability/) explain classic
Tendermint concepts, not Autobahn certificate or recovery guarantees.
