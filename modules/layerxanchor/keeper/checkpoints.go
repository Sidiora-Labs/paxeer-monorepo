package keeper

import (
	"encoding/json"
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func (k Keeper) GetCheckpoint(ctx sdk.Context, batchNumber uint64) (types.Checkpoint, bool) {
	var checkpoint types.Checkpoint
	ok := k.get(ctx, types.CheckpointKey(batchNumber), &checkpoint)
	return checkpoint, ok
}

func (k Keeper) setCheckpoint(ctx sdk.Context, checkpoint types.Checkpoint) {
	k.set(ctx, types.CheckpointKey(checkpoint.BatchNumber), checkpoint)
	k.set(ctx, types.CheckpointIDKey(checkpoint.CheckpointID), checkpoint.BatchNumber)
}

func (k Keeper) deleteCheckpoint(ctx sdk.Context, checkpoint types.Checkpoint) {
	store := ctx.KVStore(k.storeKey)
	store.Delete(types.CheckpointKey(checkpoint.BatchNumber))
	store.Delete(types.CheckpointIDKey(checkpoint.CheckpointID))
}

// CheckpointByID is the recorded checkpoint carrying the identifier. A removed
// checkpoint, or a batch since recorded under another identifier, is unknown.
func (k Keeper) CheckpointByID(ctx sdk.Context, checkpointID [32]byte) (types.Checkpoint, bool) {
	var batchNumber uint64
	if !k.get(ctx, types.CheckpointIDKey(checkpointID), &batchNumber) {
		return types.Checkpoint{}, false
	}
	checkpoint, ok := k.GetCheckpoint(ctx, batchNumber)
	if !ok || checkpoint.CheckpointID != types.Hash32(checkpointID) {
		return types.Checkpoint{}, false
	}
	return checkpoint, true
}

func (k Keeper) GetCheckpoints(ctx sdk.Context) []types.Checkpoint {
	var out []types.Checkpoint
	k.iterate(ctx, types.CheckpointPrefix, func(value []byte) bool {
		var checkpoint types.Checkpoint
		if err := json.Unmarshal(value, &checkpoint); err != nil {
			panic(err)
		}
		out = append(out, checkpoint)
		return false
	})
	return out
}

// SubmitCheckpoint refuses the retired guarantor checkpoint path.
func (k Keeper) SubmitCheckpoint(ctx sdk.Context, submitter sdk.AccAddress, header []byte, headerSignature [64]byte, certificateBytes []byte) (types.Checkpoint, error) {
	return types.Checkpoint{}, types.ErrModuleRetired.Wrap("SubmitCheckpoint")
}

// finalizable is the finality rule ported from lxp_checkpoint_finalisable and
// CheckpointRegistry: the certificate declares exactly the required threshold,
// at least that many of its guarantors are still bonded and active (an
// equivocator was ejected), no challenge is open, the challenge window has
// elapsed, and the checkpoint continues the finalized chain.
func (k Keeper) finalizable(ctx sdk.Context, checkpoint types.Checkpoint, params types.Params) error {
	if checkpoint.Status != types.CheckpointSubmitted {
		return types.ErrNotFinalizable.Wrap("checkpoint is not in the submitted state")
	}
	if uint32(checkpoint.DeclaredThreshold) != params.Threshold {
		return types.ErrNotFinalizable.Wrap("certificate threshold is not the required threshold")
	}
	bonded := uint32(0)
	for _, id := range checkpoint.Guarantors {
		if guarantor, ok := k.GetGuarantor(ctx, id); ok && eligible(guarantor, params) {
			bonded++
		}
	}
	if bonded < params.Threshold {
		return types.ErrNotFinalizable.Wrap("bonded guarantor signatures are below the threshold")
	}
	if checkpoint.OpenChallenges != 0 {
		return types.ErrNotFinalizable.Wrap("a challenge is open")
	}
	if ctx.BlockTime().Unix() < checkpoint.SubmittedTime+int64(params.ChallengeWindowSeconds) { //nolint:gosec
		return types.ErrNotFinalizable.Wrap("the challenge window is open")
	}
	if latest, ok := k.LatestFinalizedBatch(ctx); ok && checkpoint.BatchNumber != latest+1 {
		return types.ErrNotFinalizable.Wrap("checkpoint does not follow the finalized batch")
	}
	return nil
}

func (k Keeper) finalize(ctx sdk.Context, checkpoint types.Checkpoint) types.Checkpoint {
	checkpoint.Status = types.CheckpointFinal
	checkpoint.FinalizedHeight = ctx.BlockHeight()
	checkpoint.FinalizedTime = ctx.BlockTime().Unix()
	k.setCheckpoint(ctx, checkpoint)
	k.set(ctx, types.LatestFinalizedKey, checkpoint.BatchNumber)
	ctx.EventManager().EmitEvent(sdk.NewEvent(types.EventCheckpointFinalized,
		sdk.NewAttribute(types.AttributeBatchNumber, fmt.Sprint(checkpoint.BatchNumber)),
		sdk.NewAttribute(types.AttributeCheckpointID, fmt.Sprintf("%x", checkpoint.CheckpointID[:])),
		sdk.NewAttribute(types.AttributeStateRoot, fmt.Sprintf("%x", checkpoint.StateRoot[:])),
		sdk.NewAttribute(types.AttributeReceiptRoot, fmt.Sprintf("%x", checkpoint.ReceiptRoot[:]))))
	return checkpoint
}

// Finalize finalizes a submitted checkpoint once the rule holds, for a
// checkpoint that waited out a challenge window or an open challenge.
func (k Keeper) Finalize(ctx sdk.Context, batchNumber uint64) (types.Checkpoint, error) {
	checkpoint, ok := k.GetCheckpoint(ctx, batchNumber)
	if !ok {
		return checkpoint, types.ErrCheckpointUnknown
	}
	if err := k.finalizable(ctx, checkpoint, k.GetParams(ctx)); err != nil {
		return checkpoint, err
	}
	return k.finalize(ctx, checkpoint), nil
}

// Availability.

func (k Keeper) GetAvailability(ctx sdk.Context) []types.AvailabilityAttestation {
	var out []types.AvailabilityAttestation
	k.iterate(ctx, types.AvailabilityPrefix, func(value []byte) bool {
		var attestation types.AvailabilityAttestation
		if err := json.Unmarshal(value, &attestation); err != nil {
			panic(err)
		}
		out = append(out, attestation)
		return false
	})
	return out
}

func (k Keeper) clearAvailability(ctx sdk.Context, batchNumber uint64) {
	for _, attestation := range k.GetAvailability(ctx) {
		if attestation.BatchNumber == batchNumber {
			ctx.KVStore(k.storeKey).Delete(types.AvailabilityKey(batchNumber, attestation.GuarantorID))
		}
	}
}

// availabilityMask is the set of the five classes that at least the threshold
// of bonded active guarantors attested to possess.
func (k Keeper) availabilityMask(ctx sdk.Context, batchNumber uint64, params types.Params) uint8 {
	var counts [codec.AvailabilityClassCount]uint32
	k.iterate(ctx, types.AvailabilityBatchPrefix(batchNumber), func(value []byte) bool {
		var attestation types.AvailabilityAttestation
		if err := json.Unmarshal(value, &attestation); err != nil {
			panic(err)
		}
		if guarantor, ok := k.GetGuarantor(ctx, attestation.GuarantorID); ok && eligible(guarantor, params) {
			for class := 0; class < codec.AvailabilityClassCount; class++ {
				if attestation.ClassMask&(1<<class) != 0 {
					counts[class]++
				}
			}
		}
		return false
	})
	mask := uint8(0)
	for class, count := range counts {
		if count >= params.Threshold {
			mask |= 1 << class
		}
	}
	return mask
}

// SubmitAvailabilityAttestation refuses the retired guarantor availability path.
func (k Keeper) SubmitAvailabilityAttestation(ctx sdk.Context, encoded []byte) (types.Checkpoint, types.AvailabilityAttestation, error) {
	return types.Checkpoint{}, types.AvailabilityAttestation{}, types.ErrModuleRetired.Wrap("SubmitAvailabilityAttestation")
}
