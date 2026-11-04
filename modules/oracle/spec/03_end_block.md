<!--
order: 3
-->

# End Block

Oracle work is divided between `MidBlocker` and `EndBlocker` in [`abci.go`](../abci.go). [`module.go`](../module.go) exposes both hooks. In the current node path, [`node/app.go`](../../../node/app.go) executes transactions, then calls `MidBlock` from `CompleteBlock`, then performs EndBlock processing. Oracle precedes `pricefeed` in the registered MidBlock order. The tally therefore does not provide newly calculated prices to transactions that already executed in that block.

Both period checks use [`utils.IsPeriodLastBlock`](../utils/period.go):

```text
(blockHeight + 1) % blocksPerPeriod == 0
```

## Tally Exchange Rate Votes in MidBlock

When the current block ends a `VotePeriod`, `MidBlocker` runs the [voting procedure](01_concepts.md#voting-procedure):

1. Build a claim map from selected bonded validators, up to staking's `MaxValidators`, with their consensus voting power. Capture the current vote-target set and its size. Genesis initializes that set from the validated whitelist before any recovered ballot is processed.
2. Organize stored aggregate votes into sorted denom ballots. Only validators present in the claim map participate; non-positive rates carry zero power.
3. Discard non-target ballots. Separate target ballots that have zero power or power below the rounded threshold `VoteThreshold * totalBondedPower`. They will contribute participation wins without updating rates.
4. Choose the passing target with the highest turnout as the reference denom, using ascending denom name to break ties. Calculate its weighted median. For every passing ballot, in sorted denom order, run `Tally`; non-reference ballots are first converted to sorted cross rates against each validator's reference vote.
5. Skip zero median results. Convert other cross-rate medians back with `referenceMedian / crossRateMedian`, then call `SetBaseExchangeRateWithEvent`. This stores the rate with the current block height and millisecond timestamp and emits [exchange_rate_update](05_events.md#midblocker).
6. Run `UpdateDidVote` for each below-threshold target ballot, recording one win and participation for every tuple in it.
7. For each validator claim, increment success if its wins equal the original target count; otherwise increment abstain if `DidVote` is false, or miss if it is true. See [counter rules](01_concepts.md#slashing).
8. Delete all aggregate votes with `ClearBallots`. Run `ApplyWhitelist` to synchronize vote targets with the whitelist and register missing bank denom metadata when rebuilding targets.

If no target ballot passes, no exchange rate is updated, but participation accounting, vote deletion, and target synchronization still run. Existing exchange rates are retained with their original source height and Unix millisecond timestamp until replaced or removed by the feed cleanup below; running a tally without a replacement does not refresh either field. There are no current prevotes to preserve and no ballot-winner token distribution or snapshot creation in this hook.

The tally and cross-rate helpers are in [`tally.go`](../tally.go), [`keeper/ballot.go`](../keeper/ballot.go), and [`types/ballot.go`](../types/ballot.go).

## Slash-Window Processing in EndBlock

When the current block ends a `SlashWindow`, `EndBlocker` performs two operations:

1. `SlashAndResetCounters` evaluates each stored penalty counter. If its total is nonzero, it computes `success / (success + abstain + miss)` and compares it with `MinValidPerWindow`. A strictly lower rate causes a bonded, unjailed validator to be slashed by `SlashFraction` and jailed. The slash uses `blockHeight - sdk.ValidatorUpdateDelay - 1` as its distribution height. It then emits [end_slash_window](05_events.md#endblocker) with the counts and deletes the counter. An all-zero counter is skipped without an event or deletion.
2. `RemoveExcessFeeds` deletes stored exchange rates whose denoms are absent from the current vote-target set. This cleanup does not emit `exchange_rate_update`.

If both boundaries occur in the same block, MidBlock has already accounted for the period before EndBlock evaluates the counters. Default `MinValidPerWindow = 0` and `SlashFraction = 0` do not trigger penalties; the window still reports and resets nonempty counters and removes excess feeds. See [`keeper/slash.go`](../keeper/slash.go), [`keeper/keeper.go`](../keeper/keeper.go), and the [default parameters](../README.md#parameters).

Outside a slash-window boundary, the oracle EndBlock hook does not tally prices or reset counters. See [State](02_state.md) for retained rates and [Messages](04_messages.md) for how votes enter the store.
