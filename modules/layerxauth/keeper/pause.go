package keeper

import (
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func CheckPauseScope(module string, paused bool, method string) error {
	return govtypes.CheckPauseScope(module, paused, method)
}

func (k *Keeper) checkPause(ctx sdk.Context, method string) error {
	params := k.GetParams(ctx)
	return CheckPauseScope("layerxauth", params.Paused, method)
}
