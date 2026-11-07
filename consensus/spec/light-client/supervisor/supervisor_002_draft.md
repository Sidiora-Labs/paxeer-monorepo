# Draft of Light Client Supervisor for discussion

## Modification to the initialization

This page replaces the inherited initialization pseudocode with the constructor
behavior in [`light/client.go`](../../../light/client.go). The original
`LCInitData` union, genesis fallback and `InitLightClient` function were research
proposals, not the current Go API. The
[sequential supervisor](supervisor_001_draft.md) retains the trust and timing
model; [IBC fork detection](../detection/req-ibc-detection.md) describes the
separate on-chain client and misbehaviour interfaces.

These constructors operate on classic Tendermint `types.LightBlock` values.
They do not initialize or recover Autobahn consensus. In the merged Paxeer
network, `paxd` runs the BFT chain and the kernel participates in proposing
blocks; a committed receipt is final without a further light-client stage.
EVM chain ID 125 (`0x7d`) identifies the Ethereum API network, while the
constructor's `chainID` must match the consensus header's chain identity.

### **[LC-DATA-INIT.2]**

The implemented input is
[`light.TrustOptions`](../../../light/trust_options.go):

```go
type TrustOptions struct {
    Period time.Duration
    Height int64
    Hash   []byte
}
```

For an empty store, `ValidateBasic` requires a positive period, a positive
height and a hash of `crypto.HashSize` bytes. Height and hash identify the
checkpoint the operator has authenticated through a trusted source. The
trusting period should leave room for checking, reporting and punishment
before the corresponding unbonding period ends; this page does not set a
mainnet duration.

The constructor signatures are:

```go
func NewClient(
    ctx context.Context,
    chainID string,
    trustOptions TrustOptions,
    primary provider.Provider,
    witnesses []provider.Provider,
    trustedStore store.Store,
    blacklistTTL time.Duration,
    options ...Option,
) (*Client, error)

func NewClientFromTrustedStore(
    chainID string,
    trustingPeriod time.Duration,
    primary provider.Provider,
    witnesses []provider.Provider,
    trustedStore store.Store,
    blacklistTTL time.Duration,
    options ...Option,
) (*Client, error)
```

