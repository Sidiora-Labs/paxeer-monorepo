package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

type UpgradeReader interface {
	IsUpgradeActiveAtHeight(ctx sdk.Context, name string, height int64) bool
}

func MergeRetired(ctx sdk.Context, upgrades UpgradeReader) bool {
	if upgrades == nil {
		return false
	}
	return upgrades.IsUpgradeActiveAtHeight(ctx.WithGasMeter(sdk.NewInfiniteGasMeter(1, 1)), types.MergeUpgrade, ctx.BlockHeight())
}
