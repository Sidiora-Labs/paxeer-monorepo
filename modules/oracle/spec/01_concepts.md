<!--
order: 1
-->

# Concepts

The native `oracle` module maintains validator-voted exchange rates in Paxeer's committed chain state. Its voting and penalty policy is controlled by the parameters summarized in the [module README](../README.md#parameters).

## Voting Procedure

Voting is single-step. A bonded validator's operator account or delegated feeder signs a [MsgAggregateExchangeRateVote](04_messages.md#msgaggregateexchangeratevote) containing decimal rate/denom pairs. Only current vote targets are accepted. There is no commit/reveal round, prevote, hash, or salt.

On a vote-period boundary, `MidBlocker` builds claims for the selected bonded validators in staking power order, up to `MaxValidators`. It groups their stored aggregate votes into sorted denom ballots; tuples with non-positive rates have zero power. Votes outside the selected claim set do not participate.

For each target, a ballot passes if its power is nonzero and at least `RoundInt(VoteThreshold * totalBondedPower)`. The passing target with the greatest ballot power becomes the reference denom; equal power is resolved by ascending denom name. Non-target ballots are discarded. Below-threshold target ballots do not update exchange rates, but each submitted tuple in them gives its validator one win and records participation, including tuples with zero power.

The reference rate is its ballot's weighted median. For each other passing denom, the tally converts each validator's vote to `referenceVote / denomVote`, sorts those cross rates, and calculates a weighted median. A missing positive reference vote, a non-positive denom rate, or division overflow makes that cross-rate entry zero with zero power. The stored result is `referenceMedian / crossRateMedian`; a zero median is skipped. The weighted median ignores zero-power entries and selects the first rate whose cumulative power reaches `ceil(totalPower / 2)`, choosing the lower rate at an exact even-power half.

Each accepted result updates the rate's observation height and millisecond timestamp and emits `exchange_rate_update`. Rates without a replacement persist with their original metadata. After accounting for participation, the module clears aggregate votes and synchronizes vote targets with `Whitelist`. See [State](02_state.md) and [MidBlock and EndBlock](03_end_block.md).

Implementation: [`abci.go`](../abci.go), [`tally.go`](../tally.go), [`keeper/ballot.go`](../keeper/ballot.go), and [`types/ballot.go`](../types/ballot.go).

## Reward Band

For a passing ballot, `Tally` uses an inclusive winning interval around its median:

```text
spread = max(median * RewardBand / 2, StandardDeviation(ballot, median))
median - spread <= vote <= median + spread
```

For non-reference denoms, this comparison uses cross rates. The current `StandardDeviation` implementation averages squared deviations from the median over all ballot entries, without power weighting, and uses a floating-point square root. The median itself is power-weighted. `RewardBand` defaults to `0.02`, giving a nominal one-percent spread on either side before any widening.

Winning entries increment `WinCount` and add voting power to the claim's `Weight`; every entry processed by `Tally` sets `DidVote`. These claims drive penalty counters. The current block hooks do not pay token rewards to ballot winners.

## Slashing

At each vote-period boundary, the module increments exactly one counter for each selected validator, in this order:

1. **Success** if `WinCount` equals the number of vote targets captured at the start of the period's tally.
2. **Abstain** if success did not apply and `DidVote` is false.
3. **Miss** otherwise.

Thus a missing target vote prevents success even if all submitted votes won. Below-threshold tuples count toward wins as described above; a target with no ballot contributes no wins. With no vote targets, the first condition succeeds with zero wins.

On a slash-window boundary, EndBlock computes `success / (success + abstain + miss)`. A rate strictly below `MinValidPerWindow` causes slashing by `SlashFraction` and jailing only if the validator is bonded and not already jailed. A zero slash fraction does not itself prevent jailing when the minimum is positive. The default minimum is zero, so no nonnegative success rate fails it.

For each nonempty counter, EndBlock emits [end_slash_window](05_events.md#endblocker), then deletes the counter. An all-zero counter is skipped, without an event or deletion. See [`keeper/slash.go`](../keeper/slash.go).

## Abstaining from Voting

A zero-rate tuple expresses no positive price and has zero voting power. Ballot organization also treats any stored negative rate as zero-power. That ballot treatment differs from a period-level abstain: submitting zero-rate tuples can still set `DidVote`, count as below-threshold wins, or lead to a miss. Omitting all effective participation normally increments the abstain counter, subject to the success-first rule above.

Do not treat a stored rate or an `actives` result as evidence of a new vote. Use the rate's observation metadata to assess its age; [state and freshness](02_state.md#exchangerate) explains the units and persistence behavior.

## Messages

The module exposes `MsgAggregateExchangeRateVote` and `MsgDelegateFeedConsent`. The operator may always vote with its own account; delegating permits an additional feeder account to vote on its behalf. See [Messages](04_messages.md) for fields, authorization, transaction restrictions, and errors, and [Events](05_events.md) for the emitted attributes.
