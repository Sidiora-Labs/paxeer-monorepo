package app

import (
	"testing"
	"time"

	ledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/prefix"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/stretchr/testify/require"
)

func TestRepair_22_31_589_AnchorDeleteRefusesWithoutMerge(t *testing.T) {
	w := NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), false)
	a, ctx := w.App, w.Ctx.WithBlockHeight(100)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	require.True(t, a.UpgradeKeeper.HasHandler(AnchorStoreDeleteUpgrade))
	require.Zero(t, a.UpgradeKeeper.GetDoneHeight(ctx, MergeUpgrade))

	for _, tc := range []struct {
		name        string
		mergeHeight int64
	}{
		{name: "merge not applied"},
		{name: "merge at current height", mergeHeight: 100},
		{name: "merge at later height", mergeHeight: 101},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cache, _ := ctx.CacheContext()
			if tc.mergeHeight > 0 {
				a.UpgradeKeeper.SetDone(cache.WithBlockHeight(tc.mergeHeight), MergeUpgrade)
			}
			require.Equal(t, tc.mergeHeight, a.UpgradeKeeper.GetDoneHeight(cache, MergeUpgrade))
			params := ledgertypes.DefaultParams()
			params.Paused = true
			require.NoError(t, a.LayerXLedgerKeeper.SetParams(cache, params))
			versions := prefix.NewStore(cache.KVStore(a.GetKey(upgradetypes.StoreKey)), []byte{upgradetypes.VersionMapByte})
			versions.Delete([]byte(ledgertypes.ModuleName))
			fromVM := a.UpgradeKeeper.GetModuleVersionMap(cache)
			require.NotContains(t, fromVM, ledgertypes.ModuleName)
			plan := upgradetypes.Plan{Name: AnchorStoreDeleteUpgrade, Height: cache.BlockHeight()}

			migrated, err := a.runAnchorStoreDeleteUpgrade(cache, plan, fromVM)
			require.ErrorContains(t, err, AnchorStoreDeleteUpgrade)
			require.ErrorContains(t, err, MergeUpgrade)
			require.Nil(t, migrated)
			require.PanicsWithError(t, err.Error(), func() {
				a.UpgradeKeeper.ApplyUpgrade(cache, plan)
			})
			require.Equal(t, params, a.LayerXLedgerKeeper.GetParams(cache))
			require.Equal(t, fromVM, a.UpgradeKeeper.GetModuleVersionMap(cache))
			require.Zero(t, a.UpgradeKeeper.GetDoneHeight(cache, AnchorStoreDeleteUpgrade))
		})
	}
}

func TestRepair_22_31_589_AnchorDeleteRunsAfterMerge(t *testing.T) {
	w := NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), false)
	a, ctx := w.App, w.Ctx.WithBlockHeight(100)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(99), MergeUpgrade)
	require.Equal(t, int64(99), a.UpgradeKeeper.GetDoneHeight(ctx, MergeUpgrade))
	params := ledgertypes.DefaultParams()
	params.Paused = true
	require.NoError(t, a.LayerXLedgerKeeper.SetParams(ctx, params))
	versions := prefix.NewStore(ctx.KVStore(a.GetKey(upgradetypes.StoreKey)), []byte{upgradetypes.VersionMapByte})
	versions.Delete([]byte(ledgertypes.ModuleName))
	fromVM := a.UpgradeKeeper.GetModuleVersionMap(ctx)
	require.NotContains(t, fromVM, ledgertypes.ModuleName)
	plan := upgradetypes.Plan{Name: AnchorStoreDeleteUpgrade, Height: ctx.BlockHeight()}

	cache, _ := ctx.CacheContext()
	migrated, err := a.runAnchorStoreDeleteUpgrade(cache, plan, fromVM)
	require.NoError(t, err)
	require.Equal(t, a.mm.GetVersionMap(), migrated)
	require.Equal(t, ledgertypes.DefaultGenesis(), a.LayerXLedgerKeeper.ExportGenesis(cache))

	require.NoError(t, a.UpgradeKeeper.ScheduleUpgrade(ctx, plan))
	a.UpgradeKeeper.ApplyUpgrade(ctx, plan)
	require.Equal(t, migrated, a.UpgradeKeeper.GetModuleVersionMap(ctx))
	require.Equal(t, ledgertypes.DefaultGenesis(), a.LayerXLedgerKeeper.ExportGenesis(ctx))
	require.Equal(t, ctx.BlockHeight(), a.UpgradeKeeper.GetDoneHeight(ctx, AnchorStoreDeleteUpgrade))
	require.Equal(t, int64(99), a.UpgradeKeeper.GetDoneHeight(ctx, MergeUpgrade))
}
