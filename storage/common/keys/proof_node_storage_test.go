// Proves: keys.MemIAVLStoreKeys and LayerX*StoreKey -> CompositeCommitStore.ApplyChangeSets and migration.BuildRouter -> CompositeCommitStore.GetChildStoreByName/Get.
//
// The replay mount list crosses SortedMemIAVLStoreKeys; the legacy mount list
// crosses AllModulesExcept. Real tree upgrades, memiavl WALs, FlatKV and the
// production migration routers carry eight merge stores across full closes and
// reopens, including an in-flight migration. No storage layer is substituted.
//
// Repairs storage/common/keys/store_keys_test.go: the exact inventory in
// TestXWebStoreKeyFollowsTheV6120StoreKeys omitted LayerXChainsStoreKey even
// though MemIAVLStoreKeys mounts it. The golden list is extended, and every
// merge store must be mounted exactly once. LayerXHostStoreKey remains lxphost,
// the immutable name used by modules/lxphost, rather than the planned layerxhost.
package keys_test

import (
	"slices"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/proto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/composite"
	flatkvconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/flatkv/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/memiavl"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/migration"
	"github.com/stretchr/testify/require"
)

var proofNodeStorageModules = []struct {
	name string
	want string
}{
	{keys.LayerXLedgerStoreKey, "layerxledger"},
	{keys.LayerXAuthStoreKey, "layerxauth"},
	{keys.LayerXPayStoreKey, "layerxpay"},
	{keys.LayerXChainsStoreKey, "layerxchains"},
	{keys.LayerXProgramsStoreKey, "layerxprograms"},
	{keys.LayerXAgentStoreKey, "layerxagent"},
	{keys.LayerXHostStoreKey, "lxphost"},
	{keys.SequencerStoreKey, "sequencer"},
}

func proofNodeStorageOpen(t *testing.T, dir string, cfg config.StateCommitConfig, mounts []string) *composite.CompositeCommitStore {
	t.Helper()
	cs, err := composite.NewCompositeCommitStore(t.Context(), dir, cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cs.Close()) })
	require.NoError(t, cs.Initialize(mounts))
	_, err = cs.LoadVersion(0, false)
	require.NoError(t, err)
	return cs
}

func proofNodeStorageChanges() []*proto.NamedChangeSet {
	changes := make([]*proto.NamedChangeSet, 0, len(proofNodeStorageModules))
	for _, module := range proofNodeStorageModules {
		changes = append(changes, &proto.NamedChangeSet{
			Name: module.name,
			Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{{
				Key: []byte("node-storage"), Value: []byte("committed/" + module.name),
			}}},
		})
	}
	return changes
}

func proofNodeStorageObserve(t *testing.T, cs *composite.CompositeCommitStore) {
	t.Helper()
	for _, module := range proofNodeStorageModules {
		child := cs.GetChildStoreByName(module.name)
		require.NotNil(t, child, module.name)
		require.Equal(t, []byte("committed/"+module.want), child.Get([]byte("node-storage")), module.name)
		value, found, err := cs.Get(module.name, []byte("node-storage"))
		require.NoError(t, err)
		require.True(t, found, module.name)
		require.Equal(t, child.Get([]byte("node-storage")), value, module.name)
		require.True(t, child.Has([]byte("node-storage")), module.name)
		require.Nil(t, child.Get([]byte("absent")), module.name)

		// Repeating the producer must leave exactly one logical record.
		it := child.Iterator(nil, nil, true)
		require.True(t, it.Valid(), module.name)
		require.Equal(t, []byte("node-storage"), it.Key(), module.name)
		require.Equal(t, value, it.Value(), module.name)
		it.Next()
		require.False(t, it.Valid(), module.name)
		require.NoError(t, it.Error())
		require.NoError(t, it.Close())
	}
	bank, found, err := cs.Get(keys.BankStoreKey, []byte("legacy"))
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, []byte("bank-state"), bank)
}

