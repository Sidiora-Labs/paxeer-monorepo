package keeper

import (
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	upgradekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/keeper"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func newProgramActivationKeeper(t *testing.T) (Keeper, sdk.Context, *upgradekeeper.Keeper) {
	t.Helper()
	xwebKey := sdk.NewKVStoreKey(types.StoreKey)
	upgradeKey := sdk.NewKVStoreKey(upgradetypes.StoreKey)
	db := dbm.NewMemDB()
	cms := store.NewCommitMultiStore(db)
	for _, key := range []sdk.StoreKey{xwebKey, upgradeKey} {
		cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	require.NoError(t, cms.LoadLatestVersion())
	ctx := sdk.NewContext(cms, tmproto.Header{Height: 1}, false)
	cdc := codec.NewProtoCodec(cdctypes.NewInterfaceRegistry())
	upgrades := upgradekeeper.NewKeeper(nil, upgradeKey, cdc, t.TempDir(), nil)
	return NewKeeper(xwebKey, nil, nil), ctx, &upgrades
}

func TestProgramOriginActiveFromTheMergePlanHeight(t *testing.T) {
	k, ctx, upgrades := newProgramActivationKeeper(t)
	k.upgrades = upgrades
	require.Equal(t, "v7.0", ProgramOriginPlan)
	require.EqualError(t, ErrProgramModuleDisabled, "LXP_MODULE_DISABLED")

	assertActive := func(height int64, active bool) {
		t.Helper()
		meter := sdk.NewGasMeter(7, 1, 1)
		meter.ConsumeGas(7, "caller gas already consumed")
		callCtx := ctx.WithBlockHeight(height).WithGasMeter(meter)
		require.Equal(t, active, k.ProgramOriginActive(callCtx), "height %d", height)
		require.Equal(t, sdk.Gas(7), meter.GasConsumed(), "height %d charged caller gas", height)
		require.Same(t, meter, callCtx.GasMeter())
	}
	for _, height := range []int64{1, 99, 100, 101, 1000} {
		assertActive(height, false)
	}
	upgrades.SetDone(ctx.WithBlockHeight(50), "v6.9")
	assertActive(100, false)
	upgrades.SetDone(ctx.WithBlockHeight(100), ProgramOriginPlan)
	for _, height := range []int64{1, 99, 100, 101, 1000} {
		assertActive(height, height >= 100)
	}
}

func TestProgramOriginInactiveWithoutAnUpgradeReader(t *testing.T) {
	k, ctx, upgrades := newProgramActivationKeeper(t)
	upgrades.SetDone(ctx.WithBlockHeight(100), ProgramOriginPlan)
	require.Nil(t, k.upgrades)
	for _, height := range []int64{1, 99, 100, 101, 1000} {
		meter := sdk.NewGasMeter(0, 1, 1)
		callCtx := ctx.WithBlockHeight(height).WithGasMeter(meter)
		require.False(t, k.ProgramOriginActive(callCtx), "height %d", height)
		require.False(t, (Keeper{}).ProgramOriginActive(callCtx), "zero keeper at height %d", height)
		require.Zero(t, meter.GasConsumed(), "height %d charged caller gas", height)
	}
}
