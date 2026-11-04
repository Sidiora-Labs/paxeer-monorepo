<!-- markdown-link-check-disable -->

# Proposer-Based Time (first draft)

## Applicability to the Merged Network

This versioned page preserves the first Tendermint PBTS design as research.
Its pseudocode and clock assumptions explain the design; they are not an
Autobahn specification or evidence that a running deployment meets those
assumptions. Paxeer runs consensus inside `paxd`, with the kernel participating
in the same network and proposing blocks. A receipt is final when its block
is committed.

The current tree has two distinct timestamp-validation paths:

| Mode | Timestamp source and validation |
| --- | --- |
| Classic consensus, including pinned proposer operation | The proposal carries the block time. `Proposal.IsTimely` checks receive time against synchrony parameters; the consensus state machine also checks proposal/block timestamp equality and block-time progression. |
| Optional Autobahn | Proposals assign times to ranges of global blocks. Fresh proposals have a future-clock bound; certified reproposals retain the certified time. Classic PBTS parameters do not govern this path. |

See [proposer selection](../../proposer-selection.md) for mode selection and
leader rules, [validator signing](../../signing.md) for signed timestamps, and
[WAL recovery](../../wal.md) for preserving the receive time across restart.

### Classic Runtime Checks

[`Proposal.IsTimely`](../../../../types/proposal.go) uses the closed interval

```text
proposal.Timestamp - Precision <= receiveTime
receiveTime <= proposal.Timestamp + scaledMessageDelay + Precision
```

`SynchronyParams` in [`types/params.go`](../../../../types/params.go) supplies
`Precision` and `MessageDelay` (JSON keys `precision` and `message_delay` under
`synchrony`, with durations represented as nanosecond strings). The message
delay doubles every ten rounds, with a cap to avoid duration overflow. A
negative round is rejected. This interval uses the validator's recorded
proposal receive time, not the time when the proposal is eventually processed.

In [`defaultDoPrevote`](../../../../internal/consensus/state.go), a proposal
whose timestamp differs from the reconstructed block's time receives a nil
prevote. A proposal with `POLRound == -1`, when the validator is not locked,
also receives a nil prevote if it fails the timeliness check. Reproposals and
locked-state handling must be read with the rest of that state machine; the
interval is not an unconditional rule for discarding all received proposals.

[`validateBlock`](../../../../internal/state/validation.go) requires the initial
block time to be at or after genesis and subsequent block times to be strictly
after the previous block time. [`State.MakeBlock`](../../../../internal/state/state.go)
uses the proposer's current time. The integrated pending-block path in
[`CreateProposalBlock`](../../../../internal/state/execution.go) uses the sealed
block's `OpenTime`, rejecting a missing or non-increasing open time. It does not
recompute that time from a weighted median of the previous precommits.

### Autobahn Runtime Checks

[`autobahn/types/proposal.go`](../../../../autobahn/types/proposal.go) defines
`BlockTimestamp`, `NextTimestamp`, `NewProposal` and `FullProposal.VerifyAt`.
Each global block in a proposal's range has the proposal base time plus its
zero-based offset multiplied by `minTimestampDiff` (one microsecond). The next
proposal cannot start before the previous proposal's `NextTimestamp`; the first
proposal is bounded below by the committee's genesis timestamp. Offset
arithmetic is checked for overflow.

Fresh proposals must keep even the last block in their range at or before
`now + MaxFutureProposalDrift`, currently one second. A violation returns
`ErrFutureProposalTime`. The normal proposal loop supplies `time.Now()`;
`NewProposal` also enforces the preceding timestamp lower bound. A justified
reproposal selected by a verified timeout certificate preserves the certified
proposal and bypasses the fresh-proposal clock check. Historical certificate
verification is independent of today's local clock.

These are the implemented Autobahn rules. The `PRECISION`, `MSGDELAY` and
`ACCURACY` assumptions below belong to the classic research model and cannot
be substituted for them.

## Current BFTTime

### Description

“Current” in this section names the baseline of the historical draft. In that
Tendermint design, validators attach local times to precommits and the next
proposer computes a voting-power-weighted median from the commit used in the
next block. The current Paxeer block-building paths described above do not
use that median as the new block timestamp.

### Analysis

The useful properties of that historical baseline are:

1. **Fault tolerance.** With less than one third faulty voting power and a
   commit containing more than two thirds of total power, the median lies
   between correct timestamps represented in the commit.
2. **Proposer influence.** A proposer can influence the result by choosing
   which available precommits to include, subject to a valid commit. Correct
   validators may have observed commits in different rounds.
3. **Liveness.** The baseline does not require synchronized clocks, but its
   consensus liveness still depends on eventual bounds on message delays.
