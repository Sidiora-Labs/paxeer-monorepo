<!-- markdown-link-check-disable -->

# Light Client Verification

This page retains the useful trust, timing and validator-overlap reasoning from
the upstream Tendermint verification draft, with an explicit map to this tree.
Its five-part model concerns classic signed headers. The named draft operations
are conceptual operations, not executable Go or the Autobahn certificate API.

Paxeer is one BFT network run by `paxd`, with EVM chain ID 125 (`0x7d`). The kernel
is part of that network and proposes blocks. A transaction receipt is final
when its block is committed. Light verification checks evidence of that history;
it does not introduce an additional settlement or finality stage.

# Status

This is inherited research material for the classic Tendermint light-client
model. It does not establish that the same trust period, validator-transition
rules, verification trace or recovery argument applies to Autobahn.

## Previous Versions

[Version 001](./verification_001_published.md) is the upstream published model.
Its formalization and model-checking history concern that model, not a runtime
qualification of this fork. The
[signature-verification companion](./verification_003_draft.md) describes the
implemented classic early-exit routines in place of the old incomplete fragment.

## Issues that are addressed in this revision

The draft introduced a single trusted light block as the root of each forward
or backward verification operation, and proposed recording the block that
justified each verification step. Those ideas remain useful for reasoning about
attack detection and relayer traces. They must be distinguished from the
actual Go light store and Autobahn's certificate and persistence types.

## Current Implementation Map

| Subject | Source of behavior | Applicability |
| --- | --- | --- |
| Classic light-block shape | [`types/light.go`](../../../types/light.go) | `LightBlock` contains a signed header and validator set |
| Classic adjacent, skipping and backward checks | [`light/verifier.go`](../../../light/verifier.go) | Signed-header verification under the trust model below |
| Fetching, scheduling, witnesses and storage | [`light/client.go`](../../../light/client.go), [`light/store/store.go`](../../../light/store/store.go) | Classic client orchestration and persistent light blocks |
| Classic commit signatures | [`types/validation.go`](../../../types/validation.go) | Validator-set voting power, chain-bound vote signatures |
| Autobahn committee and certificates | [`autobahn/types/committee.go`](../../../autobahn/types/committee.go), [`commit_qc.go`](../../../autobahn/types/commit_qc.go), [`msg.go`](../../../autobahn/types/msg.go) | Committee-bound signatures, lane ranges and certified headers |
| Autobahn certificate and payload ingestion | [`internal/p2p/giga/data.go`](../../../internal/p2p/giga/data.go), [`internal/autobahn/data/state.go`](../../../internal/autobahn/data/state.go) | Full commit-QC streams and block fetches |
| Autobahn vote/QC restart state | [`persisted_inner.go`](../../../internal/autobahn/consensus/persisted_inner.go), [`persist`](../../../internal/autobahn/consensus/persist/) | Durable consensus artifacts, separate from the draft light store |

