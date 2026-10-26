package keeper

import (
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

var exitPaths = map[string]bool{
	"closePosition":           true,
	"partialClose":            true,
	"closePositionWithParams": true,
	"cancelOrder":             true,
	"cancelByCloid":           true,
	"deposit":                 true,
	"addCollateral":           true,
	"topUpPosition":           true,
}

func (k *Keeper) CheckPause(ctx sdk.Context, method string) error {
	params, err := k.GetParams(ctx)
	if err != nil {
		return err
	}
	if !params.Paused || exitPaths[method] {
		return nil
	}
	return types.ErrPaused
}

func (k *Keeper) SetPaused(ctx sdk.Context, authority string, paused bool) error {
	if authority != k.authority {
		return fmt.Errorf("%w: %s", types.ErrUnauthorized, authority)
	}
	params, err := k.GetParams(ctx)
	if err != nil {
		return err
	}
	params.Paused = paused
	return k.setParams(ctx, params)
}
