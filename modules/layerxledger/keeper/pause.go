package keeper

import (
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

var ErrPausedScope = govtypes.ErrPausedScope

func CheckPauseScope(module string, paused bool, method string) error {
	return govtypes.CheckPauseScope(module, paused, method)
}

func (k *Keeper) checkPause(ctx sdk.Context, method string) error {
	params := k.GetParams(ctx)
	return CheckPauseScope(types.ModuleName, params.Paused, method)
}
