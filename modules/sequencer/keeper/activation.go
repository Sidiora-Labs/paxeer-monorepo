package keeper

import (
	"encoding/binary"
	"math"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func (k Keeper) ActivationHeight(ctx sdk.Context) (int64, error) {
	height, err := k.readU64(ctx, types.ActivationHeightKey)
	if err != nil {
		return 0, err
	}
	if height > math.MaxInt64 {
		return 0, ErrCorruptStore.Wrapf("activation height %d exceeds int64", height)
	}
	return int64(height), nil
}

func (k Keeper) SetActivationHeight(ctx sdk.Context, height int64) error {
	if height < 1 {
		return types.ErrInvalidGenesis.Wrapf("activation height %d must be positive", height)
	}
	stored, err := k.ActivationHeight(ctx)
	if err != nil {
		return err
	}
	if stored != 0 && stored != height {
		return types.ErrInvalidGenesis.Wrapf("activation height is already %d, cannot set %d", stored, height)
	}
	k.store(ctx).Set(types.ActivationHeightKey, binary.BigEndian.AppendUint64(nil, uint64(height)))
	return nil
}

func (k Keeper) Sequenced(ctx sdk.Context, height int64) (bool, error) {
	activation, err := k.ActivationHeight(ctx)
	if err != nil {
		return false, err
	}
	return activation > 0 && height >= activation, nil
}
