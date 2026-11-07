package composite

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/proto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/migration"
	"github.com/stretchr/testify/require"
)

func openM1197Composite(t *testing.T, dir string, mode config.WriteMode, mounted []string) *CompositeCommitStore {
	t.Helper()
	cfg := config.DefaultStateCommitConfig()
	cfg.WriteMode = mode
	cfg.KeysToMigratePerBlock = 1
	cfg.MemIAVLConfig.AsyncCommitBuffer = 0
	cfg.MemIAVLConfig.SnapshotInterval = 100
	cfg.MemIAVLConfig.SnapshotKeepRecent = 100
	cfg.FlatKVConfig.SnapshotInterval = 100
	cfg.FlatKVConfig.SnapshotKeepRecent = 100
	cs, err := NewCompositeCommitStore(t.Context(), dir, cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cs.Close()) })
	require.NoError(t, cs.Initialize(mounted))
	_, err = cs.LoadVersion(0, false)
	require.NoError(t, err)
	return cs
}

func m1197SeedChanges() []*proto.NamedChangeSet {
	var changes []*proto.NamedChangeSet
	for _, name := range []string{keys.AuthStoreKey, keys.BankStoreKey, keys.EVMStoreKey} {
		ncs := &proto.NamedChangeSet{Name: name}
		for i := byte(1); i <= 4; i++ {
			key, value := []byte{i}, []byte{name[0], i}
			if name == keys.EVMStoreKey {
				key, value = evmStorageTestKey(i), evmStorageTestValue(i)
			}
			ncs.Changeset.Pairs = append(ncs.Changeset.Pairs, &proto.KVPair{Key: key, Value: value})
		}
		changes = append(changes, ncs)
	}
	return changes
}

// Produce each predecessor schema through real commits and completed migrations.
// No migration metadata or backend files are synthesized by the fixture.
func seedM1197Predecessor(t *testing.T, dir string, target config.WriteMode, mounted []string) proto.CommitInfo {
	t.Helper()
	seedMode := config.MemiavlOnly
	if target == config.TestOnlyDualWrite {
		seedMode = target
	}
	cs := openM1197Composite(t, dir, seedMode, mounted)
	require.NoError(t, cs.ApplyChangeSets(m1197SeedChanges()))
	v, err := cs.Commit()
	require.NoError(t, err)
	require.Equal(t, int64(1), v)
	info := cloneCommitInfo(cs.LastCommitInfo())
	require.NoError(t, cs.Close())

	completed := 0
	switch target {
	case config.MemiavlOnly, config.MigrateEVM, config.TestOnlyDualWrite:
	case config.EVMMigrated, config.MigrateAllButBank:
		completed = 1
	case config.AllMigratedButBank, config.MigrateBank:
		completed = 2
	case config.FlatKVOnly:
		completed = 3
	default:
		t.Fatalf("fixture does not support mode %q", target)
	}
	stages := []config.WriteMode{config.MigrateEVM, config.MigrateAllButBank, config.MigrateBank}
	for i := 0; i < completed; i++ {
		cs = openM1197Composite(t, dir, stages[i], mounted)
		done := false
		for block := 0; block < 32; block++ {
			require.NoError(t, cs.ApplyChangeSets(nil))
			_, err = cs.Commit()
			require.NoError(t, err)
			done, err = migration.IsAtVersion(flatKVReaderFor(cs), uint64(i+1))
			require.NoError(t, err)
			if done {
				break
			}
		}
		require.True(t, done, "fixture migration %s must finish within 32 blocks", stages[i])
		info = cloneCommitInfo(cs.LastCommitInfo())
		require.NoError(t, cs.Close())
	}
	if target == config.FlatKVOnly {
		// Finish the schema switch before capturing this fixture's baseline:
		// FlatKVOnly no longer contributes the empty memiavl tree roots.
		previousLattice := storeInfo(info, "evm_lattice")
		require.NotNil(t, previousLattice)
		cs = openM1197Composite(t, dir, target, mounted)
		require.Nil(t, cs.memIAVL)
		requireM1197Seed(t, cs)
		require.Equal(t, info.Version, cs.Version())
		require.Equal(t, previousLattice.CommitId.Hash, cs.flatKV.CommittedRootHash())
		require.NoError(t, cs.ApplyChangeSets(nil))
		v, err := cs.Commit()
		require.NoError(t, err)
		require.Equal(t, info.Version+1, v)
		info = cloneCommitInfo(cs.LastCommitInfo())
		require.Len(t, info.StoreInfos, 1)
		require.Equal(t, "evm_lattice", info.StoreInfos[0].Name)
		require.Equal(t, previousLattice.CommitId.Hash, info.StoreInfos[0].CommitId.Hash)
		require.NoError(t, cs.Close())
	}
	return info
}

