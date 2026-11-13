# x/slashing

## Overview

The slashing module tracks validator liveness and applies staking penalties on
Paxeer's BFT network. Paxeer has one network: its kernel participates in the
network and proposes blocks, and committed blocks determine finality.

This fork stores missed signatures in packed arrays and calculates liveness
updates concurrently before applying store mutations in vote order. Start with
the [slashing specification](spec/README.md), the
[current state layout](spec/02_state.md) and the
[BeginBlock lifecycle](spec/04_begin_block.md).

Penalties can reduce stake according to the configured slash fraction and jail a
validator, excluding it from the active validator set. Downtime does not
set the tombstone flag. Unjailing requires a transaction and the checks in
[`keeper/unjail.go`](keeper/unjail.go), including an expired jail period,
sufficient self-delegation and no tombstone.

## State

### ValidatorSigningInfo

`ValidatorSigningInfo` stores the consensus address, `StartHeight`, circular
`IndexOffset`, `JailedUntil`, `Tombstoned` and `MissedBlocksCounter`. The counter
tracks the number of missed slots in the active window without scanning every
bit on each update.

[`AfterValidatorCreated`](keeper/hooks.go) adds the consensus public-key relation.
`AfterValidatorBonded` creates signing info only when none exists; it does not
reset an existing record's start height on every rebond. Signature handling
requires both records and panics if either prerequisite is missing.

### ValidatorMissedBlockArray

The keeper stores one protobuf `ValidatorMissedBlockArray` per consensus address,
containing `Address`, `WindowSize` and `MissedBlocks []uint64`. It allocates the
whole bitmap on the first signature calculation when no bitmap exists, then
persists it during BeginBlock's write phase. There are no new per-height keys as
the window fills.

For a positive window `W`, the number of words is `(W + 63) / 64` using integer
division. A 65-block window needs two words. Slot `i` uses word `i / 64` and mask
`uint64(1) << (i % 64)`. A set bit means a missed signature; a cleared bit means
no miss. Bits beyond `WindowSize` in the last word are unused and remain zero
when the handler allocates or resizes the array. The counter counts only active
slots; padding never contributes to it.

`IndexOffset` identifies the next slot to overwrite and advances modulo the
current window after each vote. `MissedBlocksCounter` increases for a clear-to-set
transition and decreases for a set-to-clear transition. Fresh slots are clear,
so the initial bitmap is not a history of signed blocks.

`WindowSize` detects parameter changes even when the number of words stays the
same. On expansion, `ResizeMissedBlockArray` inserts clear slots at the next write
position, retains all existing slots and preserves the missed counter. On
shrinkage, it clears the bitmap and resets the counter and offset. The
[state specification](spec/02_state.md) describes persistence and migration.

### Params

These are the constructor defaults from [`types/params.go`](types/params.go).
Live network parameters may differ.

| Parameter-store key | Go type | Constructor default |
| --- | --- | --- |
| `SignedBlocksWindow` | `int64` | `108000` blocks |
| `MinSignedPerWindow` | `sdk.Dec` | `0.050000000000000000` |
| `DowntimeJailDuration` | `time.Duration` | 10 minutes |
| `SlashFractionDoubleSign` | `sdk.Dec` | `0.000000000000000000` |
| `SlashFractionDowntime` | `sdk.Dec` | `0.000000000000000000` |

The window and jail duration must be positive. The decimal parameters must lie
in `[0, 1]`. A zero slash fraction still permits jailing for downtime.
The keeper converts the minimum signing fraction to a block count with
`MinSignedPerWindow * SignedBlocksWindow`, rounded using `RoundInt64`.

Parameters can be changed through a `param-change` governance proposal in the
`slashing` subspace. In that proposal's legacy JSON, integers and durations use
strings; ten minutes is `"600000000000"` nanoseconds. See the
[governance client](../gov/spec/07_client.md#submit-proposal) and
[parameter subspace](../params/spec/02_subspace.md).

## Begin-Block

[`BeginBlocker`](abci.go) takes the previous commit's votes. One worker per vote
calls `HandleValidatorSignatureConcurrent` to read state and calculate the next
bitmap, signing info and possible penalty. Workers fill a result slice at the
original vote indices. After all workers finish, the main loop writes those
results in the original order.

A validator is eligible for downtime punishment only when the current height
strictly exceeds `StartHeight + SignedBlocksWindow`, its missed count strictly
exceeds the permitted misses, and the staking validator exists and is not
already jailed. The write phase clears the bitmap, calls
`SlashJailAndUpdateSigningInfo`, and stores signing info with the counter and
offset reset. Otherwise it persists the updated bitmap and signing info.

The calculation phase returns liveness events for missed signatures with each
vote's result. The ordered write loop emits those events before applying that
vote's penalty, so both liveness and slash events follow the original vote order.
See [BeginBlock](spec/04_begin_block.md) for the exact threshold, resizing and
penalty sequence.

## Metrics

- `validator_slashed` is a counter with `type` and `validator` attributes. The
  type is `missing_signature` for downtime or `double_sign` for the keeper's
  double-sign slash path; the validator is its consensus address. It counts
  penalty calls even when the configured slash fraction is zero.
- `slashing_begin_blocker_duration` records BeginBlock latency in seconds.

Downtime slash events include `reason=missing_signature`; the keeper's
double-sign slash event uses `reason=double_sign`.
