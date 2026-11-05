<!--
order: 8
-->

# Parameters

The staking keeper reads these parameters from the `staking` subspace. This
table follows the registered `ParamSetPairs` in
[types/params.go](../types/params.go), the
[protobuf Params](../../../proto/cosmos/staking/v1beta1/staking.proto) and the
[keeper accessors](../keeper/params.go). Defaults are the values returned by
`DefaultParams()` in this tree, not a claim about a deployed chain's governed
values. The complete default response and a separately labeled transaction
fixture are in the [client reference](./09_client.md#params-1). The SDK-wide
defaults used below are in
[sdk/types/staking.go](../../../types/staking.go).

| Go field / protobuf field | Parameter-store key | Type | Code default |
| --- | --- | --- | --- |
| `UnbondingTime` / `unbonding_time` | `UnbondingTime` | Protobuf `Duration`, Go `time.Duration` | 21 days (`1814400` seconds) |
| `MaxValidators` / `max_validators` | `MaxValidators` | `uint32` | `35` |
| `MaxEntries` / `max_entries` | `MaxEntries` | `uint32` | `7` |
| `HistoricalEntries` / `historical_entries` | `HistoricalEntries` | `uint32` | `10000` |
| `BondDenom` / `bond_denom` | `BondDenom` | `string` | `uhpx` |
| `MinCommissionRate` / `min_commission_rate` | `MinCommissionRate` | Decimal string, Go `sdk.Dec` | `0.050000000000000000` (5%) |
| `MaxVotingPowerRatio` / `max_voting_power_ratio` | `MaxVotingPower` | Decimal string, Go `sdk.Dec` | `0.200000000000000000` (20%) |
| `MaxVotingPowerEnforcementThreshold` / `max_voting_power_enforcement_threshold` | `MaxVotingPowerEnforcementThreshold` | Integer string, Go `sdk.Int` | `1000000` consensus-power units |

## Meaning and validation

- `UnbondingTime` must be positive. It determines the full waiting period for
  each undelegation request, validator unbonding and the full-period
  redelegation case. [Redelegation](./02_state_transitions.md#begin-redelegation)
  can use a source validator's existing deadline or complete immediately instead.
- `MaxValidators` must be nonzero. EndBlock selects up to this number of
  non-jailed validators with positive consensus power.
- `MaxEntries` must be nonzero. It bounds pending unbonding entries per
  delegator/validator pair and redelegation entries per
  delegator/source/destination triple.
- `HistoricalEntries` is a `uint32`; zero is allowed and disables new
  [historical records](./01_state.md#historicalinfo). The registered parameter
  validator checks its type.
- `BondDenom` must be a nonblank, valid SDK denomination. All staking message
  coins must use it.
- `MinCommissionRate` must be in `[0, 1]`. Creation and commission edits reject
  rates below it, in addition to the validator's own maximum-rate and
  rate-increase restrictions. See [messages](./03_messages.md#msgeditvalidator).
- `MaxVotingPowerRatio` must be initialized and in `(0, 1]`.
- `MaxVotingPowerEnforcementThreshold` must be initialized and nonnegative;
  zero is allowed.

## Voting-power controls

At block heights greater than zero, `Delegate` checks the destination's
post-delegation power, including the initial self-delegation and redelegation
paths that call it. It converts tokens to integer power and projects the same
highest-power set selected at EndBlock: up to `MaxValidators` non-jailed,
positive-power candidates. In a redelegation the source shares have already
been removed in the cached context before this projection.

When that projected total power is positive and **at least**
`MaxVotingPowerEnforcementThreshold`, the keeper rejects a destination whose
post-delegation power is greater than
`projected total power * MaxVotingPowerRatio`, returning
`ErrExceedMaxVotingPowerRatio`. Equality with the ratio limit is allowed. The
threshold is in consensus-power units, not raw bond-denomination units. With
the application default reduction of `1000000`, a threshold of `1000000`
corresponds to `1000000000000` raw bond units across the projected set.

This check admits or rejects a delegation; it does not clip a validator's voting
power during EndBlock. See
[the real projection and admission check](../keeper/delegation.go) and
[delegation messages](./03_messages.md#msgdelegate).

## Power reduction

`Keeper.PowerReduction(ctx)` returns the application-wide
`sdk.DefaultPowerReduction`, currently `1000000` raw staking-token units per
consensus-power unit. Positive token balances are divided by that value using
integer division to calculate power.

Power reduction is separate from the on-chain `Params` fields. Although
`KeyPowerReduction` and a validation helper exist in the source, it is not
registered in `ParamSetPairs` or included in the protobuf `Params`; changing a
parameter-store value named `PowerReduction` does not change this keeper's
conversion. This application default is fixed by node code, rather than
changed through staking governance. Test applications may deliberately override
the global; such overrides do not define production defaults. The
[state reference](./01_state.md#validator) describes the indexes that use the
application reduction.
