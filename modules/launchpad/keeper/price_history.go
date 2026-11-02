package keeper

import (
	"encoding/json"
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
)

func (k *Keeper) PriceHistory(ctx sdk.Context, denom string, fromHeight, toHeight uint64) ([]types.PricePoint, error) {
	raw := k.store(ctx).Get(types.MarketKey(denom))
	if raw == nil {
		return nil, sdkerrors.Wrap(types.ErrUnknownMarket, denom)
	}
	var market types.Market
	if err := json.Unmarshal(raw, &market); err != nil {
		return nil, fmt.Errorf("%w: corrupt market %s: %v", types.ErrInvalidMarket, denom, err)
	}
	if err := market.Validate(); err != nil {
		return nil, err
	}
	return market.PriceHistory(fromHeight, toHeight)
}
