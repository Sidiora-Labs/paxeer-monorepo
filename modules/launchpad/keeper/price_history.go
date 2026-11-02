package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
)

func (k *Keeper) PriceHistory(ctx sdk.Context, denom string, fromHeight, toHeight uint64) ([]types.PricePoint, error) {
	market, found := k.GetMarket(ctx, denom)
	if !found {
		return nil, sdkerrors.Wrap(types.ErrUnknownMarket, denom)
	}
	return market.PriceHistory(fromHeight, toHeight)
}
