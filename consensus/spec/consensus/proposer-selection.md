---
order: 3
---

# Proposer Selection Procedure

Paxeer runs this consensus engine inside `paxd`. The kernel participates in the
same network and proposes blocks; a receipt is final once its block is committed.
The proposer rule depends on the engine mode and the active on-chain leader
schedule. The presence of proposer-priority fields alone does not identify the
rule used to accept proposals.

## Requirements for Proposer Selection

### R1: Determinism

Honest validators with the same consensus inputs must select the same proposer
for a height and round (classic consensus), or road index and view number
(Autobahn). Validator ordering, weights, chain context and any active leader pin
are part of those inputs.

### R2: Fairness

Hash-based weighted selection gives each validator an interval proportional to
its voting power. It does not promise an exact number of elections in any finite
window. An active leader pin instead follows its ordered primary/standby list;
it does not provide voting-power-proportional proposer rotation.

The weighted round-robin priority calculation retained below has different
fairness properties. Its examples must not be used to predict live round leaders.

## Basic Algorithm

### Classic round leaders

The authoritative implementation is
[`RoundState.Leader`](../../internal/consensus/types/round_state.go), called by
proposal production and verification in
[`State`](../../internal/consensus/state.go).

Without an applicable pin, the selection is:

```text
material = fixed 32-byte leaderElectionSeed
           || big-endian uint64(height) || big-endian uint64(round)
pos = integer(SHA256(material)) mod totalVotingPower
for validator in Validators.Ordered():
    pos -= validator.VotingPower
    if pos < 0:
        return validator.PubKey
```

[`ValidatorSet.Ordered`](../../types/validator_set.go) supplies the deterministic
validator order. This is a deterministic weighted draw for each `(height, round)`;
it does not choose `ValidatorSet.GetProposer()` from the priority queue.

### Pinned primary and standby leaders

`LeaderPin` in the same round-state source contains an activation height and an
ordered list `L` of consensus public keys. At and above activation:

```text
leader(height, round) = L[round mod len(L)]
```

Round zero therefore chooses the primary at every height. Subsequent rounds
choose standbys and wrap around the list. Below activation, or without a pin,
the hash-based choice applies. `NewLeaderPin` rejects a non-positive activation
height, an empty list, zero keys and duplicate keys.

`State` refreshes the schedule through its leader-schedule provider. The list
comes from the on-chain `sequencer` module; the name denotes an integrated
runtime capability. Pinned leaders must also belong to the validator set:
proposal construction and vote signing look up the selected consensus key there.
The `[consensus.sequencer]` settings control the associated timing policy.
They do not define an additional network component.

### Autobahn leaders

A non-empty top-level `autobahn-config-file` selects optional Autobahn operation;
an empty value disables it. [`Config.ValidateBasic`](../../config/config.go)
rejects enabling `[consensus.sequencer].enable` together with that file.

[`Committee.Leader`](../../autobahn/types/committee.go) uses this separate rule:

```text
material = "pax/autobahn/leader/v1\x00" || committeeDomain
           || big-endian uint64(view.Index) || big-endian uint64(view.Number)
pos = integer(SHA256(material)) mod totalWeight
select the public key whose cumulative weight interval contains pos
```

`view.Index` identifies the road consensus instance; `view.Number` identifies a
view within that instance. The committee domain binds the chain ID, first global
block number, genesis timestamp, and the sorted public keys with their weights.
`NewCommittee` sorts keys with `PublicKey.Compare`, removes zero-weight members,
and rejects zero total weight, overflow and more than `MaxCommitteeSize` (16)
replicas.

Despite its name, `NewRoundRobinElection` constructs an equal-weight committee
and uses this hash-based `Leader` method. Node assembly in
[`NewGigaRouter`](../../internal/p2p/giga_router.go) calls that constructor with
the configured validator keys and the genesis chain ID, initial height and time.
It does not import classic proposer priorities or genesis voting-power ratios
into Autobahn leader election. Lane production by committee members is distinct
from selecting the road leader that proposes certified lane ranges.

