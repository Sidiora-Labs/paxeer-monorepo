package keeper

import (
	"encoding/binary"
	"math"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

var (
	feeAccountingPoolCountKey  = []byte{0x4c}
	feeAccountingEpochCountKey = []byte{0x4d}
	feeAccountingClaimCountKey = []byte{0x4e}
)

// Counts are derived indexes, maintained with their collection writes and
// reconstructed by the same writers during genesis import and migration.
func (k *Keeper) feeAccountingCount(ctx sdk.Context, key []byte) (uint64, error) {
	bz := k.store(ctx).Get(key)
	if bz == nil {
		return 0, nil
	}
	if len(bz) != 8 {
		return 0, types.ErrOverflow.Wrapf("corrupt fee accounting count %x", key)
	}
	return binary.BigEndian.Uint64(bz), nil
}

func (k *Keeper) incrementFeeAccountingCount(ctx sdk.Context, key []byte) error {
	count, err := k.feeAccountingCount(ctx, key)
	if err != nil {
		return err
	}
	if count == math.MaxUint64 {
		return types.ErrOverflow
	}
	k.setUint64(ctx, key, count+1)
	return nil
}

func (k *Keeper) setFeeAccountingEntry(ctx sdk.Context, key, value, countKey []byte) error {
	if !k.store(ctx).Has(key) {
		if err := k.incrementFeeAccountingCount(ctx, countKey); err != nil {
			return err
		}
	}
	k.store(ctx).Set(key, value)
	return nil
}
