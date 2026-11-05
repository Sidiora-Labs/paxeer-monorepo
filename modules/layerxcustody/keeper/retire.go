package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

// Retired observes imported retirement or the live upgrade keeper, so existing
// services and proposal handlers observe activation without reconstruction.
func (k *Keeper) Retired(ctx sdk.Context) bool {
	// Imported retirement is a lifecycle state, including on zero-height
	// exports. The original activation height remains available as provenance.
	if k.retirementHeight(ctx.WithGasMeter(sdk.NewInfiniteGasMeter(1, 1))) > 0 {
		return true
	}
	if k.upgrade == nil {
		return false
	}
	return k.upgrade.IsUpgradeActiveAtHeight(ctx.WithGasMeter(sdk.NewInfiniteGasMeter(1, 1)), types.MergePlanUpgrade, ctx.BlockHeight())
}