A non-empty top-level `autobahn-config-file` selects Autobahn in node setup.
The [P2P message map](../../p2p/messages/README.md#mode-applicability) explains
which compatibility reactors and streaming RPCs are used. Neither classic
`VerifyCommitLight` nor the algorithms below consume Autobahn certificates.

In Autobahn, `CommitQC.Verify` validates the proposal and verifies signatures
against `Committee.CommitQuorum()`. If total committee weight is `W`, the tolerated
fault weight is `floor((W - 1) / 3)` and commit quorum is `W - Faulty()`.
Certificate verification rejects duplicate signers and verifies every supplied
signature using the committee's domain. That domain incorporates chain identity,
ordered replicas and weights, initial global block number and genesis time.
The node currently constructs an equal-weight committee from its configured
validator addresses.

`FullCommitQC.Verify` additionally checks the number, lane, block number and
hash linkage of the certified headers. The streaming consumer verifies this
certificate and requires its global range to cover and advance the next needed
block. Payload fetching and data-state insertion then verify the block and its
match to the certified header. This is a different verification path from the
classic trusting-period argument. Restart safety also depends on persisted votes
and QCs; the conceptual `LightStore` below is not a substitute for those records.

# Outline

- [Part I](#part-i---tendermint-blockchain): classic headers, validator sets and trust assumptions.
- [Part II](#part-ii---sequential-definition-of-the-verification-problem): safety and progress goals.
- [Part III](#part-iii---light-client-as-distributed-system): faulty providers and timing assumptions.
- [Part IV](#part-iv---light-client-verification-protocol): fetching, local verification and scheduling.
- [Part V](#part-v---supporting-the-ibc-relayer): backward verification and research trace semantics.

The retained TMBC and LCV tags identify model statements, not exported symbols.
All assumptions in Parts I–V concern the classic model unless a paragraph names
a current implementation explicitly.

# Part I - Tendermint Blockchain

## Header Fields necessary for the Light Client

#### **[TMBC-HEADER.1]**

A classic light client authenticates a header rather than downloading all block
transactions. The header commits to the data and relevant consensus state by
hashes. A signed header combines that header with its commit.

#### **[TMBC-HASH-UNIQUENESS.1]**

The model assumes collision-resistant hashes and unforgeable signatures. Hashes
identify committed data; they do not themselves prove that a validator quorum
signed a block.

#### **[TMBC-HEADER-FIELDS.2]**

The current classic `Header` includes `ChainID`, `Height`, `Time`, `LastBlockID`,
`LastCommitHash`, `DataHash`, `ValidatorsHash`, `NextValidatorsHash`, `AppHash`,
`LastResultsHash`, `EvidenceHash`, `ConsensusHash`, `Version` and
`ProposerAddress`. See [`types/block.go`](../../../types/block.go) for the actual
types. In particular, `ValidatorsHash` and `NextValidatorsHash` commit to
validator sets; they are not the sets themselves. `LastBlockID` links the previous
block; the commit authenticating the current header is in `SignedHeader.Commit`.

#### **[TMBC-SEQ.1]**

The model treats the committed classic chain as a sequence of headers ordered by
height. A validator set may change from one height to the next.

#### **[TMBC-VALIDATOR-PAIR.1]**

A validator is identified by its validator address and public key and has voting
power. This identity is distinct from a transport peer's node ID. A full node
serving headers need not itself be a validator.

#### **[TMBC-VALIDATOR-SET.1]**

`TotalVotingPower(V)` is the sum of the voting powers in validator set `V`.
Thresholds are measured in voting power, not a count of signatures.

#### **[TMBC-VOTE.1]**

Classic votes bind their type, height, round and block ID through chain-specific
sign bytes. Round zero is valid. A commit authenticates a block with precommit
signatures; nil and absent votes do not count toward that block's quorum.

#### **[TMBC-COMMIT.1]**

The current `Commit` has `Height`, `Round`, `BlockID` and `Signatures`. A successful
classic light commit check requires signed voting power strictly greater than
two thirds of the candidate validator set's total power.

## Tendermint Failure Model

#### **[TMBC-AUTH-BYZ.1]**

A Byzantine participant can send arbitrary data or withhold replies, but cannot
forge another validator's signature. A provider's claims and local consistency
checks alone therefore do not establish trust.

#### **[TMBC-TIME-PARAMS.1]**

The research model assumes a trusting period shorter than the unbonding period.
This is a security assumption about validator behavior, not a duration proved
by a signature or a configuration rule enforced by the local verifier.

#### **[TMBC-CORRECT.1]**

`correctUntil(v, t)` means validator `v` follows the consensus protocol until
real time `t`. The model abstracts away the difference between real time and
header time; implementations also need explicit clock-drift checks.

#### **[TMBC-FM-2THIRDS.1]**

For a trusted header `h`, the model assumes validators holding more than two
thirds of its next validator set's voting power remain correct until
`h.Time + trustingPeriod`. A client must obtain its initial trusted header and
associated trust information independently of an untrusted provider.

#### **[TMBC-CORR-FULL.1]**

A correct provider serves a prefix of the committed chain. In practice a node
may prune old blocks, so availability of a requested height is an additional
condition for progress; [block-sync status](../../p2p/messages/block-sync.md#statusresponse)
describes retained storage bounds, not proof of correctness.

## What the Light Client Checks

#### **[TMBC-VAL-CONTAINS-CORR.1]**

Within the trusting period, signatures from more than one third of a trusted
set's voting power include at least one correct validator under the failure
model. For skipping verification, this trusted overlap is checked in addition
to the new set's own greater-than-two-thirds commit quorum.

#### **[TMBC-SOUND-DISTR-POSS-COMMIT.1]**

A possible classic commit for block `b` contains valid precommit signatures for
the same height, round and block ID, with signed power greater than two thirds
of `b`'s validator set. Duplicate or mismatched identities cannot add voting
power. Matching a header's hashes without checking signatures is insufficient.

#### **[TMBC-VAL-COMMIT.1]**

The safety argument combines valid candidate-quorum evidence with trusted
validator overlap under the stated fault assumptions. It does not follow from
the mere presence of a `Commit` object. The implementation's signature selection
and early-exit behavior are described in the [companion](./verification_003_draft.md).

## Context of this document

The core verifier answers whether an untrusted classic signed header can be
authenticated from a trusted header and validator information. Provider selection,
witness cross-checking and attack handling belong to the surrounding classic
client. The upstream detection research is retained under
[light-client detection](../detection/detection_003_reviewed.md).

# Part II - Sequential Definition of the Verification Problem

## Verification Informal Problem statement

Given a trusted root at height `h1` and a target `h2 > h1`, obtain an authenticated
header at `h2`. Checking every intervening header is possible but expensive;
skipping verification can reduce downloads when enough trusted voting power
also signed the target.

## Sequential Problem statement

#### **[LCV-SEQ-LIVE.1]**

With a correct available provider, timely communication and a root that remains
within its trusting period during verification, the operation should eventually
authenticate the target header.

#### **[LCV-SEQ-SAFE.1]**

Only headers authenticated under the trust model may be recorded as verified.
Downloaded but unchecked candidates must not become trusted merely because they
were stored locally.

# Part III - Light Client as Distributed System

## Incentives

A faulty provider can try to supply a forged header, inconsistent validator set
or unavailable target. The classic client's witness and attack-detection work
supplements the local verifier; this model does not make an unresponsive peer
cooperate or remove the need for an independently trusted starting point.

## Computational Model

#### **[LCV-A-PEER.1]**

The model calls the queried full node the primary and assumes it may be faulty.

#### **[LCV-A-COMM.1]**

For the model's progress argument, communication with a correct provider is
reliable and has a known one-way bound `Delta`; a round-trip timeout must allow
at least `2 * Delta`. This is an analytical assumption, not the node's actual
configured P2P or request timeout. Real client fetches are context-controlled.

## Distributed Problem Statement

### Two Kinds of Termination

A verification operation may succeed with authenticated data or fail because a
provider is unavailable, data is invalid, or the trusted root has expired.
Safety must hold even when success cannot be guaranteed. Having an unexpired
root at the start is insufficient if it expires before verification finishes.

### Design choices

The research `LightStore` distinguished `StateUnverified`, `StateVerified`,
`StateFailed` and `StateTrusted`, and proposed a verification-root link for each
accepted block. These labels describe the draft's reasoning. The implemented
store persists light blocks through a different interface described below.

### Temporal Properties

The intended invariant is that every verified header belongs to the chain under
the failure model. Conditional progress additionally requires an available
correct provider and a usable trust root throughout the operation. Bounded
failure is preferable to promoting unchecked data after a timeout.

### Solving the sequential specification

The distributed safety invariant implies the sequential safety property.
Sequential progress follows only under the provider, timing and trust conditions;
a Byzantine primary does not supply unconditional liveness.

# Part IV - Light Client Verification Protocol

## Definitions

### Data Types

#### **[LCV-DATA-LIGHTBLOCK.1]**

The actual classic type in `types/light.go` is:

```go
type LightBlock struct {
    *SignedHeader
    ValidatorSet *ValidatorSet
}
```

`SignedHeader` contains `*Header` and `Commit *Commit`. It does not embed both the
current and next validator sets. Basic validation checks structure and hash
consistency; cryptographic verification is a separate step.

#### **[LCV-DATA-LIGHTSTORE.2]**

The draft `LightStore` is an abstract collection of candidates, verified blocks
and verification links. The current `light/store.Store` instead exposes
`SaveLightBlock`, `DeleteLightBlock`, `LightBlock`, `LastLightBlockHeight`,
`FirstLightBlockHeight`, `LightBlockBefore`, `Prune` and `Size`. It does not expose
the draft's `Update`, `TraceTo` or verification-state enum. A successful save does
not by itself authenticate a candidate.

### Inputs

The research operation receives a primary, an independently trusted root and a
target height. The implemented classic client's public entry point is
`Client.VerifyLightBlockAtHeight(ctx, height, now)`; the low-level verifier
functions take signed headers, validator sets and explicit time parameters.

### Configuration Parameters

`trustingPeriod` is a duration; `maxClockDrift` is the allowed future offset.
`trustLevel` is an exact `math.Fraction`, not floating point.
`DefaultTrustLevel` is `1/3`, and `ValidateTrustLevel` accepts the interval
`[1/3, 1)`. The actual overlap check requires signed power strictly greater than
the integer threshold derived from that fraction.

### Invariants

#### **[LCV-INV-TP.1]**

For forward classic verification, the trusted header must satisfy
`trusted.Time + trustingPeriod > now`. Equality means expired. If trust has
expired, a newer trust root must be obtained independently; repeated fetching
from the same untrusted source does not renew it.

### Used Remote Functions

The classic [provider](../../../light/provider/http/http.go) obtains a signed
header and validator set using the consensus client's `Commit` and `Validators`
methods. These are consensus-provider calls, not instructions for the public
EVM JSON-RPC service. The client's expected chain identity is the consensus
chain ID, distinct from the EVM numeric chain ID.

#### **[LCV-FUNC-FETCH.1]**

The conceptual `FetchLightBlock` obtains the candidate signed header and the
validator set required for its validation. Each network response remains
untrusted until verification. Providers may fail, time out, serve a pruned
height or return malformed data; these are errors, not authenticated absence
proofs.

## Core Verification

### Outline

The draft separates fetching, local checking and scheduling. The classic client
implements these responsibilities in `light/client.go`; the draft names
`VerifyToTarget`, `ValidAndVerified` and `Schedule` are not exported Go APIs.

#### **[LCV-FUNC-MAIN.2]**

Conceptually, try the target using the latest verified root. If it verifies,
record the successful step. If overlap is insufficient, choose an intermediate
height and verify that before retrying the target. Invalid signatures,
inconsistent data, expiration and fetch failures must not be treated as merely
insufficient overlap.

### Details of the Functions

#### **[LCV-FUNC-VALID.2]**

The real classic `Verify` dispatches to `VerifyAdjacent` when heights differ by
one, and to `VerifyNonAdjacent` otherwise. Both forward paths check trust expiry,
candidate basic validity, matching chain identity, strictly increasing height
and time, candidate time strictly before `now + maxClockDrift`, and equality of
`untrustedHeader.ValidatorsHash` with the supplied validator set hash.

For adjacent headers, the candidate's `ValidatorsHash` must equal the trusted
header's `NextValidatorsHash`, and the candidate set must provide a valid
commit with more than two thirds signed power. For non-adjacent headers,
`VerifyCommitLightTrusting` first checks the configured trusted-set overlap;
then `VerifyCommitLight` checks the candidate set's greater-than-two-thirds
quorum. The trusted validator information must be authenticated; a provider's
replacement validator set cannot become a root of trust.

The verifier reports expired trust as `ErrOldHeaderExpired`, invalid candidates
as `ErrInvalidHeader`, and insufficient trusted overlap as
`ErrNewValSetCantBeTrusted`. Those are Go error types, not network error codes.

#### **[LCV-FUNC-SCHEDULE.1]**

The research scheduler maintains these bounds, where `verified` is the highest
successfully authenticated height and `attempted` is the last attempted height:

- After a successful step below the target, choose `verified < next <= target`.
- After insufficient overlap, choose `verified < next < attempted`.
- At the target, stop successfully.

The current client's skipping routine manages intermediate light blocks and
verification traces internally. This does not implement the draft's proposed
persistent `LightStore.TraceTo` interface.

### Solving the distributed specification

Each accepted intermediate step must preserve authentication from the original
root. Fetching fewer headers changes cost, not the required checks. Provider
failure must leave unverified data outside the trusted result.

## Liveness Scenarios

Stable validator membership permits large skips. Rapid changes reduce trusted
overlap and can require more intermediate headers. Raising the trust level can
increase that work. None of these strategies permits verification to continue
from an expired root, or guarantees success against an unavailable primary.

# Part V - Supporting the IBC Relayer

## Older Targets and Backward Verification

For targets already present in trusted storage, reuse the authenticated light
block. To reach an earlier unavailable header, the research `Backwards` operation
walks predecessor hashes from a trusted later header.

The actual `VerifyBackwards` checks the older header's basic validity, equal
chain identity, strictly earlier time and equality of its hash with the trusted
header's `LastBlockID.Hash`. Its caller supplies the adjacent predecessor. It
does not recheck trusting-period expiration or verify a new signature quorum;
the trust derives from the authenticated hash link. It is distinct from forward
skipping verification and does not describe Autobahn recovery.

## Verification Traces

The draft proposed `verification-root` links so a relayer could reconstruct which
lower trusted block justified each later block. A link must point to a block in
the trace, and the earliest trusted root has no predecessor. `LatestPrevious`,
`Lowest`, `MinVerified` and `TraceTo` were proposed store operations for selecting
roots and recovering such traces. They are research names, not the implemented
store API.

Backward traversal and trace construction remain useful design topics, but this
draft does not claim a complete IBC integration or an Autobahn light-client
proof. Use the current consumer and persistence sources in the
[implementation map](#current-implementation-map) for runtime behavior.

# References

- [Published classic model](./verification_001_published.md)
- [Classic signature-verification notes](./verification_003_draft.md)
- [Classic attack-detection research](../detection/detection_003_reviewed.md)
- [Light-client supervisor draft](../supervisor/supervisor_001_draft.md)
- [P2P protocol applicability and Autobahn streams](../../p2p/messages/README.md)
- [Block-sync storage bounds and data fetch](../../p2p/messages/block-sync.md)
