package keeper

import (
	"bytes"
	"context"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
)

type msgServer struct{ k Keeper }

func NewMsgServerImpl(k Keeper) types.MsgServer { return msgServer{k: k} }

var _ types.MsgServer = msgServer{}

func (s msgServer) UpdateParams(goCtx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if msg.Authority != s.k.Authority() {
		return nil, sdkerrors.Wrapf(sdkerrors.ErrUnauthorized, "%s is not the pay authority", msg.Authority)
	}
	if err := msg.Params.Validate(); err != nil {
		return nil, err
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	var enabledRecord [32]byte
	enabledRecord[31] = 1
	for _, family := range []struct {
		moduleID uint16
		name     string
		enabled  bool
	}{
		{types.EscrowModuleID, "escrow", msg.Params.EscrowEnabled},
		{types.BudgetModuleID, "budget", msg.Params.BudgetEnabled},
		{types.StreamModuleID, "stream", msg.Params.StreamEnabled},
		{types.ServiceModuleID, "service", msg.Params.ServiceEnabled},
	} {
		if !family.enabled {
			continue
		}
		key, err := types.KernelModuleEnableKey(family.moduleID)
		if err != nil {
			return nil, types.ErrKernelGateMismatch.Wrapf("%s: %v", family.name, err)
		}
		if !bytes.Equal(ctx.KVStore(s.k.hostKey).Get(key), enabledRecord[:]) {
			return nil, types.ErrKernelGateMismatch.Wrap(family.name)
		}
	}
	s.k.SetParams(ctx, msg.Params)
	return &types.MsgUpdateParamsResponse{}, nil
}
