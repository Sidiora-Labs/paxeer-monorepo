package keeper

import (
	"context"
	"strconv"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
)

func (s *msgServer) SetPaused(goCtx context.Context, msg *types.MsgSetPaused) (*types.MsgSetPausedResponse, error) {
	if msg == nil {
		return nil, sdkerrors.ErrInvalidRequest.Wrap("missing message")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	cacheCtx, write := ctx.CacheContext()
	before, err := s.keeper.GetParams(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.keeper.SetPaused(cacheCtx, msg.Authority, msg.Paused); err != nil {
		return nil, err
	}
	if before.Paused != msg.Paused {
		write()
	}
	ctx.EventManager().EmitEvent(sdk.NewEvent("perps_pause_set",
		sdk.NewAttribute("paused", strconv.FormatBool(msg.Paused))))
	return &types.MsgSetPausedResponse{}, nil
}
