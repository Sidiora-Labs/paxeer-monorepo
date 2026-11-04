---
order: 2
title: Methods
---

# Methods

This page maps the inherited ABCI++ draft to the application API implemented in
this fork. The filename and method anchors are retained for existing links.
The upstream draft's wire schemas, same-block/next-block execution modes and
vote-extension lifecycle do not define the current runtime contract.

Paxeer is one BFT network run by `paxd`, with EVM chain ID **125** (`0x7d`).
The kernel is part of that network and proposes blocks. A transaction receipt
is final when its block is committed. The methods below are internal Go calls,
not public JSON-RPC methods. Public JSON-RPC uses the root path on the node
hosts `api1.mainnet-beta.paxeer.network` through
`api14.mainnet-beta.paxeer.network` and `api-mainnet-beta.paxeer.network`.

## Current interface and mode applicability

The source of truth is [`Application`](../../abci/types/application.go), called
in process through [`internal/proxy`](../../internal/proxy/proxy.go).
There is no socket or gRPC ABCI transport in this fork. Request and response
fields come from three sources:

- [`plain_types.go`](../../abci/types/plain_types.go): Go requests, proposal
  responses, commit information and snapshot types. These have no protobuf
  field numbers.
- [`types.go`](../../abci/types/types.go): local mempool extensions and result
  hashing helpers.
- [`types.proto`](../../proto/tendermint/abci/types.proto): generated response,
  event, validator-update and execution-result messages.

The node selects Autobahn when `autobahn-config-file` is non-empty; an empty
value leaves the classic consensus path enabled. See
[`config.go`](../../config/config.go) and [`node.go`](../../node/node.go).
These are implementations within the same network, not separate application
interfaces or execution/settlement networks.

| Operation | Classic consensus path | Autobahn path |
| --- | --- | --- |
| Startup | Handshake/replay reconciles application and consensus stores. | The execution loop reads `Info`, initializes a fresh application and resumes from its committed height. |
| Admission | Mempool calls `CheckTx` through the local proxy. | The producer calls `CheckTxSafe`, tracks EVM nonces and seals lane payloads. |
| Proposal construction | `BlockExecutor.CreateProposalBlock` reaps transactions or uses the configured sealed source. | The producer builds payloads for the availability layer. |
| Proposal application check | `BlockExecutor.ProcessProposal` calls the application. | The committed-global-block execution loop does not call `ProcessProposal`. |
| Execution | `ApplyBlock` calls `FinalizeBlock`, stores its response, then calls `Commit`. | `executeBlock` calls `FinalizeBlock`, stores its response, calls `Commit`, then publishes the application hash to the data layer. |
| Consensus updates | Validator and consensus parameter updates are validated and applied by the classic executor. | The committee is configured independently; execution does not apply response updates to that committee. |
| Draft hooks | `PrepareProposal`, `ExtendVote` and `VerifyVoteExtension` are absent from `Application`. | The same hooks are absent. |

The call sites are [`execution.go`](../../internal/state/execution.go),
[`producer`](../../internal/autobahn/producer/) and
[`giga_router.go`](../../internal/p2p/giga_router.go). The last file is internal
peer/consensus integration, not a public gateway service.

## Methods existing in ABCI

### Echo

The upstream transport's string echo operation is not a method of the current
`Application` interface. It has no application request/response contract here.

### Flush

There is no application `Flush` method or queued ABCI transport to flush.
The proxy calls the Go application directly and records method timings.

### Info

```go
Info(context.Context, *RequestInfo) (*ResponseInfo, error)
```

`RequestInfo` contains `Version string`, `BlockVersion uint64`,
`P2PVersion uint64` and `AbciVersion string`.

| Response field | Go type | Meaning in `BaseApp.Info` |
| --- | --- | --- |
| `Data` | `string` | Application name. |
| `Version` | `string` | Application software version. |
| `AppVersion` | `uint64` | Application protocol version. |
| `LastBlockHeight` | `int64` | Last committed store version. |
| `LastBlockAppHash` | `[]byte` | Hash from the last store commit. |
| `MinimumGasPrices` | `string` | Configured minimum gas prices. |