func requireM1197Seed(t *testing.T, cs *CompositeCommitStore) {
	t.Helper()
	for _, change := range m1197SeedChanges() {
		for _, pair := range change.Changeset.Pairs {
			value, found, err := cs.Get(change.Name, pair.Key)
			require.NoError(t, err)
			require.True(t, found, change.Name)
			require.Equal(t, pair.Value, value, change.Name)
			require.Equal(t, pair.Value, cs.GetChildStoreByName(change.Name).Get(pair.Key))
		}
	}
}

func requireM1197SameState(t *testing.T, want, got *CompositeCommitStore) {
	t.Helper()
	require.Equal(t, cloneCommitInfo(want.LastCommitInfo()), cloneCommitInfo(got.LastCommitInfo()))
	require.Equal(t, cloneCommitInfo(want.WorkingCommitInfo()), cloneCommitInfo(got.WorkingCommitInfo()))
	require.Equal(t, mergeStateSyncCommitHash(*want.LastCommitInfo()), mergeStateSyncCommitHash(*got.LastCommitInfo()))
	require.Equal(t, mergeStateSyncCommitHash(*want.WorkingCommitInfo()), mergeStateSyncCommitHash(*got.WorkingCommitInfo()))
	require.Equal(t, want.PendingStores(), got.PendingStores())
	for _, change := range m1197SeedChanges() {
		require.Equal(t, collectIterator(t, want, change.Name, true), collectIterator(t, got, change.Name, true))
		for _, pair := range change.Changeset.Pairs {
			wantValue, wantFound, err := want.Get(change.Name, pair.Key)
			require.NoError(t, err)
			value, found, err := got.Get(change.Name, pair.Key)
			require.NoError(t, err)
			require.Equal(t, wantFound, found)
			require.Equal(t, wantValue, value)
			memWant, memWantFound := memiavlGetForTest(want, change.Name, pair.Key)
			memGot, memGotFound := memiavlGetForTest(got, change.Name, pair.Key)
			require.Equal(t, memWantFound, memGotFound)
			require.Equal(t, memWant, memGot)
			flatWant, flatWantFound := flatKVGetForTest(want, change.Name, pair.Key)
			flatGot, flatGotFound := flatKVGetForTest(got, change.Name, pair.Key)
			require.Equal(t, flatWantFound, flatGotFound)
			require.Equal(t, flatWant, flatGot)
		}
	}
}

func TestRepairM_1197_SteadyStateRollback(t *testing.T) {
	for _, mode := range []config.WriteMode{config.MemiavlOnly, config.EVMMigrated, config.AllMigratedButBank, config.FlatKVOnly, config.TestOnlyDualWrite} {
		t.Run(string(mode), func(t *testing.T) { runM1197Rollback(t, mode) })
	}
}

func TestRepairM_1197_FirstMigrationRollback(t *testing.T) {
	for _, mode := range []config.WriteMode{config.MigrateEVM, config.MigrateAllButBank, config.MigrateBank} {
		t.Run(string(mode), func(t *testing.T) { runM1197Rollback(t, mode) })
	}
}

