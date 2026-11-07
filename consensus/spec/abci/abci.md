---
order: 1
title: Method and Types
---

# Methods and Types

This page describes the in-process ABCI boundary in the Paxeer consensus fork.
The authoritative method set is the Go `Application` interface in
[`application.go`](../../abci/types/application.go). Requests and several responses
are ordinary Go structs in [`plain_types.go`](../../abci/types/plain_types.go);
the remaining response and event messages are defined in
[`types.proto`](../../proto/tendermint/abci/types.proto). Do not infer the current
API from an upstream ABCI version or from protobuf definitions alone.

Paxeer is one BFT network run by `paxd`, with EVM chain ID **125** (`0x7d`).
The merged kernel is part of the network and proposes blocks; execution and
commitment belong to that network. A receipt is final once its block is committed.
ABCI is an internal application boundary. Public JSON-RPC requests go to a node's
root URL, for example `https://api1.mainnet-beta.paxeer.network/`; the public node
names are `api1` through `api14.mainnet-beta.paxeer.network` and
`api-mainnet-beta.paxeer.network`. These public URLs do not expose ABCI Go methods.

The upstream lifecycle and transport terminology retained below is protocol
history. It does not specify the current Autobahn execution or recovery path.

## Connections

The consensus engine and application run in the same process and call Go methods
directly. “Info/Query Connection”, “Mempool Connection”, “Consensus Connection”,
and “State Sync Connection” are groupings in `Application`, not four network
connections. Socket and gRPC ABCI application transports are not implemented in
this fork.

Most methods accept `context.Context` and return a response pointer plus `error`.
`CheckTx` returns only `*ResponseCheckTxV2`. `GetValidators`, `EvmNonce`, and
`EvmBalance` are direct helper calls without a context parameter.

The older [ABCI Applications](apps.md) and [Client and Server](client-server.md)
pages provide historical background. Use this page and the linked implementation
for current method signatures and lifecycle behavior.

## Errors

`ResponseQuery`, the `ResponseCheckTx` embedded in `ResponseCheckTxV2`, and each
`ExecTxResult` carry `Code` and `Codespace`. `CodeTypeOK` is `0`; a nonzero code
reports an application error. `Codespace` identifies the namespace of the code.
ABCI codes are distinct from public JSON-RPC error codes.

An application result code is also distinct from a Go `error`. A failed
`FinalizeBlock` or `Commit` call stops the current execution path and is propagated
to its caller; it is not a successful block containing a failed transaction.
Applications must not continue from a state that cannot be committed consistently.

### CheckTx

`CheckTx` checks a transaction for local mempool admission. The mempool rejects a
nonzero response code, and removes transactions that fail a subsequent recheck.
Admission is not a promise of inclusion or successful execution. Capacity,
priority, and other local mempool checks may also reject a transaction with a
successful application result. See the
[mempool implementation](../../internal/mempool/mempool.go).

### DeliverTx

The current `Application` interface has no `DeliverTx` method. Transaction execution
outcomes are returned in `ResponseFinalizeBlock.TxResults`, one `ExecTxResult` per
transaction in block order. A nonzero result code records a transaction failure;
it does not remove that transaction from the already decided block.
`ResponseDeliverTx` remains a compatibility type, not the current consensus call.

### Query

`Query` reads application state. A nonzero `ResponseQuery.Code` reports the
application query failure to the caller. Queries must not mutate committed state.
The SDK query implementation selects a committed store version; unavailable or
pruned versions can fail even when the requested height is below the chain tip.

## Events

