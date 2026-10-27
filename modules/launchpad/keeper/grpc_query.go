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

func (q Querier) MarketsByCreator(goCtx context.Context, req *types.QueryMarketsByCreatorRequest) (*types.QueryMarketsByCreatorResponse, error) {
	ctx, err := queryContext(goCtx, req)
	if err != nil {
		return nil, err
	}
	return q.UnimplementedQueryServer.MarketsByCreator(sdk.WrapSDKContext(ctx), req)
}

func (q Querier) PriceHistory(goCtx context.Context, req *types.QueryPriceHistoryRequest) (*types.QueryPriceHistoryResponse, error) {
	ctx, err := queryContext(goCtx, req)
	if err != nil {
		return nil, err
	}
	return q.UnimplementedQueryServer.PriceHistory(sdk.WrapSDKContext(ctx), req)
}

func (q Querier) Quote(goCtx context.Context, req *types.QueryQuoteRequest) (*types.QueryQuoteResponse, error) {
	ctx, err := queryContext(goCtx, req)
	if err != nil {
		return nil, err
	}
	return q.UnimplementedQueryServer.Quote(sdk.WrapSDKContext(ctx), req)
}

func (q Querier) Treasury(goCtx context.Context, req *types.QueryTreasuryRequest) (*types.QueryTreasuryResponse, error) {
	ctx, err := queryContext(goCtx, req)
	if err != nil {
		return nil, err
	}
	return q.UnimplementedQueryServer.Treasury(sdk.WrapSDKContext(ctx), req)
}

func (q Querier) Surface(goCtx context.Context, req *types.QuerySurfaceRequest) (*types.QuerySurfaceResponse, error) {
	ctx, err := queryContext(goCtx, req)
	if err != nil {
		return nil, err
	}
	return q.UnimplementedQueryServer.Surface(sdk.WrapSDKContext(ctx), req)
}

func (q Querier) Retired(goCtx context.Context, req *types.QueryRetiredRequest) (*types.QueryRetiredResponse, error) {
	ctx, err := queryContext(goCtx, req)
	if err != nil {
		return nil, err
	}
	return q.UnimplementedQueryServer.Retired(sdk.WrapSDKContext(ctx), req)
}

func (q Querier) FeeRight(goCtx context.Context, req *types.QueryFeeRightRequest) (*types.QueryFeeRightResponse, error) {
	ctx, err := queryContext(goCtx, req)
	if err != nil {
		return nil, err
	}
	return q.UnimplementedQueryServer.FeeRight(sdk.WrapSDKContext(ctx), req)
}

func (q Querier) AirdropEpoch(goCtx context.Context, req *types.QueryAirdropEpochRequest) (*types.QueryAirdropEpochResponse, error) {
	ctx, err := queryContext(goCtx, req)
	if err != nil {
		return nil, err
	}
	return q.UnimplementedQueryServer.AirdropEpoch(sdk.WrapSDKContext(ctx), req)
}

func (q Querier) AirdropClaimed(goCtx context.Context, req *types.QueryAirdropClaimedRequest) (*types.QueryAirdropClaimedResponse, error) {
	ctx, err := queryContext(goCtx, req)
	if err != nil {
		return nil, err
	}
	return q.UnimplementedQueryServer.AirdropClaimed(sdk.WrapSDKContext(ctx), req)
}

func (q Querier) HasRole(goCtx context.Context, req *types.QueryHasRoleRequest) (*types.QueryHasRoleResponse, error) {
	ctx, err := queryContext(goCtx, req)
	if err != nil {
		return nil, err
	}
	return q.UnimplementedQueryServer.HasRole(sdk.WrapSDKContext(ctx), req)
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
