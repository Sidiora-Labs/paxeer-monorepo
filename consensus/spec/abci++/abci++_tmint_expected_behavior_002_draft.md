---
order: 4
title: Tendermint's expected behavior
---

# Tendermint's expected behavior

This page describes the application calls implemented by this fork. It replaces
an upstream ABCI++ draft whose callback grammar does not match the current API.
Paxeer is one network: the kernel runs inside the network and participates in
block proposal. A transaction receipt is final when its block is committed.
Application execution and persistence below are stages of that same chain.

The authoritative API is the Go
[`Application` interface](../../abci/types/application.go), called in process
through [`Proxy`](../../internal/proxy/proxy.go). The neighboring upstream
[basic concepts](abci++_basic_concepts_002_draft.md) and
[method draft](abci++_methods_002_draft.md) are historical background; their
method lists and response fields must be checked against the Go interface and
[current request and response types](../../abci/types/plain_types.go).

## Valid method call sequences

The application exposes `Info`, `Query`, `GetValidators`, `CheckTx`,
`GetTxPriorityHint`, `EvmNonce`, `EvmBalance`, `InitChain`, `ProcessProposal`,
`FinalizeBlock`, `Commit`, and four snapshot methods. It does not expose
`PrepareProposal`, `VerifyHeader`, `ExtendVote`, `VerifyVoteExtension`, `Echo`,
`Flush`, `BeginBlock`, `DeliverTx`, or `EndBlock` as consensus callbacks.
Legacy message types remaining in the tree do not add methods to this interface.

There is no promise of exactly one proposal callback per height, of a decision
in round zero, or of a `ProcessProposal` call before every `FinalizeBlock`.
Rounds can fail, nodes can catch up without voting, and startup can replay
committed blocks. The useful lifecycle distinction is between inspecting a
candidate, applying a decided block, and committing application state.

### Applicability by consensus mode

[`node.go`](../../node/node.go) selects the Autobahn path when
`AutobahnConfigFile` is non-empty. The serialized configuration key is
`autobahn-config-file` in [`config/toml.go`](../../config/toml.go).

| Behavior | Classic consensus | Autobahn |
| --- | --- | --- |
| Proposal construction | `BlockExecutor.CreateProposalBlock` | Lane producer and certificate-based ordering |
| Application proposal check | `BlockExecutor.ProcessProposal` before a block prevote | No `ProcessProposal` call in the global-block execution loop |
| Application execution | `BlockExecutor.ApplyBlock` calls `FinalizeBlock`, then `Commit` | `GigaRouter.executeBlock` calls `FinalizeBlock`, then `Commit` |
| Startup and recovery | `Handshaker` reconciles application, block store and state store | `runExecute` resumes from `Info.LastBlockHeight`; Autobahn restores its own persisted data and consensus state |

