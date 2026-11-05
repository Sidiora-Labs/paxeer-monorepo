package keeper

import (
	"context"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (q queryServer) Request(goCtx context.Context, req *types.QueryRequestRequest) (*types.QueryRequestResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	request, found := q.GetRequest(sdk.UnwrapSDKContext(goCtx), req.RequestId)
	if !found {
		return nil, status.Error(codes.NotFound, types.ErrUnknownRequest.Error())
	}
	return &types.QueryRequestResponse{Request: request}, nil
}

func (q queryServer) Result(goCtx context.Context, req *types.QueryResultRequest) (*types.QueryResultResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	result, found := q.GetResult(sdk.UnwrapSDKContext(goCtx), req.RequestId)
	if !found {
		return nil, status.Error(codes.NotFound, types.ErrUnknownRequest.Error())
	}
	return &types.QueryResultResponse{Result: result}, nil
}
