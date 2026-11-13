<!--
order: 5
-->

# Hooks

This section contains a description of the module's `hooks`. Hooks are operations that are executed automatically when events are raised.

## Staking hooks

The slashing module implements the `StakingHooks` defined in `x/staking` and are used as record-keeping of validators information. During the app initialization, these hooks should be registered in the staking module struct.

The following hooks impact the slashing state:

+ `AfterValidatorBonded` creates a `ValidatorSigningInfo` instance as described in the following section.
+ `AfterValidatorCreated` stores a validator's consensus key.
+ `AfterValidatorRemoved` removes a validator's consensus key.

## Validator Bonded

Upon successful first-time bonding of a new validator, we create a new `ValidatorSigningInfo` structure for the
now-bonded validator, with `StartHeight` set to the current block height.

```text
AfterValidatorBonded(ctx sdk.Context, address sdk.ConsAddress, valAddr sdk.ValAddress)

  signingInfo, found = GetValidatorSigningInfo(address)
  if !found {
    signingInfo = ValidatorSigningInfo {
      StartHeight          : CurrentHeight,
      IndexOffset          : 0,
      JailedUntil          : time.Unix(0, 0),
      Tombstoned           : false,
      MissedBlocksCounter  : 0
    }
    SetValidatorSigningInfo(ctx, address, signingInfo)
  }

  return
```
