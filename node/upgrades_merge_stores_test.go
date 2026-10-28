package app

import (
	"testing"
	"time"

	agenttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	ledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/stretchr/testify/require"
)

func TestMergeStoreUpgradesAddsLayerXStores(t *testing.T) {
	want := []string{keys.SequencerStoreKey, keys.LayerXLedgerStoreKey, keys.LayerXAuthStoreKey,
		keys.LayerXPayStoreKey, keys.LayerXProgramsStoreKey, keys.LayerXAgentStoreKey, keys.LayerXHostStoreKey}
	upgrades := mergeStoreUpgrades()
	require.Equal(t, want, upgrades.Added)
	require.Empty(t, upgrades.Deleted)
	require.Empty(t, upgrades.Renamed)
	registered, ok := layerxStoreUpgrades(MergeUpgrade)
	require.True(t, ok)
	require.Equal(t, upgrades, registered)

	w := NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), true)
	a, ctx := w.App, w.Ctx.WithBlockHeight(mergeHeight)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	rewindBeforeMerge(t, a, ctx)
	for _, name := range want {
		key := a.GetKey(name)
		require.NotNil(t, key, name)
		require.NotNil(t, a.CommitMultiStore().GetCommitKVStore(key), name)
		store := ctx.KVStore(key)
		iter := store.Iterator(nil, nil)
		var stale [][]byte
		for ; iter.Valid(); iter.Next() {
			stale = append(stale, iter.Key())
		}
		require.NoError(t, iter.Close())
		for _, key := range stale {
			store.Delete(key)
		}
	}
	require.Empty(t, a.unmountedPlanStores(MergeUpgrade))
	versions := a.UpgradeKeeper.GetModuleVersionMap(ctx)
	mergedModules := []string{ledgertypes.ModuleName, authtypes.ModuleName, paytypes.ModuleName, agenttypes.ModuleName}
	for _, name := range mergedModules {
		delete(versions, name)
	}
	plan := upgradetypes.Plan{Name: MergeUpgrade, Height: mergeHeight, Info: activationInfo(t)}
	require.NoError(t, a.UpgradeKeeper.ScheduleUpgrade(ctx, plan))
	returned, err := a.continueMergeUpgradePlan(ctx, plan, versions)
	require.NoError(t, err)
	for _, name := range append(mergedModules, seqtypes.ModuleName) {
		require.Equal(t, a.mm.Modules[name].ConsensusVersion(), returned[name], name)
		require.NotContains(t, versions, name)
	}
	require.Equal(t, ledgertypes.DefaultGenesis(), a.LayerXLedgerKeeper.ExportGenesis(ctx))
	require.Equal(t, authtypes.DefaultGenesis(), a.LayerXAuthKeeper.ExportGenesis(ctx))
	require.Equal(t, paytypes.DefaultGenesis(), a.LayerXPayKeeper.ExportGenesis(ctx))
	require.Equal(t, agenttypes.DefaultGenesis(), a.LayerXAgentKeeper.ExportGenesis(ctx))
	state, err := a.SequencerKeeper.ExportGenesis(ctx)
	require.NoError(t, err)
	require.Equal(t, [][32]byte{mergePrimary, mergeStandby}, state.Leaders)
	require.Zero(t, state.LastSeq)
	require.True(t, a.Sequenced(ctx, mergeHeight+1))
}
