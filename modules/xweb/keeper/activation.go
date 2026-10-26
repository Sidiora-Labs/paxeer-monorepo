package keeper

import (
	"errors"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

const ProgramOriginPlan = "v7.0"

var ErrProgramModuleDisabled = errors.New("LXP_MODULE_DISABLED")

func (k Keeper) ProgramOriginActive(ctx sdk.Context) bool {
	if k.upgrades == nil {
		return false
	}
	return k.upgrades.IsUpgradeActiveAtHeight(ctx.WithGasMeter(sdk.NewInfiniteGasMeter(1, 1)), ProgramOriginPlan, ctx.BlockHeight())
}
