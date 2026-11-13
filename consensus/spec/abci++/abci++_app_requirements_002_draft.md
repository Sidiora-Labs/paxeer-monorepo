---
order: 3
title: Application Requirements
---

# Application Requirements

## Current fork requirements

This page documents the in-process application contract used by the consensus
engine embedded in `paxd`. Paxeer mainnet-beta is one BFT network, with EVM chain
ID 125 (`0x7d`); the merged kernel is part of the network and proposes blocks.
A receipt is final when its block is committed. See the
[specification overview](../README.md#mode-applicability) for mode selection and
the [Autobahn protocol map](../README.md#autobahn-protocol-and-implementation-map)
for block identity, quorum, time, transport and persistence.

The upstream draft is retained [below](#archived-upstream-draft) for its formal
reasoning and historical design discussion. Its method set, same-block execution
options, connection model and recovery instructions do not define this fork.
The current requirements in this section take precedence over the archive.

### Implemented application interface

[`Application`](../../abci/types/application.go) is the authoritative Go API.
Calls use Go values and `context.Context` in the node process. The
[proxy](../../internal/proxy/proxy.go) forwards directly to the application and
records method timings; it supplies neither a global state lock nor a socket or
gRPC ABCI transport. The draft's four “connections” are useful categories of
responsibility, not four network connections to provision.

| Responsibility                                          | Current methods                                                                                                 |
| ------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------- |
| Application information and queries                     | `Info`, `Query`, `GetValidators`                                                                                |
| Transaction admission and scheduling                    | `CheckTx(context.Context, *RequestCheckTxV2) *ResponseCheckTxV2`, `GetTxPriorityHint`, `EvmNonce`, `EvmBalance` |
| Initialization, proposal validation and block execution | `InitChain`, `ProcessProposal`, `FinalizeBlock`, `Commit`                                                       |
| Snapshot interface                                      | `ListSnapshots`, `OfferSnapshot`, `LoadSnapshotChunk`, `ApplySnapshotChunk`                                     |

`PrepareProposal`, `ExtendVote` and `VerifyVoteExtension` are absent from this
interface. Consequently the draft's Requirements 1–5 and 8–12 are not an
implemented API checklist: their references to those methods are historical.
The deterministic validation and execution principles in Requirements 6–7 and
13–14 remain useful, subject to the actual caller and types described here.
The archive's advice to always accept proposals or vote extensions is not a
substitute for validating the current protocol's inputs.

### Proposal validation, execution and persistence

On the classic path,
[`BlockExecutor.ProcessProposal`](../../internal/state/execution.go) calls the
application to validate a proposal. It must make a deterministic accept/reject
decision from the proposed block and the appropriate application state. Proposal
work must not mutate the committed store. Repeated or discarded proposals must
not leave committed side effects, and speculative state must be bounded and
safe to discard. The SDK implementation uses separate proposal/delivery state;
see [`BaseApp.ProcessProposal` and `BaseApp.FinalizeBlock`](../../../sdk/baseapp/abci.go).

For a decided block, classic `ApplyBlock` performs these operations:

1. Call `FinalizeBlock` with ordered transactions, header, block hash, last-commit
   information and evidence.
2. Save the finalization response; validate validator updates and apply consensus
   parameter updates to the next state; compute the deterministic results hash.
3. Lock the classic mempool, call `Commit`, and update/recheck the mempool.
4. Save the resulting application hash and consensus state, request pruning, and
   publish execution events.

The application must persist its state before `Commit` returns. Do not wait for
synchronous transaction admission from inside `Commit`: classic execution holds
the mempool lock during that call. Admission checks, queries and background
snapshot work must synchronize their access to application state; direct calls
through the proxy do not provide that synchronization.

The actual response fields are defined in
[`abci/types/types.pb.go`](../../abci/types/types.pb.go):

- `ResponseFinalizeBlock` carries `Events`, ordered `TxResults`, `ValidatorUpdates`,
  `ConsensusParamUpdates` and `AppHash`.
- `ResponseCommit` carries `RetainHeight`. It has no `Data` or application hash
  field. The archive's instructions to return a Merkle root in
  `ResponseCommit.Data` do not apply.
- `Info` must report `LastBlockHeight` and `LastBlockAppHash` for the last durable
  application commit, including after process restart.

### Autobahn execution and recovery

With `autobahn-config-file` set,
[`GigaRouter.runExecute` and `executeBlock`](../../internal/p2p/giga_router.go)
consume finalized global blocks in increasing global number. This path does not
invoke `ProcessProposal`, and does not implement the archive's speculative
`PrepareProposal` execution modes.

The request uses `GlobalBlock.GlobalNumber` as `Header.Height`, the global block's
timestamp as `Header.Time`, and the **lane-header hash** as
`RequestFinalizeBlock.Hash`. `DecidedLastCommit` is empty by design. The adapter
selects a deterministic proposer address from `GetValidators`; that address is
not a classic proposer-election proof. Autobahn finality comes from its commit
certificate, not from signatures in this adapter's last-commit field.

For each block the adapter calls `FinalizeBlock`, saves the response, calls
`Commit`, then reports `ResponseFinalizeBlock.AppHash` through `PushAppHash`.
Only afterward does it publish `NewBlockHeader` and per-transaction `Tx` events.
Return exactly one non-nil transaction result for each input transaction, in the
same order: the event publisher uses each result's matching input index.
[`PushAppHash`](../../internal/autobahn/data/state.go) waits for the block and its
certificate to be persisted before the data layer can issue application votes.

At fresh initialization (`Info.LastBlockHeight == 0`), `runExecute` calls
`InitChain` and starts at the genesis initial height. After a durable commit it
restores the last app hash into the data layer and resumes at the next height.
The node bypasses the classic handshaker in this mode. The archived three-height
replay procedure is therefore not an Autobahn recovery recipe.

Persist the application store together with the Autobahn consensus safety state
and data WALs described in the [overview](../README.md#application-state-and-durable-restart).
The Autobahn JSON key is `persistent_state_dir`; ordinary configurations require
it. Application `RetainHeight` controls data and saved-response pruning, so an
application must not request removal of history still needed for recovery.
Returning from `Commit` with only in-memory state breaks this restart contract.

### Transaction admission and results

`CheckTx` is admission, not final execution. A successful check does not establish
inclusion or a successful execution result. The proxy's `CheckTxSafe` rejects a
nil response; an EVM response must also supply a nonzero `EVMHash` and a nonempty
`PaxSenderAddress`. The application must implement replay protection in its
execution path as well as admission checks: a local duplicate cache cannot be a
consensus guarantee.

Every correct executor must derive the same application state and deterministic
transaction results from the same ordered block and preceding state. A nonzero
`ExecTxResult.Code` records a failed transaction that is still part of the block;
it does not remove that transaction from the input order. Do not infer that
failed transactions cannot be indexed from the archived draft's wording.

Classic [`TxResultsHash`](../../internal/state/execution.go) hashes the canonical
transaction-result encoding from
[`MarshalTxResults`](../../abci/types/types.go). Deterministic result fields
are `Code`, `Data`, `GasWanted` and `GasUsed`; logs and events are not leaves of
that commitment. Gas accounting and resource limits must be enforced by the
application and the selected admission/producer path. The archive's gas defaults
and Tendermint version history are not current network configuration values.

### Validators, parameters, queries and snapshots

The classic executor validates and applies `ValidatorUpdates` and
`ConsensusParamUpdates` from finalization. In contrast, Autobahn constructs its
committee from configured validator keys at startup; `executeBlock` does not
apply those response fields to change that committee. Do not assume that the
archive's validator-update timing or consensus timeout list configures Autobahn.
Use the [current configuration](../../config/config.go) and
[Autobahn JSON type](../../config/autobahn.go), rather than the draft's v0.36.x
parameter list.

`Query` reads the local application's state; it does not itself run consensus.
An application proof must be checked against an authenticated application-state
commitment for the relevant height. A Tendermint-shaped Autobahn RPC response
alone does not establish the classic light client's header/proof assumptions.
The archive's generic proof discussion remains background material.

The snapshot methods remain in `Application`, but
[node construction](../../node/node.go) installs the classic state-sync reactor
only on the non-Autobahn path. The classic TOML setting is `[statesync]` with
`enable = true` (not the archive's `statesync.enabled`). Snapshot consistency,
height/hash verification and untrusted metadata handling remain useful design
requirements. The archived snapshot workflow is not an Autobahn bootstrap
procedure or a claim that every application implements snapshots.

## Archived upstream draft

The remainder of this page preserves the upstream ABCI++ draft as historical
material, including its original section anchors and links to companion drafts.
All references below to Tendermint versions, methods, transports, parameters,
execution modes and recovery algorithms belong to that draft. They must be read
with the [current fork requirements](#current-fork-requirements) and the
[mode applicability map](../README.md#mode-applicability); they assert no
current-network or Autobahn conformance.

---

## Formal Requirements

This section specifies what Tendermint expects from the Application. It is structured as a set
of formal requirements that can be used for testing and verification of the Application's logic.

Let _p_ and _q_ be two different correct proposers in rounds _r<sub>p</sub>_ and _r<sub>q</sub>_
respectively, in height _h_.
Let _s<sub>p,h-1</sub>_ be _p_'s Application's state committed for height _h-1_.
Let _v<sub>p</sub>_ (resp. _v<sub>q</sub>_) be the block that _p_'s (resp. _q_'s) Tendermint passes
on to the Application
via `RequestPrepareProposal` as proposer of round _r<sub>p</sub>_ (resp _r<sub>q</sub>_), height _h_,
also known as the raw proposal.
Let _v'<sub>p</sub>_ (resp. _v'<sub>q</sub>_) the possibly modified block _p_'s (resp. _q_'s) Application
returns via `ResponsePrepareProposal` to Tendermint, also known as the prepared proposal.

Process _p_'s prepared proposal can differ in two different rounds where _p_ is the proposer.

- Requirement 1 [`PrepareProposal`, header-changes]: When the blockchain is in same-block execution mode,
  _p_'s Application provides values for the following parameters in `ResponsePrepareProposal`:
  `AppHash`, `TxResults`, `ConsensusParams`, `ValidatorUpdates`. Provided values for
  `ConsensusParams` and `ValidatorUpdates` MAY be empty to denote that the Application
  wishes to keep the current values.

Parameters `AppHash`, `TxResults`, `ConsensusParams`, and `ValidatorUpdates` are used by Tendermint to
compute various hashes in the block header that will finally be part of the proposal.

- Requirement 2 [`PrepareProposal`, no-header-changes]: When the blockchain is in next-block execution
  mode, _p_'s Application does not provide values for the following parameters in `ResponsePrepareProposal`:
  `AppHash`, `TxResults`, `ConsensusParams`, `ValidatorUpdates`.

In practical terms, Requirements 1 and 2 imply that Tendermint will (a) panic if the Application is in
same-block execution mode and _does not_ provide values for
`AppHash`, `TxResults`, `ConsensusParams`, and `ValidatorUpdates`, or
(b) log an error if the Application is in next-block execution mode and _does_ provide values for
`AppHash`, `TxResults`, `ConsensusParams`, or `ValidatorUpdates` (the values provided will be ignored).

- Requirement 3 [`PrepareProposal`, timeliness]: If _p_'s Application fully executes prepared blocks in
  `PrepareProposal` and the network is in a synchronous period while processes _p_ and _q_ are in _r<sub>p</sub>_,
  then the value of _TimeoutPropose_ at _q_ must be such that _q_'s propose timer does not time out
  (which would result in _q_ prevoting `nil` in _r<sub>p</sub>_).

Full execution of blocks at `PrepareProposal` time stands on Tendermint's critical path. Thus,
Requirement 3 ensures the Application will set a value for `TimeoutPropose` such that the time it takes
to fully execute blocks in `PrepareProposal` does not interfere with Tendermint's propose timer.

- Requirement 4 [`PrepareProposal`, tx-size]: When _p_'s Application calls `ResponsePrepareProposal`, the
  total size in bytes of the transactions returned does not exceed `RequestPrepareProposal.max_tx_bytes`.

Busy blockchains might seek to maximize the amount of transactions included in each block. Under those conditions,
Tendermint might choose to increase the transactions passed to the Application via `RequestPrepareProposal.txs`
beyond the `RequestPrepareProposal.max_tx_bytes` limit. The idea is that, if the Application drops some of
those transactions, it can still return a transaction list whose byte size is as close to
`RequestPrepareProposal.max_tx_bytes` as possible. Thus, Requirement 4 ensures that the size in bytes of the
transaction list returned by the application will never cause the resulting block to go beyond its byte size
limit.

- Requirement 5 [`PrepareProposal`, `ProcessProposal`, coherence]: For any two correct processes _p_ and _q_,
  if _q_'s Tendermint calls `RequestProcessProposal` on _v'<sub>p</sub>_,
  _q_'s Application returns Accept in `ResponseProcessProposal`.

Requirement 5 makes sure that blocks proposed by correct processes _always_ pass the correct receiving process's
`ProcessProposal` check.
On the other hand, if there is a deterministic bug in `PrepareProposal` or `ProcessProposal` (or in both),
strictly speaking, this makes all processes that hit the bug byzantine. This is a problem in practice,
as very often validators are running the Application from the same codebase, so potentially _all_ would
likely hit the bug at the same time. This would result in most (or all) processes prevoting `nil`, with the
serious consequences on Tendermint's liveness that this entails. Due to its criticality, Requirement 5 is a
target for extensive testing and automated verification.

- Requirement 6 [`ProcessProposal`, determinism-1]: `ProcessProposal` is a (deterministic) function of the current
  state and the block that is about to be applied. In other words, for any correct process _p_, and any arbitrary block _v'_,
  if _p_'s Tendermint calls `RequestProcessProposal` on _v'_ at height _h_,
  then _p_'s Application's acceptance or rejection **exclusively** depends on _v'_ and _s<sub>p,h-1</sub>_.

- Requirement 7 [`ProcessProposal`, determinism-2]: For any two correct processes _p_ and _q_, and any arbitrary
  block _v'_,
  if _p_'s (resp. _q_'s) Tendermint calls `RequestProcessProposal` on _v'_ at height _h_,
  then _p_'s Application accepts _v'_ if and only if _q_'s Application accepts _v'_.
  Note that this requirement follows from Requirement 6 and the Agreement property of consensus.

Requirements 6 and 7 ensure that all correct processes will react in the same way to a proposed block, even
if the proposer is Byzantine. However, `ProcessProposal` may contain a bug that renders the
acceptance or rejection of the block non-deterministic, and therefore prevents processes hitting
the bug from fulfilling Requirements 6 or 7 (effectively making those processes Byzantine).
In such a scenario, Tendermint's liveness cannot be guaranteed.
Again, this is a problem in practice if most validators are running the same software, as they are likely
to hit the bug at the same point. There is currently no clear solution to help with this situation, so
the Application designers/implementors must proceed very carefully with the logic/implementation
of `ProcessProposal`. As a general rule `ProcessProposal` SHOULD always accept the block.

According to the Tendermint algorithm, a correct process can broadcast at most one precommit
message in round _r_, height _h_.
Since, as stated in the [Methods](./abci++_methods_002_draft.md#extendvote) section, `ResponseExtendVote`
is only called when Tendermint
is about to broadcast a non-`nil` precommit message, a correct process can only produce one vote extension
in round _r_, height _h_.
Let _e<sup>r</sup><sub>p</sub>_ be the vote extension that the Application of a correct process _p_ returns via
`ResponseExtendVote` in round _r_, height _h_.
Let _w<sup>r</sup><sub>p</sub>_ be the proposed block that _p_'s Tendermint passes to the Application via `RequestExtendVote`
in round _r_, height _h_.

- Requirement 8 [`ExtendVote`, `VerifyVoteExtension`, coherence]: For any two correct processes _p_ and _q_, if _q_
  receives _e<sup>r</sup><sub>p</sub>_
  from _p_ in height _h_, _q_'s Application returns Accept in `ResponseVerifyVoteExtension`.

Requirement 8 constrains the creation and handling of vote extensions in a similar way as Requirement 5
constrains the creation and handling of proposed blocks.
Requirement 8 ensures that extensions created by correct processes _always_ pass the `VerifyVoteExtension`
checks performed by correct processes receiving those extensions.
However, if there is a (deterministic) bug in `ExtendVote` or `VerifyVoteExtension` (or in both),
we will face the same liveness issues as described for Requirement 5, as Precommit messages with invalid vote
extensions will be discarded.

- Requirement 9 [`VerifyVoteExtension`, determinism-1]: `VerifyVoteExtension` is a (deterministic) function of
  the current state, the vote extension received, and the prepared proposal that the extension refers to.
  In other words, for any correct process _p_, and any arbitrary vote extension _e_, and any arbitrary
  block _w_, if _p_'s (resp. _q_'s) Tendermint calls `RequestVerifyVoteExtension` on _e_ and _w_ at height _h_,
  then _p_'s Application's acceptance or rejection **exclusively** depends on _e_, _w_ and _s<sub>p,h-1</sub>_.

- Requirement 10 [`VerifyVoteExtension`, determinism-2]: For any two correct processes _p_ and _q_,
  and any arbitrary vote extension _e_, and any arbitrary block _w_,
  if _p_'s (resp. _q_'s) Tendermint calls `RequestVerifyVoteExtension` on _e_ and _w_ at height _h_,
  then _p_'s Application accepts _e_ if and only if _q_'s Application accepts _e_.
  Note that this requirement follows from Requirement 9 and the Agreement property of consensus.

Requirements 9 and 10 ensure that the validation of vote extensions will be deterministic at all
correct processes.
Requirements 9 and 10 protect against arbitrary vote extension data from Byzantine processes,
in a similar way as Requirements 6 and 7 protect against arbitrary proposed blocks.
Requirements 9 and 10 can be violated by a bug inducing non-determinism in
`VerifyVoteExtension`. In this case liveness can be compromised.
Extra care should be put in the implementation of `ExtendVote` and `VerifyVoteExtension`.
As a general rule, `VerifyVoteExtension` SHOULD always accept the vote extension.

- Requirement 11 [*all*, no-side-effects]: _p_'s calls to `RequestPrepareProposal`,
  `RequestProcessProposal`, `RequestExtendVote`, and `RequestVerifyVoteExtension` at height _h_ do
  not modify _s<sub>p,h-1</sub>_.

- Requirement 12 [`ExtendVote`, `FinalizeBlock`, non-dependency]: for any correct process _p_,
  and any vote extension _e_ that _p_ received at height _h_, the computation of
  _s<sub>p,h</sub>_ does not depend on _e_.

The call to correct process _p_'s `RequestFinalizeBlock` at height _h_, with block _v<sub>p,h</sub>_
passed as parameter, creates state _s<sub>p,h</sub>_.
Additionally,

- in next-block execution mode, _p_'s `FinalizeBlock` creates a set of transaction results _T<sub>p,h</sub>_,
- in same-block execution mode, _p_'s `PrepareProposal` creates a set of transaction results _T<sub>p,h</sub>_
  if _p_ was the proposer of _v<sub>p,h</sub>_. If _p_ was not the proposer of _v<sub>p,h</sub>_,
  `ProcessProposal` creates _T<sub>p,h</sub>_. `FinalizeBlock` MAY re-create _T<sub>p,h</sub>_ if it was
  removed from memory during the execution of height _h_.

- Requirement 13 [`FinalizeBlock`, determinism-1]: For any correct process _p_,
  _s<sub>p,h</sub>_ exclusively depends on _s<sub>p,h-1</sub>_ and _v<sub>p,h</sub>_.

- Requirement 14 [`FinalizeBlock`, determinism-2]: For any correct process _p_,
  the contents of _T<sub>p,h</sub>_ exclusively depend on _s<sub>p,h-1</sub>_ and _v<sub>p,h</sub>_.

Note that Requirements 13 and 14, combined with Agreement property of consensus ensure
state machine replication, i.e., the Application state evolves consistently at all correct processes.

Finally, notice that neither `PrepareProposal` nor `ExtendVote` have determinism-related
requirements associated.
Indeed, `PrepareProposal` is not required to be deterministic:

- _v'<sub>p</sub>_ may depend on _v<sub>p</sub>_ and _s<sub>p,h-1</sub>_, but may also depend on other values or operations.
- _v<sub>p</sub> = v<sub>q</sub> &#8655; v'<sub>p</sub> = v'<sub>q</sub>_.

Likewise, `ExtendVote` can also be non-deterministic:

- _e<sup>r</sup><sub>p</sub>_ may depend on _w<sup>r</sup><sub>p</sub>_ and _s<sub>p,h-1</sub>_,
  but may also depend on other values or operations.
- _w<sup>r</sup><sub>p</sub> = w<sup>r</sup><sub>q</sub> &#8655;
  e<sup>r</sup><sub>p</sub> = e<sup>r</sup><sub>q</sub>_

## Managing the Application state and related topics

### Connection State

Tendermint maintains four concurrent ABCI++ connections, namely
[Consensus Connection](#consensus-connection),
[Mempool Connection](#mempool-connection),
[Info/Query Connection](#infoquery-connection), and
[Snapshot Connection](#snapshot-connection).
It is common for an application to maintain a distinct copy of
the state for each connection, which are synchronized upon `Commit` calls.

#### Concurrency

In principle, each of the four ABCI++ connections operates concurrently with one
another. This means applications need to ensure access to state is
thread safe. Up to v0.35.x, both the
[default in-process ABCI client](https://github.com/tendermint/tendermint/blob/v0.35.x/abci/client/local_client.go#L18)
and the
[default Go ABCI server](https://github.com/tendermint/tendermint/blob/v0.35.x/abci/server/socket_server.go#L32)
used a global lock to guard the handling of events across all connections, so they were not
concurrent at all. This meant whether your app was compiled in-process with
Tendermint using the `NewLocalClient`, or run out-of-process using the `SocketServer`,
ABCI messages from all connections were received in sequence, one at a
time.
This is no longer the case starting from v0.36.0: the global locks have been removed and it is
up to the Application to synchronize access to its state when handling
ABCI++ methods on all connections.
Nevertheless, as all ABCI calls are now synchronous, ABCI messages using the same connection are
still received in sequence.

#### FinalizeBlock

When the consensus algorithm decides on a block, Tendermint uses `FinalizeBlock` to send the
decided block's data to the Application, which uses it to transition its state.

The Application must remember the latest height from which it
has run a successful `Commit` so that it can tell Tendermint where to
pick up from when it recovers from a crash. See information on the Handshake
[here](#crash-recovery).

#### Commit

The Application should persist its state during `Commit`, before returning from it.

Before invoking `Commit`, Tendermint locks the mempool and flushes the mempool connection. This ensures that
no new messages
will be received on the mempool connection during this processing step, providing an opportunity to safely
update all four
connection states to the latest committed state at the same time.

When `Commit` returns, Tendermint unlocks the mempool.

WARNING: if the ABCI app logic processing the `Commit` message sends a
`/broadcast_tx_sync` or `/broadcast_tx` and waits for the response
before proceeding, it will deadlock. Executing `broadcast_tx` calls
involves acquiring the mempool lock that Tendermint holds during the `Commit` call.
Synchronous mempool-related calls must be avoided as part of the sequential logic of the
`Commit` function.

#### Candidate States

Tendermint calls `PrepareProposal` when it is about to send a proposed block to the network.
Likewise, Tendermint calls `ProcessProposal` upon reception of a proposed block from the
network. In both cases, the proposed block's data
is disclosed to the Application, in the same conditions as is done in `FinalizeBlock`.
The block data disclosed the to Application by these three methods are the following:

- the transaction list
- the `LastCommit` referring to the previous block
- the block header's hash (except in `PrepareProposal`, where it is not known yet)
- list of validators that misbehaved
- the block's timestamp
- `NextValidatorsHash`
- Proposer address

The Application may decide to _immediately_ execute the given block (i.e., upon `PrepareProposal`
or `ProcessProposal`). There are two main reasons why the Application may want to do this:

- _Avoiding invalid transactions in blocks_.
  In order to be sure that the block does not contain _any_ invalid transaction, there may be
  no way other than fully executing the transactions in the block as though it was the _decided_
  block.
- _Quick `FinalizeBlock` execution_.
  Upon reception of the decided block via `FinalizeBlock`, if that same block was executed
  upon `PrepareProposal` or `ProcessProposal` and the resulting state was kept in memory, the
  Application can simply apply that state (faster) to the main state, rather than reexecuting
  the decided block (slower).

`PrepareProposal`/`ProcessProposal` can be called many times for a given height. Moreover,
it is not possible to accurately predict which of the blocks proposed in a height will be decided,
being delivered to the Application in that height's `FinalizeBlock`.
Therefore, the state resulting from executing a proposed block, denoted a _candidate state_, should
be kept in memory as a possible final state for that height. When `FinalizeBlock` is called, the Application should
check if the decided block corresponds to one of its candidate states; if so, it will apply it as
its _ExecuteTxState_ (see [Consensus Connection](#consensus-connection) below),
which will be persisted during the upcoming `Commit` call.

Under adverse conditions (e.g., network instability), Tendermint might take many rounds.
In this case, potentially many proposed blocks will be disclosed to the Application for a given height.
By the nature of Tendermint's consensus algorithm, the number of proposed blocks received by the Application
for a particular height cannot be bound, so Application developers must act with care and use mechanisms
to bound memory usage. As a general rule, the Application should be ready to discard candidate states
before `FinalizeBlock`, even if one of them might end up corresponding to the
decided block and thus have to be reexecuted upon `FinalizeBlock`.

### States and ABCI++ Connections

#### Consensus Connection

The Consensus Connection should maintain an _ExecuteTxState_ &mdash; the working state
for block execution. It should be updated by the call to `FinalizeBlock`
during block execution and committed to disk as the "latest
committed state" during `Commit`. Execution of a proposed block (via `PrepareProposal`/`ProcessProposal`)
**must not** update the _ExecuteTxState_, but rather be kept as a separate candidate state until `FinalizeBlock`
confirms which of the candidate states (if any) can be used to update _ExecuteTxState_.

#### Mempool Connection

The mempool Connection maintains _CheckTxState_. Tendermint sequentially processes an incoming
transaction (via RPC from client or P2P from the gossip layer) against _CheckTxState_.
If the processing does not return any error, the transaction is accepted into the mempool
and Tendermint starts gossipping it.
_CheckTxState_ should be reset to the latest committed state
at the end of every `Commit`.

During the execution of a consensus instance, the _CheckTxState_ may be updated concurrently with the
_ExecuteTxState_, as messages may be sent concurrently on the Consensus and Mempool connections.
At the end of the consensus instance, as described above, Tendermint locks the mempool and flushes
the mempool connection before calling `Commit`. This ensures that all pending `CheckTx` calls are
responded to and no new ones can begin.

After the `Commit` call returns, while still holding the mempool lock, `CheckTx` is run again on all
transactions that remain in the node's local mempool after filtering those included in the block.
Parameter `Type` in `RequestCheckTx`
indicates whether an incoming transaction is new (`CheckTxType_New`), or a
recheck (`CheckTxType_Recheck`).

Finally, after re-checking transactions in the mempool, Tendermint will unlock
the mempool connection. New transactions are once again able to be processed through `CheckTx`.

Note that `CheckTx` is just a weak filter to keep invalid transactions out of the mempool and,
utimately, ouf of the blockchain.
Since the transaction cannot be guaranteed to be checked against the exact same state as it
will be executed as part of a (potential) decided block, `CheckTx` shouldn't check _everything_
that affects the transaction's validity, in particular those checks whose validity may depend on
transaction ordering. `CheckTx` is weak because a Byzantine node need not care about `CheckTx`;
it can propose a block full of invalid transactions if it wants. The mechanism ABCI++ has
in place for dealing with such behavior is `ProcessProposal`.

##### Replay Protection

It is possible for old transactions to be sent again to the Application. This is typically
undesirable for all transactions, except for a generally small subset of them which are idempotent.

The mempool has a mechanism to prevent duplicated transactions from being processed.
This mechanism is nevertheless best-effort (currently based on the indexer)
and does not provide any guarantee of non duplication.
It is thus up to the Application to implement an application-specific
replay protection mechanism with strong guarantees as part of the logic in `CheckTx`.

#### Info/Query Connection

The Info (or Query) Connection should maintain a `QueryState`. This connection has two
purposes: 1) having the application answer the queries Tendermint receives from users
(see section [Query](#query)),
and 2) synchronizing Tendermint and the Application at start up time (see
[Crash Recovery](#crash-recovery))
or after state sync (see [State Sync](#state-sync)).

`QueryState` is a read-only copy of _ExecuteTxState_ as it was after the last
`Commit`, i.e.
after the full block has been processed and the state committed to disk.

#### Snapshot Connection

The Snapshot Connection is used to serve state sync snapshots for other nodes
and/or restore state sync snapshots to a local node being bootstrapped.
Snapshop management is optional: an Application may choose not to implement it.

For more information, see Section [State Sync](#state-sync).

### Transaction Results

The Application is expected to return a list of
[`ExecTxResult`](./abci%2B%2B_methods_002_draft.md#exectxresult) in
[`ResponseFinalizeBlock`](./abci%2B%2B_methods_002_draft.md#finalizeblock). The list of transaction
results must respect the same order as the list of transactions delivered via
[`RequestFinalizeBlock`](./abci%2B%2B_methods_002_draft.md#finalizeblock).
This section discusses the fields inside this structure, along with the fields in
[`ResponseCheckTx`](./abci%2B%2B_methods_002_draft.md#checktx),
whose semantics are similar.

The `Info` and `Log` fields are
non-deterministic values for debugging/convenience purposes. Tendermint logs them but they
are otherwise ignored.

#### Gas

Ethereum introduced the notion of _gas_ as an abstract representation of the
cost of the resources consumed by nodes when processing a transaction. Every operation in the
Ethereum Virtual Machine uses some amount of gas.
Gas has a market-variable price based on which miners can accept or reject to execute a
particular operation.

Users propose a maximum amount of gas for their transaction; if the transaction uses less, they get
the difference credited back. Tendermint adopts a similar abstraction,
though uses it only optionally and weakly, allowing applications to define
their own sense of the cost of execution.

In Tendermint, the [ConsensusParams.Block.MaxGas](#consensus-parameters) limits the amount of
total gas that can be used by all transactions in a block.
The default value is `-1`, which means the block gas limit is not enforced, or that the concept of
gas is meaningless.

Responses contain a `GasWanted` and `GasUsed` field. The former is the maximum
amount of gas the sender of a transaction is willing to use, and the latter is how much it actually
used. Applications should enforce that `GasUsed <= GasWanted` &mdash; i.e. transaction execution
or validation should fail before it can use more resources than it requested.

When `MaxGas > -1`, Tendermint enforces the following rules:

- `GasWanted <= MaxGas` for every transaction in the mempool
- `(sum of GasWanted in a block) <= MaxGas` when proposing a block

If `MaxGas == -1`, no rules about gas are enforced.

In v0.35.x and earlier versions, Tendermint does not enforce anything about Gas in consensus,
only in the mempool.
This means it does not guarantee that committed blocks satisfy these rules.
It is the application's responsibility to return non-zero response codes when gas limits are exceeded
when executing the transactions of a block.
Since the introduction of `PrepareProposal` and `ProcessProposal` in v.0.36.x, it is now possible
for the Application to enforce that all blocks proposed (and voted for) in consensus &mdash; and thus all
blocks decided &mdash; respect the `MaxGas` limits described above.

Since the Application should enforce that `GasUsed <= GasWanted` when executing a transaction, and
it can use `PrepareProposal` and `ProcessProposal` to enforce that `(sum of GasWanted in a block) <= MaxGas`
in all proposed or prevoted blocks,
we have:

- `(sum of GasUsed in a block) <= MaxGas` for every block

The `GasUsed` field is ignored by Tendermint.

#### Specifics of `ResponseCheckTx`

If `Code != 0`, it will be rejected from the mempool and hence
not broadcasted to other peers and not included in a proposal block.

`Data` contains the result of the `CheckTx` transaction execution, if any. It does not need to be
deterministic since, given a transaction, nodes' Applications
might have a different _CheckTxState_ values when they receive it and check their validity
via `CheckTx`.
Tendermint ignores this value in `ResponseCheckTx`.

From v0.35.x on, there is a `Priority` field in `ResponseCheckTx` that can be
used to explicitly prioritize transactions in the mempool for inclusion in a block
proposal.

#### Specifics of `ExecTxResult`

`FinalizeBlock` is the workhorse of the blockchain. Tendermint delivers the decided block,
including the list of all its transactions synchronously to the Application.
The block delivered (and thus the transaction order) is the same at all correct nodes as guaranteed
by the Agreement property of Tendermint consensus.

In same block execution mode, field `LastResultsHash` in the block header refers to the results
of all transactions stored in that block. Therefore,
`PrepareProposal` must return `ExecTxResult` so that it can
be used to build the block to be proposed in the current height.

The `Data` field in `ExecTxResult` contains an array of bytes with the transaction result.
It must be deterministic (i.e., the same value must be returned at all nodes), but it can contain arbitrary
data. Likewise, the value of `Code` must be deterministic.
If `Code != 0`, the transaction will be marked invalid,
though it is still included in the block. Invalid transaction are not indexed, as they are
considered analogous to those that failed `CheckTx`.

Both the `Code` and `Data` are included in a structure that is hashed into the
`LastResultsHash` of the block header in the next height (next block execution mode), or the
header of the block to propose in the current height (same block execution mode, `ExecTxResult` as
part of `PrepareProposal`).

`Events` include any events for the execution, which Tendermint will use to index
the transaction by. This allows transactions to be queried according to what
events took place during their execution.

### Updating the Validator Set

The application may set the validator set during
[`InitChain`](./abci%2B%2B_methods_002_draft.md#initchain), and may update it during
[`FinalizeBlock`](./abci%2B%2B_methods_002_draft.md#finalizeblock)
(next block execution mode) or
[`PrepareProposal`](./abci%2B%2B_methods_002_draft.md#prepareproposal)/[`ProcessProposal`](./abci%2B%2B_methods_002_draft.md#processproposal)
(same block execution mode). In all cases, a structure of type
[`ValidatorUpdate`](./abci%2B%2B_methods_002_draft.md#validatorupdate) is returned.

The `InitChain` method, used to initialize the Application, can return a list of validators.
If the list is empty, Tendermint will use the validators loaded from the genesis
file.
If the list returned by `InitChain` is not empty, Tendermint will use its contents as the validator set.
This way the application can set the initial validator set for the
blockchain.

Applications must ensure that a single set of validator updates does not contain duplicates, i.e.
a given public key can only appear once within a given update. If an update includes
duplicates, the block execution will fail irrecoverably.

Structure `ValidatorUpdate` contains a public key, which is used to identify the validator.
Tendermint now only accepts Ed25519 consensus keys for validators.

Structure `ValidatorUpdate` also contains an `int64` field denoting the validator's new power.
Applications must ensure that
`ValidatorUpdate` structures abide by the following rules:

- power must be non-negative
- if power is set to 0, the validator must be in the validator set; it will be removed from the set
- if power is greater than 0:
  - if the validator is not in the validator set, it will be added to the
    set with the given power
  - if the validator is in the validator set, its power will be adjusted to the given power
- the total power of the new validator set must not exceed `MaxTotalVotingPower`, where
  `MaxTotalVotingPower = MaxInt64 / 8`

Note the updates returned after processing the block at height `H` will only take effect
at block `H+2` (see Section [Methods](./abci%2B%2B_methods_002_draft.md)).

### Consensus Parameters

`ConsensusParams` are global parameters that apply to all validators in a blockchain.
They enforce certain limits in the blockchain, like the maximum size
of blocks, amount of gas used in a block, and the maximum acceptable age of
evidence. They can be set in
[`InitChain`](./abci%2B%2B_methods_002_draft.md#initchain), and updated in
[`FinalizeBlock`](./abci%2B%2B_methods_002_draft.md#finalizeblock)
(next block execution mode) or
[`PrepareProposal`](./abci%2B%2B_methods_002_draft.md#prepareproposal)/[`ProcessProposal`](./abci%2B%2B_methods_002_draft.md#processproposal)
(same block execution model).
These parameters are deterministically set and/or updated by the Application, so
all full nodes have the same value at a given height.

#### List of Parameters

These are the current consensus parameters (as of v0.36.x):

1. [BlockParams.MaxBytes](#blockparamsmaxbytes)
2. [BlockParams.MaxGas](#blockparamsmaxgas)
3. [EvidenceParams.MaxAgeDuration](#evidenceparamsmaxageduration)
4. [EvidenceParams.MaxAgeNumBlocks](#evidenceparamsmaxagenumblocks)
5. [EvidenceParams.MaxBytes](#evidenceparamsmaxbytes)
6. [SynchronyParams.MessageDelay](#synchronyparamsmessagedelay)
7. [SynchronyParams.Precision](#synchronyparamsprecision)
8. [TimeoutParams.Propose](#timeoutparamspropose)
9. [TimeoutParams.ProposeDelta](#timeoutparamsproposedelta)
10. [TimeoutParams.Vote](#timeoutparamsvote)
11. [TimeoutParams.VoteDelta](#timeoutparamsvotedelta)
12. [TimeoutParams.Commit](#timeoutparamscommit)
13. [TimeoutParams.BypassCommitTimeout](#timeoutparamsbypasscommittimeout)

##### BlockParams.MaxBytes

The maximum size of a complete Protobuf encoded block.
This is enforced by Tendermint consensus.

This implies a maximum transaction size that is this `MaxBytes`, less the expected size of
the header, the validator set, and any included evidence in the block.

Must have `0 < MaxBytes < 100 MB`.

##### BlockParams.MaxGas

The maximum of the sum of `GasWanted` that will be allowed in a proposed block.
This is _not_ enforced by Tendermint consensus.
It is left to the Application to enforce (ie. if transactions are included past the
limit, they should return non-zero codes). It is used by Tendermint to limit the
transactions included in a proposed block.

Must have `MaxGas >= -1`.
If `MaxGas == -1`, no limit is enforced.

##### EvidenceParams.MaxAgeDuration

This is the maximum age of evidence in time units.
This is enforced by Tendermint consensus.

If a block includes evidence older than this (AND the evidence was created more
than `MaxAgeNumBlocks` ago), the block will be rejected (validators won't vote
for it).

Must have `MaxAgeDuration > 0`.

##### EvidenceParams.MaxAgeNumBlocks

This is the maximum age of evidence in blocks.
This is enforced by Tendermint consensus.

If a block includes evidence older than this (AND the evidence was created more
than `MaxAgeDuration` ago), the block will be rejected (validators won't vote
for it).

Must have `MaxAgeNumBlocks > 0`.

##### EvidenceParams.MaxBytes

This is the maximum size of total evidence in bytes that can be committed to a
single block. It should fall comfortably under the max block bytes.

Its value must not exceed the size of
a block minus its overhead ( ~ `BlockParams.MaxBytes`).

Must have `MaxBytes > 0`.

##### SynchronyParams.MessageDelay

This sets a bound on how long a proposal message may take to reach all
validators on a network and still be considered valid.

This parameter is part of the
[proposer-based timestamps](../consensus/proposer-based-timestamp)
(PBTS) algorithm.

##### SynchronyParams.Precision

This sets a bound on how skewed a proposer's clock may be from any validator
on the network while still producing valid proposals.

This parameter is part of the
[proposer-based timestamps](../consensus/proposer-based-timestamp)
(PBTS) algorithm.

##### TimeoutParams.Propose

Timeout in ms of the propose step of the Tendermint consensus algorithm.
This value is the initial timeout at every height (round 0).

The value in subsequent rounds is modified by parameter `ProposeDelta`.
When a new height is started, the `Propose` timeout value is reset to this
parameter.

If a node waiting for a proposal message does not receive one matching its
current height and round before this timeout, the node will issue a
`nil` prevote for the round and advance to the next step.

##### TimeoutParams.ProposeDelta

Increment in ms to be added to the `Propose` timeout every time the Tendermint
consensus algorithm advances one round in a given height.

When a new height is started, the `Propose` timeout value is reset.

##### TimeoutParams.Vote

Timeout in ms of the prevote and precommit steps of the Tendermint consensus
algorithm.
This value is the initial timeout at every height (round 0).

The value in subsequent rounds is modified by parameter `VoteDelta`.
When a new height is started, the `Vote` timeout value is reset to this
parameter.

The `Vote` timeout does not begin until a quorum of votes has been received.
Once a quorum of votes has been seen and this timeout elapses, Tendermint will
procced to the next step of the consensus algorithm. If Tendermint receives
all of the remaining votes before the end of the timeout, it will proceed
to the next step immediately.

##### TimeoutParams.VoteDelta

Increment in ms to be added to the `Vote` timeout every time the Tendermint
consensus algorithm advances one round in a given height.

When a new height is started, the `Vote` timeout value is reset.

##### TimeoutParams.Commit

This configures how long Tendermint will wait after receiving a quorum of
precommits before beginning consensus for the next height. This can be
used to allow slow precommits to arrive for inclusion in the next height
before progressing.

##### TimeoutParams.BypassCommitTimeout

This configures the node to proceed immediately to the next height once the
node has received all precommits for a block, forgoing the remaining commit timeout.
Setting this parameter to `false` (the default) causes Tendermint to wait
for the full commit timeout configured in `TimeoutParams.Commit`.

#### Updating Consensus Parameters

The application may set the `ConsensusParams` during
[`InitChain`](./abci%2B%2B_methods_002_draft.md#initchain),
and update them during
[`FinalizeBlock`](./abci%2B%2B_methods_002_draft.md#finalizeblock)
(next block execution mode) or
[`PrepareProposal`](./abci%2B%2B_methods_002_draft.md#prepareproposal)/[`ProcessProposal`](./abci%2B%2B_methods_002_draft.md#processproposal)
(same block execution mode).
If the `ConsensusParams` is empty, it will be ignored. Each field
that is not empty will be applied in full. For instance, if updating the
`Block.MaxBytes`, applications must also set the other `Block` fields (like
`Block.MaxGas`), even if they are unchanged, as they will otherwise cause the
value to be updated to the default.

##### `InitChain`

`ResponseInitChain` includes a `ConsensusParams` parameter.
If `ConsensusParams` is `nil`, Tendermint will use the params loaded in the genesis
file. If `ConsensusParams` is not `nil`, Tendermint will use it.
This way the application can determine the initial consensus parameters for the
blockchain.

##### `FinalizeBlock`, `PrepareProposal`/`ProcessProposal`

In next block execution mode, `ResponseFinalizeBlock` accepts a `ConsensusParams` parameter.
If `ConsensusParams` is `nil`, Tendermint will do nothing.
If `ConsensusParams` is not `nil`, Tendermint will use it.
This way the application can update the consensus parameters over time.

Likewise, in same block execution mode, `PrepareProposal` and `ProcessProposal` include
a `ConsensusParams` parameter. `PrepareProposal` may return a `ConsensusParams` to update
the consensus parameters in the block that is about to be proposed. If it returns `nil`
the consensus parameters will not be updated. `ProcessProposal` also accepts a
`ConsensusParams` parameter, which Tendermint will use it to calculate the corresponding
hashes and sanity-check them against those of the block that triggered `ProcessProposal`
at the first place.

Note the updates returned in block `H` will take effect right away for block
`H+1` (both in next block and same block execution mode).

### `Query`

`Query` is a generic method with lots of flexibility to enable diverse sets
of queries on application state. Tendermint makes use of `Query` to filter new peers
based on ID and IP, and exposes `Query` to the user over RPC.

Note that calls to `Query` are not replicated across nodes, but rather query the
local node's state - hence they may return stale reads. For reads that require
consensus, use a transaction.

The most important use of `Query` is to return Merkle proofs of the application state at some height
that can be used for efficient application-specific light-clients.

Note Tendermint has technically no requirements from the `Query`
message for normal operation - that is, the ABCI app developer need not implement
Query functionality if they do not wish to.

#### Query Proofs

The Tendermint block header includes a number of hashes, each providing an
anchor for some type of proof about the blockchain. The `ValidatorsHash` enables
quick verification of the validator set, the `DataHash` gives quick
verification of the transactions included in the block.

The `AppHash` is unique in that it is application specific, and allows for
application-specific Merkle proofs about the state of the application.
While some applications keep all relevant state in the transactions themselves
(like Bitcoin and its UTXOs), others maintain a separated state that is
computed deterministically _from_ transactions, but is not contained directly in
the transactions themselves (like Ethereum contracts and accounts).
For such applications, the `AppHash` provides a much more efficient way to verify light-client proofs.

ABCI applications can take advantage of more efficient light-client proofs for
their state as follows:

- in next block executon mode, return the Merkle root of the deterministic application state in
  `ResponseCommit.Data`. This Merkle root will be included as the `AppHash` in the next block.
- in same block execution mode, return the Merkle root of the deterministic application state
  in `ResponsePrepareProposal.AppHash`. This Merkle root will be included as the `AppHash` in
  the block that is about to be proposed.
- return efficient Merkle proofs about that application state in `ResponseQuery.Proof`
  that can be verified using the `AppHash` of the corresponding block.

For instance, this allows an application's light-client to verify proofs of
absence in the application state, something which is much less efficient to do using the block hash.

Some applications (eg. Ethereum, Cosmos-SDK) have multiple "levels" of Merkle trees,
where the leaves of one tree are the root hashes of others. To support this, and
the general variability in Merkle proofs, the `ResponseQuery.Proof` has some minimal structure:

```protobuf
message ProofOps {
  repeated ProofOp ops = 1
}

message ProofOp {
  string type = 1;
  bytes key   = 2;
  bytes data  = 3;
}
```

Each `ProofOp` contains a proof for a single key in a single Merkle tree, of the specified `type`.
This allows ABCI to support many different kinds of Merkle trees, encoding
formats, and proofs (eg. of presence and absence) just by varying the `type`.
The `data` contains the actual encoded proof, encoded according to the `type`.
When verifying the full proof, the root hash for one ProofOp is the value being
verified for the next ProofOp in the list. The root hash of the final ProofOp in
the list should match the `AppHash` being verified against.

#### Paths

Queries are directed at paths, and may optionally include additional data.

The expectation is for there to be some number of high level paths
differentiating concerns, like `/store` and `/app`. For more advanced use, see the
implementation of
[Query in the Cosmos-SDK](https://github.com/cosmos/cosmos-sdk/blob/v0.23.1/baseapp/baseapp.go#L333).

### Crash Recovery

On startup, Tendermint calls the `Info` method on the Info Connection to get the latest
committed state of the app. The app MUST return information consistent with the
last block it succesfully completed Commit for.

If the app succesfully committed block H, then `last_block_height = H` and `last_block_app_hash = <hash returned by Commit for block H>`. If the app
failed during the Commit of block H, then `last_block_height = H-1` and
`last_block_app_hash = <hash returned by Commit for block H-1, which is the hash in the header of block H>`.

We now distinguish three heights, and describe how Tendermint syncs itself with
the app.

```md
storeBlockHeight = height of the last block Tendermint saw a commit for
stateBlockHeight = height of the last block for which Tendermint completed all
block processing and saved all ABCI results to disk
appBlockHeight = height of the last block for which ABCI app succesfully
completed Commit
```

Note we always have `storeBlockHeight >= stateBlockHeight` and `storeBlockHeight >= appBlockHeight`
Note also Tendermint never calls Commit on an ABCI app twice for the same height.

The procedure is as follows.

First, some simple start conditions:

If `appBlockHeight == 0`, then call InitChain.

If `storeBlockHeight == 0`, we're done.

Now, some sanity checks:

If `storeBlockHeight < appBlockHeight`, error
If `storeBlockHeight < stateBlockHeight`, panic
If `storeBlockHeight > stateBlockHeight+1`, panic

Now, the meat:

If `storeBlockHeight == stateBlockHeight && appBlockHeight < storeBlockHeight`,
replay all blocks in full from `appBlockHeight` to `storeBlockHeight`.
This happens if we completed processing the block, but the app forgot its height.

If `storeBlockHeight == stateBlockHeight && appBlockHeight == storeBlockHeight`, we're done.
This happens if we crashed at an opportune spot.

If `storeBlockHeight == stateBlockHeight+1`
This happens if we started processing the block but didn't finish.

If `appBlockHeight < stateBlockHeight`
replay all blocks in full from `appBlockHeight` to `storeBlockHeight-1`,
and replay the block at `storeBlockHeight` using the WAL.
This happens if the app forgot the last block it committed.

If `appBlockHeight == stateBlockHeight`,
replay the last block (storeBlockHeight) in full.
This happens if we crashed before the app finished Commit

If `appBlockHeight == storeBlockHeight`
update the state using the saved ABCI responses but dont run the block against the real app.
This happens if we crashed after the app finished Commit but before Tendermint saved the state.

### State Sync

A new node joining the network can simply join consensus at the genesis height and replay all
historical blocks until it is caught up. However, for large chains this can take a significant
amount of time, often on the order of days or weeks.

State sync is an alternative mechanism for bootstrapping a new node, where it fetches a snapshot
of the state machine at a given height and restores it. Depending on the application, this can
be several orders of magnitude faster than replaying blocks.

Note that state sync does not currently backfill historical blocks, so the node will have a
truncated block history - users are advised to consider the broader network implications of this in
terms of block availability and auditability. This functionality may be added in the future.

For details on the specific ABCI calls and types, see the
[methods](abci%2B%2B_methods_002_draft.md) section.

#### Taking Snapshots

Applications that want to support state syncing must take state snapshots at regular intervals. How
this is accomplished is entirely up to the application. A snapshot consists of some metadata and
a set of binary chunks in an arbitrary format:

- `Height (uint64)`: The height at which the snapshot is taken. It must be taken after the given
  height has been committed, and must not contain data from any later heights.

- `Format (uint32)`: An arbitrary snapshot format identifier. This can be used to version snapshot
  formats, e.g. to switch from Protobuf to MessagePack for serialization. The application can use
  this when restoring to choose whether to accept or reject a snapshot.

- `Chunks (uint32)`: The number of chunks in the snapshot. Each chunk contains arbitrary binary
  data, and should be less than 16 MB; 10 MB is a good starting point.

- `Hash ([]byte)`: An arbitrary hash of the snapshot. This is used to check whether a snapshot is
  the same across nodes when downloading chunks.

- `Metadata ([]byte)`: Arbitrary snapshot metadata, e.g. chunk hashes for verification or any other
  necessary info.

For a snapshot to be considered the same across nodes, all of these fields must be identical. When
sent across the network, snapshot metadata messages are limited to 4 MB.

When a new node is running state sync and discovering snapshots, Tendermint will query an existing
application via the ABCI `ListSnapshots` method to discover available snapshots, and load binary
snapshot chunks via `LoadSnapshotChunk`. The application is free to choose how to implement this
and which formats to use, but must provide the following guarantees:

- **Consistent:** A snapshot must be taken at a single isolated height, unaffected by
  concurrent writes. This can be accomplished by using a data store that supports ACID
  transactions with snapshot isolation.

- **Asynchronous:** Taking a snapshot can be time-consuming, so it must not halt chain progress,
  for example by running in a separate thread.

- **Deterministic:** A snapshot taken at the same height in the same format must be identical
  (at the byte level) across nodes, including all metadata. This ensures good availability of
  chunks, and that they fit together across nodes.

A very basic approach might be to use a datastore with MVCC transactions (such as RocksDB),
start a transaction immediately after block commit, and spawn a new thread which is passed the
transaction handle. This thread can then export all data items, serialize them using e.g.
Protobuf, hash the byte stream, split it into chunks, and store the chunks in the file system
along with some metadata - all while the blockchain is applying new blocks in parallel.

A more advanced approach might include incremental verification of individual chunks against the
chain app hash, parallel or batched exports, compression, and so on.

Old snapshots should be removed after some time - generally only the last two snapshots are needed
(to prevent the last one from being removed while a node is restoring it).

#### Bootstrapping a Node

An empty node can be state synced by setting the configuration option `statesync.enabled =
true`. The node also needs the chain genesis file for basic chain info, and configuration for
light client verification of the restored snapshot: a set of Tendermint RPC servers, and a
trusted header hash and corresponding height from a trusted source, via the `statesync`
configuration section.

Once started, the node will connect to the P2P network and begin discovering snapshots. These
will be offered to the local application via the `OfferSnapshot` ABCI method. Once a snapshot
is accepted Tendermint will fetch and apply the snapshot chunks. After all chunks have been
successfully applied, Tendermint verifies the app's `AppHash` against the chain using the light
client, then switches the node to normal consensus operation.

##### Snapshot Discovery

When the empty node joins the P2P network, it asks all peers to report snapshots via the
`ListSnapshots` ABCI call (limited to 10 per node). After some time, the node picks the most
suitable snapshot (generally prioritized by height, format, and number of peers), and offers it
to the application via `OfferSnapshot`. The application can choose a number of responses,
including accepting or rejecting it, rejecting the offered format, rejecting the peer who sent
it, and so on. Tendermint will keep discovering and offering snapshots until one is accepted or
the application aborts.

##### Snapshot Restoration

Once a snapshot has been accepted via `OfferSnapshot`, Tendermint begins downloading chunks from
any peers that have the same snapshot (i.e. that have identical metadata fields). Chunks are
spooled in a temporary directory, and then given to the application in sequential order via
`ApplySnapshotChunk` until all chunks have been accepted.

The method for restoring snapshot chunks is entirely up to the application.

During restoration, the application can respond to `ApplySnapshotChunk` with instructions for how
to continue. This will typically be to accept the chunk and await the next one, but it can also
ask for chunks to be refetched (either the current one or any number of previous ones), P2P peers
to be banned, snapshots to be rejected or retried, and a number of other responses - see the ABCI
reference for details.

If Tendermint fails to fetch a chunk after some time, it will reject the snapshot and try a
different one via `OfferSnapshot` - the application can choose whether it wants to support
restarting restoration, or simply abort with an error.

##### Snapshot Verification

Once all chunks have been accepted, Tendermint issues an `Info` ABCI call to retrieve the
`LastBlockAppHash`. This is compared with the trusted app hash from the chain, retrieved and
verified using the light client. Tendermint also checks that `LastBlockHeight` corresponds to the
height of the snapshot.

This verification ensures that an application is valid before joining the network. However, the
snapshot restoration may take a long time to complete, so applications may want to employ additional
verification during the restore to detect failures early. This might e.g. include incremental
verification of each chunk against the app hash (using bundled Merkle proofs), checksums to
protect against data corruption by the disk or network, and so on. However, it is important to
note that the only trusted information available is the app hash, and all other snapshot metadata
can be spoofed by adversaries.

Apps may also want to consider state sync denial-of-service vectors, where adversaries provide
invalid or harmful snapshots to prevent nodes from joining the network. The application can
counteract this by asking Tendermint to ban peers. As a last resort, node operators can use
P2P configuration options to whitelist a set of trusted peers that can provide valid snapshots.

##### Transition to Consensus

Once the snapshots have all been restored, Tendermint gathers additional information necessary for
bootstrapping the node (e.g. chain ID, consensus parameters, validator sets, and block headers)
from the genesis file and light client RPC servers. It also calls `Info` to verify the following:

- that the app hash from the snapshot it has delivered to the Application matches the apphash
  stored in the next height's block (in next block execution), or the current block's height
  (same block execution)
- that the version that the Application returns in `ResponseInfo` matches the version in the
  current height's block header

Once the state machine has been restored and Tendermint has gathered this additional
information, it transitions to block sync (if enabled) to fetch any remaining blocks up the chain
head, and then transitions to regular consensus operation. At this point the node operates like
any other node, apart from having a truncated block history at the height of the restored snapshot.
