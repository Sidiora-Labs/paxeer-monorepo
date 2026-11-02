package composite

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/proto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/types"
)

var compositeMergeStoreNames = []string{
	keys.LayerXLedgerStoreKey,
	keys.LayerXAuthStoreKey,
	keys.LayerXPayStoreKey,
	keys.LayerXProgramsStoreKey,
	keys.SequencerStoreKey,
}

var compositeMergeWriteModes = []config.WriteMode{
	config.MemiavlOnly,
	config.MigrateEVM,
	config.EVMMigrated,
	config.MigrateAllButBank,
	config.AllMigratedButBank,
	config.MigrateBank,
	config.FlatKVOnly,
	config.TestOnlyDualWrite,
}

func newMergeTestComposite(t *testing.T, dir string, mode config.WriteMode) *CompositeCommitStore {
	t.Helper()
	cfg := config.DefaultStateCommitConfig()
	cfg.WriteMode = mode
	cs, err := NewCompositeCommitStore(t.Context(), dir, cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cs.Close()) })
	return cs
}

func TestCompositeInitializeMountsMergeStoresInMigrationModes(t *testing.T) {
	for _, mode := range compositeMergeWriteModes {
		t.Run(string(mode), func(t *testing.T) {
			cs := newMergeTestComposite(t, t.TempDir(), mode)
			for _, name := range compositeMergeStoreNames {
				require.Contains(t, keys.MemIAVLStoreKeys, name)
			}
			require.NoError(t, cs.Initialize(keys.MemIAVLStoreKeys))
			require.Equal(t, keys.MemIAVLStoreKeys, cs.initialStores)
			_, err := cs.LoadVersion(0, false)
			require.NoError(t, err)
			require.Empty(t, cs.PendingStores())
			if mode != config.FlatKVOnly {
				for _, name := range compositeMergeStoreNames {
					require.NotNil(t, cs.memIAVL.GetChildStoreByName(name), name)
				}
			}
		})
	}
}

func TestGetChildStoreByNameServesMergeStoresInMigrationModes(t *testing.T) {
	for _, mode := range compositeMergeWriteModes {
		t.Run(string(mode), func(t *testing.T) {
			cs := newMergeTestComposite(t, t.TempDir(), mode)
			require.NoError(t, cs.Initialize(keys.MemIAVLStoreKeys))
			_, err := cs.LoadVersion(0, false)
			require.NoError(t, err)
			for _, name := range compositeMergeStoreNames {
				t.Run(name, func(t *testing.T) {
					var child types.CommitKVStore
					require.NotPanics(t, func() { child = cs.GetChildStoreByName(name) })
					require.NotNil(t, child)
					require.False(t, cs.IsPendingStore(name))
					require.Nil(t, child.Get([]byte("missing")))
					require.False(t, child.Has([]byte("missing")))
					require.Equal(t, cs.Version(), child.Version())
				})
			}
		})
	}
}

func TestCompositeApplyUpgradesEndsPendingMergeStores(t *testing.T) {
	dir := t.TempDir()
	legacy, err := keys.AllModulesExcept(compositeMergeStoreNames...)
	require.NoError(t, err)
	require.True(t, t.Run("seed", func(t *testing.T) {
		cs := newMergeTestComposite(t, dir, config.MemiavlOnly)
		require.NoError(t, cs.Initialize(legacy))
		_, err := cs.LoadVersion(0, false)
		require.NoError(t, err)
		require.NoError(t, cs.ApplyChangeSets(bankWrite()))
		version, err := cs.Commit()
		require.NoError(t, err)
		require.Equal(t, int64(1), version)
	}))

	require.True(t, t.Run("upgrade", func(t *testing.T) {
		cs := newMergeTestComposite(t, dir, config.MemiavlOnly)
		require.NoError(t, cs.Initialize(keys.MemIAVLStoreKeys))
		_, err := cs.LoadVersion(0, false)
		require.NoError(t, err)
		require.Equal(t, int64(1), cs.Version())
		require.ElementsMatch(t, compositeMergeStoreNames, cs.PendingStores())

		var upgrades []*proto.TreeNameUpgrade
		var held, writes []*proto.NamedChangeSet
		for _, name := range compositeMergeStoreNames {
			require.True(t, cs.IsPendingStore(name), name)
			require.Nil(t, cs.memIAVL.GetChildStoreByName(name), name)
			require.Nil(t, storeInfo(cloneCommitInfo(cs.LastCommitInfo()), name), name)
			upgrades = append(upgrades, &proto.TreeNameUpgrade{Name: name})
			held = append(held, &proto.NamedChangeSet{
				Name: name,
				Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{
					{Key: []byte("before-upgrade"), Value: []byte(name)},
				}},
			})
			writes = append(writes, &proto.NamedChangeSet{
				Name: name,
				Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{
					{Key: []byte("created"), Value: []byte(name)},
				}},
			})
		}
		require.NoError(t, cs.ApplyChangeSets(held))
		for _, name := range compositeMergeStoreNames {
			require.Len(t, cs.HeldPendingWrites(name), 1, name)
			require.Nil(t, cs.GetChildStoreByName(name).Get([]byte("before-upgrade")), name)
		}

		require.NoError(t, cs.ApplyUpgrades(upgrades))
		require.Equal(t, int64(1), cs.Version())
		require.Empty(t, cs.PendingStores())
		for _, name := range compositeMergeStoreNames {
			require.False(t, cs.IsPendingStore(name), name)
			require.NotNil(t, cs.memIAVL.GetChildStoreByName(name), name)
			require.Empty(t, cs.HeldPendingWrites(name), name)
		}
		require.NoError(t, cs.ApplyChangeSets(writes))
		for _, name := range compositeMergeStoreNames {
			require.Empty(t, cs.HeldPendingWrites(name), name)
		}
		version, err := cs.Commit()
		require.NoError(t, err)
		require.Equal(t, int64(2), version)
		for _, name := range compositeMergeStoreNames {
			child := cs.GetChildStoreByName(name)
			require.NotNil(t, child)
			require.Equal(t, version, child.Version())
			require.Equal(t, []byte(name), child.Get([]byte("created")), name)
			require.Nil(t, child.Get([]byte("before-upgrade")), name)
			info := storeInfo(cloneCommitInfo(cs.LastCommitInfo()), name)
			require.NotNil(t, info, name)
			require.Equal(t, version, info.CommitId.Version, name)
			require.NotEmpty(t, info.CommitId.Hash, name)
		}
	}))

	cs := newMergeTestComposite(t, dir, config.MemiavlOnly)
	require.NoError(t, cs.Initialize(keys.MemIAVLStoreKeys))
	_, err = cs.LoadVersion(0, false)
	require.NoError(t, err)
	require.Equal(t, int64(2), cs.Version())
	require.Empty(t, cs.PendingStores())
	for _, name := range compositeMergeStoreNames {
		require.False(t, cs.IsPendingStore(name), name)
		value, ok, err := cs.Get(name, []byte("created"))
		require.NoError(t, err)
		require.True(t, ok, name)
		require.Equal(t, []byte(name), value, name)
		require.Equal(t, value, cs.GetChildStoreByName(name).Get([]byte("created")), name)
		require.Nil(t, cs.GetChildStoreByName(name).Get([]byte("before-upgrade")), name)
	}
}
