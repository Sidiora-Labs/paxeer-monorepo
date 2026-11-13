<!-- Source: https://docs.paxeer.app/modules/epoch/ -->

# Epoch Module

Time-based hooks and epoch lifecycle management for coordinating periodic chain operations.

**Source:** `Paxeer-X-Network/modules/epoch/`

## Overview

The epoch module manages fixed time periods (epochs) and triggers registered hooks when an epoch ends and the next begins. An epoch defaults to **60 seconds** (`time.Minute` in genesis). Hooks run during `BeginBlock`.

In this codebase only one hook consumer is wired in `node/app.go`: the mint keeper. Validator-set updates, parameter activation, and periodic cleanup are not epoch-hook consumers here.

## State

The epoch module maintains a single state object:

```
> paxd q epoch epoch --output json
{
  "epoch": {
    "genesis_time": "2023-04-27T19:08:11.958027Z",
    "epoch_duration": "60s",
    "current_epoch": "0",
    "current_epoch_start_time": "2023-04-27T19:08:11.958027Z",
    "current_epoch_height": "0"
  }
}
```

- **genesis_time:** Chain genesis time (fixed)

- **epoch_duration:** Duration of each epoch (default 60s)

- **current_epoch:** Current epoch number (increments at each epoch boundary)

- **current_epoch_start_time:** Timestamp when the current epoch started

- **current_epoch_height:** Block height at which the current epoch started

## Epoch Boundaries

Epochs are defined by wall-clock time, not block height. In `BeginBlock` (`modules/epoch/keeper/abci.go`) a new epoch begins only when the block time is *strictly after* the boundary:

```
if ctx.BlockTime().Sub(lastEpoch.CurrentEpochStartTime) > lastEpoch.EpochDuration {
    // advance exactly one epoch
}
```

- The comparison is strict (`>`), not `≥`.

- It is a single `if`, not a loop: at most one epoch advance per block, even after a long halt.

- The new start time is `oldStart + epoch_duration`, not the current block time, so the schedule does not drift to block time.

## Hooks

Other modules implement the `EpochHooks` interface (`modules/epoch/types/hooks.go`):

```
type EpochHooks interface {
    AfterEpochEnd(ctx sdk.Context, epoch Epoch)
    BeforeEpochStart(ctx sdk.Context, epoch Epoch)
}
```

At a boundary the keeper runs them in this order:

1. `AfterEpochEnd(ctx, lastEpoch)` — for the epoch that is ending
2. `SetEpoch(newEpoch)` — store the incremented epoch
3. `BeforeEpochStart(ctx, newEpoch)` — for the epoch that is starting
4. Emit the `new_epoch` event

Each hook runs in a cached context (`ctx.CacheContext()`). Its writes are committed only if the hook completes; a hook panic is re-raised and aborts the block rather than advancing the epoch without the hook's state transition.

### Example: Mint Module

The mint module (`modules/mint/keeper/hooks.go`) implements epoch hooks: `BeforeEpochStart` is a no-op and `AfterEpochEnd` performs the daily release check. It is the only hook registered in `node/app.go`:

```
app.EpochKeeper = *epochmodulekeeper.NewKeeper(
  appCodec,
  keys[epochmoduletypes.StoreKey],
  keys[epochmoduletypes.MemStoreKey],
  app.GetSubspace(epochmoduletypes.ModuleName),
).SetHooks(epochmoduletypes.NewMultiEpochHooks(
  app.MintKeeper.Hooks()))
```

## Events

The epoch module emits a `new_epoch` event at each epoch boundary:

- **epoch_number:** The new epoch's epoch number

- **epoch_time:** The new epoch's start time

- **epoch_height:** The block height at which the new epoch started

## Messages

The epoch module does not expose any transactions. Its handler rejects every message as unrecognized. Only the module itself updates epoch state during `BeginBlock`.

## Parameters

The epoch module has no runtime parameters: `Params` is empty and validation rejects any non-empty value. The epoch duration lives in the epoch genesis state.

## Use Cases

- **Mint module:** The only wired consumer; its `AfterEpochEnd` hook checks whether a daily release is due. See [Mint module](https://docs.paxeer.app/modules/mint)

## Next Steps

- [Understand how the mint module uses epochs](https://docs.paxeer.app/modules/mint)

- [Review all Paxeer modules](https://docs.paxeer.app/modules)