The classic round, lock, prevote and precommit terminology in the
[versioned design references](#historical-design-variants) applies to the
classic algorithm. It does not specify Autobahn block identity, certificates,
voting, or recovery.

### Classic proposal processing

The proposer builds a block through `BlockExecutor.CreateProposalBlock` in
[`state/execution.go`](../../internal/state/execution.go). That method handles
transaction selection and, when configured, a sealed sequence batch; it does
not call an application `PrepareProposal` hook.

In [`State.defaultDoPrevote`](../../internal/consensus/state.go), a missing,
untimely or invalid proposal can lead to a nil prevote before any application
call. For a candidate that passes these checks, the engine calls
`BlockExecutor.ProcessProposal` synchronously, including on the local proposer
when it follows this prevote path. The request contains `Txs`,
`ProposedLastCommit`, `ByzantineValidators`, `Hash`, and `Header`.

The verdict is `ResponseProcessProposal.Status`, with `ACCEPT`, `REJECT`, and
`UNKNOWN` values. It is not the draft's Boolean `accept`. A rejection causes a
nil prevote. `UNKNOWN` causes a panic in the block executor; an application
error is wrapped as `ErrInvalidBlock` and the classic prevote caller panics.
There is no draft `evidence_list` result or application vote-extension callback.
Application acceptance must be deterministic for the same candidate and state.

### Applying and committing a decided block

In the classic path, `BlockExecutor.ApplyBlock` validates the block and calls
`FinalizeBlock` with its transactions, `DecidedLastCommit`, misbehavior, hash
and header. It saves the finalization response before committing the
application, processes validator and consensus-parameter updates, calls
`Commit`, updates the mempool, and saves the new consensus state.

[`ResponseFinalizeBlock`](../../proto/tendermint/abci/types.proto) carries
`TxResults`, `Events`, `ValidatorUpdates`, `ConsensusParamUpdates`, and
`AppHash` in Go. `ResponseCommit` carries `RetainHeight`; it does not return the
application hash. Thus `FinalizeBlock` does not replace the separate `Commit`
call. The engine uses the finalization response's `AppHash` for the resulting
state. `TxResultsHash` commits deterministic transaction results, excluding
finalization events.

The actual node handlers are `ProcessProposalHandler` and `FinalizeBlocker` in
[`node/app.go`](../../../node/app.go), with persistence in
[`App.Commit`](../../../node/abci.go). `FinalizeBlocker` can reuse eligible
optimistic results or execute the block through `ProcessBlock`. Candidate work
alone is not a committed result, and finalization must work without a prior
proposal check, including during recovery.

### Classic startup, replay and snapshots

[`Handshaker.Handshake`](../../internal/consensus/replay.go) starts with `Info`
and reads `LastBlockHeight` and `LastBlockAppHash`. At application height zero,
`ReplayBlocks` calls `InitChain` using genesis data. It compares the application,
block-store and consensus-state heights to determine whether replay is needed.
A fresh consensus height comes from genesis and persisted state, rather than a
hard-coded zero in research pseudocode.

When the application is behind, replay executes stored blocks using
`ExecCommitBlock` or `ApplyBlock`, with `FinalizeBlock` followed by `Commit`.
If the application has committed the last stored block but the consensus state
has not been saved, recovery uses the saved finalization response to rebuild
that state without executing the same block against the real application
again. Height and application-hash checks are part of this reconciliation.
Recovery is therefore more than repeated calls to `FinalizeBlock` alone.

The snapshot methods are `ListSnapshots`, `LoadSnapshotChunk`, `OfferSnapshot`,
and `ApplySnapshotChunk`. The classic
[`state-sync implementation`](../../internal/statesync/syncer.go) can retry or
reject snapshots and chunks and verifies restored application state with
`Info`. These methods' presence in the interface does not imply that Autobahn
uses the classic snapshot startup sequence.

### Autobahn execution and restart

[`GigaRouter.runExecute`](../../internal/p2p/giga_router.go) owns application
startup in Autobahn mode; the classic handshaker is skipped. It calls `Info`.
At application height zero it calls `InitChain` and begins at the genesis
initial height. Otherwise it publishes the last committed application hash to
the data layer and starts execution at `LastBlockHeight + 1`.

For each global block, `executeBlock` supplies the global payload's transactions
and the Autobahn header hash to `FinalizeBlock`. Its application header carries
the chain ID, global height and timestamp. `DecidedLastCommit` is empty on this
path; it must not be interpreted as a set of absent classic validator votes.
The method saves the finalization response, calls `Commit`, and then reports
the response's application hash through `PushAppHash`.

Application progress is distinct from the consensus recovery state. The
[`data layer`](../../internal/autobahn/data/state.go) retains global blocks and
commit certificates in its WALs; `PushAppHash` waits for the block and its
certificate to be durably persisted. The
[`consensus state`](../../internal/autobahn/consensus/inner.go) persists votes
and view justifications when persistence is configured, using the
[A/B persistence implementation](../../internal/autobahn/consensus/persist/persist.go).
Those mechanisms, rather than classic round pseudocode, govern restart.

### Other application calls

`CheckTx` uses `RequestCheckTxV2` and returns `*ResponseCheckTxV2` directly.
The proxy's `CheckTxSafe` converts a panic or invalid response into an error.
Admission checks, priority hints, account nonce and balance reads, queries and
snapshot serving have separate callers; the consensus sequence above is not a
global serialization or concurrency guarantee for every application method.
`Info` can also be called outside startup.

## Adapting existing Applications that use ABCI

Implement the current Go interface and its actual types. A legacy application's
begin-block, per-transaction execution and end-block logic can inform its
`FinalizeBlock` implementation, which must produce the ordered transaction
results and resulting application hash. Keep durable persistence in `Commit`
and return the committed height and hash from `Info` for restart.

Do not copy the old draft's `TxRecord`/`UNMODIFIED` preparation adapter, Boolean
proposal verdicts, empty vote-extension handlers, or its instruction to fold
`Commit` into `FinalizeBlock`. Those hooks and shapes are absent from the
current interface. `BaseApplication` illustrates the method signatures but its
default acceptance behavior is not the network application's validation policy.

## Historical design variants

The filenames below identify stages of an upstream design exercise, not node
releases, selectable application versions or Autobahn conformance levels.
Their pseudocode is retained as historical material with a current-code map on
each page.

| Reference | Historical subject |
| --- | --- |
| [v0](v0.md) | Classic round and locking algorithm without application hooks |
| [v1](v1.md) | Legacy begin-block, transaction delivery and end-block integration |
| [v2](v2.md) | Synchronous proposal processing and proposed vote-extension hooks |
| [v3](v3.md) | Proposed split between application header and proposal checks |
| [v4](v4.md) | Proposed fork/join proposal processing before precommit |
