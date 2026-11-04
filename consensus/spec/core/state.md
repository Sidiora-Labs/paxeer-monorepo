# State

The consensus engine in `paxd` maintains the information needed to validate the
next block. Paxeer mainnet-beta is one BFT network (EVM chain ID 125); its kernel
is part of the network and proposes blocks. A receipt is final when its block is
committed.

This page describes the classic Go consensus state and the current in-process
application lifecycle. [Autobahn state and recovery](#autobahn-state-and-recovery)
have a separate persistence path. The inherited
[light-client accountability models](../light-client/accountability/README.md)
apply to classic Tendermint messages, not Autobahn certificates.

The `State` object is local bookkeeping: it is not itself a block, a gossiped
message, or a single hashed object. Individual fields supply header commitments
and validation inputs. The source definition is
[`internal/state/state.go`](../../internal/state/state.go).

| Fields | Role |
| --- | --- |
| `ChainID`, `InitialHeight` | Immutable consensus identity and starting height from [genesis](genesis.md). Zero genesis height is normalized to 1; the initial `LastBlockHeight` is zero. |
| `LastBlockHeight`, `LastBlockID`, `LastBlockTime` | Latest committed classic block. |
| `Version` | Consensus protocol and software versions. |
| `LastValidators`, `Validators`, `NextValidators` | Sets for the previous, current and following validation steps; historical sets are persisted separately. |
| `LastHeightValidatorsChanged` | First height at which the latest validator update takes effect. |
| `ConsensusParams`, `LastHeightConsensusParamsChanged` | Validation parameters and their effective height. |
| `LastResultsHash` | Merkle commitment to canonical transaction results from the previous execution. |
| `AppHash` | Latest application commitment supplied by `ResponseFinalizeBlock.AppHash`. |

Further definitions of headers, validators and parameters are in
[data structures](data_structures.md); the Go types and execution code remain
authoritative when inherited descriptions differ.

## Execution

The Go [`Application` interface](../../abci/types/application.go) calls the
application in-process. The relevant response shapes are declared in
[`abci/types.proto`](../../proto/tendermint/abci/types.proto):

```protobuf
message ResponseFinalizeBlock {
  repeated Event events = 1;
  repeated ExecTxResult tx_results = 2;
  repeated ValidatorUpdate validator_updates = 3;
  tendermint.types.ConsensusParams consensus_param_updates = 4;
  bytes app_hash = 5;
}

message ResponseCommit {
  int64 retain_height = 3;
}
```

These excerpts omit protobuf generation options. `ResponseCommit` has no `Data`
or application-hash field. The older `ResponseEndBlock` message still exists in
the schema, but it does not supply updates for this execution path.

`BlockExecutor.ApplyBlock` in
[`internal/state/execution.go`](../../internal/state/execution.go) performs the
following transition:

1. Validate the block and call `FinalizeBlock` with the decided transactions.
2. Save the finalize response with `SaveFinalizeBlockResponses` before committing
   the application, so recovery can reconstruct consensus state after a crash.
3. Validate and convert validator updates. A negative power is rejected, zero
   removes a validator, and positive-power keys must have a permitted type.
4. Compute `TxResultsHash` from the canonical transaction results and call
   `State.Update` with that hash, the block identity, validator updates and
   consensus-parameter updates.
5. Lock the mempool, call application `Commit`, and update the mempool against
   the committed state. `RetainHeight` is a pruning request.
6. Update the evidence pool, set `state.AppHash` from the saved finalize response,
   save consensus state, request pruning, then publish events.

Validator updates produced at height `H` become effective at `H + 2` in classic
mode. Consensus-parameter updates become effective at `H + 1`; they are merged
and validated, including update-height constraints. `State.Update` leaves
`AppHash` unset until the post-commit assignment. The next header uses the
resulting state commitments.

### Restart and recovery

The classic `Handshaker` in
[`internal/consensus/replay.go`](../../internal/consensus/replay.go) reads the
application's `Info` response (`LastBlockHeight`, `LastBlockAppHash`) and compares
it with the block and consensus-state stores. At application height zero it
performs the [genesis handshake](genesis.md#application-initialization).

When the application is behind, stored blocks are replayed through the
application. If a block was saved but neither application nor consensus state
advanced, the final block is executed against the real application. If the
application already committed that block but consensus state was not saved,
replay uses the persisted finalize response through the recovery proxy to
reconstruct state without applying the transactions to the application twice.
This is an internal recovery mechanism, not substitute execution of a new block.

The handshaker checks height consistency and application hashes through the
configured consensus policy. Missing required history and impossible height
relationships produce errors. Events are published after the state save;
recovery also has event-reindexing logic, so event delivery must not be treated
as the durable application commit boundary.

## Version

`Version` contains `Consensus` (block and application protocol versions) and
`Software`. `InitStateVersion` initializes the block protocol and software
versions; handshake establishes application version information. Valid
consensus-parameter updates also update `Version.Consensus.App` from
`Version.AppVersion`.

## Block

`ConsensusParams.Block.MaxBytes` limits classic block bytes. Transaction
selection uses the configured gas and gas-wanted limits in
[`execution.go`](../../internal/state/execution.go); the application enforces
transaction execution rules. The historical statement that gas limits are
unimplemented is not applicable to this tree. Parameter names and their encodings
are listed in [Genesis](genesis.md#consensus-parameters).

## Evidence

The classic evidence pool rejects evidence as expired only when **both** its
block age exceeds `MaxAgeNumBlocks` and its duration exceeds `MaxAgeDuration`.
Equality alone does not expire it. This is the predicate in
[`internal/evidence/pool.go`](../../internal/evidence/pool.go) and
[`verify.go`](../../internal/evidence/verify.go), rather than a requirement that
both ages remain strictly below their limits.

Block validation in
[`internal/state/validation.go`](../../internal/state/validation.go) also rejects
evidence bytes above `ConsensusParams.Evidence.MaxBytes`. Signature, validator
membership and attack-specific validation are additional requirements; see
[attack isolation](../light-client/attacks/isolate-attackers_001_draft.md).

## Validator

Positive-power application updates must use a key type allowed by
`ConsensusParams.Validator.PubKeyTypes`. The supported key-type map currently
contains `ed25519`. Historical validator sets support commit and evidence
verification; the three sets in `State` must not be collapsed into one set when
reasoning about update delays.

## Autobahn state and recovery

[`node/node.go`](../../node/node.go) selects Autobahn when
`autobahn-config-file` is nonempty. The classic `State` transition and handshake
above do not describe Autobahn's certificate storage or recovery protocol.
Autobahn maintains committee-bound certificates and persisted vote/view state;
see [`autobahn/types`](../../autobahn/types/) and
[`internal/autobahn/consensus/persisted_inner.go`](../../internal/autobahn/consensus/persisted_inner.go).
Its persistence code is in
[`internal/autobahn/consensus/persist`](../../internal/autobahn/consensus/persist/).

The Autobahn execution loop `runExecute` in
[`internal/p2p/giga_router.go`](../../internal/p2p/giga_router.go) reads application
`Info`, initializes only at height zero, and otherwise resumes at the next
committed application height while restoring the last application hash to its
data layer. Preserve its certified data and application database together with
its safety state. `persistent_state_dir` is required by
[`config/autobahn.go`](../../config/autobahn.go), except for an explicit test-only
in-memory opt-out; that opt-out is unsuitable for a node whose votes must remain
safe across restarts. The classic Tendermint [Ivy proofs](../ivy-proofs/README.md)
do not establish Autobahn recovery safety.
