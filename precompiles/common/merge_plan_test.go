package common_test

import (
	"testing"

	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/stretchr/testify/require"
)

func TestPlanActiveNilUpgradeKeeper(t *testing.T) {
	k := &evmkeeper.Keeper{}
	require.Nil(t, k.UpgradeKeeper())
	require.NotPanics(t, func() {
		require.False(t, common.PlanActive(sdk.Context{}, k, common.MergeUpgrade))
		require.False(t, common.MergePlanActive(sdk.Context{}, k))
	})
	require.False(t, common.PlanActive(sdk.Context{}, nil, common.MergeUpgrade))
	require.False(t, common.PlanActive(sdk.Context{}, struct{}{}, common.MergeUpgrade))
}

func TestMergePlanActiveAtPlanHeight(t *testing.T) {
	const planHeight = int64(200)
	k := &testkeeper.EVMTestApp.EvmKeeper
	ctx, _ := testkeeper.EVMTestApp.GetContextForDeliverTx(nil).CacheContext()
	upgrades := k.UpgradeKeeper()
	require.NotNil(t, upgrades)
	require.False(t, common.MergePlanActive(ctx.WithBlockHeight(planHeight), k))
	upgrades.SetDone(ctx.WithBlockHeight(planHeight), common.MergeUpgrade)

	for _, tc := range []struct {
		name   string
		height int64
		active bool
	}{
		{name: "below", height: planHeight - 1, active: false},
		{name: "at", height: planHeight, active: true},
		{name: "above", height: planHeight + 1, active: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			meter := sdk.NewGasMeter(1_000_000, 1, 1)
			meter.ConsumeGas(17, "existing consumption")
			callCtx := ctx.WithBlockHeight(tc.height).WithGasMeter(meter)
			gasBefore := meter.GasConsumed()
			require.Equal(t, tc.active, common.MergePlanActive(callCtx, k))
			require.Equal(t, gasBefore, meter.GasConsumed())
			require.Same(t, meter, callCtx.GasMeter())
			require.False(t, common.PlanActive(callCtx, k, "unapplied-plan"))
			require.Equal(t, gasBefore, meter.GasConsumed())
		})
	}
}
