package keeper_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/stretchr/testify/require"
)

func TestMergeRetiredFollowsPlanHeight(t *testing.T) {
	s := newSuite(t, nil)
	upgrades := &s.app.UpgradeKeeper
	const scheduledHeight = int64(100)
	const appliedHeight = int64(200)
	heights := []int64{1, scheduledHeight - 1, scheduledHeight, scheduledHeight + 1, appliedHeight - 1, appliedHeight, appliedHeight + 1}
	meter := sdk.NewGasMeter(1, 1, 1)
	meter.ConsumeGas(1, "caller gas already consumed")
	ctx := s.ctx.WithGasMeter(meter)

	for _, height := range heights {
		require.False(t, keeper.MergeRetired(ctx.WithBlockHeight(height), upgrades), "no applied plan at height %d", height)
		require.Equal(t, uint64(1), meter.GasConsumed())
	}

	upgrades.SetDone(s.ctx.WithBlockHeight(1), types.StoreDeleteUpgrade)
	for _, height := range heights {
		require.False(t, keeper.MergeRetired(ctx.WithBlockHeight(height), upgrades), "another plan must not retire at height %d", height)
		require.Equal(t, uint64(1), meter.GasConsumed())
	}

	require.NoError(t, upgrades.ScheduleUpgrade(s.ctx, upgradetypes.Plan{
		Name: types.MergeUpgrade, Height: scheduledHeight,
	}))
	for _, height := range heights {
		require.False(t, keeper.MergeRetired(ctx.WithBlockHeight(height), upgrades), "scheduled plan must not retire at height %d", height)
		require.Equal(t, uint64(1), meter.GasConsumed())
	}

	upgrades.SetDone(s.ctx.WithBlockHeight(appliedHeight), types.MergeUpgrade)
	require.Equal(t, appliedHeight, upgrades.GetDoneHeight(s.ctx, types.MergeUpgrade))
	for _, height := range heights {
		require.Equal(t, height >= appliedHeight, keeper.MergeRetired(ctx.WithBlockHeight(height), upgrades), "applied plan at height %d", height)
		require.Equal(t, uint64(1), meter.GasConsumed())
		require.Same(t, meter, ctx.GasMeter())
	}
}

func TestMergeRetiredWithoutReader(t *testing.T) {
	s := newSuite(t, nil)
	const appliedHeight = int64(200)
	s.app.UpgradeKeeper.SetDone(s.ctx.WithBlockHeight(appliedHeight), types.MergeUpgrade)
	meter := sdk.NewGasMeter(1, 1, 1)
	meter.ConsumeGas(1, "caller gas already consumed")
	ctx := s.ctx.WithGasMeter(meter)
	for _, height := range []int64{appliedHeight - 1, appliedHeight, appliedHeight + 1} {
		require.False(t, keeper.MergeRetired(ctx.WithBlockHeight(height), nil), "nil reader at height %d", height)
		require.Equal(t, uint64(1), meter.GasConsumed())
	}
	require.False(t, keeper.MergeRetired(sdk.Context{}, nil))
}
