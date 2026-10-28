package keeper_test

import (
	"context"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestAgentMsgUpdateParamsAuthority(t *testing.T) {
	h := newHarness(t)
	ctx := sdk.WrapSDKContext(h.ctx)
	authority := authtypes.NewModuleAddress(govtypes.ModuleName).String()
	server := keeper.NewMsgServerImpl(h.keeper, authority)
	before := h.keeper.GetParams(h.ctx)
	params := before
	params.MaxBlockGas = 20_000_000
	params.Paused = true

	for _, signer := range []string{h.creatorAcc.String(), ""} {
		response, err := server.UpdateParams(ctx, &types.MsgUpdateParams{Authority: signer, Params: params})
		require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
		require.Nil(t, response)
		require.Equal(t, before, h.keeper.GetParams(h.ctx))
	}

	response, err := server.UpdateParams(ctx, &types.MsgUpdateParams{Authority: authority, Params: params})
	require.NoError(t, err)
	require.NotNil(t, response)
	require.Equal(t, params, h.keeper.GetParams(h.ctx))
	require.Equal(t, uint64(20_000_000), h.keeper.GetParams(h.ctx).MaxBlockGas)

	invalid := params
	invalid.MaxBlockGas = invalid.MaxJobGasLimit - 1
	response, err = server.UpdateParams(ctx, &types.MsgUpdateParams{Authority: authority, Params: invalid})
	require.EqualError(t, err, invalid.Validate().Error())
	require.Nil(t, response)
	require.Equal(t, params, h.keeper.GetParams(h.ctx))

	response, err = server.UpdateParams(context.Background(), nil)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	require.Nil(t, response)
	require.Equal(t, params, h.keeper.GetParams(h.ctx))

	custom := keeper.NewMsgServerImpl(h.keeper, h.creatorAcc.String())
	response, err = custom.UpdateParams(ctx, &types.MsgUpdateParams{Authority: authority, Params: before})
	require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
	require.Nil(t, response)
	require.Equal(t, params, h.keeper.GetParams(h.ctx))
	response, err = custom.UpdateParams(ctx, &types.MsgUpdateParams{Authority: h.creatorAcc.String(), Params: before})
	require.NoError(t, err)
	require.NotNil(t, response)
	require.Equal(t, before, h.keeper.GetParams(h.ctx))
}
