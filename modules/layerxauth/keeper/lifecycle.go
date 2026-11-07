package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func (k *Keeper) RecordLifecycle(ctx sdk.Context, actor Actor, payload []byte, idempotencyKey [32]byte) (Result, error) {
	if len(payload) < 4 || payload[0] != 0x71 {
		return Result{}, types.ErrNonCanonical
	}
	switch payload[1] {
	case 1, 3:
	case 2:
		if len(payload) != 92 || payload[2] != 0 || payload[3] != 4 {
			return Result{}, types.ErrNonCanonical
		}
	default:
		return Result{}, types.ErrNonCanonical
	}
	return k.execute(ctx, actor, payload, idempotencyKey)
}
