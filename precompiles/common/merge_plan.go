package common

import (
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	upgradekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/keeper"
)

func PlanActive(ctx sdk.Context, keeper any, plan string) bool {
	getter, ok := keeper.(interface {
		UpgradeKeeper() *upgradekeeper.Keeper
	})
	if !ok {
		return false
	}
	upgrades := getter.UpgradeKeeper()
	if upgrades == nil {
		return false
	}
	return upgrades.IsUpgradeActiveAtHeight(ctx.WithGasMeter(sdk.NewInfiniteGasMeter(1, 1)), plan, ctx.BlockHeight())
}

func MergePlanActive(ctx sdk.Context, keeper any) bool {
	return PlanActive(ctx, keeper, MergeUpgrade)
}
