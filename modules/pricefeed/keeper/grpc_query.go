package keeper

import (
	"context"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type queryServer struct {
	types.UnimplementedQueryServer
	keeper Keeper
}

func NewQueryServerImpl(k Keeper) types.QueryServer {
	return &queryServer{keeper: k}
}

var _ types.QueryServer = (*queryServer)(nil)

func (s queryServer) Params(goCtx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	params := s.keeper.GetParams(sdk.UnwrapSDKContext(goCtx))
	return &types.QueryParamsResponse{
		Authority:                    params.Authority,
		Markets:                      params.Markets,
		StalenessWindowMs:            params.StalenessWindowMs,
		MaxClockSkewMs:               params.MaxClockSkewMs,
		MinSources:                   params.MinSources,
		MaxDivergenceBps:             params.MaxDivergenceBps,
		MaxDeviationBps:              params.MaxDeviationBps,
		MaxObservationsPerSubmission: params.MaxObservationsPerSubmission,
		MaxSubmissionBytes:           params.MaxSubmissionBytes,
		MarkEmaTauMs:                 params.MarkEmaTauMs,
		MaxMarkBasisBps:              params.MaxMarkBasisBps,
		MaxPriceAgeMs:                params.MaxPriceAgeMs,
	}, nil
}

func (s queryServer) Price(goCtx context.Context, req *types.QueryPriceRequest) (*types.QueryPriceResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	price, err := s.keeper.GetPrice(sdk.UnwrapSDKContext(goCtx), req.MarketId)
	if err != nil {
		return nil, err
	}
	return &types.QueryPriceResponse{
		Index:       price.Index.String(),
		Mark:        price.Mark.String(),
		TimestampMs: price.TimestampMs,
		Height:      price.Height,
		Sources:     price.Sources,
		Fresh:       price.Fresh,
	}, nil
}
