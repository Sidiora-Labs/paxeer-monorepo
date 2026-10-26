package keeper

import (
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func (k *Keeper) PriceHistory(ctx sdk.Context, denom string, fromHeight, toHeight uint64) ([]types.PricePoint, error) {
	market, found := k.GetMarket(ctx, denom)
	if !found {
		return nil, fmt.Errorf("%w: %s", types.ErrUnknownMarket, denom)
	}
	return market.PriceHistory(fromHeight, toHeight)
}
