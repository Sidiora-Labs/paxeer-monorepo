package keeper

import (
	"context"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s queryServer) Prices(goCtx context.Context, req *types.QueryPricesRequest) (*types.QueryPricesResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	if len(req.MarketIds) > 64 {
		return nil, types.ErrInvalidParams.Wrap("Prices takes at most 64 market ids")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	prices := make([]types.QueryPriceResponse, len(req.MarketIds))
	for i, id := range req.MarketIds {
		price, err := s.keeper.GetPrice(ctx, id)
		if err != nil {
			return nil, err
		}
		prices[i] = types.QueryPriceResponse{
			Index:       price.Index.String(),
			Mark:        price.Mark.String(),
			TimestampMs: price.TimestampMs,
			Height:      price.Height,
			Sources:     price.Sources,
			Fresh:       price.Fresh,
		}
	}
	return &types.QueryPricesResponse{Prices: prices}, nil
}