These are signatures from package `light`, not a standalone program.
[`provider.Provider`](../../../light/provider/provider.go) retrieves light
blocks and reports evidence;
[`store.Store`](../../../light/store/store.go) persists trusted blocks. The
[supervisor configuration section](supervisor_001_draft.md#configuration-parameters)
lists the functional options. Neither constructor takes a `GenesisDoc` or a
direct `TrustedBlock` parameter.

### Initialization

Initialization is subjective: a hash match establishes consistency with the
operator's chosen checkpoint, not that the checkpoint belongs to the intended
chain. Cross-checking providers helps detect inconsistent configuration and
responses but cannot replace authentication of that checkpoint.

#### **[LC-FUNC-INIT.2]:**

`NewClient` first calls `trustedStore.LastLightBlockHeight()`. A store error is
returned. If a stored height is positive, it delegates immediately to
`NewClientFromTrustedStore`, using `trustOptions.Period`. In that case, the
supplied trust height and hash do not override the stored checkpoint and are
not checked by `TrustOptions.ValidateBasic`.

With an empty store, the constructor performs the following steps:

1. Require at least one witness, otherwise return `ErrNoWitnesses`.
2. Validate `TrustOptions`, apply functional options and validate the trust
   level (`1/3 <= level < 1`).
3. Fetch the checkpoint light block from the primary at `Height`. Provider
   responses pass through basic validation against `chainID`.
4. Require its header hash to equal `Hash`. Verify the commit with
   `ValidatorSet.VerifyCommitLight`, requiring more than two-thirds of the
   checkpoint's validator voting power.
5. Run `compareFirstLightBlockWithWitnesses` on the checkpoint.
6. Save the checkpoint through `updateTrustedLightBlock`, apply configured
   pruning and set the latest trusted block.

A fetch failure, hash mismatch, invalid commit, witness-comparison error or
store/pruning error prevents a successful constructor return. The store is not
a transactional wrapper around the whole constructor: saving precedes pruning,
so a pruning error can occur after the checkpoint has been written.

The initial checkpoint's time is deliberately not checked for expiry here.
Forward verification later applies the trusting-period rule. An old checkpoint
can therefore initialize successfully and then fail to verify a newer header
with `ErrOldHeaderExpired`. Reauthenticate trust when it expires; changing a
clock or silently extending the period does not establish a new root of trust.

### Witness consistency at initialization

The original pseudocode compared all hashes and rejected any disagreement.
The implementation uses `compareNewLightBlockWithWitness`, including its
handling of lagging providers and validator proposer priorities.

A conflicting header returns `ErrConflictingHeaders`; equal header hashes with
different proposer-priority hashes return `ErrProposerPrioritiesDiverge`.
Cancellation and deadline errors are returned. Bad witnesses can be removed;
removing every witness fails with `ErrNoWitnesses`. Some unavailable or lagging
responses are ignored, so successful initialization does not mean every
configured witness responded or that every witness positively matched.

This differs from normal forward detection, which requires a witness match
after processing responses. Initialization does not run a historical attack
investigation to turn every differing initial response into accountable
validator evidence. The operator must resolve a checkpoint disagreement before
relying on that checkpoint.

### Restart from the trusted store

`NewClientFromTrustedStore` requires at least one witness, a positive trusting
period, a non-nil primary and store, and non-nil witness entries. It applies
options, validates the trust level, loads the highest stored light block and
runs its `ValidateBasic(chainID)` check. An empty store or invalid restored
block returns an error.

Restoration does not compare the checkpoint with providers and does not redo
cryptographic commit verification or check its age. `ValidateBasic` checks
structure and internal bindings, not signatures. The restored store is trusted
input; persisting a block is not a new source of consensus trust. The
[`database store`](../../../light/store/db/db.go) supplies the storage
implementation, while first/last-height queries and pruning determine which
checkpoints are still available.

Provider selection and blacklist entries are in-memory client state, not
fields persisted with the light blocks. Reconstruct the providers and options
when restarting. The `NewClient` store-first behavior means that changing only
`TrustOptions.Height` and `Hash` while reusing a populated store will not
reinitialize it from that new checkpoint.

### Current-mode boundary: Autobahn

The node's `autobahn-config-file` setting in
[`config/config.go`](../../../config/config.go) selects the optional Autobahn
path when nonempty. The classic HTTP provider needs `Commit` and `Validators`;
under Autobahn the [`Commit` handler](../../../internal/rpc/core/blocks.go)
returns `ErrCommitNotAvailable`. The
[provider](../../../light/provider/http/http.go) classifies that as
`ErrUnreliableProvider`. Reinitializing a classic trusted store does not add
Autobahn certificate support.

Autobahn [`CommitQC` and `FullCommitQC`](../../../autobahn/types/commit_qc.go)
verify against a [`Committee`](../../../autobahn/types/committee.go) and bind
proposals, lane headers and finalized global ranges. Their verification rules
are not the classic validator-set overlap and trusting-period algorithm.

Autobahn consensus recovery uses the package-level
[`NewState`](../../../internal/autobahn/consensus/state.go) constructor, which
loads persisted data when `PersistentStateDir` is set. The
[`persistedInner`](../../../internal/autobahn/consensus/persisted_inner.go)
state contains commit, prepare and timeout certificates and local votes; its
validation checks signatures and view consistency. The
[`persistence implementation`](../../../internal/autobahn/consensus/persist/)
also owns block and certificate storage. These records serve consensus safety
and recovery and cannot be reconstructed by inserting a Tendermint checkpoint
into `store.Store`.

The classic initialization and IBC research pages make no equivalence claim
for those certificates or restart paths. An Autobahn light-client protocol
needs its own explicit trust and application-state binding before it can use
those paths as a proof to an external observer.
