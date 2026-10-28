package keeper

import (
	"context"
	"errors"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (q queryServer) ProgramRequest(goCtx context.Context, req *types.QueryProgramRequestRequest) (*types.QueryProgramRequestResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	var programID types.Hash32
	if len(req.ProgramId) != len(programID) {
		return nil, status.Error(codes.InvalidArgument, "program_id must be 32 bytes")
	}
	copy(programID[:], req.ProgramId)
	ctx := sdk.UnwrapSDKContext(goCtx)
	record, found, err := q.GetProgramPendingRecord(ctx, programID, req.RequestId)
	if err != nil {
		return nil, status.Error(codes.DataLoss, err.Error())
	}
	if !found {
		return nil, status.Error(codes.NotFound, errProgramUnknownField.Error())
	}
	denom, found := q.ProgramFeeDenom(ctx, record.FeeAsset)
	if !found {
		return nil, status.Error(codes.NotFound, errProgramUnknownField.Error())
	}
	raw, err := record.Bytes()
	if err != nil {
		return nil, status.Error(codes.DataLoss, err.Error())
	}
	return &types.QueryProgramRequestResponse{Record: raw, FeeDenom: denom}, nil
}

func (q queryServer) ProgramResult(goCtx context.Context, req *types.QueryProgramResultRequest) (*types.QueryProgramResultResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	var programID types.Hash32
	if len(req.ProgramId) != len(programID) {
		return nil, status.Error(codes.InvalidArgument, "program_id must be 32 bytes")
	}
	copy(programID[:], req.ProgramId)
	result, err := q.GetProgramResult(sdk.UnwrapSDKContext(goCtx), programID, req.RequestId)
	if err != nil {
		switch {
		case errors.Is(err, errProgramUnknownField):
			return nil, status.Error(codes.NotFound, err.Error())
		case errors.Is(err, errProgramNonCanonical):
			return nil, status.Error(codes.DataLoss, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	return &types.QueryProgramResultResponse{
		ContentDigest: result.ContentDigest, FullLength: result.FullLength,
		ResponseLength: result.ResponseLength, Response: result.Response,
	}, nil
}

func (q queryServer) ProgramCounters(goCtx context.Context, req *types.QueryProgramCountersRequest) (*types.QueryProgramCountersResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	return &types.QueryProgramCountersResponse{
		Pending: q.ProgramPendingCount(ctx), Committed: q.ProgramCommittedCount(ctx),
		PendingCapacity: ProgramPendingCapacity, CommittedCapacity: ProgramCommittedCapacity,
	}, nil
}
