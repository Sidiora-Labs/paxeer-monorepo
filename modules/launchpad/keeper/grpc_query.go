package keeper

import (
	"context"
	"errors"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
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
	type addressField struct {
		name  string
		value string
	}
	var addresses []addressField
	switch request := any(req).(type) {
	case *types.QuerySurfaceRequest:
		addresses = []addressField{{"address", request.Address}}
	case *types.QueryRetiredRequest:
		addresses = []addressField{{"address", request.Address}}
	case *types.QueryPoolMetadataRequest:
		addresses = []addressField{{"pool", request.Pool}}
	case *types.QueryAirdropEpochRequest:
		addresses = []addressField{{"pool", request.Pool}}
	case *types.QueryAirdropClaimedRequest:
		addresses = []addressField{{"pool", request.Pool}, {"holder", request.Holder}}
	case *types.QueryHasRoleRequest:
		addresses = []addressField{{"contract", request.Contract}, {"account", request.Account}}
	case *types.QueryFeeRightRequest:
		addresses = []addressField{{"collection", request.Collection}}
	}
	for _, field := range addresses {
		if _, err := types.ParseHexAddress(field.name, field.value); err != nil {
			if errors.Is(err, sdkerrors.ErrInvalidAddress) {
				return sdk.Context{}, status.Error(codes.InvalidArgument, err.Error())
			}
			break
		}
	}
	return sdk.UnwrapSDKContext(goCtx), nil
}
