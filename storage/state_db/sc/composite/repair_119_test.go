package composite

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/proto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/migration"
	"github.com/stretchr/testify/require"
)

func TestRepair_119_MigrationBoundaryRollback(t *testing.T) {
	repair119MigrationRecovery(t, true, true)
}

func TestRepair_119_MigrationBoundaryReload(t *testing.T) {
	t.Run("committed", func(t *testing.T) { repair119MigrationRecovery(t, false, true) })
	t.Run("uncommitted", func(t *testing.T) { repair119MigrationRecovery(t, false, false) })
}

func repair119Open(t *testing.T, dir string, mode config.WriteMode, mounted []string, version int64) *CompositeCommitStore {
	t.Helper()
	cfg := config.DefaultStateCommitConfig()
	cfg.WriteMode = mode
	cfg.MemIAVLConfig.AsyncCommitBuffer = 0
	cfg.KeysToMigratePerBlock = 1
	cs, err := NewCompositeCommitStore(t.Context(), dir, cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cs.Close()) })
	require.NoError(t, cs.Initialize(mounted))
	_, err = cs.LoadVersion(version, false)
	require.NoError(t, err)
	return cs
}

func repair119Recover(t *testing.T, cs *CompositeCommitStore, rollback bool) {
	t.Helper()
	if rollback {
		require.NoError(t, cs.Rollback(1))
	} else {
		loaded, err := cs.LoadVersion(1, false)
		require.NoError(t, err)
		require.Same(t, cs, loaded)
	}
	require.Equal(t, int64(1), cs.Version())
}

