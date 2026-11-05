package keeper

import (
	"context"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (q Querier) Market(goCtx context.Context, req *types.QueryMarketRequest) (*types.QueryMarketResponse, error) {
	ctx, err := queryContext(goCtx, req)
	if err != nil {
		return nil, err
	}
	if req.Denom == "" {
		return nil, status.Error(codes.InvalidArgument, "empty denom")
	}
	market, found := q.GetMarket(ctx, req.Denom)
	if !found {
		return nil, types.ErrUnknownMarket
	}
	return &types.QueryMarketResponse{Market: types.MarketInfoOf(market)}, nil
}
