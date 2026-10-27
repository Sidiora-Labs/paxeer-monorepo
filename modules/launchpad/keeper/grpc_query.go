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
