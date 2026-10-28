package keeper

import (
	"context"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
)

func (q Querier) PriceHistory(goCtx context.Context, req *types.QueryPriceHistoryRequest) (*types.QueryPriceHistoryResponse, error) {
	ctx, err := queryContext(goCtx, req)
	if err != nil {
		return nil, err
	}
	points, err := q.Keeper.PriceHistory(ctx, req.Denom, req.FromHeight, req.ToHeight)
	if err != nil {
		return nil, err
	}
	return &types.QueryPriceHistoryResponse{Points: points}, nil
}
