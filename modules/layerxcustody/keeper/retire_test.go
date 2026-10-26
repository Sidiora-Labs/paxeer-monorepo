package keeper_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/stretchr/testify/require"
)

func TestRetiredFollowsTheMergePlanDoneHeight(t *testing.T) {
	e := newEnv(t, 0)
	ctx := e.ctx.WithBlockHeight(20).WithGasMeter(sdk.NewGasMeter(1, 1, 1))
	require.False(t, e.k.Retired(ctx))
	require.Zero(t, ctx.GasMeter().GasConsumed())

	app := testkeeper.EVMTestApp
	app.UpgradeKeeper.SetDone(e.ctx.WithBlockHeight(20), types.MergePlanUpgrade)
	require.Equal(t, int64(20), app.UpgradeKeeper.GetDoneHeight(e.ctx, types.MergePlanUpgrade))

	for _, tc := range []struct {
		name    string
		height  int64
		retired bool
	}{
		{name: "at done height", height: 20, retired: true},
		{name: "above done height", height: 21, retired: true},
		{name: "below done height", height: 19, retired: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := e.ctx.WithBlockHeight(tc.height).WithGasMeter(sdk.NewGasMeter(1, 1, 1))
			require.Equal(t, tc.retired, e.k.Retired(ctx))
			require.Zero(t, ctx.GasMeter().GasConsumed())
		})
	}
}

func TestRetiredIsFalseWithoutAnUpgradeReader(t *testing.T) {
	e := newEnv(t, 0)
	app := testkeeper.EVMTestApp
	app.UpgradeKeeper.SetDone(e.ctx, types.MergePlanUpgrade)
	k := keeper.NewKeeper(app.AppCodec(), app.GetKey(types.StoreKey),
		app.AccountKeeper, app.BankKeeper, &app.EvmKeeper, nil)
	ctx := e.ctx.WithGasMeter(sdk.NewGasMeter(1, 1, 1))

	require.False(t, k.Retired(ctx))
	require.Zero(t, ctx.GasMeter().GasConsumed())
}
