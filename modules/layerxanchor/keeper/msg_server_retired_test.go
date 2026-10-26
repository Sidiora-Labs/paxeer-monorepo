package keeper_test

import (
	"strconv"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/stretchr/testify/require"
)

func TestMsgUpdateParamsRefusedAfterMerge(t *testing.T) {
	s := newSuite(t, nil)
	t.Cleanup(func() { require.NoError(t, s.app.Close()) })
	upgrades := &s.app.UpgradeKeeper
	server := keeper.NewMsgServerImplWithUpgrades(s.k, upgrades)
	const scheduledHeight = int64(100)
	const appliedHeight = int64(200)

	update := func(t *testing.T, ctx sdk.Context, server types.MsgServer) {
		t.Helper()
		params := s.k.GetParams(ctx)
		params.Threshold++
		response, err := server.UpdateParams(sdk.WrapSDKContext(ctx), &types.MsgUpdateParams{
			Authority: s.authority.String(), Params: params,
		})
		require.NoError(t, err)
		require.Equal(t, &types.MsgUpdateParamsResponse{}, response)
		require.Equal(t, params, s.k.GetParams(ctx))
	}

	t.Run("no_applied_plan", func(t *testing.T) {
		update(t, s.ctx.WithBlockHeight(appliedHeight), server)
	})

	upgrades.SetDone(s.ctx.WithBlockHeight(1), types.StoreDeleteUpgrade)
	t.Run("unrelated_plan", func(t *testing.T) {
		update(t, s.ctx.WithBlockHeight(appliedHeight), server)
	})

	require.NoError(t, upgrades.ScheduleUpgrade(s.ctx, upgradetypes.Plan{
		Name: types.MergeUpgrade, Height: scheduledHeight,
	}))
	t.Run("scheduled_plan", func(t *testing.T) {
		update(t, s.ctx.WithBlockHeight(scheduledHeight), server)
		update(t, s.ctx.WithBlockHeight(appliedHeight), server)
	})

	upgrades.SetDone(s.ctx.WithBlockHeight(appliedHeight), types.MergeUpgrade)
	t.Run("before_applied_height", func(t *testing.T) {
		update(t, s.ctx.WithBlockHeight(appliedHeight-1), server)
	})

	params := s.k.GetParams(s.ctx)
	changed := params
	changed.Threshold++
	for _, height := range []int64{appliedHeight, appliedHeight + 1} {
		ctx := s.ctx.WithBlockHeight(height)
		for _, tc := range []struct {
			name string
			msg  *types.MsgUpdateParams
		}{
			{name: "nil_message"},
			{name: "malformed_authority", msg: &types.MsgUpdateParams{Authority: "invalid", Params: changed}},
			{name: "unauthorized", msg: &types.MsgUpdateParams{Authority: s.reporter.String(), Params: changed}},
			{name: "authorized", msg: &types.MsgUpdateParams{Authority: s.authority.String(), Params: changed}},
			{name: "invalid_params", msg: &types.MsgUpdateParams{Authority: s.authority.String()}},
		} {
			t.Run(tc.name+"/"+strconv.FormatInt(height, 10), func(t *testing.T) {
				state := s.k.ExportGenesis(ctx)
				events := ctx.EventManager().Events()
				response, err := server.UpdateParams(sdk.WrapSDKContext(ctx), tc.msg)
				require.ErrorIs(t, err, types.ErrModuleRetired)
				require.Nil(t, response)
				require.Equal(t, state, s.k.ExportGenesis(ctx))
				require.Equal(t, events, ctx.EventManager().Events())
			})
		}
	}

	for _, tc := range []struct {
		name   string
		server types.MsgServer
	}{
		{name: "legacy_replay", server: keeper.NewMsgServerImpl(s.k)},
		{name: "nil_reader", server: keeper.NewMsgServerImplWithUpgrades(s.k, nil)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := s.ctx.WithBlockHeight(appliedHeight + 1)
			update(t, ctx, tc.server)
			for _, msg := range []*types.MsgUpdateParams{nil, {Authority: "invalid"}, {Authority: s.reporter.String()}} {
				state := s.k.ExportGenesis(ctx)
				response, err := tc.server.UpdateParams(sdk.WrapSDKContext(ctx), msg)
				require.ErrorIs(t, err, types.ErrUnauthorized)
				require.Nil(t, response)
				require.Equal(t, state, s.k.ExportGenesis(ctx))
			}
		})
	}
}