4. **Relation to real time.** Without a clock-accuracy assumption, consensus
   agreement on a timestamp does not establish its distance from real time.
5. **Aggregate signatures.** Different precommit timestamps mean different
   signed messages. Replacing them with a common value can simplify some
   signature schemes, but does not itself implement aggregation.

## Suggested Proposer-Based Time

### Outline

The first-draft alternative has the proposer send its time with the proposed
value. Validators compare that proposed time with their own clocks and vote
on an identity that includes both value and time. Agreement on a block then
includes agreement on its timestamp.

This adds synchrony assumptions. Any safety, liveness or real-time claim below
is conditional on the fault and clock model of the linked research drafts.

### Comparison of the Suggested Method to the Old One

PBTS moves the choice of time from a median of individual precommit times to
a timestamp carried in a proposal. Correct validators constrain that choice
through a local timeliness test. This reduces the proposer's timestamp freedom
under the assumed clock and delivery bounds, while making liveness depend on
those bounds as well as message delivery. Agreement on time alone does not
prove wall-clock accuracy or change the Byzantine fault threshold.

The draft explores time-free precommits as a way to allow common-message
aggregation. Current classic [`CanonicalVote`](../../../../proto/tendermint/types/canonical.proto)
still signs a timestamp. The draft's aggregation motivation is not a description
of the current vote wire format.

### Protocol Overview

The notation in this section is research pseudocode, not Go source or
configuration syntax. `2f + 1` uses the equal-weight model of the draft;
implementations must apply their actual voting-power quorum rules.

#### Proposed Time

A proposed value is represented as `(v, time)`. The block identity committed
by votes binds both components.

#### Reception Step

Record a proposal's arrival time and mark it timely if it falls within the
model's clock and message-delay window. Keep received proposals so later
justification can still be processed. The implemented classic interval is
written explicitly in [Classic Runtime Checks](#classic-runtime-checks);
it replaces the ambiguous timestamp-side inequality in the early draft.

#### Processing Step

The draft modifies the classic round algorithm along these lines:

```text
StartRound(round):
    enter the propose step
    if this validator is proposer:
        wait until local time is later than the previous block time
        v = validValue if one is retained, otherwise getValue()
        broadcast PROPOSAL(height, round, (v, localTime), validRound)
    otherwise:
        schedule the proposal timeout
```

For a timely proposal supported by the required earlier-round prevotes:

```text
if valid(v) and (lockedRound <= validRound or lockedValue == v):
    broadcast PREVOTE(height, round, id(v, proposedTime))
otherwise:
    broadcast PREVOTE(height, round, nil)
```

When the necessary precommits for `id(v, t)` are available:

```text
if valid(v):
    decide (v, t)
    advance height
    reset round-local lock and valid-value state
    start round zero
```

This retains the draft's central idea: the decided value includes time.
It omits other classic locking and reproposal rules; use the linked full
algorithm for the research model and the source links above for runtime rules.

### Property Overview

#### Safety and Liveness

The draft assumes a parameter `PRECISION` such that any two correct clocks
satisfy `|C_V(t) - C_W(t)| < PRECISION` at each real time `t`. It also assumes
proposal delivery between correct nodes takes less than `MSGDELAY` under
synchrony. Choosing values that do not cover real clock skew and delivery delay
can prevent timely voting. These assumptions are not measured or certified by
this document.

#### Relation to Real-Time

The additional assumption `|C_V(t) - t| < ACCURACY` connects a correct local
clock with real time. `ACCURACY` is an analysis parameter, not a node
configuration key. Pairwise clock precision alone does not establish accuracy:
all clocks could agree while sharing the same offset from real time.

### Detailed Specification

The versioned artifacts below retain the upstream model, protocol changes and
formalization. They remain research references rather than a current-network
conformance claim.

- [Part I - System Model and Properties][sysmodel_v1]
- [Part II - Protocol specification][algorithm_v1]
- [TLA+ Specification][proposertla]
- [Historical BFT time][bfttime]
- [Classic consensus paper][arXiv]
- [Upstream TLA+ accountability model][tlatender]
- [Inherited light-client specification][lcspec]

[algorithm_v1]: ./pbts-algorithm_001_draft.md
[sysmodel_v1]: ./pbts-sysmodel_001_draft.md
[proposertla]: ../tla/TendermintPBT_001_draft.tla
[bfttime]: ../../bft-time.md
[tlatender]: https://github.com/tendermint/spec/blob/master/rust-spec/tendermint-accountability/README.md
[lcspec]: ../../light-client/
[arXiv]: https://arxiv.org/abs/1807.04938
