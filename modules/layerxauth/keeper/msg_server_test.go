package keeper

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/stretchr/testify/require"
)

func TestAuthUpdateParamsAuthority(t *testing.T) {
	h := newHarness(t)
	server := NewMsgServerImpl(h.keeper)
	ctx := sdk.WrapSDKContext(h.ctx)
	initial := types.Params{IdempotencyRetention: 32}
	require.NoError(t, h.keeper.SetParams(h.ctx, initial))
	before := append([]byte(nil), h.ctx.KVStore(h.key).Get(types.ParamsKey)...)
	params := types.Params{Paused: true, IdempotencyRetention: 64}
	otherAuthority := sdk.AccAddress([]byte("other-auth-authority")).String()
	require.NotEqual(t, h.authority, otherAuthority)
	for _, authority := range []string{otherAuthority, ""} {
		t.Run("unauthorized/"+authority, func(t *testing.T) {
			response, err := server.UpdateParams(ctx, &types.MsgUpdateParams{Authority: authority, Params: params})
			require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
			require.Nil(t, response)
			require.Equal(t, initial, h.keeper.GetParams(h.ctx))
			require.Equal(t, before, h.ctx.KVStore(h.key).Get(types.ParamsKey))
		})
	}

	response, err := server.UpdateParams(ctx, nil)
	require.ErrorIs(t, err, sdkerrors.ErrInvalidRequest)
	require.Nil(t, response)
	require.Equal(t, before, h.ctx.KVStore(h.key).Get(types.ParamsKey))

	response, err = server.UpdateParams(ctx, &types.MsgUpdateParams{Authority: h.authority, Params: params})
	require.NoError(t, err)
	require.Equal(t, &types.MsgUpdateParamsResponse{}, response)
	require.Equal(t, params, h.keeper.GetParams(h.ctx))
	var persisted types.Params
	h.keeper.cdc.MustUnmarshal(h.ctx.KVStore(h.key).Get(types.ParamsKey), &persisted)
	require.Equal(t, params, persisted)
}

func TestAuthUpdateParamsPauses(t *testing.T) {
	h := newHarness(t)
	server := NewMsgServerImpl(h.keeper)
	ctx := sdk.WrapSDKContext(h.ctx)
	methods := []string{"grantSession", "grantSessionSuccessor", "grantCapability"}
	for _, method := range methods {
		require.NoError(t, h.keeper.checkPause(h.ctx, method), method)
	}

	params := types.Params{Paused: true, IdempotencyRetention: 64}
	response, err := server.UpdateParams(ctx, &types.MsgUpdateParams{Authority: h.authority, Params: params})
	require.NoError(t, err)
	require.Equal(t, &types.MsgUpdateParamsResponse{}, response)
	require.Equal(t, params, h.keeper.GetParams(h.ctx))
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			err := h.keeper.checkPause(h.ctx, method)
			require.ErrorIs(t, err, govtypes.ErrPausedScope)
			require.Contains(t, err.Error(), "layerxauth."+method)
			codespace, code, _ := sdkerrors.ABCIInfo(err, false)
			require.Equal(t, govtypes.ModuleName, codespace)
			require.Equal(t, uint32(735), code)
		})
	}
	require.NoError(t, h.keeper.checkPause(h.ctx, "revokeGrant"))

	params.Paused = false
	response, err = server.UpdateParams(ctx, &types.MsgUpdateParams{Authority: h.authority, Params: params})
	require.NoError(t, err)
	require.Equal(t, &types.MsgUpdateParamsResponse{}, response)
	require.Equal(t, params, h.keeper.GetParams(h.ctx))
	for _, method := range methods {
		require.NoError(t, h.keeper.checkPause(h.ctx, method), method)
	}
}