[`BaseApp.Info`](../../../sdk/baseapp/abci.go) reads the committed multistore;
it does not report speculative proposal state. Autobahn's `runExecute` uses
this height to decide between genesis initialization and recovery. See
[FinalizeBlock](#finalizeblock) for the restart sequence.

### InitChain

```go
InitChain(context.Context, *RequestInitChain) (*ResponseInitChain, error)
```

| Request field | Go type | Meaning |
| --- | --- | --- |
| `Time` | `time.Time` | Genesis time. |
| `ChainId` | `string` | Consensus chain identifier, distinct from the numeric EVM chain ID. |
| `ConsensusParams` | `*tmproto.ConsensusParams` | Initial consensus parameters. |
| `AppStateBytes` | `[]byte` | Serialized application genesis state. |
| `InitialHeight` | `int64` | First block height. |

The response contains `Validators []ValidatorUpdate` and `AppHash []byte`.
Unlike the upstream draft, the request has no `Validators` field and the
response has no `ConsensusParams` field.

[`BaseApp.InitChain`](../../../sdk/baseapp/abci.go) initializes check, deliver
and proposal state and invokes the configured genesis handler. It does not
commit the store. On a fresh Autobahn start, `runExecute` calls it before
executing the first block, using the genesis document's `InitialHeight`.
After a committed height exists, that loop does not call `InitChain` again.
If initialization was interrupted before the first commit, the next start
initializes again because `Info.LastBlockHeight` is still zero.

### Query

```go
Query(context.Context, *RequestQuery) (*ResponseQuery, error)
```

| Request field | Go type | Meaning |
| --- | --- | --- |
| `Data` | `[]byte` | Query payload. |
| `Path` | `string` | Application query route. |
| `Height` | `int64` | Requested store version; zero selects the latest committed height. |
| `Prove` | `bool` | Request a proof where the route supports it. |

`ResponseQuery` contains `Code uint32`, `Log string`, `Info string`,
`Index int64`, `Key []byte`, `Value []byte`, `ProofOps *crypto.ProofOps`,
`Height int64` and `Codespace string`.

[`BaseApp.Query`](../../../sdk/baseapp/abci.go) first checks registered gRPC
query routes, then dispatches the `app`, `store` and `custom` path prefixes.
These are application query routes, not public HTTP endpoint paths.
Unknown paths return an application error in the response. Proof availability
and encoding depend on the route and store. `CreateQueryContext` rejects
negative or future heights and proof requests at height 1 or below; loading
an unavailable historical store version also fails. Do not assume the
upstream draft's blanket height-minus-one interpretation for these queries.

### CheckTx

```go
CheckTx(context.Context, *RequestCheckTxV2) *ResponseCheckTxV2
```

The request contains `Tx []byte` and `Type CheckTxTypeV2`:
`CheckTxTypeV2New = 0`, `CheckTxTypeV2Recheck = 1`.
The result embeds `*ResponseCheckTx`, whose fields are `Code uint32`,
`Data []byte`, `Log string`, `GasWanted int64`, `Codespace string`,
`Priority int64` and `GasEstimated int64`.

`ResponseCheckTxV2` also supplies local mempool metadata: `IsEVM bool`,
`EVMNonce uint64`, `EVMHash common.Hash`, `EVMSenderAddress common.Address`,
`PaxSenderAddress []byte` and `EVMRequiredBalance uint256.Int`. These are Go
fields, not additional fields in the protobuf `ResponseCheckTx` message.

[`App.CheckTx`](../../../node/abci.go) decodes and checks transactions through
the application's admission logic. `CodeTypeOK` is **0**; a nonzero code
reports admission failure in its `Codespace`. Admission does not establish
execution success or commitment. The node also refuses client submission of
the internal sequence-batch envelope through this method.

[`Proxy.CheckTxSafe`](../../internal/proxy/proxy.go) wraps the call with panic
recovery and rejects a nil response. For EVM responses it requires a nonzero
`EVMHash` and nonempty `PaxSenderAddress`. Autobahn's producer uses this
wrapper before enforcing its payload limits and expected EVM nonce.

### Additional local admission methods

The fork's interface also declares:

```go
GetTxPriorityHint(context.Context, *RequestGetTxPriorityHintV2) (*ResponseGetTxPriorityHint, error)
EvmNonce(common.Address) uint64
EvmBalance(common.Address, []byte) uint256.Int
GetValidators() []ValidatorUpdate
```

The priority request contains `Tx []byte`; its response contains
`Priority int64`. It is a hint, not an admission decision.
`EvmNonce` reads the account nonce from check state. `EvmBalance` reads the EVM
balance and adds the supplied Pax account's balance if its address differs.
`GetValidators` returns the application's bonded validators; at height zero
it can use the uncommitted genesis deliver context. Autobahn execution uses
this list to choose a deterministic proposer address for application hooks;
it does not replace the configured Autobahn committee with this list.
Implementations are in [`node/abci.go`](../../../node/abci.go),
[`node/app.go`](../../../node/app.go) and
[`sdk/baseapp/abci.go`](../../../sdk/baseapp/abci.go).

### ListSnapshots

```go
ListSnapshots(context.Context, *RequestListSnapshots) (*ResponseListSnapshots, error)
```

The request is empty. The response contains `Snapshots []*Snapshot`.
`BaseApp` delegates to its snapshot manager; without one it returns an empty
list. This interface supports application state sync. Its presence does not
establish an Autobahn consensus-WAL restoration protocol.

### LoadSnapshotChunk

```go
LoadSnapshotChunk(context.Context, *RequestLoadSnapshotChunk) (*ResponseLoadSnapshotChunk, error)
```

The request selects `Height uint64`, `Format uint32` and `Chunk uint32`.
The response contains `Chunk []byte`. `BaseApp` loads the chunk through its
snapshot manager. An absent manager or a load error produces an empty
response, with load errors logged. Transport size limits belong to the
state-sync implementation, not this in-process method's schema.

### OfferSnapshot

```go
OfferSnapshot(context.Context, *RequestOfferSnapshot) (*ResponseOfferSnapshot, error)
```

The request contains `Snapshot *Snapshot` and `AppHash []byte`.
The response contains `Result ResponseOfferSnapshot_Result`.

#### Result

| Value | Name | Decision |
| --- | --- | --- |
| 0 | `UNKNOWN` | No valid decision. |
| 1 | `ACCEPT` | Accept the snapshot and apply its chunks. |
| 2 | `ABORT` | Abort restoration. |
| 3 | `REJECT` | Reject this snapshot. |
| 4 | `REJECT_FORMAT` | Reject this snapshot format. |
| 5 | `REJECT_SENDER` | Reject this sender. |

These are the enum values in `plain_types.go`. `BaseApp` returns `ABORT`
without a snapshot manager, `REJECT` for missing or invalid metadata,
`REJECT_FORMAT` for an unsupported format and `ACCEPT` when restoration
starts successfully. Other restore errors return `ABORT`. The application
accepting metadata does not by itself prove restored state: the state-sync
caller must verify the resulting committed height and application hash.
See the [state-sync protocol](../p2p/messages/state-sync.md) for that path.

### ApplySnapshotChunk

```go
ApplySnapshotChunk(context.Context, *RequestApplySnapshotChunk) (*ResponseApplySnapshotChunk, error)
```

The request contains `Index uint32`, `Chunk []byte` and `Sender string`.
The response contains `Result ResponseApplySnapshotChunk_Result`,
`RefetchChunks []uint32` and `RejectSenders []string`.

| Value | Name | Decision |
| --- | --- | --- |
| 0 | `UNKNOWN` | No valid decision. |
| 1 | `ACCEPT` | Accept the chunk. |
| 2 | `ABORT` | Abort restoration. |
| 3 | `RETRY` | Retry the chunk. |
| 4 | `RETRY_SNAPSHOT` | Restart this snapshot. |
| 5 | `REJECT_SNAPSHOT` | Reject this snapshot. |

`BaseApp` delegates chunk restoration to its snapshot manager. A chunk hash
mismatch returns `RETRY`, names the current index in `RefetchChunks` and the
sender in `RejectSenders`. Other restore errors, or a missing manager,
return `ABORT`. A successful chunk returns `ACCEPT`; completion also resets
the inter-block cache if one is present.

### Commit

```go
Commit(context.Context) (*ResponseCommit, error)
```

`ResponseCommit` contains **only** `RetainHeight int64` (protobuf field 3).
The application hash comes from [FinalizeBlock](#finalizeblock), not this
response. `BaseApp.Commit` requires a selected state to commit, writes it to
the multistore, commits the store, refreshes check state and resets execution
states. It also triggers snapshot creation when configured.

`RetainHeight` is the requested pruning boundary. Zero requests no pruning;
positive values allow history below the boundary to be removed by the
caller. Preserve the history required for replay, state synchronization,
proofs and auditing. Autobahn prunes its data and stored finalization
responses using this value. The classic executor submits it to its
asynchronous pruner after saving state.

The node's [`App.Commit`](../../../node/abci.go) publishes the stashed
`newHeads` notification after a successful base application commit, then
calls the configured commit listener. Speculative proposal execution alone
must not be treated as a committed head.

## New methods introduced in ABCI++

### PrepareProposal

There is no `PrepareProposal` method, `RequestPrepareProposal` or
`ResponsePrepareProposal` in the current `Application` API. In the classic
path, `BlockExecutor.CreateProposalBlock` constructs the proposal, using the
sealed transaction source at heights where it is active. Autobahn's producer
constructs lane payloads directly. Neither path uses the draft's transaction
record editing response or its selectable same-block/next-block modes.

### ProcessProposal

```go
ProcessProposal(context.Context, *RequestProcessProposal) (*ResponseProcessProposal, error)
```

| Request field | Go type | Meaning |
| --- | --- | --- |
| `Txs` | `[][]byte` | Proposed transaction sequence. |
| `ProposedLastCommit` | `CommitInfo` | Previous-height commit information. |
| `ByzantineValidators` | `[]Misbehavior` | Misbehavior supplied with the proposal. |
| `Hash` | `[]byte` | Proposed block hash. |
| `Header` | `*tmproto.Header` | Block header, including height, time and proposer. |

The response contains `Status ResponseProcessProposal_ProposalStatus`,
`AppHash []byte`, `TxResults []*ExecTxResult`,
`ValidatorUpdates []ValidatorUpdate` and
`ConsensusParamUpdates *tmproto.ConsensusParams`.

The classic executor consumes the status: `ACCEPT = 1` accepts,
`REJECT = 2` rejects and `UNKNOWN = 0` causes a panic. The response's other
fields do not substitute for the finalization response or commit.
`BaseApp.ProcessProposal` validates the header's chain identifier, prepares
proposal state and invokes [`App.ProcessProposalHandler`](../../../node/app.go).
That handler checks the sequence batch when sequencing is active and can
start optimistic execution. A speculative result may be discarded; it is
not persisted merely because the proposal was accepted.

Autobahn's committed-block execution loop proceeds directly to
`FinalizeBlock`. Application implementations must handle finalization without
a preceding `ProcessProposal` call.

### ExtendVote

This draft hook is absent from `Application`. There is no current
`RequestExtendVote` or `ResponseExtendVote` contract. Autobahn votes and
certificates use [`autobahn/types`](../../autobahn/types/); the upstream
precommit-extension sequence is not a description of those messages.

### VerifyVoteExtension

This draft hook is also absent from `Application`. Do not infer an application
vote-extension verification call from the inherited consensus parameter
schema or the upstream draft's enum definitions.

### FinalizeBlock

```go
FinalizeBlock(context.Context, *RequestFinalizeBlock) (*ResponseFinalizeBlock, error)
```

#### Parameters and Types

| Request field | Go type | Meaning |
| --- | --- | --- |
| `Txs` | `[][]byte` | Decided transaction sequence. |
| `DecidedLastCommit` | `CommitInfo` | Commit information supplied by the active consensus path. |
| `ByzantineValidators` | `[]Misbehavior` | Supplied misbehavior records. |
| `Hash` | `[]byte` | Block identity supplied by the caller. |
| `Header` | `*tmproto.Header` | Header carrying height, time, chain identifier and proposer. |

Height, time, next-validator hash and proposer address are not separate Go
request fields. Read header values from `Header`, while respecting which
fields the active consensus path actually populates.

| Response field | Go type | Protobuf field | Meaning |
| --- | --- | --- | --- |
| `Events` | `[]Event` | 1 | Block events. |
| `TxResults` | `[]*ExecTxResult` | 2 | Execution results in transaction order. |
| `ValidatorUpdates` | `[]ValidatorUpdate` | 3 | Application validator updates. |
| `ConsensusParamUpdates` | `*tmproto.ConsensusParams` | 4 | Application consensus parameter updates. |
| `AppHash` | `[]byte` | 5 | Resulting application state hash. |

There is no `RetainHeight` field in this response; it belongs to `Commit`.

#### Application execution

[`BaseApp.FinalizeBlock`](../../../sdk/baseapp/abci.go) checks the header's
chain identifier, initializes or refreshes deliver state and invokes the
configured finalization handler. [`App.FinalizeBlocker`](../../../node/app.go)
selects one of these paths:

1. Reuse sealed execution state when its height and entire transaction
   sequence match the decided block and sequencing is active.
2. Reuse completed optimistic proposal state when no sequence source is
   configured, processing was not aborted and the block hash matches.
3. Execute the decided block with `ProcessBlock` otherwise.

Each normal path selects the state to commit and returns its working hash,
events, ordered transaction results and updates. The handler stages the head
notification for the subsequent commit. Returning from `FinalizeBlock` does
not replace the separate store commit. Deterministic execution and results
are required for replicas starting from the same committed state and block.
A failed transaction remains in the decided block with a nonzero result
code; its failure is distinct from an error returned by the block method.

#### When does consensus call it?

In the classic path, [`BlockExecutor.ApplyBlock`](../../internal/state/execution.go)
validates the decided block and supplies its transactions, header, hash,
last-commit information and evidence. It saves the finalization response,
validates updates, computes the deterministic transaction-results hash and
commits the application while coordinating with the mempool. It then saves
consensus state with the returned application hash. That hash is carried in
the next classic block's header. Validator updates from height `H` take
effect at `H+2`; consensus parameter updates take effect at `H+1`.

In Autobahn, `runExecute` consumes committed global blocks in global-number
order. `executeBlock`:

1. Uses the global number as the application height and the global block's
   timestamp as the header time. Heights beyond `int64` are rejected.
2. Passes the payload's transactions and **the Autobahn block-header hash**
   as `RequestFinalizeBlock.Hash`. Do not recompute this identity from the
   synthesized Tendermint header.
3. Builds the header with the genesis chain identifier and a deterministic
   proposer address from `GetValidators`. It fills the parent hash after the
   committee's first block. `DecidedLastCommit` is empty and no misbehavior
   records are supplied; empty votes do not mean absent validator signatures.
4. Calls `FinalizeBlock`, saves its response, then calls `Commit`.
5. Sends `ResponseFinalizeBlock.AppHash` to the Autobahn data layer with
   `PushAppHash`, publishes execution events and applies the commit's pruning
   boundary in the execution loop.

This path does not apply the classic validator-update schedule to Autobahn's
configured committee. Nor should the synthesized header be assumed to carry
all classic header commitments. The EVM head notifier uses the supplied block
hash and finalization response's state root; see
[`BlockHeaderNotifier`](../../../rpc/notifier.go).

#### Restart and recovery

Autobahn uses its own persisted consensus/data state and bypasses the classic
handshaker. `runExecute` reads `Info.LastBlockHeight` on startup. At zero it
initializes genesis and starts at `InitialHeight`; otherwise it restores the
last committed application hash into the data layer and resumes at the next
global height. `BaseApp.FinalizeBlock` rebuilds deliver state from committed
storage when needed. An in-memory optimistic candidate is not the restart
authority, and a saved finalization response alone does not prove an
application commit completed.

The Autobahn JSON configuration requires `persistent_state_dir` unless the
explicit test-only `unsafe_test_only_disable_persistence` option is used;
the two settings are mutually exclusive. This persistence holds consensus
safety state as well as the data needed by that mode. See
[`autobahn.go`](../../config/autobahn.go). Application snapshot methods do
not replace this persistence requirement.

## Data Types existing in ABCI

### Validator

The Go type has `Address []byte` and `Power int64`. It identifies a validator
in vote and misbehavior records without carrying its public key.

### ValidatorUpdate

The protobuf-backed type has `PubKey crypto.PublicKey` (field 1) and
`Power int64` (field 2). In the classic update path, zero power removes a
validator. Its presence in an application response does not reconfigure the
Autobahn committee.

### Misbehavior

The Go fields are `Type MisbehaviorType`, `Validator Validator`,
`Height int64`, `Time time.Time` and `TotalVotingPower int64`.
The classic executor derives these records from block evidence. The Autobahn
execution request described above does not supply them.

#### MisbehaviorType

The protobuf enum values are `UNKNOWN = 0`, `DUPLICATE_VOTE = 1` and
`LIGHT_CLIENT_ATTACK = 2`. These are misbehavior kinds, not method error codes.

### ConsensusParams

`RequestInitChain.ConsensusParams`, proposal updates and finalization updates
use `tendermint.types.ConsensusParams` from
[`types/params.proto`](../../proto/tendermint/types/params.proto).
It has `block`, `evidence`, `validator`, `version`, `synchrony`, `timeout` and
`abci` fields. The similarly named `tendermint.abci.ConsensusParams` in the
ABCI protobuf file is a different, legacy type; do not substitute it in
`ResponseFinalizeBlock`.

The block parameters include byte, estimated-gas and wanted-gas limits and a
minimum transaction count. Mode-specific application of these values is
controlled by the call sites, not by the upstream draft's execution modes.
An inherited `abci.vote_extensions_enable_height` field does not create
missing vote-extension methods on the Go interface.

### ProofOps

`ResponseQuery.ProofOps` uses
[`crypto.ProofOps`](../../proto/tendermint/crypto/proof.proto), a repeated
sequence of `ProofOp` messages. Verification must use the supported proof
format and the trusted application state commitment for the query.

### ProofOp

The protobuf fields are `type string` (1), `key bytes` (2) and `data bytes`
(3). The type identifies the proof encoding; an opaque proof is not verified
merely by receiving it.

### Snapshot

The current Go fields are `Height uint64`, `Format uint32`, `Chunks uint32`,
`Hash []byte` and `Metadata []byte`. There are no protobuf field numbers for
this plain Go type. Snapshot metadata and chunk encoding are interpreted by
the application's snapshot manager. Consult the
[state-sync specification](../p2p/messages/state-sync.md) for the classic
peer protocol and [`sdk/baseapp/abci.go`](../../../sdk/baseapp/abci.go) for
the application implementation.

## Data types introduced or modified in ABCI++

### VoteInfo

`VoteInfo` has `Validator Validator` and `SignedLastBlock bool`.
The classic executor constructs votes from the last commit. Autobahn's empty
`CommitInfo` must not be expanded into fictitious missed-signature entries.

### ExtendedVoteInfo

This upstream draft type is absent from the current ABCI Go interface.
There is no application `VoteExtension` field in the fork's `VoteInfo`.

### CommitInfo

The Go fields are `Round int32` and `Votes []VoteInfo`.
`ProposedLastCommit` uses this type in `ProcessProposal`;
`DecidedLastCommit` uses it in `FinalizeBlock`.

### ExtendedCommitInfo

This upstream draft type is absent from the current interface. Autobahn's
certificates must be interpreted using its own types, not as this draft's
extended previous-height commit information.

### ExecTxResult

| Go field | Go type | Protobuf field | Meaning |
| --- | --- | --- | --- |
| `Code` | `uint32` | 1 | Zero for success; nonzero for application failure. |
| `Data` | `[]byte` | 2 | Result bytes. |
| `Log` | `string` | 3 | Diagnostic log. |
| `Info` | `string` | 4 | Additional diagnostic information. |
| `GasWanted` | `int64` | 5 | Requested gas. |
| `GasUsed` | `int64` | 6 | Consumed gas. |
| `Events` | `[]Event` | 7 | Application transaction events. |
| `Codespace` | `string` | 8 | Namespace for the result code. |
| `EvmTxInfo` | `*EvmTxInfo` | 9 | EVM sender, nonce, transaction hash and VM error. |

`TxResults[i]` corresponds to `Txs[i]`. The fork's `MarshalTxResults` hashes
only `Code`, `Data`, `GasWanted` and `GasUsed`; logs, info, events, codespace
and EVM metadata are excluded from that deterministic encoding. The classic
`TxResultsHash` hashes these encoded results for `LastResultsHash`; block
events are not additional leaves. See [`types.go`](../../abci/types/types.go)
and [`execution.go`](../../internal/state/execution.go).

### TxAction

The draft's `UNKNOWN`, `UNMODIFIED`, `ADDED` and `REMOVED` action enum is not
part of the current API. No proposal response directs mempool edits using
these actions.

### TxRecord

There is no current `TxRecord` message or `tx_records` response. Proposal
construction supplies transaction bytes directly; finalization results stay
aligned with the decided sequence.

### ProposalStatus

The current Go enum is `ResponseProcessProposal_ProposalStatus`:
`UNKNOWN = 0`, `ACCEPT = 1`, `REJECT = 2`. See
[ProcessProposal](#processproposal) for the classic caller's handling.

### VerifyStatus

The draft's vote-extension verification enum is absent from the application
interface, along with `VerifyVoteExtension`.

### CanonicalVoteExtension

The draft's signed extension structure is not a message supplied to or
returned from the current `Application`. Use the implemented
[Autobahn types](../../autobahn/types/) for Autobahn signatures and
certificates. The inherited [application requirements](abci++_app_requirements_002_draft.md),
[basic concepts](abci++_basic_concepts_002_draft.md) and
[client/server draft](abci++_client_server_002_draft.md) provide upstream
background; their draft lifecycle assumptions must be read with the
[current applicability map](#current-interface-and-mode-applicability).
