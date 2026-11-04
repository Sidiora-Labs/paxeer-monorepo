
<!--
order: 7
-->

# Parameters

The `oracle` module registers the following parameter-store keys in
[`types/params.go`](../types/params.go). The field names used by the parameter
message are defined in [`oracle.proto`](../../../api/oracle/oracle.proto).
Defaults below are the values returned by `DefaultParams()`; existing chain
state can contain different values.

| Parameter key | Message field | Go type | Default | Meaning |
| --- | --- | --- | --- | --- |
| `VotePeriod` | `vote_period` | `uint64` | `2` | Blocks per voting period. |
| `VoteThreshold` | `vote_threshold` | `sdk.Dec` | `0.667000000000000000` | Fraction of total bonded voting power required for a denom's ballot to pass. |
| `RewardBand` | `reward_band` | `sdk.Dec` | `0.020000000000000000` | Full relative band around the weighted median used to count successful votes. |
| `Whitelist` | `whitelist` | `DenomList` | `[{"name":"uatom"},{"name":"ueth"}]` | Denoms used to synchronize vote targets after each tally. |
| `SlashFraction` | `slash_fraction` | `sdk.Dec` | `0.000000000000000000` | Stake fraction slashed when a validator fails the valid-vote requirement. |
| `SlashWindow` | `slash_window` | `uint64` | `216000` | Blocks between penalty assessments and counter resets. |
| `MinValidPerWindow` | `min_valid_per_window` | `sdk.Dec` | `0.000000000000000000` | Minimum ratio of successful votes to all counted voting periods. |
| `LookbackDuration` | `lookback_duration` | `uint64` | `3600` | Snapshot retention lookback and maximum TWAP query lookback, in seconds. |

`SlashWindow` is `2 * utils.BlocksPerDay`, where `BlocksPerDay` is `108000`
in [`utils/period.go`](../utils/period.go); the nominal two-day window is a
block count, not an elapsed-time guarantee. `MinValidPerWindow` is a decimal
ratio, not an integer. With its default of zero, no validator's valid-vote
ratio falls below the penalty threshold. The default slash fraction is also zero.
The removed `reward_distribution_window` (`rewarddistributionwindow` in historical documentation) is not part of the current set. Proto field number **8 is reserved** and must never be reused; `lookback_duration` remains field 9.

[`Tally`](../tally.go) uses half of `RewardBand` on each side of the weighted
median, widening that spread to the ballot's standard deviation if larger.
The band determines vote success accounting; it does not filter votes out of
the median calculation.

[`AddPriceSnapshot`](../keeper/keeper.go) retains snapshots within
`LookbackDuration` and one older snapshot for TWAP calculations. TWAP requests must
use a positive lookback no greater than `LookbackDuration`; a valid lookback
alone does not guarantee that price history is available.

See [Oracle MidBlock](MidBlock.md) for the tally schedule, phase order, and when
updated prices become visible.

## Validation and Governance Examples

`Params.Validate` requires `VotePeriod > 0`, `VoteThreshold > 0.33`, `SlashWindow >= VotePeriod`, and `SlashWindow % VotePeriod == 0`. `RewardBand`, `SlashFraction` and `MinValidPerWindow` must lie in `[0, 1]`, and whitelist names must be nonempty. Parameter-store validators also cap `VoteThreshold` at 1 and `LookbackDuration` at `math.MaxInt64`. The store permits a zero lookback duration, which leaves no valid positive TWAP query; every TWAP request must have `0 < lookback_seconds <= LookbackDuration`.

For illustration only, governance could use `VotePeriod = 4`, `SlashWindow = 200`, `MinValidPerWindow = 0.75`, `SlashFraction = 0.01`, and `LookbackDuration = 600`, leaving the other defaults unchanged. This is not a default or a recommendation; the slash window is divisible by the vote period and these values enable penalties. A slash window of 201 with vote period 4 is invalid.
