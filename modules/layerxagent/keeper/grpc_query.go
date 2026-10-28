package keeper

import (
	"context"
	"strings"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Querier struct{ Keeper }

var _ types.QueryServer = Querier{}

func (q Querier) Params(goCtx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	return &types.QueryParamsResponse{Params: q.GetParams(sdk.UnwrapSDKContext(goCtx))}, nil
}

func (q Querier) Job(goCtx context.Context, req *types.QueryJobRequest) (*types.QueryJobResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	job, found := q.GetJob(sdk.UnwrapSDKContext(goCtx), req.Id)
	if !found {
		return nil, status.Errorf(codes.NotFound, "job %d not found", req.Id)
	}
	return &types.QueryJobResponse{Job: JobToProto(job)}, nil
}

func (q Querier) Pending(goCtx context.Context, req *types.QueryPendingRequest) (*types.QueryPendingResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	if !strings.HasPrefix(req.Creator, "0x") || !common.IsHexAddress(req.Creator) {
		return nil, status.Error(codes.InvalidArgument, "creator must be a 0x hex address")
	}
	ids := q.Keeper.Pending(sdk.UnwrapSDKContext(goCtx), common.HexToAddress(req.Creator))
	return &types.QueryPendingResponse{JobIds: ids}, nil
}
