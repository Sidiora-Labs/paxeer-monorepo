<!--
order: 4
-->

# BeginBlock

## Liveness Tracking

[`BeginBlocker`](../abci.go) receives the previous commit's `[]abci.VoteInfo`.
For each vote it calls
[`HandleValidatorSignatureConcurrent`](../keeper/infractions.go) with the
validator's consensus address, voting power and `SignedLastBlock` flag. Liveness
uses the [packed per-validator state](02_state.md), rather than separate
per-slot store entries.

The public-key relation and `ValidatorSigningInfo` must already exist. A missing
record causes a panic. The staking hooks establish these records; the signature
handler only initializes a missing bitmap, not missing signing info.

### Calculation phase

The implementation starts one goroutine per vote. Each worker reads the current
records and calculates its `SlashingWriteInfo`; it does not write keeper state.
Results are placed at the input vote's index in a slice. For a positive
`SignedBlocksWindow`, each worker performs these steps:

1. Read signing info and use `IndexOffset` as the next bitmap slot.
2. Load `ValidatorMissedBlockArray`. If absent, allocate `(window + 63) / 64`
   zero-filled words and record the consensus address and `WindowSize`.
3. If `WindowSize` differs from the configured window, call
   `ResizeMissedBlockArray`. Expansion inserts clear slots at the next write
   index and preserves every old slot and the missed counter. Shrinkage creates
   a clear bitmap, resets the counter and offset to zero, and uses slot zero.
4. Read the previous bit with `GetBooleanFromBitGroups` and set the current slot
   to `!SignedLastBlock` with `SetBooleanInBitGroups`. Increment the counter only
   for a clear-to-set transition; decrement it only for a set-to-clear transition.
5. Advance `IndexOffset = (IndexOffset + 1) % window` after any resizing.
6. If the current signature is missing, emit a `liveness` event with `address`,
   `missed_blocks` and `height` attributes.
7. Calculate the downtime threshold and, if exceeded, check the staking
   validator before returning a pending penalty.

`WindowSize` is compared directly, so changes within the same number of packed
words still trigger resizing. The preserved counter on expansion and reset
counter on shrinkage keep it consistent with the active bits.

### Threshold and grace period

The parameter `MinSignedPerWindow` is a decimal fraction. The keeper method with
the same name returns a rounded block count:

```text
minSigned = RoundInt64(MinSignedPerWindow * SignedBlocksWindow)
maxMissed = SignedBlocksWindow - minSigned
minHeight = signingInfo.StartHeight + SignedBlocksWindow
```

A downtime penalty requires both `height > minHeight` and
`MissedBlocksCounter > maxMissed`. It also requires a staking validator that
exists and is not already jailed. Otherwise the updated records are returned
without a penalty. The counter measures misses across the circular window;
the misses need not be consecutive.

### Ordered mutation phase

After all workers finish, `BeginBlocker` applies their results in the original
vote order. A missing result is an invariant failure and causes a panic.

For a result marked `ShouldSlash`, it first deletes the validator's bitmap with
`ClearValidatorMissedBlockBitArray`, then calls
`SlashJailAndUpdateSigningInfo`. That method:

1. Emits a `slash` event with `address`, `power`, `reason=missing_signature` and
   `jailed` attributes, and increments `validator_slashed`.
2. Calls staking `Slash` with the vote's power, `SlashFractionDowntime` and
   `distributionHeight = height - sdk.ValidatorUpdateDelay - 1`. The extra block
   accounts for the previous commit. This historical height can be negative
   near genesis and is used to filter unbonding delegations and redelegations.
3. Calls staking `Jail` and sets `JailedUntil` to the current block time plus
   `DowntimeJailDuration`.
4. Resets `MissedBlocksCounter` and `IndexOffset` to zero. It leaves `StartHeight`
   and `Tombstoned` unchanged.

For other results, `BeginBlocker` persists the updated bitmap with
`SetValidatorMissedBlocks`. Both branches finish by storing the returned
`ValidatorSigningInfo` with `SetValidatorSigningInfo`. A penalty leaves the
bitmap absent until another signature calculation allocates it again.

Store mutations and downtime slash events occur in vote order. Liveness events
are emitted inside the concurrent calculation phase, so their order is not
guaranteed by the ordered write loop.

Downtime does not tombstone the validator. Even with a zero downtime slash
fraction, the jail and state reset still occur. Expiration of `JailedUntil`
alone does not unjail a validator: the operator must submit an unjail transaction
and satisfy [`Keeper.Unjail`](../keeper/unjail.go), including its self-delegation
and tombstone checks. See the [module overview](../README.md) for parameters and
metrics.
