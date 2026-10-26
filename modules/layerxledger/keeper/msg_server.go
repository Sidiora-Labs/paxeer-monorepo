package keeper

import (
	"context"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
)

type msgServer struct{ keeper *Keeper }

var _ types.MsgServer = msgServer{}

func NewMsgServerImpl(k *Keeper) types.MsgServer { return msgServer{keeper: k} }

func (s msgServer) UpdateParams(goCtx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if msg == nil {
		return nil, sdkerrors.ErrInvalidRequest.Wrap("missing message")
	}
	if msg.Authority != s.keeper.authority {
		return nil, sdkerrors.ErrUnauthorized.Wrapf("%s is not the ledger authority", msg.Authority)
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	if err := s.keeper.SetParams(ctx, msg.Params); err != nil {
		return nil, err
	}
	return &types.MsgUpdateParamsResponse{}, nil
}