func repair119MigrationRecovery(t *testing.T, rollback, committed bool) {
	t.Helper()
	mounted := []string{keys.BankStoreKey, keys.EVMStoreKey}
	seed := []*proto.NamedChangeSet{
		{Name: keys.BankStoreKey, Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{
			{Key: []byte("balance"), Value: []byte("100")},
		}}},
		{Name: keys.EVMStoreKey, Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{
			{Key: evmStorageTestKey(1), Value: evmStorageTestValue(0x11)},
			{Key: evmStorageTestKey(2), Value: evmStorageTestValue(0x22)},
			{Key: evmStorageTestKey(3), Value: evmStorageTestValue(0x33)},
		}}},
	}
	dir, freshDir := t.TempDir(), t.TempDir()
	var baseline proto.CommitInfo
	for _, home := range []string{dir, freshDir} {
		cs := repair119Open(t, home, config.MemiavlOnly, mounted, 0)
		require.NoError(t, cs.ApplyChangeSets(seed))
		v, err := cs.Commit()
		require.NoError(t, err)
		require.Equal(t, int64(1), v)
		baseline = cloneCommitInfo(cs.LastCommitInfo())
		require.Nil(t, storeInfo(baseline, "evm_lattice"))
		require.NoError(t, cs.Close())
	}

	cs := repair119Open(t, dir, config.MigrateEVM, mounted, 0)
	fresh := repair119Open(t, freshDir, config.MigrateEVM, mounted, 0)
	require.Equal(t, baseline, cloneCommitInfo(cs.LastCommitInfo()))
	workingBaseline := cloneCommitInfo(fresh.WorkingCommitInfo())
	require.NoError(t, cs.ApplyChangeSets(nil))
	if committed {
		v, err := cs.Commit()
		require.NoError(t, err)
		require.Equal(t, int64(2), v)
		require.NotNil(t, storeInfo(cloneCommitInfo(cs.LastCommitInfo()), "evm_lattice"))
		// Leave a second migration batch and caller writes uncommitted.
		require.NoError(t, cs.ApplyChangeSets([]*proto.NamedChangeSet{
			{Name: keys.BankStoreKey, Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{
				{Key: []byte("abandoned"), Value: []byte("discard")},
			}}},
		}))
	}
	require.NotNil(t, storeInfo(cloneCommitInfo(cs.WorkingCommitInfo()), "evm_lattice"))
	require.True(t, cs.latticeAppendLatched.Load())
	require.True(t, cs.migrationAdvancedThisCommit)

	repair119Recover(t, cs, rollback)
	require.False(t, cs.latticeAppendLatched.Load())
	require.False(t, cs.migrationAdvancedThisCommit)
	require.Equal(t, baseline, cloneCommitInfo(cs.LastCommitInfo()))
	require.Equal(t, mergeStateSyncCommitHash(baseline), mergeStateSyncCommitHash(*cs.LastCommitInfo()))
	require.Equal(t, workingBaseline, cloneCommitInfo(cs.WorkingCommitInfo()))
	require.Equal(t, mergeStateSyncCommitHash(workingBaseline), mergeStateSyncCommitHash(*cs.WorkingCommitInfo()))
	for _, handle := range []*CompositeCommitStore{cs, fresh} {
		require.Equal(t, []byte("100"), handle.GetChildStoreByName(keys.BankStoreKey).Get([]byte("balance")))
		require.Nil(t, handle.GetChildStoreByName(keys.BankStoreKey).Get([]byte("abandoned")))
		for i := byte(1); i <= 3; i++ {
			key := evmStorageTestKey(i)
			require.Equal(t, evmStorageTestValue(i*0x11), handle.GetChildStoreByName(keys.EVMStoreKey).Get(key))
			_, found := handle.flatKV.Get(keys.EVMStoreKey, key)
			require.False(t, found, "rewound EVM keys must route to memiavl")
		}
	}

	// Exercise both sides of the reset boundary: the first key migrates,
	// while a write to the third key must still reach memiavl.
	writes := []*proto.NamedChangeSet{
		{Name: keys.BankStoreKey, Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{
			{Key: []byte("balance"), Value: []byte("200")},
		}}},
		{Name: keys.EVMStoreKey, Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{
			{Key: evmStorageTestKey(1), Value: evmStorageTestValue(0x44)},
			{Key: evmStorageTestKey(3), Value: evmStorageTestValue(0x55)},
		}}},
	}
	for _, handle := range []*CompositeCommitStore{cs, fresh} {
		require.NoError(t, handle.ApplyChangeSets(writes))
		require.Equal(t, evmStorageTestValue(0x44), handle.GetChildStoreByName(keys.EVMStoreKey).Get(evmStorageTestKey(1)))
		require.Equal(t, evmStorageTestValue(0x55), handle.GetChildStoreByName(keys.EVMStoreKey).Get(evmStorageTestKey(3)))
		boundaryBytes, found := handle.flatKV.Get(migration.MigrationStore, []byte(migration.MigrationBoundaryKey))
		require.True(t, found)
		boundary, err := migration.DeserializeMigrationBoundary(boundaryBytes)
		require.NoError(t, err)
		require.True(t, boundary.Equals(migration.NewMigrationBoundary(keys.EVMStoreKey, evmStorageTestKey(1))))
		_, found = handle.flatKV.Get(keys.EVMStoreKey, evmStorageTestKey(3))
		require.False(t, found)
	}
	require.Equal(t, cloneCommitInfo(fresh.WorkingCommitInfo()), cloneCommitInfo(cs.WorkingCommitInfo()))
	require.Equal(t, mergeStateSyncCommitHash(*fresh.WorkingCommitInfo()), mergeStateSyncCommitHash(*cs.WorkingCommitInfo()))

	if !rollback && committed {
		// A historical LoadVersion selects a height without truncating future
		// WAL entries. Reopen that selected height and prove buffered writes
		// did not become committed state; Rollback owns timeline replacement.
		require.NoError(t, cs.Close())
		cs = repair119Open(t, dir, config.MigrateEVM, mounted, 1)
		require.Equal(t, baseline, cloneCommitInfo(cs.LastCommitInfo()))
		require.Equal(t, workingBaseline, cloneCommitInfo(cs.WorkingCommitInfo()))
		require.Equal(t, evmStorageTestValue(0x11), cs.GetChildStoreByName(keys.EVMStoreKey).Get(evmStorageTestKey(1)))
		require.Equal(t, []byte("100"), cs.GetChildStoreByName(keys.BankStoreKey).Get([]byte("balance")))
		return
	}

	working := cloneCommitInfo(cs.WorkingCommitInfo())
	for _, handle := range []*CompositeCommitStore{cs, fresh} {
		v, err := handle.Commit()
		require.NoError(t, err)
		require.Equal(t, int64(2), v)
		require.Equal(t, mergeStateSyncCommitHash(working), mergeStateSyncCommitHash(*handle.LastCommitInfo()))
		require.NoError(t, handle.Close())
	}
	cs = repair119Open(t, dir, config.MigrateEVM, mounted, 0)
	fresh = repair119Open(t, freshDir, config.MigrateEVM, mounted, 0)
	require.Equal(t, cloneCommitInfo(fresh.LastCommitInfo()), cloneCommitInfo(cs.LastCommitInfo()))
	require.Equal(t, mergeStateSyncCommitHash(working), mergeStateSyncCommitHash(*cs.LastCommitInfo()))
	require.Equal(t, cloneCommitInfo(fresh.WorkingCommitInfo()), cloneCommitInfo(cs.WorkingCommitInfo()))
	require.Equal(t, evmStorageTestValue(0x44), cs.GetChildStoreByName(keys.EVMStoreKey).Get(evmStorageTestKey(1)))
	require.Equal(t, evmStorageTestValue(0x55), cs.GetChildStoreByName(keys.EVMStoreKey).Get(evmStorageTestKey(3)))
	require.Equal(t, []byte("200"), cs.GetChildStoreByName(keys.BankStoreKey).Get([]byte("balance")))
}

