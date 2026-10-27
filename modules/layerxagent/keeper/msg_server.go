package keeper

import (
	"context"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type msgServer struct {
	k         Keeper
	authority string
}

var _ types.MsgServer = msgServer{}

func NewMsgServerImpl(k Keeper, authority string) types.MsgServer {
	return msgServer{k: k, authority: authority}
}

func (s msgServer) UpdateParams(goCtx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if msg == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	if msg.Authority != s.authority {
		return nil, sdkerrors.Wrapf(sdkerrors.ErrUnauthorized, "invalid authority; expected %s, got %s", s.authority, msg.Authority)
	}
	if err := s.k.SetParams(sdk.UnwrapSDKContext(goCtx), msg.Params); err != nil {
		return nil, err
	}
	return &types.MsgUpdateParamsResponse{}, nil
}
