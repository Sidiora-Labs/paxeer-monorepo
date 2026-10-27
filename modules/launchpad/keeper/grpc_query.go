package keeper

import (
	"context"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Querier struct {
	*Keeper
	types.UnimplementedQueryServer
}

var _ types.QueryServer = Querier{}

func NewQuerier(k *Keeper) Querier {
	return Querier{Keeper: k}
}

func queryContext[T any](goCtx context.Context, req *T) (sdk.Context, error) {
	if req == nil {
		return sdk.Context{}, status.Error(codes.InvalidArgument, "empty request")
	}
	return sdk.UnwrapSDKContext(goCtx), nil
}

func (q Querier) QuoteLedger(goCtx context.Context, req *types.QueryQuoteLedgerRequest) (*types.QueryQuoteLedgerResponse, error) {
	ctx, err := queryContext(goCtx, req)
	if err != nil {
		return nil, err
	}
	return q.UnimplementedQueryServer.QuoteLedger(sdk.WrapSDKContext(ctx), req)
}

func (q Querier) FeeAccounting(goCtx context.Context, req *types.QueryFeeAccountingRequest) (*types.QueryFeeAccountingResponse, error) {
	ctx, err := queryContext(goCtx, req)
	if err != nil {
		return nil, err
	}
	return q.UnimplementedQueryServer.FeeAccounting(sdk.WrapSDKContext(ctx), req)
}

func (q Querier) PoolMetadata(goCtx context.Context, req *types.QueryPoolMetadataRequest) (*types.QueryPoolMetadataResponse, error) {
	ctx, err := queryContext(goCtx, req)
	if err != nil {
		return nil, err
	}
	return q.UnimplementedQueryServer.PoolMetadata(sdk.WrapSDKContext(ctx), req)
}
