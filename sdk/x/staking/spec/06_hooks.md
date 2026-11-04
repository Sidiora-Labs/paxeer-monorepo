<!--
order: 5
-->

# Hooks

Modules implement the [`StakingHooks` interface](../types/expected_keepers.go)
and register it through `Keeper.SetHooks`. The
[keeper wrappers](../keeper/hooks.go) call the registered implementation when
present. [`MultiStakingHooks`](../types/hooks.go) dispatches each callback
synchronously in registration order. In the [node application](../../../../node/app.go),
distribution hooks run before slashing hooks.

The following signatures use the SDK types from `sdk/types` and have no return
value:

| Callback | State and timing visible to the callback |
| --- | --- |
| `AfterValidatorCreated(ctx sdk.Context, valAddr sdk.ValAddress)` | Validator and indexes exist; the create-message self-delegation has not yet run. |
| `BeforeValidatorModified(ctx sdk.Context, valAddr sdk.ValAddress)` | Called before saving an edited commission and at the start of staking slashing. This is not a notification for every validator-store write. |
| `AfterValidatorRemoved(ctx sdk.Context, consAddr sdk.ConsAddress, valAddr sdk.ValAddress)` | Validator primary, consensus-address and power-index records have been deleted. |
| `AfterValidatorBonded(ctx sdk.Context, consAddr sdk.ConsAddress, valAddr sdk.ValAddress)` | Bonded status and power index have been saved and the validator's queue address removed. EndBlock pool netting and last-power updates follow. |
| `AfterValidatorBeginUnbonding(ctx sdk.Context, consAddr sdk.ConsAddress, valAddr sdk.ValAddress)` | Unbonding status, deadline, height and validator queue entry have been saved. EndBlock pool netting follows. |
| `BeforeDelegationCreated(ctx sdk.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress)` | A new delegation is about to be funded and stored; no delegation record exists yet. |
| `BeforeDelegationSharesModified(ctx sdk.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress)` | Existing delegation and validator shares are still available before a delegation increase or `Unbond` share removal. |
| `BeforeDelegationRemoved(ctx sdk.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress)` | Called by `RemoveDelegation` before deleting its stored record. |
| `AfterDelegationModified(ctx sdk.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress)` | New or reduced delegation shares have been stored. See the operation-specific ordering below. |
| `BeforeValidatorSlashed(ctx sdk.Context, valAddr sdk.ValAddress, fraction sdk.Dec)` | Eligible unbonding/redelegation slashes have been processed; the source validator's remaining tokens have not yet been reduced. `fraction` is the effective remaining validator slash fraction. |

## Delegation ordering

On `Delegate`, the before hook runs first, followed by funding/pool movement,
validator token/share updates, delegation storage and `AfterDelegationModified`.
For an existing delegation, distribution's before hook withdraws accrued
rewards using the old shares; the after hook initializes the new delegation
reward period.

On `Unbond`, `BeforeDelegationSharesModified` runs before reducing shares. A
nonempty delegation is saved and `AfterDelegationModified` runs **before** the
validator's tokens and aggregate shares are reduced. A zero-share delegation
instead invokes `BeforeDelegationRemoved` and is deleted, with no
`AfterDelegationModified` call for that removal. Consumers must use the state
available at their particular callback rather than assume every after hook sees
the final validator aggregate.

## Slashing ordering

`Slash` calls `BeforeValidatorModified` before scanning eligible unbonding and
redelegation entries. If the source validator has a positive token balance,
`BeforeValidatorSlashed` follows that scan and precedes the source token
reduction and burn. Its effective fraction is the bounded remaining burn divided
by current source tokens, rounded up and capped at one; it can differ from the
original infraction factor. The distribution hook records that effective slash
for reward accounting. Slashing's implementation of this callback is empty.

See the [distribution hooks](../../distribution/keeper/hooks.go),
[slashing hooks](../../slashing/keeper/hooks.go),
[delegation implementation](../keeper/delegation.go) and
[staking slash implementation](../keeper/slash.go). The related operations are
described in [state transitions](./02_state_transitions.md).
