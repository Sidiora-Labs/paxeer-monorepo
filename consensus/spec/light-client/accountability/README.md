---
order: 1
parent:
  title: Accountability
  order: 4
---

# Fork accountability

This page retains the useful attack taxonomy from the upstream **classic
Tendermint** research, with explicit limits on its applicability to this tree.
Paxeer mainnet-beta is one BFT network run by `paxd` (EVM chain ID 125). The
kernel is part of the network and proposes blocks; receipts are final once their
blocks commit. Accountability concerns detecting violations of consensus
assumptions, rather than adding another finality stage.

The Tendermint round, locking and light-client arguments below are not a proof
of Autobahn certificates, committee changes, availability or crash recovery.
See [State](../../core/state.md#autobahn-state-and-recovery) for that boundary,
the [TLA+ synopsis](Synopsis.md) for finite model assumptions, and
[attack isolation](../attacks/isolate-attackers_001_draft.md) for the current Go
evidence behavior.

## Problem Statement

For classic Tendermint, agreement means that two correct nodes cannot decide
different blocks at one height; validity means a decided block satisfies the
protocol's validity predicate. These safety arguments assume less than one third
of the relevant validator voting power is Byzantine. Termination additionally
requires eventual network synchrony and protocol progress assumptions; arbitrary
asynchrony cannot guarantee it.

A commit has signatures for a block from more than two thirds of its validator
set's voting power. Trusting a header also requires authenticating the validator
set and its relationship to a trusted chain. A collection of signatures from an
attacker-chosen set is not sufficient. Sequential and skipping light-client
verification have different trust requirements; see
[`light/verifier.go`](../../../light/verifier.go) and
[`light/trust_options.go`](../../../light/trust_options.go).

A fork here means conflicting commits at one height. It may divide correct
full nodes, or it may be presented only to a light client. Accountability seeks
signed evidence of the validators that deviated, without framing correct ones.
Failure to receive a message in an asynchronous network is not by itself such
evidence.

**Conceptual limit:** evidence is specific to the algorithm it alleges was
violated. Changing vote rules, locking, membership or signature formats requires
revisiting attribution. Classic Tendermint commits cannot be substituted for
Autobahn quorum certificates in these arguments.

## The Misbehavior of Faulty Validators

The retained taxonomy distinguishes:

1. **Double proposal:** conflicting proposals at the same height and round.
2. **Double signing:** conflicting votes at the same height, round and vote type.
3. **Lunatic behavior:** signing a header whose deterministic state commitments
   conflict with the trusted chain.
4. **Amnesia:** voting for another value across rounds without the unlocking
   justification required by the classic locking rules. Changing a vote across
   rounds is not automatically misconduct; legitimate unlocking is possible.
5. **Spurious messages:** sending a message without observing its prerequisite
   quorum. Proving an absent observation is harder than presenting two
   contradictory signatures.

This is an analysis taxonomy, not a list of individually supported punishment
messages. A protocol violation can occur without a fork; conversely, conflicting
commits alone may not identify which cross-round voters violated locking rules.

## Two types of forks

- **Fork-Full:** correct validators decide different blocks at one height. The
  branches can also commit different future validator sets, so divergence can
  propagate to full nodes and their light-client users.
- **Fork-Light:** correct validators agree, while an attacker presents a forged
  alternative header and commit to a light client. A client that does not
  execute the application needs trusted header verification and independent
  witnesses to detect conflicts.

# Attack scenarios

## On-chain attacks

### Equivocation (one round)

**F1:** a validator signs conflicting prevotes or precommits at the same height,
round and type. Two commits for different blocks in one round have a voting-power
intersection greater than one third. The overlapping commit signers provide
attributable evidence once the commits and their signatures are verified.

### Flip-flopping

Classic locking permits a change of value with the required later-round
justification. Two cases must be distinguished:

- **F2, faulty flip-flopping (amnesia):** a process abandons a lock without the
  required justification. Conflicting decisions can occur across rounds without
  a pair of same-round duplicate votes from every faulty process.
- **F3, back to the past:** an attacker combines delayed or newly produced old-round
  signatures with honest signatures from that round to manufacture a conflicting
  commit after the honest chain has moved on. An old message is not misconduct
  merely because it was received late.

## Off-chain attacks

- **F4, phantom validators:** signers outside the canonical validator set appear
  in a forged branch. A full node knows canonical membership; a skipping light
  client must authenticate the transition from its trusted set.
- **F5, lunatic validators:** signers support deterministic header commitments
  inconsistent with the trusted chain. A light client checks headers without
  executing all application transitions, making the trust and witness boundary
  essential.

## Types of victims

A full node executes and validates blocks against known chain state. A sequential
light client follows validator-set transitions one height at a time. A skipping
or bisection client authenticates jumps using overlap with a trusted validator
set, within its trusting period and configured trust level.

| Attack | Full-node concern | Light-client concern |
| --- | --- | --- |
| F1 | Conflicting same-round decisions after the fault bound is violated. | Conflicting commit or a fork relayed by a full node. |
| F2 | Conflicting decisions across rounds after invalid unlocking. | Cross-round conflicts whose culpable signers cannot be inferred from two commits alone. |
| F3 | An alternative historical commit; delayed messages alone prove no fault. | A forged historical branch may appear valid without a witness. |
| F4 | Canonical membership checks reject nonmembers. | A forged validator set must still pass the client's trusted-overlap verification. |
| F5 | Execution and known state constrain acceptable commitments. | Header-only verification relies on a trusted base and independent conflict detection. |

The exact attack threshold for a light client depends on verification mode,
trusted height, trust level, timing and membership. The table is not a universal
threshold or completeness proof.

## Detailed Attack Scenarios

### Equivocation based attacks

#### Scenario 1: Equivocation on the main chain

Partition honest validators into groups `CA` and `CB`. A Byzantine set `F`
equivocates, sending votes for `A` to `CA` and `B` to `CB`. If `CA + F` and
`CB + F` each exceed two thirds of voting power, both groups can see a commit.
This violates the classic fault bound. Verified conflicting same-round votes
identify the double signers; those votes must reach a party able to submit
evidence.

#### Scenario 2: Equivocation to a light client (LCS)

A Byzantine set with more than two thirds of the authenticated set can sign an
alternative header while continuing to participate in the canonical chain. A
sequential header-only client can accept it if it sees no contradiction. A
correct independent witness or a separately trusted chain commitment is needed
to expose the conflict. Do not infer application validity from signatures alone.

### Flip-flopping: Amnesia based attacks

#### Scenario 3: At most 2/3 of faults

An attacker controlling at least one third of voting power can help commit `A`
in round `r`, then abandon its lock and help commit `B` in a later round with
honest validators that did not lock `A`. Each commit still needs more than two
thirds of voting power. Honest participation constrains each accepted proposal,
but the resulting decisions can conflict. The research models investigate
attribution using more execution evidence; the Go implementation does not
identify amnesia culprits from the two commits.

#### Scenario 4: More than 2/3 of faults

A faulty quorum can sign an alternative state without honest participation.
Some faulty signers may have abandoned an earlier lock while others never held
it. Attributing all signers as lock violators would frame processes whose
observed messages do not prove that violation. The finite-model result guarded
by `~FaultyQuorum` does not cover this case.

### Back to the past

#### Scenario 5

Honest signatures for `A` in an early round can be combined with attacker
signatures produced later for that same round, after a different value `B`
committed. The completed old-round commit can be shown to an isolated client.
Same-round contradictory votes prove equivocation where present; different-round
signatures and message arrival order alone do not establish amnesia. This
scenario does not justify a separate implemented evidence type.

### Phantom validators

#### Scenario 6

Starting from a trusted height, a client may be shown a forged later header with
a different validator set. Processes absent from the canonical set sign for the
forged branch. A valid skipping attack must first satisfy the verification
conditions against the trusted set; introducing arbitrary new signers does not
by itself accomplish this. The implementation attributes lunatic-attack signers
that belong to the common trusted validator set, rather than exposing a separate
phantom-validator evidence type.

### Lunatic validator

A lunatic attack is classified by conflicting deterministic header fields,
including validator-set hashes, consensus parameters and application commitment.
This classification uses a trusted reference header. It does not independently
reexecute arbitrary application state from an untrusted header.

## Implementation boundary

The current classic evidence type and attribution logic live in
[`types/evidence.go`](../../../types/evidence.go):

- `LightClientAttackEvidence` carries `ConflictingBlock`, `CommonHeight`,
  `ByzantineValidators`, `TotalVotingPower` and `Timestamp`.
- `ConflictingHeaderIsInvalid` compares `ValidatorsHash`, `NextValidatorsHash`,
  `ConsensusHash`, `AppHash`, and `LastResultsHash`. The last comparison is
  conditional on the existing `SkipLastResultsHashValidation` setting.
- `GetByzantineValidators` selects common-set commit signers for a lunatic attack
  and overlapping block signers for same-round equivocation. **For different
  rounds it returns an empty validator set.**
- `ValidateABCI` checks that supplied attribution matches the computed result;
  `ABCI` converts the attributed validators to `LIGHT_CLIENT_ATTACK` misbehavior
  entries. Neither conversion establishes a punishment policy.

[`internal/evidence/verify.go`](../../../internal/evidence/verify.go) validates
signatures, trusted overlap and conflicting headers; basic validation and
non-expiration are prerequisites to `VerifyLightClientAttack`. The evidence pool
also checks local history and evidence metadata. Its age rules are documented in
[State](../../core/state.md#evidence); a research unbonding-period assumption
must not replace those checks.

The [TLA+ synopsis](Synopsis.md) and [Ivy proof notes](../../ivy-proofs/README.md)
retain the model results and their execution limitations. Neither an archived
report nor this documentation's validation asserts that a model checker ran or
that the current node implements every modeled detector.