func TestRepair_119_PendingRecovery(t *testing.T) {
	for _, rollback := range []bool{true, false} {
		name := "reload"
		if rollback {
			name = "rollback"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			seed := repair119Open(t, dir, config.MemiavlOnly, []string{keys.BankStoreKey}, 0)
			require.NoError(t, seed.ApplyChangeSets(bankWrite()))
			_, err := seed.Commit()
			require.NoError(t, err)
			baseline := cloneCommitInfo(seed.LastCommitInfo())
			require.NoError(t, seed.Close())

			mounted := []string{keys.BankStoreKey, keys.LaunchpadStoreKey, keys.XWebStoreKey}
			cs := repair119Open(t, dir, config.MemiavlOnly, mounted, 0)
			workingBaseline := cloneCommitInfo(cs.WorkingCommitInfo())
			pending := cs.PendingStores()
			require.Len(t, pending, 2)
			require.NoError(t, cs.ApplyUpgrades([]*proto.TreeNameUpgrade{{Name: keys.LaunchpadStoreKey}}))
			require.False(t, cs.IsPendingStore(keys.LaunchpadStoreKey))
			require.NoError(t, cs.ApplyChangeSets([]*proto.NamedChangeSet{launchpadWrite()}))
			if rollback {
				_, err = cs.Commit()
				require.NoError(t, err)
			}
			cs.GetChildStoreByName(keys.XWebStoreKey).Set([]byte("discard"), []byte("pending"))
			require.NotEmpty(t, cs.pendingWrites)

			repair119Recover(t, cs, rollback)
			require.Equal(t, pending, cs.PendingStores())
			require.Empty(t, cs.pendingWrites)
			require.Nil(t, cs.GetChildStoreByName(keys.LaunchpadStoreKey).Get([]byte("params")))
			require.Equal(t, baseline, cloneCommitInfo(cs.LastCommitInfo()))
			require.Equal(t, workingBaseline, cloneCommitInfo(cs.WorkingCommitInfo()))
			require.NoError(t, cs.Close())

			cs = repair119Open(t, dir, config.MemiavlOnly, mounted, 0)
			require.Equal(t, pending, cs.PendingStores())
			require.Equal(t, baseline, cloneCommitInfo(cs.LastCommitInfo()))
			require.Equal(t, workingBaseline, cloneCommitInfo(cs.WorkingCommitInfo()))
			require.NoError(t, cs.ApplyUpgrades([]*proto.TreeNameUpgrade{{Name: keys.LaunchpadStoreKey}}))
			require.NoError(t, cs.ApplyChangeSets([]*proto.NamedChangeSet{launchpadWrite()}))
			_, err = cs.Commit()
			require.NoError(t, err)
			require.NoError(t, cs.Close())
			cs = repair119Open(t, dir, config.MemiavlOnly, mounted, 0)
			require.False(t, cs.IsPendingStore(keys.LaunchpadStoreKey))
			require.True(t, cs.IsPendingStore(keys.XWebStoreKey))
			require.Equal(t, []byte("paused"), cs.GetChildStoreByName(keys.LaunchpadStoreKey).Get([]byte("params")))
		})
	}
}
