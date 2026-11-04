<!--
order: 5
-->

# Hooks

The distribution keeper implements the staking hooks in
[keeper/hooks.go](../keeper/hooks.go). They maintain historical reward references
when staking changes a validator or delegation. Reward calculations and starting
information are described under [Messages](04_messages.md).

## Create or modify delegation distribution

Staking delegation, redelegation and undelegation operations call the relevant
hooks around changes to shares.

### Before

- `BeforeDelegationCreated` ends the validator's current period with
  `IncrementValidatorPeriod`. A new delegation has no previous rewards to withdraw.
- `BeforeDelegationSharesModified` withdraws the existing delegation's rewards.
  That withdrawal ends the current period, pays integer rewards to the resolved
  withdrawal address, sends decimal dust to the community pool, decrements the
  reference count for the old starting period, and deletes the old starting info.
  A withdrawal error in this hook causes a panic.

The ended period includes rewards accrued since the delegation's baseline
period. Ending it before changing shares preserves the reward ratio for the old
stake distribution.

### After

`AfterDelegationModified` calls `initializeDelegation` after the staking change.
It increments the reference count for the historical period just ended and stores
three distinct values:

- `PreviousPeriod`: the validator's current period minus one, identifying the
  reward-ratio baseline.
- `Stake`: the token value of the updated delegation shares, calculated with
  `TokensFromSharesTruncated`.
- `Height`: the current block height, `uint64(ctx.BlockHeight())`.

The block height determines which slash events are considered and prevents a
delegation initialized in this block from earning rewards in the same block. It
is not a validator period. See
[Initialize delegation](04_messages.md#initialize-delegation).

## Validator created

`AfterValidatorCreated`, called during validator creation, initializes:

| Record | Initial value |
| --- | --- |
| Historical rewards at period `0` | Empty cumulative reward ratio, reference count `1` |
| Current rewards | Empty rewards, period `1` |
| Accumulated commission | Empty decimal coins |
| Outstanding rewards | Empty decimal coins |

The implementation is [initializeValidator](../keeper/validator.go).

## Validator removed

`AfterValidatorRemoved` cleans up a validator after its delegations are gone.
It subtracts accumulated commission from outstanding rewards, transfers the
integer commission to the withdrawal address resolved for the validator's
account, and adds fractional commission to the community pool. It also adds the
remaining outstanding rewards, normally dust, to that pool. A failed commission
transfer causes a panic.

The hook deletes outstanding rewards, commission, slash events, historical
rewards and current rewards for the validator.

## Validator is slashed

`BeforeValidatorSlashed` records the slash through
[updateValidatorSlashFraction](../keeper/validator.go):

1. Validate that the slash fraction lies in the inclusive range `[0, 1]`.
2. End the current validator period and advance to the next period.
3. Increment the historical reference count for the period just ended.
4. Store `ValidatorSlashEvent` with that ended `ValidatorPeriod` and the slash
   `Fraction`, keyed by validator address, current block height and period.

[Reward withdrawal](04_messages.md#rewards-across-slash-periods) later uses these
records to accumulate rewards before each slash and reduce surviving stake by
`1 - Fraction`.

`BeforeValidatorModified`, `AfterValidatorBonded`, `AfterValidatorBeginUnbonding`
and `BeforeDelegationRemoved` perform no distribution work in this implementation.
