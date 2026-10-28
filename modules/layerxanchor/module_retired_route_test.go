package layerxanchor_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/stretchr/testify/require"
)

func TestAppRoutesAnchorUpdateParamsRetired(t *testing.T) {
	testApp := app.Setup(t, false, false, false)
	t.Cleanup(func() { require.NoError(t, testApp.Close()) })
	ctx := testApp.GetContextForDeliverTx(nil).WithBlockHeight(8).WithBlockTime(time.Unix(1_800_000_000, 0))
	k := testApp.LayerXAnchorKeeper
	upgrades := &testApp.UpgradeKeeper
	handler := testApp.MsgServiceRouter().Handler(&types.MsgUpdateParams{})
	require.NotNil(t, handler)
	const scheduledHeight = int64(100)
	const appliedHeight = int64(200)

	update := func(t *testing.T, height int64) {
		t.Helper()
		at := ctx.WithBlockHeight(height)
		params := k.GetParams(at)
		params.Threshold++
		msg := &types.MsgUpdateParams{Authority: params.Authority, Params: params}
		require.NoError(t, msg.ValidateBasic())
		result, err := handler(at, msg)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, params, k.GetParams(at))
		require.NotEmpty(t, result.Events)
		require.Equal(t, types.EventParamsUpdated, result.Events[len(result.Events)-1].Type)
	}

	t.Run("no_applied_plan", func(t *testing.T) {
		update(t, appliedHeight)
	})
	upgrades.SetDone(ctx.WithBlockHeight(1), types.StoreDeleteUpgrade)
	t.Run("unrelated_plan", func(t *testing.T) {
		update(t, appliedHeight)
	})
	require.NoError(t, upgrades.ScheduleUpgrade(ctx, upgradetypes.Plan{
		Name: types.MergeUpgrade, Height: scheduledHeight,
	}))
	t.Run("scheduled_plan", func(t *testing.T) {
		update(t, scheduledHeight)
		update(t, appliedHeight)
	})

	upgrades.SetDone(ctx.WithBlockHeight(appliedHeight), types.MergeUpgrade)
	require.Equal(t, appliedHeight, upgrades.GetDoneHeight(ctx, types.MergeUpgrade))
	t.Run("before_applied_height", func(t *testing.T) {
		update(t, appliedHeight-1)
	})
	for _, height := range []int64{appliedHeight, appliedHeight + 1} {
		t.Run("retired/"+strconv.FormatInt(height, 10), func(t *testing.T) {
			at := ctx.WithBlockHeight(height)
			before := k.ExportGenesis(at)
			events := at.EventManager().Events()
			params := k.GetParams(at)
			params.Threshold++
			msg := &types.MsgUpdateParams{Authority: params.Authority, Params: params}
			require.NoError(t, msg.ValidateBasic())
			result, err := handler(at, msg)
			require.ErrorIs(t, err, types.ErrModuleRetired)
			require.Nil(t, result)
			require.Equal(t, before, k.ExportGenesis(at))
			require.Equal(t, events, at.EventManager().Events())
		})
	}
}
