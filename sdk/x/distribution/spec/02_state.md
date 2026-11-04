<!--
order: 2
-->

# State

The distribution keeper stores reward accounting in the `distribution` KV
store. The layouts below follow [types/keys.go](../types/keys.go) and
[keeper/store.go](../keeper/store.go). `|` denotes byte concatenation.
`V` is a validator operator address and `D` a delegator account address, both
raw bytes. `L(A)` is a one-byte address length followed by address `A`, as
produced by `address.MustLengthPrefix`; keys do not contain Bech32 strings.

| Key | Stored value |
| --- | --- |
| `0x00` | Protobuf `FeePool` |
| `0x01` | Protobuf `gogotypes.BytesValue` containing the previous proposer's consensus address |
| `0x02 \| L(V)` | Protobuf `ValidatorOutstandingRewards` |
| `0x03 \| L(D)` | Raw withdrawal account address bytes |
| `0x04 \| L(V) \| L(D)` | Protobuf `DelegatorStartingInfo` |
| `0x05 \| L(V) \| periodLE` | Protobuf `ValidatorHistoricalRewards` |
| `0x06 \| L(V)` | Protobuf `ValidatorCurrentRewards` |
| `0x07 \| L(V)` | Protobuf `ValidatorAccumulatedCommission` |
| `0x08 \| L(V) \| heightBE \| periodBE` | Protobuf `ValidatorSlashEvent` |

`periodLE` is an eight-byte little-endian `uint64`. `heightBE` and `periodBE`
are eight-byte big-endian `uint64` values. Slash-event keys include both height
and period; their ordering supports height-range iteration and multiple events
at one height. Distribution parameters are managed separately through the
parameter subspace; they are not fields of `FeePool`.

## FeePool

`FeePool.CommunityPool` holds community funds as `sdk.DecCoins`. Decimal rewards
support fractional accounting even though the bank module account holds integer
`sdk.Coins`. `FeePool` is an accounting claim against those funds, not a separate
bank balance. See [Begin Block](03_begin_block.md) for how fees are moved into
the distribution module and allocated.

A missing fee-pool value makes `GetFeePool` panic. The previous proposer is
also initialized in genesis; `BeginBlocker` updates its consensus address for
the next allocation.

## Validator Distribution

A validator has several records, rather than a combined `ValidatorDistInfo`:

- `ValidatorOutstandingRewards.Rewards` tracks all unwithdrawn rewards,
  including accumulated commission and delegator entitlements.
- `ValidatorAccumulatedCommission.Commission` tracks commission available to
  the operator.
- `ValidatorCurrentRewards` tracks the current `Rewards` and `Period`.
- `ValidatorHistoricalRewards` stores `CumulativeRewardRatio` and
  `ReferenceCount` for a completed period.
- `ValidatorSlashEvent` stores `ValidatorPeriod` and the slash `Fraction`.

[keeper/validator.go](../keeper/validator.go) initializes historical period `0`
with reference count `1`, current period `1` with empty rewards, and empty
commission and outstanding rewards. `IncrementValidatorPeriod` converts the
current rewards into a cumulative reward-per-token ratio, writes the ended
period's history, clears current rewards and advances the period. If the
validator has no tokens, its current rewards go to the community pool instead.
Historical records are removed when their reference count reaches zero.

Reward allocation adds commission to its own record, the shared portion to
current rewards, and the entire allocation to outstanding rewards. These are
overlapping accounting views, not balances to sum together. Validator removal
settles commission, moves remaining dust to the community pool and removes
that validator's reward and slash records through [keeper/hooks.go](../keeper/hooks.go).

## Delegation Distribution

`DelegatorStartingInfo` is keyed by validator first, then delegator, under
`0x04`. It records `PreviousPeriod`, decimal `Stake`, and creation `Height`;
it is not simply a last-withdrawal height.

[keeper/delegation.go](../keeper/delegation.go) calculates rewards from the
change in historical cumulative ratios multiplied by the delegation's stake,
accounting for intervening slash events. Starting information and slash records
retain references to the historical periods they need. Staking hooks withdraw
rewards before shares change and initialize starting information after a
delegation is modified.

A withdrawal pays the integer coin portion from the distribution module account,
returns the delegation reward's fractional remainder to the community pool,
updates outstanding rewards and releases the old starting-period reference.
Operator commission withdrawals retain their fractional remainder in the
commission record. A stored withdrawal address is raw bytes under `0x03`;
lookup defaults to the delegator if no address is set or the stored recipient
cannot receive the withdrawal.

## Genesis import and export

[keeper/genesis.go](../keeper/genesis.go) imports and exports the fee pool,
parameters, withdrawal addresses, previous proposer, outstanding rewards,
commissions, historical and current rewards, delegation starting information,
and slash events. Genesis address strings are decoded before constructing
store keys. Export retains the periods, reference counts and slash heights
needed to resume reward calculations.

Initialization checks that the distribution module account exists and that
its bank balance equals the truncated sum of outstanding rewards and the
community pool. Commission, current rewards and historical ratios are not
added again when computing those holdings.
