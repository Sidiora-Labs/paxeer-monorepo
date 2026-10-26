package keeper

import (
	"context"
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
)

type msgServer struct {
	types.UnimplementedMsgServer
	keeper *Keeper
}

var _ types.MsgServer = (*msgServer)(nil)

func NewMsgServerImpl(k *Keeper) types.MsgServer { return &msgServer{keeper: k} }

func (s *msgServer) UpdateParams(goCtx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if msg == nil {
		return nil, sdkerrors.ErrInvalidRequest.Wrap("missing message")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	cacheCtx, write := ctx.CacheContext()
	before, err := s.keeper.GetParams(cacheCtx)
	if err != nil {
		return nil, err
	}
	if err := s.keeper.UpdateParams(cacheCtx, msg.Authority, msg.Params); err != nil {
		return nil, err
	}
	if msg.Params.CollateralToken != before.CollateralToken {
		return nil, fmt.Errorf("%w: collateral token cannot change", types.ErrInvalidParams)
	}
	cacheCtx.EventManager().EmitEvent(sdk.NewEvent("perps_params_updated",
		sdk.NewAttribute("authority", msg.Authority)))
	write()
	ctx.EventManager().EmitEvents(cacheCtx.EventManager().Events())
	return &types.MsgUpdateParamsResponse{}, nil
}
