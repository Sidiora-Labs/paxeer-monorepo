package keeper

import (
	"context"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

type msgServer struct {
	keeper   Keeper
	upgrades UpgradeReader
}

var _ types.MsgServer = msgServer{}

func NewMsgServerImpl(k Keeper) types.MsgServer { return msgServer{keeper: k} }

func NewMsgServerImplWithUpgrades(k Keeper, upgrades UpgradeReader) types.MsgServer {
	return msgServer{keeper: k, upgrades: upgrades}
}

func (s msgServer) UpdateParams(ctx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if MergeRetired(sdk.UnwrapSDKContext(ctx), s.upgrades) {
		return nil, types.ErrModuleRetired
	}
	if msg == nil {
		return nil, types.ErrUnauthorized.Wrap("missing message")
	}
	authority, err := sdk.AccAddressFromBech32(msg.Authority)
	if err != nil {
		return nil, types.ErrUnauthorized.Wrapf("authority: %v", err)
	}

	if err := s.keeper.UpdateParams(sdk.UnwrapSDKContext(ctx), authority, msg.Params); err != nil {
		return nil, err
	}
	return &types.MsgUpdateParamsResponse{}, nil
}
