package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

// SetLeaders replaces the pinned leader list, preserving primary-first order.
func (k Keeper) SetLeaders(ctx sdk.Context, leaders [][32]byte) error {
	if err := types.ValidateLeaders(leaders); err != nil {
		return err
	}
	packed := make([]byte, 0, len(leaders)*32)
	for _, leader := range leaders {
		packed = append(packed, leader[:]...)
	}
	k.store(ctx).Set(types.LeaderListKey, packed)
	return nil
}

// LeaderSchedule returns the activation height and pinned leaders, or no
// schedule while activation is unset.
func (k Keeper) LeaderSchedule(ctx sdk.Context) (int64, [][32]byte, error) {
	height, err := k.ActivationHeight(ctx)
	if err != nil {
		return 0, nil, err
	}
	if height == 0 {
		return 0, nil, nil
	}
	leaders, err := k.Leaders(ctx)
	if err != nil {
		return 0, nil, err
	}
	return height, leaders, nil
}
