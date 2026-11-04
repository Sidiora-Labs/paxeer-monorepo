---
order: 2
---
# BFT Time

Block time is agreed as part of consensus. The current tree has different time
rules for the classic and Autobahn paths; neither obtains block time by taking
the weighted median of the previous commit's vote timestamps. See the
[consensus mode map](consensus.md#consensus-modes) before applying a time rule.

## Representation

Classic headers, proposals, and votes use Go `time.Time`, encoded as protobuf
seconds since the Unix epoch plus a nanosecond component. Autobahn's
[`Timestamp`](../../internal/autobahn/autobahn.proto) also has `seconds` and
`nanos`; [`TimeConv`](../../autobahn/types/time.go) converts these fields to and
from UTC `time.Time`. These are not integer Unix milliseconds. The deterministic
microsecond spacing described below is an algorithmic increment, not the wire
unit. Ethereum-facing block timestamps have whole-second precision, so clients
must not infer a distinct EVM timestamp for every microsecond-spaced block.

## Classic proposal timestamps

[`State.MakeBlock`](../../internal/state/state.go) uses the proposer clock for
an ordinary new block. The integrated sealed-block path in
[`CreateProposalBlock`](../../internal/state/execution.go) instead takes the
pending block's `OpenTime` and requires it to be later than `LastBlockTime`.
The proposer waits when its clock has not advanced past the previous block time.

[`validateBlock`](../../internal/state/validation.go) requires a block after the
initial height to have `Time > LastBlockTime`. At the initial height, time must
be at least genesis time. The consensus state machine also requires
`Proposal.Timestamp == ProposalBlock.Time` before voting for the block.

For a fresh proposal (`POLRound == -1`) when the validator is not locked,
[`defaultDoPrevote`](../../internal/consensus/state.go) checks reception time
with [`Proposal.IsTimely`](../../types/proposal.go). With timestamp `T`, receive
time `R`, precision `P`, and effective message delay `D`, the accepted interval is:

```text
T - P <= R <= T + D + P
```

`P` is `ConsensusParams.Synchrony.Precision`. `D` starts at
`Synchrony.MessageDelay` and doubles every ten rounds, with overflow protection.
A validator prevotes nil for an untimely fresh proposal. The check is conditional:
a certified reproposal or an existing lock does not follow this fresh-proposal
branch. Timestamp equality and block validation still apply. The
[proposer-based timestamp material](proposer-based-timestamp/README.md) supplies
protocol background; the local implementation defines the active checks.

## Autobahn timestamps

The rules are implemented in
[`autobahn/types/proposal.go`](../../autobahn/types/proposal.go). A proposal
orders a half-open range of global blocks `[First, Next)`. If its base timestamp
is `T`, the block at global number `n` receives:

```text
BlockTimestamp(n) = T + (n - First) * 1 microsecond
NextTimestamp    = T + (Next - First) * 1 microsecond
```

`BlockTimestamp` returns no value outside the proposal's range. Checked
arithmetic rejects a timestamp-offset overflow. The first proposal's lower bound
is the committee's genesis timestamp; later proposals must start at or after the
previous commit certificate's `NextTimestamp`. `NewProposal` raises a supplied
creation time to this lower bound when needed. Blocks within a nonempty proposal
therefore have increasing assigned times, and the next nonempty range starts
after the preceding range's last block. An empty range consumes no time offset.

Fresh proposals also have an upper bound.
`FullProposal.VerifyAt` checks timeliness using the validator's consensus clock;
`verifyTimeliness` requires the last assigned block timestamp (or the base time
for an empty range) to be no later than `now + MaxFutureProposalDrift`.
`MaxFutureProposalDrift` is one second. Violation returns
`ErrFutureProposalTime`; this assumes honest consensus clocks remain within that
bound after synchrony.

A valid timeout certificate carrying a prepared reproposal preserves the
certified proposal and its timestamp. After verifying that justification and
matching the reproposed hash, `VerifyAt` does not apply the fresh-proposal
wall-clock test. Historical commit-certificate verification is likewise
independent of the verifier's current clock. This exception does not allow a
leader to invent a different timestamp for the prepared value.

The application receives the assigned global time in `RequestFinalizeBlock.Header.Time`.
The lane payload's `CreatedAt` is separate metadata; it is not the formula above.
See [Creating a proposal](creating-proposal.md#autobahn-proposals) for the
proposal range and byte bounds.

## Historical weighted-median rule

The inherited Tendermint BFT-time design counted each included vote timestamp
with its validator's voting power and chose a weighted median. With less than
one-third Byzantine power and a valid supermajority commit, the median lies
within the honest included timestamps. For example, included votes with weights
27, 10, and 10 at times 98, 1000, and 500 respectively have weighted median 98.

This remains useful protocol history, but it is not a validation rule of either
current path. The old recipe that forced votes to be at least one millisecond
after a locked/proposed block is also not the current signing implementation:
[`signVote`](../../internal/consensus/state.go) initializes votes with the local
clock. Do not derive current block time or application timeouts from that
historical recipe.