func TestProofNodeStorage(t *testing.T) {
	mounts := keys.SortedMemIAVLStoreKeys()
	require.True(t, slices.IsSorted(mounts))
	require.ElementsMatch(t, keys.MemIAVLStoreKeys, mounts)
	canonical := slices.Clone(keys.MemIAVLStoreKeys)
	mounts[0] = "caller-owned-copy"
	require.Equal(t, canonical, keys.MemIAVLStoreKeys)
	mounts = keys.SortedMemIAVLStoreKeys()
	mergeNames := make([]string, 0, len(proofNodeStorageModules))
	for _, module := range proofNodeStorageModules {
		require.Equal(t, module.want, module.name)
		require.True(t, keys.IsMemIAVLStoreKey(module.name))
		mounted := 0
		for _, name := range mounts {
			if name == module.name {
				mounted++
			}
		}
		require.Equal(t, 1, mounted, module.name)
		mergeNames = append(mergeNames, module.name)
	}
	legacy, err := keys.AllModulesExcept(mergeNames...)
	require.NoError(t, err)
	require.Len(t, legacy, len(mounts)-len(mergeNames))
	require.Contains(t, legacy, keys.LayerXAnchorStoreKey)

	dir := t.TempDir()
	cfg := config.DefaultStateCommitConfig()
	cfg.MemIAVLConfig.AsyncCommitBuffer = 0
	cfg.FlatKVConfig = *flatkvconfig.DefaultTestConfig(t)
	cfg.KeysToMigratePerBlock = 1
	cs := proofNodeStorageOpen(t, dir, cfg, legacy)
	require.NoError(t, cs.ApplyChangeSets([]*proto.NamedChangeSet{{
		Name:      keys.BankStoreKey,
		Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{{Key: []byte("legacy"), Value: []byte("bank-state")}}},
	}}))
	_, err = cs.Commit()
	require.NoError(t, err)
	require.NoError(t, cs.Close())

	cs = proofNodeStorageOpen(t, dir, cfg, mounts)
	require.ElementsMatch(t, mergeNames, cs.PendingStores())
	upgrades := make([]*proto.TreeNameUpgrade, 0, len(mergeNames))
	for _, name := range mergeNames {
		upgrades = append(upgrades, &proto.TreeNameUpgrade{Name: name})
	}
	require.NoError(t, cs.ApplyUpgrades(upgrades))
	require.Empty(t, cs.PendingStores())
	require.NoError(t, cs.ApplyChangeSets(proofNodeStorageChanges()))
	version, err := cs.Commit()
	require.NoError(t, err)
	require.NoError(t, cs.Close())

	cs = proofNodeStorageOpen(t, dir, cfg, mounts)
	require.Equal(t, version, cs.Version())
	proofNodeStorageObserve(t, cs)
	// Retry the actual producer after recovery, then persist and recover again.
	require.NoError(t, cs.ApplyChangeSets(proofNodeStorageChanges()))
	_, err = cs.Commit()
	require.NoError(t, err)
	proofNodeStorageObserve(t, cs)
	require.NoError(t, cs.Close())

	for _, stage := range []struct {
		transition config.WriteMode
		steady     config.WriteMode
	}{
		{config.MigrateEVM, config.EVMMigrated},
		{config.MigrateAllButBank, config.AllMigratedButBank},
		{config.MigrateBank, config.FlatKVOnly},
	} {
		require.True(t, t.Run(string(stage.transition), func(t *testing.T) {
			cfg.WriteMode = stage.transition
			cs = proofNodeStorageOpen(t, dir, cfg, mounts)
			proofNodeStorageObserve(t, cs)
			require.NoError(t, cs.ApplyChangeSets(nil))
			_, err := cs.Commit()
			require.NoError(t, err)
			require.NoError(t, cs.Close())

			if stage.transition == config.MigrateAllButBank {
				// Prove this restart interrupts migration, rather than restarting
				// an already drained source. Inspect the real closed source DB.
				source := memiavl.NewCommitStore(dir, cfg.MemIAVLConfig)
				require.NoError(t, source.Initialize(mounts))
				_, err = source.LoadVersion(0, false)
				require.NoError(t, err)
				remaining := 0
				for _, name := range mergeNames {
					if source.GetChildStoreByName(name).Has([]byte("node-storage")) {
						remaining++
					}
				}
				require.NoError(t, source.Close())
				require.Greater(t, remaining, 0)
				require.Less(t, remaining, len(mergeNames))
			}

			cs = proofNodeStorageOpen(t, dir, cfg, mounts)
			proofNodeStorageObserve(t, cs)
			// One key per block, plus one block to record completion.
			for range len(mergeNames) + 1 {
				require.NoError(t, cs.ApplyChangeSets(nil))
				_, err = cs.Commit()
				require.NoError(t, err)
			}
			proofNodeStorageObserve(t, cs)
			require.NoError(t, cs.Close())
			cfg.WriteMode = stage.steady
			cs = proofNodeStorageOpen(t, dir, cfg, mounts)
			proofNodeStorageObserve(t, cs)
			require.NoError(t, cs.Close())
		}))
	}
}

func TestProofNodeStorageRefusesUnknownMount(t *testing.T) {
	for _, unknown := range []string{"", "layerxhost", "unknown-node-storage"} {
		t.Run(unknown, func(t *testing.T) {
			require.False(t, keys.IsMemIAVLStoreKey(unknown))
			names, err := keys.AllModulesExcept(keys.LayerXAuthStoreKey, unknown)
			require.Error(t, err)
			require.Nil(t, names)
			cfg := config.DefaultStateCommitConfig()
			cfg.WriteMode = config.EVMMigrated
			cfg.FlatKVConfig = *flatkvconfig.DefaultTestConfig(t)
			cs, err := composite.NewCompositeCommitStore(t.Context(), t.TempDir(), cfg)
			require.NoError(t, err)
			defer func() { require.NoError(t, cs.Close()) }()
			require.ErrorContains(t, cs.Initialize([]string{keys.LayerXAuthStoreKey, unknown}), "not routable")
			require.ErrorContains(t, cs.Initialize([]string{migration.MigrationStore}), "reserved")
			require.NoError(t, cs.Initialize(keys.SortedMemIAVLStoreKeys()))
			_, err = cs.LoadVersion(0, false)
			require.NoError(t, err)
			value, found, err := cs.Get(unknown, []byte("node-storage"))
			require.Error(t, err)
			require.False(t, found)
			require.Nil(t, value)
			require.Panics(t, func() { cs.GetChildStoreByName(unknown) })
		})
	}
}
