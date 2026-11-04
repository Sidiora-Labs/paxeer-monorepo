<!--
order: 2
-->

# State

## Signing Info (Liveness)

The slashing module consumes votes for the previous block's commit through
`VoteInfo.SignedLastBlock`. It tracks missed signatures over a sliding window
and can slash and jail validators that fail its liveness threshold. Fee rewards
are handled separately by [distribution](../../distribution/spec/03_begin_block.md).

The current key constructors are in [`types/keys.go`](../types/keys.go);
[`keeper/signing_info.go`](../keeper/signing_info.go) marshals the records.
All addresses below are consensus-address bytes, prefixed by their one-byte
length, rather than validator operator addresses.

| Key in the `slashing` store | Persisted value |
| --- | --- |
| `0x01 \| ConsAddrLen \| ConsAddress` | Protobuf `ValidatorSigningInfo` |
| `0x02 \| ConsAddrLen \| ConsAddress` | Protobuf `ValidatorMissedBlockArray` |
| `0x03 \| ConsAddrLen \| ConsAddress` | Interface-encoded consensus public key |

The public-key relation is managed by [`keeper/keeper.go`](../keeper/keeper.go).
The message definitions are in the local
[slashing protobuf schema](../../../proto/cosmos/slashing/v1beta1/slashing.proto).

`ValidatorSigningInfo` contains:

| Field | Purpose |
| --- | --- |
| `Address` | Bech32 consensus address |
| `StartHeight` | Height used to calculate the initial liveness grace period |
| `IndexOffset` | Next slot to overwrite in the circular bitmap |
| `JailedUntil` | Earliest block time at which downtime unjailing is allowed |
| `Tombstoned` | Permanent exclusion checked by the unjail path |
| `MissedBlocksCounter` | Number of missed slots in the active window |

[`AfterValidatorBonded`](../keeper/hooks.go) creates a missing signing record with
the current height, zero offset and counter, Unix epoch jail time and no
tombstone. It leaves an existing record unchanged. A missing signing record or
public-key relation during signature processing is an invariant failure and
causes a panic; the keeper does not substitute zero-valued signing info.

## Packed missed-block window

`ValidatorMissedBlockArray` contains the consensus `Address`, the persisted
`WindowSize` and `MissedBlocks`, an array of packed `uint64` words. Each validator
has a single bitmap key, with no slot index appended to it.

For window size `W`, the array contains `(W + 63) / 64` words. Slot `i` is bit
`i % 64` of word `i / 64`, counted from the least significant bit. A bit of `1`
means missed; `0` means no miss. Unused bits beyond the last active slot are
excluded from the window. For example, a 65-slot window needs two words, using
only bit zero of the second word.

When the bitmap is absent, the signature handler allocates the complete
zero-filled array. BeginBlock persists the returned array unless the validator
is being punished for downtime. Normal updates maintain the invariant that
`MissedBlocksCounter` equals the number of set bits among the `WindowSize` active
slots. The next index remains in `[0, W)` by advancing modulo `W`.

A changed `SignedBlocksWindow` is detected by comparing it to the stored
`WindowSize`, even if both sizes require the same number of words. Expansion
inserts clear slots at the next write index while preserving all old slots and
the missed counter. Shrinkage clears the window, counter and offset. The
[BeginBlock algorithm](04_begin_block.md) then processes the current vote using
the resized window.

`SetValidatorMissedBlocks` writes the complete protobuf record.
`ClearValidatorMissedBlockBitArray` deletes that one key when downtime punishment
resets the counter and offset. A later vote can allocate a fresh window. These
records reside in the persistent KV store, so committed liveness state survives
restart.

## Legacy migration input

Older per-slot keys appended a little-endian `uint64` index to the `0x02` address
key and stored a protobuf boolean wrapper. They are inputs to `Migrate2to3`,
which collects missed heights into `ValidatorMissedBlockArrayLegacyMissedHeights`
and deletes the old per-slot keys. `Migrate3to4` converts those height lists to
the packed `ValidatorMissedBlockArray` and restores a circular offset in signing
info. These legacy encodings are not the layout read by today's signature
handler. See [`keeper/migrations.go`](../keeper/migrations.go).
