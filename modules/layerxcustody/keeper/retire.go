package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func (k *Keeper) Retired(ctx sdk.Context) bool {
	if k.upgrade == nil {
		return false
	}
	return k.upgrade.IsUpgradeActiveAtHeight(ctx.WithGasMeter(sdk.NewInfiniteGasMeter(1, 1)), types.MergePlanUpgrade, ctx.BlockHeight())
}
