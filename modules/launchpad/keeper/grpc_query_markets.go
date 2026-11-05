package keeper

import (
	"context"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
)

func (q Querier) Markets(goCtx context.Context, req *types.QueryMarketsRequest) (*types.QueryMarketsResponse, error) {
	ctx, err := queryContext(goCtx, req)
	if err != nil {
		return nil, err
	}
	if err := types.ValidatePage(req.Offset, req.Limit); err != nil {
		return nil, sdkerrors.Wrap(types.ErrInvalidRange, err.Error())
	}
	markets := q.GetMarkets(ctx, req.Offset, req.Limit)
	response := &types.QueryMarketsResponse{
		Markets: make([]types.MarketInfo, len(markets)),
		Total:   q.GetMarketCount(ctx),
	}
	for i, market := range markets {
		response.Markets[i] = types.MarketInfoOf(market)
	}
	return response, nil
}