func TestRepairM_1197_PendingCreationRollback(t *testing.T) {
	t.Run("same_handle", func(t *testing.T) { runM1197PendingCreationRollback(t, false) })
	t.Run("reopen", func(t *testing.T) { runM1197PendingCreationRollback(t, true) })
}

func TestRepairM_1197_LegacyMergeUpgradeModes(t *testing.T) {
	for _, mode := range compositeMergeWriteModes {
		t.Run(string(mode), func(t *testing.T) { runM1197LegacyMergeUpgrade(t, mode) })
	}
}

func runM1197PendingCreationRollback(t *testing.T, reopen bool) {
	t.Helper()
	dir, cleanDir := t.TempDir(), t.TempDir()
	mounted := []string{keys.BankStoreKey, keys.AuthStoreKey, keys.EVMStoreKey, keys.LayerXLedgerStoreKey}
	legacy := mounted[:3]
	baseline := seedM1197Predecessor(t, dir, config.MemiavlOnly, legacy)
	require.Equal(t, baseline, seedM1197Predecessor(t, cleanDir, config.MemiavlOnly, legacy))
	cs := openM1197Composite(t, dir, config.MemiavlOnly, mounted)
	clean := openM1197Composite(t, cleanDir, config.MemiavlOnly, mounted)
	name := keys.LayerXLedgerStoreKey
	upgrades := []*proto.TreeNameUpgrade{{Name: name}}
	writes := []*proto.NamedChangeSet{{Name: name, Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{
		{Key: []byte("created"), Value: []byte("ledger")},
	}}}}
	require.ElementsMatch(t, []string{name}, cs.PendingStores())
	require.NoError(t, cs.ApplyUpgrades(upgrades))
	require.NoError(t, cs.ApplyChangeSets(writes))
	_, err := cs.Commit()
	require.NoError(t, err)
	require.False(t, cs.IsPendingStore(name))
	require.NoError(t, cs.Rollback(1))
	requireM1197SameState(t, clean, cs)
	requireM1197Seed(t, cs)
	require.True(t, cs.IsPendingStore(name))
	require.Nil(t, cs.GetChildStoreByName(name).Get([]byte("created")))
	require.NoError(t, cs.ApplyChangeSets(writes))
	require.Len(t, cs.HeldPendingWrites(name), 1)
	if reopen {
		require.NoError(t, cs.Close())
		cs = openM1197Composite(t, dir, config.MemiavlOnly, mounted)
	} else {
		require.NoError(t, cs.Rollback(1))
	}
	requireM1197SameState(t, clean, cs)
	require.Empty(t, cs.HeldPendingWrites(name))
	for _, handle := range []*CompositeCommitStore{cs, clean} {
		require.NoError(t, handle.ApplyUpgrades(upgrades))
		require.NoError(t, handle.ApplyChangeSets(writes))
	}
	require.Equal(t, cloneCommitInfo(clean.WorkingCommitInfo()), cloneCommitInfo(cs.WorkingCommitInfo()))
	workingHash := mergeStateSyncCommitHash(*clean.WorkingCommitInfo())
	for _, handle := range []*CompositeCommitStore{cs, clean} {
		v, err := handle.Commit()
		require.NoError(t, err)
		require.Equal(t, int64(2), v)
		require.Equal(t, workingHash, mergeStateSyncCommitHash(*handle.LastCommitInfo()))
		require.NoError(t, handle.Close())
	}
	cs = openM1197Composite(t, dir, config.MemiavlOnly, mounted)
	clean = openM1197Composite(t, cleanDir, config.MemiavlOnly, mounted)
	requireM1197SameState(t, clean, cs)
	require.False(t, cs.IsPendingStore(name))
	require.Equal(t, []byte("ledger"), cs.GetChildStoreByName(name).Get([]byte("created")))
}
