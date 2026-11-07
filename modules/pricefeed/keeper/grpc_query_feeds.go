package keeper

import (
	"context"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s queryServer) FeedKey(goCtx context.Context, req *types.QueryFeedKeyRequest) (*types.QueryFeedKeyResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	operator, err := sdk.ValAddressFromBech32(req.Validator)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	key, found := s.keeper.GetFeedKey(sdk.UnwrapSDKContext(goCtx), operator)
	if !found {
		return nil, types.ErrNoKey.Wrapf("validator %s", operator)
	}
	return &types.QueryFeedKeyResponse{PubKey: key.PubKey, FeedAccount: key.FeedAccount}, nil
}

func (s queryServer) ValidatorByFeedAccount(goCtx context.Context, req *types.QueryValidatorByFeedAccountRequest) (*types.QueryValidatorByFeedAccountResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	account, err := sdk.AccAddressFromBech32(req.FeedAccount)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	operator, found := s.keeper.GetValidatorByFeedAccount(sdk.UnwrapSDKContext(goCtx), account)
	if !found {
		return nil, types.ErrUnregisteredFeeder.Wrapf("feeder %s", account)
	}
	return &types.QueryValidatorByFeedAccountResponse{Validator: operator.String()}, nil
}

func (s queryServer) Observation(goCtx context.Context, req *types.QueryObservationRequest) (*types.QueryObservationResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	operator, err := sdk.ValAddressFromBech32(req.Validator)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	record, found := s.keeper.GetObservation(sdk.UnwrapSDKContext(goCtx), req.MarketId, operator)
	if !found {
		return nil, status.Errorf(codes.NotFound, "no observation for market %d and validator %s", req.MarketId, operator)
	}
	return &types.QueryObservationResponse{
		Sequence:          record.Sequence,
		IndexPrice:        record.IndexPrice.String(),
		MarkPrice:         record.MarkPrice.String(),
		SourceTimestampMs: record.SourceTimestampMs,
		Height:            record.Height,
	}, nil
}