`ResponseFinalizeBlock.Events` carries block events; `ExecTxResult.Events` carries
transaction events. Events support indexing, subscriptions, and queries. They do
not form part of the transaction-result commitment described under
[Determinism](#determinism). The current `ResponseCheckTx` has no `Events` field.

An event has a type and an ordered list of attributes. Multiple events may share
a type; do not collapse them into a map or assume that a later event replaces an
earlier event with the same key. The protobuf shape is:

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

The Go attribute keys and values are `[]byte`. `Index` requests indexing of the
attribute and is not a consensus commitment. For example, a transaction result
can include:

```go
abci.ExecTxResult{
    Code: abci.CodeTypeOK,
    Events: []abci.Event{
        {
            Type: "transfer",
            Attributes: []abci.EventAttribute{
                {Key: []byte("recipient"), Value: []byte("account"), Index: true},
                {Key: []byte("amount"), Value: []byte("100"), Index: true},
            },
        },
    },
}
```

In the Autobahn execution loop, finalized responses are saved before application
commit. After commit and recording the application hash, the engine publishes a
`NewBlockHeader` event and one `Tx` event per transaction in block order. An event
notification is not a replacement for checking committed state. The implementation
is in [`giga_router.go`](../../internal/p2p/giga_router.go); `GigaRouter` is an
internal consensus/P2P type, not a public API routing service.

## EvidenceType

The historical name `EvidenceType` is now `MisbehaviorType` in the current ABCI
protobuf. Its values remain:

| Name | Value |
| --- | --- |
| `UNKNOWN` | 0 |
| `DUPLICATE_VOTE` | 1 |
| `LIGHT_CLIENT_ATTACK` | 2 |

The classic block executor passes verified block evidence as
`RequestFinalizeBlock.ByzantineValidators`, a slice of `Misbehavior`. The field is
not named `Misbehavior`, and evidence is not delivered through an `EndBlock`
method on `Application`.

Autobahn's `executeBlock` does not populate `ByzantineValidators` and passes an
empty `DecidedLastCommit`. Its empty vote list does not mean that every validator
missed a vote. Classic evidence and slashing descriptions must not be read as a
claim that Autobahn supplies those inputs. See the classic
[data structures](../core/data_structures.md) and
[accountability](../light-client/accountability/README.md) pages for protocol
background, and the execution paths linked below for applicability.

## Determinism

Replicated execution must produce the same committed state and deterministic
transaction results for the same prior state and ordered block inputs. Use the
block's supplied time, header, and transaction order; do not derive state changes
from wall-clock time, local randomness, external network responses, filesystem
contents, unordered map iteration, races, or unspecified serialization behavior.
Speculative execution must agree with the decided block before its state is used.

The classic executor computes `LastResultsHash` with `TxResultsHash`, which calls
`MarshalTxResults`. The deterministic projection of each `ExecTxResult` contains
exactly `Code`, `Data`, `GasWanted`, and `GasUsed`. `Log`, `Info`, `Events`,
`Codespace`, and `EvmTxInfo` are excluded from that projection. Block events are
also excluded. See [`types.go`](../../abci/types/types.go) and
[`execution.go`](../../internal/state/execution.go). This result-hash rule does
not permit nondeterministic application state or application hashes.

`ResponseFinalizeBlock.AppHash` is the application-state hash; `Commit` persists
the selected application state. Autobahn records that hash with `PushAppHash`
after successful commit. These are separate from transaction-result hashing and
from diagnostic query or log fields.

## Block Execution

The application boundary uses:

```text
Fresh application: InitChain
Decided block:     FinalizeBlock -> save finalized responses -> Commit
```

The current paths are:

| Path | Application calls and behavior |
| --- | --- |
| Autobahn | `runExecute` reads `Info`, initializes a fresh application with `InitChain`, then calls `FinalizeBlock` and `Commit` for each available global block. It does not call `ProcessProposal` in this execution loop. |
| Classic consensus | `ProcessProposal` evaluates a proposal before acceptance. `BlockExecutor.ApplyBlock` finalizes a decided block, saves responses, applies validator/parameter updates, commits the application, and saves consensus state. |

Autobahn constructs a `RequestFinalizeBlock` from the global block's ordered
transactions, global number, timestamp, and Autobahn header hash. That hash is used
as the application/EVM block identifier; it is not a reconstruction of a classic
CometBFT header hash. The header's proposer address is selected deterministically
from application validators for application compatibility. It must not be used to
infer the Autobahn leader. The committee is fixed by genesis in this path;
classic validator-update timing is not an Autobahn committee-update mechanism.

[`node/app.go`](../../../node/app.go) registers `ProcessProposalHandler` and
`FinalizeBlocker`. `FinalizeBlocker` can reuse sealed execution only when the
height and ordered transactions match the decided block. It can reuse eligible
optimistic execution when its checks match; otherwise it processes the decided
transactions. Begin-block, transaction, and end-block work still exists inside
the application, but those operations are not separate methods on the current
consensus-facing interface.

On restart, Autobahn starts from `Info.LastBlockHeight + 1` and records the last
committed application hash again. It does not call `InitChain` for an application
with a nonzero committed height. If no block has committed, initialization runs
again and execution starts at the genesis initial height. See
[`runExecute` and `executeBlock`](../../internal/p2p/giga_router.go) for the exact
recovery and pruning order.

## State Sync

The interface retains snapshot discovery, offer, load, and apply methods. Their
presence does not imply that every node advertises snapshots or that all consensus
modes use the classic state-sync bootstrap sequence.

The classic state-sync implementation discovers snapshots, calls `OfferSnapshot`,
fetches chunks with `LoadSnapshotChunk`, and restores them with
`ApplySnapshotChunk`. The trusted application hash is obtained from the state
provider. After restoration, `Info` must agree with the expected application
version, snapshot height, and trusted application hash; accepting chunks alone
is not sufficient verification. See
[`syncer.go`](../../internal/statesync/syncer.go) and the historical
[state sync overview](../p2p/messages/state-sync.md).

The SDK delegates these methods to its snapshot manager. Without a configured
manager, `ListSnapshots` returns an empty list, `LoadSnapshotChunk` returns an empty
response, and offer/apply return `ABORT`. Snapshot metadata from peers requires
validation; it is not trusted merely because it accompanies a snapshot offer.

## Messages

Names below use the Go field spelling for in-process structs. Protobuf field
numbers apply only to messages actually defined in the linked protobuf files.

### Echo

Historical transport diagnostic. `Application` has no `Echo` method in this fork.

### Flush

Historical transport queue operation. `Application` has no `Flush` method and
there is no ABCI socket queue to flush.

### Info

`Info(context.Context, *RequestInfo) (*ResponseInfo, error)` reports application
metadata and the last committed state.

| Request field | Go type |
| --- | --- |
| `Version`, `AbciVersion` | `string` |
| `BlockVersion`, `P2PVersion` | `uint64` |

| Response field | Meaning |
| --- | --- |
| `Data`, `Version` | Application name/information and software version |
| `AppVersion` | Application protocol version (`uint64`) |
| `LastBlockHeight` | Committed store version (`int64`) |
| `LastBlockAppHash` | Hash of that committed version (`[]byte`) |
| `MinimumGasPrices` | Local minimum gas prices (`string`) |

The SDK implementation reads the last commit ID from its multistore. A hash
calculated during finalization is not yet a new committed height for `Info`.

### InitChain

`InitChain(context.Context, *RequestInitChain) (*ResponseInitChain, error)`
initializes application state from genesis.

| Request field | Go type / purpose |
| --- | --- |
| `Time` | `time.Time`, genesis time |
| `ChainId` | `string`, consensus chain ID, distinct from the EVM numeric chain ID |
| `ConsensusParams` | `*tendermint/types.ConsensusParams` |
| `AppStateBytes` | `[]byte`, serialized genesis application state |
| `InitialHeight` | `int64`, first block height |

The response has `Validators []ValidatorUpdate` and `AppHash []byte`.
There is no request `Validators` field and no response `ConsensusParams` field in
the current Go types. Initialization prepares state for the first block without
an intermediate `Commit`; follow the mode-specific bootstrap code rather than
the upstream request/response tables.

### Query

`Query(context.Context, *RequestQuery) (*ResponseQuery, error)` accepts
`Data []byte`, `Path string`, `Height int64`, and `Prove bool`. `Path` selects an
application query handler; it is not a public HTTP endpoint. A height of zero
selects the latest committed height. Negative or future heights fail, as do
proof requests at heights at or below one in `BaseApp.CreateQueryContext`.

`ResponseQuery` contains `Code uint32`, `Log string`, `Info string`, `Index int64`,
`Key []byte`, `Value []byte`, `ProofOps *crypto.ProofOps`, `Height int64`, and
`Codespace string`. Proof availability depends on the query handler and retained
store versions. See [`sdk/baseapp/abci.go`](../../../sdk/baseapp/abci.go).

### BeginBlock

Historical consensus method. It is absent from `Application`. Compatibility
`RequestBeginBlock` and `ResponseBeginBlock` types and application-internal
begin-block processing remain; the engine supplies block inputs through
`FinalizeBlock`.

### CheckTx

`CheckTx(context.Context, *RequestCheckTxV2) *ResponseCheckTxV2` accepts `Tx []byte`
and `Type CheckTxTypeV2`. The type values are `CheckTxTypeV2New` (`0`) and
`CheckTxTypeV2Recheck` (`1`). The node implementation in
[`node/abci.go`](../../../node/abci.go) decodes the transaction and runs the
application's admission checks.

The embedded protobuf `ResponseCheckTx` contains:

| Field | Go type | Protobuf number |
| --- | --- | --- |
| `Code` | `uint32` | 1 |
| `Data` | `[]byte` | 2 |
| `Log` | `string` | 3 |
| `GasWanted` | `int64` | 5 |
| `Codespace` | `string` | 8 |
| `Priority` | `int64` | 10 |
| `GasEstimated` | `int64` | 12 |

It has no `Info`, `GasUsed`, or `Events` fields. The V2 wrapper adds local
mempool metadata: `IsEVM`, `EVMNonce`, `EVMHash`, `EVMSenderAddress`,
`PaxSenderAddress`, and `EVMRequiredBalance`. These fields are not part of the
protobuf response.

`GetTxPriorityHint(context.Context, *RequestGetTxPriorityHintV2)` returns
`(*ResponseGetTxPriorityHint, error)`. Its request contains `Tx []byte` and its
response `Priority int64`; the value is an admission hint, not execution success.
`EvmNonce(common.Address) uint64` and
`EvmBalance(common.Address, []byte) uint256.Int` support local mempool checks.

### ProcessProposal

`ProcessProposal(context.Context, *RequestProcessProposal)` returns
`(*ResponseProcessProposal, error)`. The request contains `Txs [][]byte`,
`ProposedLastCommit CommitInfo`, `ByzantineValidators []Misbehavior`, `Hash []byte`,
and `Header *tendermint/types.Header`.

The response contains `Status`, `AppHash`, `TxResults`, `ValidatorUpdates`, and
`ConsensusParamUpdates`. Status values are `UNKNOWN` (`0`), `ACCEPT` (`1`), and
`REJECT` (`2`). Acceptance is a proposal decision, not a durable commit. This
method belongs to the classic proposal path; Autobahn's execution loop invokes
`FinalizeBlock` directly.

### FinalizeBlock

`FinalizeBlock(context.Context, *RequestFinalizeBlock)` returns
`(*ResponseFinalizeBlock, error)` for the decided block.

| Request field | Go type |
| --- | --- |
| `Txs` | `[][]byte`, in decided order |
| `DecidedLastCommit` | `CommitInfo` |
| `ByzantineValidators` | `[]Misbehavior` |
| `Hash` | `[]byte`, mode-specific block identifier |
| `Header` | `*tendermint/types.Header` |

| Response field | Go type | Protobuf number |
| --- | --- | --- |
| `Events` | `[]Event` | 1 |
| `TxResults` | `[]*ExecTxResult` | 2 |
| `ValidatorUpdates` | `[]ValidatorUpdate` | 3 |
| `ConsensusParamUpdates` | `*tendermint/types.ConsensusParams` | 4 |
| `AppHash` | `[]byte` | 5 |

Return transaction results in the same order as `Txs`. The normal node execution
path produces block events, execution results, validator updates, parameter
updates, and the working application hash. Persisting the chosen state is the
subsequent `Commit` operation.

### DeliverTx

Historical per-transaction ABCI call, replaced at the `Application` boundary by
`FinalizeBlock`. Its result shape survives as `ResponseDeliverTx`; use
`ExecTxResult` for finalized transaction results.

### EndBlock

Historical consensus method, absent from `Application`. The node still uses an
internal `ResponseEndBlock` to collect end-block results, then places its validator
updates into `ResponseFinalizeBlock`. Current parameter updates use
`tendermint/types.ConsensusParams`.

In the retained classic executor, validator updates returned at height `H` take
effect at `H+2`, and consensus parameter updates take effect at `H+1`. These rules
are implemented by `State.Update`; they do not describe Autobahn committee
rotation.

### Commit

`Commit(context.Context) (*ResponseCommit, error)` takes no request object. It
persists the application state chosen during finalization. `ResponseCommit` has
only `RetainHeight int64` (protobuf field `3`). It has no `Data` field and does
not return the application hash; use `ResponseFinalizeBlock.AppHash` and the
committed state reported by `Info`.

`RetainHeight` controls pruning: zero requests no pruning; a positive value permits
removal below that height. Autobahn passes it to `PruneBefore` and prunes saved
finalized responses below it. Removing history can prevent replay or historical
queries, so retention must account for recovery and data availability needs.

### ListSnapshots

`RequestListSnapshots` is empty. `ResponseListSnapshots.Snapshots` is
`[]*Snapshot`, listing locally available committed snapshots.

### LoadSnapshotChunk

`RequestLoadSnapshotChunk` contains `Height uint64`, `Format uint32`, and
`Chunk uint32` (a zero-based index). `ResponseLoadSnapshotChunk.Chunk` is `[]byte`.
Chunk size and encoding belong to the snapshot implementation and P2P transport;
this Go interface does not specify a universal chunk size limit.

### OfferSnapshot

`RequestOfferSnapshot` contains `Snapshot *Snapshot` and `AppHash []byte` (the
trusted application hash). `ResponseOfferSnapshot.Result` uses the following
values from `plain_types.go`:

| Result | Value | State-sync action |
| --- | --- | --- |
| `UNKNOWN` | 0 | Invalid/unknown result; abort restoration |
| `ACCEPT` | 1 | Start applying this snapshot |
| `ABORT` | 2 | Abort restoration |
| `REJECT` | 3 | Reject this snapshot |
| `REJECT_FORMAT` | 4 | Reject snapshots with this format |
| `REJECT_SENDER` | 5 | Reject snapshots from this sender |

### ApplySnapshotChunk

`RequestApplySnapshotChunk` contains `Index uint32`, `Chunk []byte`, and
`Sender string`. `ResponseApplySnapshotChunk` contains `Result`,
`RefetchChunks []uint32`, and `RejectSenders []string`.

| Result | Value | State-sync action |
| --- | --- | --- |
| `UNKNOWN` | 0 | Invalid/unknown result; abort restoration |
| `ACCEPT` | 1 | Accept this chunk |
| `ABORT` | 2 | Abort restoration |
| `RETRY` | 3 | Retry the chunk, using refetch/sender instructions |
| `RETRY_SNAPSHOT` | 4 | Restart this snapshot |
| `REJECT_SNAPSHOT` | 5 | Reject this snapshot and try another |

The SDK snapshot manager returns `RETRY` with the chunk index in `RefetchChunks`
and the peer in `RejectSenders` on a chunk-hash mismatch. Other restore failures
can abort restoration. See [State Sync](#state-sync) for completion verification.

## Data Types

These are the current ABCI types. The classic
[common data structures](../core/data_structures.md) page provides background;
use the Go declarations and protobuf links above for exact fork definitions.

### ExecTxResult

| Field | Go type | Protobuf number |
| --- | --- | --- |
| `Code` | `uint32` | 1 |
| `Data` | `[]byte` | 2 |
| `Log` | `string` | 3 |
| `Info` | `string` | 4 |
| `GasWanted` | `int64` | 5 |
| `GasUsed` | `int64` | 6 |
| `Events` | `[]Event` | 7 |
| `Codespace` | `string` | 8 |
| `EvmTxInfo` | `*EvmTxInfo` | 9 |

`EvmTxInfo` contains `SenderAddress string`, `Nonce uint64`, `TxHash string`, and
`VmError string`. Its protobuf field spellings are `senderAddress`, `nonce`,
`txHash`, and `vmError`. Inspect EVM-specific status as well as the ABCI result
when interpreting EVM execution; the ABCI code namespace is application-defined.

### Validator

`Validator` has `Address []byte` and `Power int64`. It identifies a validator in
vote and misbehavior records without carrying the public key.

### ValidatorUpdate

`ValidatorUpdate` has `PubKey crypto.PublicKey` (protobuf field `1`) and
`Power int64` (field `2`). The classic executor interprets zero power as removal.
`GetValidators() []ValidatorUpdate` exposes the application's validator list to
the engine. Autobahn's genesis committee is not updated by this response field.

### VoteInfo

`VoteInfo` has `Validator Validator` and `SignedLastBlock bool`. Its meaning
requires the calling mode's commit information; an empty Autobahn vote list must
not be expanded into a list of absent signatures.

### Evidence

The compatibility `Evidence` and current `Misbehavior` structs both contain
`Type MisbehaviorType`, `Validator Validator`, `Height int64`, `Time time.Time`,
and `TotalVotingPower int64`. `RequestFinalizeBlock.ByzantineValidators` uses
`Misbehavior`. See [EvidenceType](#evidencetype) for values and mode limitations.

#### EvidenceType

The historical enum name maps to current `MisbehaviorType`: `UNKNOWN = 0`,
`DUPLICATE_VOTE = 1`, and `LIGHT_CLIENT_ATTACK = 2`.

### LastCommitInfo

The compatibility `LastCommitInfo` and current `CommitInfo` each contain
`Round int32` and `Votes []VoteInfo`. `ProcessProposal` uses `ProposedLastCommit`;
`FinalizeBlock` uses `DecidedLastCommit`, both of type `CommitInfo`.

### ConsensusParams

Current initialization, proposal, and finalization types reference
[`tendermint/types.ConsensusParams`](../../proto/tendermint/types/params.proto).
It contains `Block`, `Evidence`, `Validator`, `Version`, `Synchrony`, `Timeout`,
and `Abci` parameter groups. `Block` includes `MaxBytes`, `MaxGas`,
`MinTxsInBlock`, and `MaxGasWanted`; `Version` contains `AppVersion`.
The presence of a parameter in the shared schema does not establish that Autobahn
uses the corresponding classic consensus behavior.

The separate `tendermint.abci.ConsensusParams` message is a compatibility type
with only block, evidence, validator, and version groups, used by
`ResponseEndBlock`. It is not the type of
`ResponseFinalizeBlock.ConsensusParamUpdates`.

### ProofOps

`crypto.ProofOps` contains `Ops []ProofOp`, a sequence of proof operations.
A query response's proof must be checked according to its store and proof format,
against a trusted application root, not merely accepted because it was returned.

### ProofOp

[`crypto.ProofOp`](../../proto/tendermint/crypto/proof.proto) contains `Type string`
(protobuf field `1`), `Key []byte` (field `2`), and `Data []byte` (field `3`).

### Snapshot

`Snapshot` is a plain Go struct with `Height uint64`, `Format uint32`,
`Chunks uint32`, `Hash []byte`, and `Metadata []byte`. Height refers to committed
application state; format, content hash, chunk count, and metadata describe the
snapshot representation. These fields have no protobuf numbers in the current
ABCI schema. Do not reuse the historical upstream field-number table or assume
that a peer's advertised hash is a trusted application hash.