`FullProposal.VerifyAt` checks the selected leader's signature and view context;
see [validator signing](signing.md#autobahn-signatures-and-certificates).

## Validator Set Changes

### Classic lifecycle

The validator set is fixed during the rounds at a height. Application updates
come from `ResponseFinalizeBlock.ValidatorUpdates` through
[`BlockExecutor`](../../internal/state/execution.go), rather than the inherited
`EndBlock` lifecycle. Updates returned at height `h` modify `NextValidators` and
become the active validator set at `h + 2`. Existing validators retain their
priority on a power change; additions and removals also update priority
normalization as described below.

An on-chain leader pin and the active validator set must agree. Updating the
validator set does not turn a pinned non-member into a validator.

### Autobahn lifecycle

Node startup constructs one immutable committee from the Autobahn configuration
and genesis context. Availability queues are initialized once for its lanes in
[`avail/inner.go`](../../internal/autobahn/avail/inner.go). Dynamic committee
changes are not implemented. Classic `FinalizeBlock` validator updates must not
be interpreted as an Autobahn membership-change mechanism.

Changing the committee changes both leader selection and the signature domain.
Editing the configuration and reusing old safety state is not a supported
committee transition. Restart uses the same committee context and restores the
certificates and votes described in [WAL recovery](wal.md#autobahn-persistence).

## Proposer Priority Reference

The remainder explains the inherited weighted round-robin bookkeeping still
implemented by [`ValidatorSet`](../../types/validator_set.go). Classic state
execution advances these priorities, but `RoundState.Leader` determines the
actual round leader.

Let `A(i)` be a validator's priority, `VP(i)` its voting power, and `P` the total
voting power. `IncrementProposerPriority(times)` first rescales and centers the
priorities, then performs `times` elections:

```text
if max(A) - min(A) > 2 * P:
    ratio = ceil((max(A) - min(A)) / (2 * P))
    A(i) = A(i) / ratio  for each validator (integer division)
avg = floor(sum(A) / numberOfValidators)
A(i) -= avg             for each validator
repeat times:
    A(i) += VP(i)       for each validator (clipped addition)
    prop = validator with greatest A(i)
    A(prop) -= P        (clipped subtraction)
```

Equal priorities are resolved by the lexicographically smaller validator
address in [`CompareProposerPriority`](../../types/validator.go).
`IncrementProposerPriority` requires a non-empty set and a positive `times`.

### Stable Set

For weights 1 and 3, starting at priorities zero and choosing `p1` on the tie,
the bookkeeping proceeds as follows:

| Election | Priorities after adding power `(p1, p2)` | Selected | Priorities after subtracting `P` |
| --- | --- | --- | --- |
| 1 | `(1, 3)` | p2 | `(1, -1)` |
| 2 | `(2, 2)` | p1 | `(-2, 2)` |
| 3 | `(-1, 5)` | p2 | `(-1, 1)` |
| 4 | `(0, 4)` | p2 | `(0, 0)` |

Adding all voting powers and subtracting their sum preserves the priority sum
within an election. Integer centering can leave a small remainder. This example
illustrates the priority algorithm, not the runtime's hash-based leader sequence.

### Voting Power Change

A power update preserves an existing validator's priority. A large power
reduction can therefore leave a priority spread that needs rescaling.

### Validator Removal

Removing a validator removes its priority from the sum. Centering the remaining
priorities prevents repeated set changes from moving the whole queue toward
integer limits.

### New Validator

A newly added validator starts with priority `-(P + (P >> 3))`, using total
power after additions and power changes but before removals. The penalty limits
the advantage of leaving and rejoining the queue. Integer truncation matters:
for total power 12 the initial priority is `-13`, not a floating-point value.

### Proposer Priority Range

A large power reduction can leave a small validator set with a large priority
spread. Rescaling divides priorities by the ceiling of `spread / (2 * P)`
before centering and election. The election itself can increase the spread
again; `2 * P` is the rescaling threshold, not a post-election invariant.

## Wrinkles

### Validator Power Overflow Conditions

`MaxTotalVotingPower` is `math.MaxInt64 / 8`. Validator-set updates reject
negative powers, duplicate updates, invalid removals and a total above that
limit. A zero power in an update means removal.

### Proposer Priority Overflow/Underflow Handling

Priority arithmetic uses signed 64-bit integers. The priority implementation
clips overflowing additions and subtractions to `MaxInt64` or `MinInt64`.
This bookkeeping protection does not change the active leader-selection rule.

## Requirement Fulfillment Claims

Determinism is conditional on all validators having identical consensus inputs.
The classic hash draw, active pin and Autobahn committee draw described above
are separate rules. Weighted round-robin examples establish no finite-window
fairness guarantee for any of those live selection paths.

Timestamp validation is described in the
[PBTS applicability and draft](proposer-based-timestamp/v1/pbts_001_draft.md).
Signature reuse and restart safety are described in [signing](signing.md) and
[WAL recovery](wal.md).
