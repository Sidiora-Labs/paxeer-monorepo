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

// Unlike the fresh-initialization cases above, this starts from committed
// legacy state without any merge trees and reopens with the enlarged mount list.
func runM1197LegacyMergeUpgrade(t *testing.T, mode config.WriteMode) {
	t.Helper()
	legacy, err := keys.AllModulesExcept(compositeMergeStoreNames...)
	require.NoError(t, err)
	dir := t.TempDir()
	baseline := seedM1197Predecessor(t, dir, mode, legacy)
	cs := openM1197Composite(t, dir, mode, keys.MemIAVLStoreKeys)
	require.Equal(t, baseline, cloneCommitInfo(cs.LastCommitInfo()))
	requireM1197Seed(t, cs)
	checkpoint := cs.Version()

	// The pending empty-view contract is currently memiavl-only. Mixed
	// modes that still route these modules to memiavl require ApplyUpgrades
	// before access; assert that unsupported pre-upgrade access fails loudly.
	rejectAbsent := mode == config.MigrateEVM || mode == config.EVMMigrated ||
		mode == config.MigrateAllButBank || mode == config.TestOnlyDualWrite
	flatStores := mode == config.AllMigratedButBank || mode == config.MigrateBank || mode == config.FlatKVOnly
	if mode == config.MemiavlOnly {
		require.ElementsMatch(t, compositeMergeStoreNames, cs.PendingStores())
	} else {
		require.Empty(t, cs.PendingStores())
	}
	var upgrades []*proto.TreeNameUpgrade
	var before, after []*proto.NamedChangeSet
	for _, name := range compositeMergeStoreNames {
		require.Equal(t, mode == config.MemiavlOnly, cs.IsPendingStore(name), name)
		if cs.memIAVL != nil {
			require.Nil(t, cs.memIAVL.GetChildStoreByName(name), name)
		}
		require.Nil(t, storeInfo(cloneCommitInfo(cs.LastCommitInfo()), name), name)
		iter, err := cs.Iterator(name, nil, nil, true)
		require.NoError(t, err)
		require.False(t, iter.Valid(), name)
		require.NoError(t, iter.Error())
		require.NoError(t, iter.Close())
		value, found, err := cs.Get(name, []byte("before-upgrade"))
		require.Nil(t, value)
		require.False(t, found)
		if rejectAbsent {
			require.ErrorContains(t, err, "store not found: "+name)
			require.Panics(t, func() { cs.GetChildStoreByName(name).Get([]byte("before-upgrade")) })
			require.Panics(t, func() { cs.GetChildStoreByName(name).Set([]byte("rejected"), []byte(name)) })
		} else {
			require.NoError(t, err)
			require.Nil(t, cs.GetChildStoreByName(name).Get([]byte("before-upgrade")))
		}
		upgrades = append(upgrades, &proto.TreeNameUpgrade{Name: name})
		before = append(before, &proto.NamedChangeSet{Name: name, Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{
			{Key: []byte("before-upgrade"), Value: []byte(name)},
		}}})
		after = append(after, &proto.NamedChangeSet{Name: name, Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{
			{Key: []byte("created"), Value: []byte(name)},
		}}})
	}
	if rejectAbsent {
		require.Error(t, cs.ApplyChangeSets(before), "an absent memiavl tree must reject the batch")
		// A failed batch can advance an in-memory migration iterator. Reopen
		// the committed baseline before continuing the supported upgrade path.
		require.NoError(t, cs.Close())
		cs = openM1197Composite(t, dir, mode, keys.MemIAVLStoreKeys)
		require.Equal(t, baseline, cloneCommitInfo(cs.LastCommitInfo()))
		requireM1197Seed(t, cs)
	} else {
		require.NoError(t, cs.ApplyChangeSets(before))
		for _, name := range compositeMergeStoreNames {
			if mode == config.MemiavlOnly {
				require.Len(t, cs.HeldPendingWrites(name), 1, name)
				require.Nil(t, cs.GetChildStoreByName(name).Get([]byte("before-upgrade")))
			} else {
				require.Equal(t, []byte(name), cs.GetChildStoreByName(name).Get([]byte("before-upgrade")))
			}
		}
	}

	require.NoError(t, cs.ApplyUpgrades(upgrades))
	require.Equal(t, checkpoint, cs.Version())
	require.Empty(t, cs.PendingStores())
	for _, name := range compositeMergeStoreNames {
		require.False(t, cs.IsPendingStore(name), name)
		require.Empty(t, cs.HeldPendingWrites(name), name)
		if cs.memIAVL != nil {
			require.NotNil(t, cs.memIAVL.GetChildStoreByName(name), name)
		}
	}
	require.NoError(t, cs.ApplyChangeSets(after))
	version, err := cs.Commit()
	require.NoError(t, err)
	require.Equal(t, checkpoint+1, version)
	committed := cloneCommitInfo(cs.LastCommitInfo())
	require.NotEmpty(t, mergeStateSyncCommitHash(committed))
	check := func(cs *CompositeCommitStore) {
		t.Helper()
		require.Equal(t, committed, cloneCommitInfo(cs.LastCommitInfo()))
		requireM1197Seed(t, cs)
		require.Empty(t, cs.PendingStores())
		for _, name := range compositeMergeStoreNames {
			value, found, err := cs.Get(name, []byte("created"))
			require.NoError(t, err)
			require.True(t, found, name)
			require.Equal(t, []byte(name), value, name)
			child := cs.GetChildStoreByName(name)
			require.Equal(t, version, child.Version())
			require.Equal(t, []byte(name), child.Get([]byte("created")))
			require.Nil(t, child.Get([]byte("rejected")))
			if flatStores {
				require.Equal(t, []byte(name), child.Get([]byte("before-upgrade")))
				physical, ok := cs.flatKV.Get(name, []byte("created"))
				require.True(t, ok)
				require.Equal(t, []byte(name), physical)
			} else {
				require.Nil(t, child.Get([]byte("before-upgrade")))
				if mode == config.MigrateAllButBank {
					physical, ok := cs.flatKV.Get(name, []byte("created"))
					require.True(t, ok)
					require.Equal(t, []byte(name), physical)
				} else {
					require.Equal(t, []byte(name), cs.memIAVL.GetChildStoreByName(name).Get([]byte("created")))
				}
			}
			if cs.memIAVL != nil {
				info := storeInfo(committed, name)
				require.NotNil(t, info, name)
				require.Equal(t, version, info.CommitId.Version)
				require.NotEmpty(t, info.CommitId.Hash)
			}
		}
	}
	check(cs)
	require.NoError(t, cs.Close())
	cs = openM1197Composite(t, dir, mode, keys.MemIAVLStoreKeys)
	check(cs)
}
