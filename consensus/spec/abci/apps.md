---
order: 2
title: Applications
---

# Applications

Paxeer's consensus engine and application run together in `paxd`. The kernel is
part of this one network and participates in proposing blocks. EVM chain ID is
125 (`0x7d`); a transaction receipt is final when its block is committed.

This page describes the current Go
[`Application`](../../abci/types/application.go) interface. The inherited
[ABCI Methods and Types](abci.md) page is protocol background; use the local Go
interface and request types when integrating this fork. In particular,
`BeginBlock`, `DeliverTx`, and `EndBlock` are not methods of `Application`.
Block execution enters through `FinalizeBlock`.

- [Connection State](#connection-state) covers concurrency and working state.
- [Transaction Results](#transaction-results) distinguishes admission from execution.
- [Updating the Validator Set](#updating-the-validator-set) and
  [Consensus Parameters](#consensus-parameters) describe classic-mode updates.
- [Query](#query) covers local reads and application proofs.
- [Crash Recovery](#crash-recovery) and [State Sync](#state-sync) describe recovery boundaries.

## Connection State

The interface groups methods by information/query, mempool, consensus, and
snapshot duties. These are logical roles, not four network connections. The
[in-process proxy](../../internal/proxy/proxy.go) calls the supplied Go object
directly and records timings. See [Client and Server](client-server.md).

### Concurrency

The proxy does not provide a global application mutex or an asynchronous request
queue. Calls from independent node services can overlap. Applications must
synchronize shared state and isolate speculative execution, mempool checks,
committed queries, and snapshot operations. The SDK's
[`BaseApp`](../../../sdk/baseapp/abci.go) implements its own state and commit
locking; an integrator must not infer serialization from the word “connection”.

### Consensus Connection

In the classic path,
[`BlockExecutor`](../../internal/state/execution.go) submits a candidate through
`ProcessProposal`. The response must give an explicit `ACCEPT` or `REJECT`;
`UNKNOWN` is treated as a programming error. Rejection causes a nil prevote.
Validation during proposal processing must not expose uncommitted state as
committed state.

After a block is decided, the executor calls `FinalizeBlock` with its ordered
transactions, header, block hash, previous commit information, and misbehavior.
The response contains transaction results, events, validator updates, consensus
parameter updates, and `AppHash`. It saves the response before calling `Commit`,
then saves the updated consensus state. Later block execution must see the state
produced by earlier committed blocks.

Autobahn uses the same application interface but a different decision and
recovery path. Its execution loop in
[`giga_router.go`](../../internal/p2p/giga_router.go) reads finalized global
blocks, calls `FinalizeBlock`, saves its response, calls `Commit`, and reports
`ResponseFinalizeBlock.AppHash` to the Autobahn data layer. It does not call
`ProcessProposal` in this execution loop. Its `RequestFinalizeBlock.Hash` is the
Autobahn lane block header hash; `Header.Time` is the consensus-assigned global
block timestamp. `DecidedLastCommit` is empty in this path: it is not a classic
precommit set. See the [mode map](../consensus/consensus.md#consensus-modes).

### Commit

`Commit(context.Context)` persists the selected application state.
`ResponseCommit` contains `RetainHeight`, not an application hash. Return the
execution hash in `ResponseFinalizeBlock.AppHash`; report the last *durably
committed* height and hash in `Info` after restart.

The SDK commit implementation writes the selected multistore, commits it,
resets check state, and clears execution states. In classic mode the block
executor holds the mempool lock across `Commit` and the mempool update. Do not
synchronously submit a transaction and wait for mempool admission from a commit
handler: admission requires that lock. Avoid application callbacks that wait on
services whose progress depends on the current commit.

### Mempool Connection

`CheckTx` receives a [`RequestCheckTxV2`](../../abci/types/types.go).
Admission is a local filter; it does not imply inclusion, execution success, or
finality. A proposer can supply transactions that were never in the local
mempool, so execution must enforce validity independently. Replay protection,
nonce checks, signatures, balances, and resource accounting belong in the
application's real transaction paths.

The proxy exposes `CheckTxSafe` to callers: it converts a panic or a nil response
into an error and validates the EVM response metadata. The interface also
provides `GetTxPriorityHint`, `EvmNonce`, and `EvmBalance` for transaction handling.
Classic mempool updates remove committed transactions and can recheck remaining
transactions according to `ConsensusParams.ABCI.RecheckTx`. Autobahn has its own
producer mempool; classic lock/recheck behavior is not its lifecycle contract.

### Query Connection

`Info` describes the application's committed height and hash. `Query` reads
application state at the requested height, if that state is available. Queries
must not accidentally observe speculative writes. A local read may lag another
node's head; submit a transaction when a state change must be ordered by
consensus.

### Snapshot Connection

The snapshot methods serve or restore committed application state. Their
presence in `Application` does not establish that every consensus mode can use
classic state sync. See [State Sync](#state-sync).

## Transaction Results

### CheckTx

A nonzero `ResponseCheckTx.Code` fails admission. Code zero is
`CodeTypeOK`; it says nothing about a future block's execution result. Application
error codes and `Codespace` describe application failures rather than transport
errors. Go errors from proxy methods represent call failures.

### FinalizeBlock

Return one non-nil `ExecTxResult` per input transaction, in the same order.
A failed transaction can remain in a committed block with a nonzero result code;
the application must apply its defined failure and fee semantics consistently.

In classic mode, `TxResultsHash` in
[`execution.go`](../../internal/state/execution.go) commits the deterministic
projection made by
[`MarshalTxResults`](../../abci/types/types.go): `Code`, `Data`, `GasWanted`, and
`GasUsed`. It becomes `LastResultsHash` in the next block. Logs, informational
text, transaction events, and block events are not part of that commitment.
Events support indexing and subscriptions; they do not by themselves constitute
an application-state proof.

### Gas

Gas is application-defined resource accounting. Classic proposal selection uses
`Block.MaxGasWanted` and `Block.MaxGas` as separate limits on wanted and estimated
gas. Both permit `-1` for an unlimited budget. Selection budgets do not replace
deterministic enforcement during execution. Autobahn production has separate
transaction and gas budgets; see [Creating a proposal](../consensus/creating-proposal.md).

## Updating the Validator Set

### InitChain

`RequestInitChain` carries genesis time, chain ID, initial height, consensus
parameters, and application state bytes. `ResponseInitChain` carries validators
and an application hash. It has no `ConsensusParams` field in this fork.

The classic handshaker checks that a nonempty genesis validator set agrees with
the application's returned set. When initializing consensus state, returned
validators supply the initial set; initialization fails if that set is empty.
Do not use the old upstream recipe of returning a different set to override
nonempty genesis validators.

### FinalizeBlock updates

In classic mode `ResponseFinalizeBlock.ValidatorUpdates` changes voting power.
Power must be nonnegative; zero removes an existing validator. Keys must use a
permitted type, updates must have no duplicate keys, and the resulting total
voting power must respect `MaxTotalVotingPower` in
[`validator_set.go`](../../types/validator_set.go). Updates from height `H` take
effect at `H+2`, as implemented by `State.Update` in
[`execution.go`](../../internal/state/execution.go).

Autobahn constructs a committee from its configuration. Classic `H+2` updates
are not an Autobahn committee reconfiguration protocol. Its quorum and signature
rules are described in [Byzantine Consensus Algorithm](../consensus/consensus.md).

## Consensus Parameters

The classic parameter schema is
[`ConsensusParams`](../../types/params.go), including `Block`, `Evidence`,
`Validator`, `Version`, `Synchrony`, `Timeout`, and `ABCI` groups.

- `Block.MaxBytes` limits the complete protobuf block and must be positive and
  at most `MaxBlockSizeBytes` (104857600 bytes).
- `Block.MaxGas` and `Block.MaxGasWanted` configure proposal selection budgets.
- `Evidence.MaxBytes` is a byte budget, not a count or a fixed fraction of a block.
  Evidence expires when both its block age and duration exceed the configured
  `MaxAgeNumBlocks` and `MaxAgeDuration` limits.
- `Synchrony.Precision` and `Synchrony.MessageDelay` control classic proposal
  timeliness; see [BFT Time](../consensus/bft-time.md).
- `ABCI.RecheckTx` controls classic mempool rechecking.

Genesis supplies the initial parameters. In classic mode, updates returned by
`FinalizeBlock` at `H` govern `H+1` after validation. For a supplied block,
evidence, or validator parameter group, preserve its other fields: the update
implementation replaces the group's fields rather than inferring omitted
scalar values. Consult `UpdateConsensusParams` for groups with optional fields.
Autobahn's payload and transport bounds are separate from these classic limits.

## Query

`RequestQuery` contains `Path`, `Data`, `Height`, and `Prove`; `ResponseQuery`
contains the response code, key/value data, height, and `ProofOps`. Paths and
proof formats are application-defined. The SDK's query dispatcher is in
[`sdk/baseapp/abci.go`](../../../sdk/baseapp/abci.go).

### Query Proofs

Proofs must verify against a trusted application root at the matching height.
In classic mode the root returned in `ResponseFinalizeBlock.AppHash` is recorded
in consensus state and carried by the next block's `AppHash`. A proof consumer
must account for this height relationship. Autobahn certifies execution roots
through `AppProposal` and `AppQC`; do not assume classic header proofs apply to
its lane block hashes.

`ProofOps` is a sequence of typed proof operations (`type`, `key`, `data`). The
proof verifier must understand those types and compose them through the
application's Merkle tree structure to the trusted root. Neither an untrusted
query response nor snapshot metadata supplies that trust anchor.

## Crash Recovery

`Info.LastBlockHeight` and `Info.LastBlockAppHash` must describe the same durable
commit. Persist them consistently with application state, not speculative
proposal processing.

### Classic recovery

The [handshaker](../../internal/consensus/replay.go) compares application height,
block-store height, and consensus-state height. It initializes a fresh
application with `InitChain`, checks available history, and replays blocks the
application has not committed. The block store may be at the consensus-state
height or one ahead. An application ahead of the block store, or too far behind
a pruned store, cannot be repaired by ordinary replay.

If the application committed the last stored block but consensus state was not
saved, recovery rebuilds consensus state from saved `FinalizeBlock` responses
without executing that height against the real application again. App-hash
comparisons detect inconsistent recovery. Preserve the block store, state
store, application database, and consensus WAL as a consistent recovery set.

### Autobahn recovery

Autobahn bypasses the classic handshaker. Its execution loop calls `Info`, calls
`InitChain` only when the application height is zero, and otherwise reports the
last committed hash to the data layer before resuming at the next global height.
The SDK rebuilds execution state from its committed multistore after restart.

The Autobahn data layer retains global blocks and commit certificates in its
own durable stores. `PushAppHash` waits until the corresponding block and
certificate are durable before acknowledging execution. Its configured
`persistent_state_dir` also protects votes and view justification across restart;
see [Autobahn recovery](../consensus/consensus.md#autobahn-recovery). A classic
store-height reconciliation is not a substitute for these records.

## State Sync

Classic state sync bootstraps application state from a committed snapshot,
verifies it with a trusted light-client application hash, then continues with
block sync and consensus. It does not restore a full historical block archive.
The node's [assembly](../../node/node.go) installs this path in classic mode;
Autobahn's recovery uses its data layer instead.

### Taking Snapshots

Snapshot metadata contains `Height`, `Format`, `Chunks`, `Hash`, and `Metadata`.
Take a snapshot at one committed height with no later speculative writes. Its
format must define deterministic bytes and chunks so peers advertising the same
snapshot can supply interchangeable chunks. Snapshot work must synchronize with
application writes without holding consensus execution indefinitely.

Peers discover snapshots with `ListSnapshots` and retrieve their bytes with
`LoadSnapshotChunk`. Keep older snapshots available while peers may still be
restoring them.

### Bootstrapping a Node

The classic TOML setting is `statesync.enable = true`. Configure its RPC servers,
trusted height, and trusted hash from an independently trusted source. These
light-client settings are distinct from public EVM JSON-RPC access.

The [syncer](../../internal/statesync/syncer.go) offers a candidate to
`OfferSnapshot` with the trusted application hash. After acceptance, it downloads
chunks and sends them to `ApplySnapshotChunk`. Responses can request refetches,
reject senders, retry a snapshot, reject it, or abort. Validate untrusted chunk
data before allocating or applying it.

After restoration, the syncer calls `Info` and checks both the restored height
and application hash against the trusted state. Accepting all chunks is not
sufficient verification. Only after these checks can the node continue to block
sync and consensus.
