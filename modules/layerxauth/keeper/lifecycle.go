package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func (k *Keeper) RecordLifecycle(ctx sdk.Context, actor Actor, payload []byte, idempotencyKey [32]byte) (Result, error) {
	if len(payload) < 4 || payload[0] != 0x71 || (payload[1] != 1 && payload[1] != 3) {
		return Result{}, types.ErrNonCanonical
	}
	return k.execute(ctx, actor, payload, idempotencyKey)
}
