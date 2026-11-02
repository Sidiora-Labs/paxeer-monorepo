package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func (k Keeper) RetireAtMerge(ctx sdk.Context) (sdk.Int, error) {
	total, err := k.ReturnAllBonds(ctx)
	if err != nil {
		return total, err
	}
	ctx.EventManager().EmitEvent(sdk.NewEvent(types.EventAnchorRetired,
		sdk.NewAttribute(types.AttributeReturned, total.String())))
	return total, nil
}
