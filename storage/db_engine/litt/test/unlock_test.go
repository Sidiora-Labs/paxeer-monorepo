package test

import (
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/disktable"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/littbuilder"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/util"
	"github.com/stretchr/testify/require"
)

// Note: this test is defined in the test package to avoid circular dependencies.

func TestUnlock(t *testing.T) {
	testDir := t.TempDir()
	rand := util.NewTestRandom()
	volumes := []string{path.Join(testDir, "volume1"), path.Join(testDir, "volume2"), path.Join(testDir, "volume3")}

	config, err := litt.DefaultConfig(volumes...)
	config.Fsync = false // Disable fsync for faster tests
	config.TargetSegmentFileSize = 100
	require.NoError(t, err)

	tableConfig := litt.DefaultTableConfig("test_table")
	tableConfig.ShardingFactor = uint8(len(volumes))

	db, err := littbuilder.NewDB(config)
	require.NoError(t, err)

	table, err := db.BuildTable(tableConfig)
	require.NoError(t, err)

	expectedData := make(map[string][]byte)

	// Write some data
	for i := 0; i < 100; i++ {
		key := rand.PrintableBytes(32)
		value := rand.PrintableVariableBytes(1, 100)

		expectedData[string(key)] = value
		err = table.Put(key, value)
		require.NoError(t, err, "Failed to put data in table")
	}

	// Look for lock files. We should see one for each volume.
	lockFileCount := 0
	err = filepath.Walk(testDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Log but do not fail. LittDB may be shuffling files around concurrently.
			t.Logf("Error walking path %s (not necessarily fatal): %v", path, err)
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, util.LockfileName) {
			lockFileCount++
		}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 3, lockFileCount)

	// Unlock the DB. This should remove all lock files, but leave other files intact.
	err = disktable.Unlock(slog.Default(), volumes)
	require.NoError(t, err, "Failed to unlock the database")

	// There should be no lock files left.
	lockFileCount = 0
	err = filepath.Walk(testDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Log but do not fail. LittDB may be shuffling files around concurrently.
			t.Logf("Error walking path %s (not necessarily fatal): %v", path, err)
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, util.LockfileName) {
			lockFileCount++
		}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 0, lockFileCount, "There should be no lock files left after unlocking")

	// Calling unlock again should not cause any issues.
	err = disktable.Unlock(slog.Default(), volumes)
	require.NoError(t, err, "Failed to unlock the database again")

	// Verify that the data is still intact.
	for key, expectedValue := range expectedData {
		value, ok, err := table.Get([]byte(key))
		require.NoError(t, err, "Failed to get data from table")
		require.True(t, ok, "Failed to get data from table")
		require.Equal(t, expectedValue, value, "Data mismatch for key %s", key)
	}

	// Restart the database and verify the data again.
	err = db.Close()
	require.NoError(t, err)

	db, err = littbuilder.NewDB(config)
	require.NoError(t, err)

	table, err = db.BuildTable(tableConfig)
	require.NoError(t, err)

	for key, expectedValue := range expectedData {
		value, ok, err := table.Get([]byte(key))
		require.NoError(t, err, "Failed to get data from table after restart")
		require.True(t, ok, "Failed to get data from table after restart")
		require.Equal(t, expectedValue, value, "Data mismatch for key %s after restart", key)
	}

	err = db.Close()
	require.NoError(t, err, "Failed to close the database after restart")
}

func TestPurgeLocks(t *testing.T) {
	purgeLocksTest(t)
}

func purgeLocksTest(t *testing.T) {
	t.Helper()
	testDir := t.TempDir()
	rand := util.NewTestRandom()
	canonicalDir, err := filepath.EvalSymlinks(testDir)
	require.NoError(t, err)
	volumes := []string{
		filepath.Join(canonicalDir, "volume1"),
		filepath.Join(canonicalDir, "volume2"),
		filepath.Join(canonicalDir, "volume3"),
	}
	distinctRoots := make(map[string]bool)
	for _, root := range volumes {
		require.True(t, filepath.IsAbs(root))
		require.Equal(t, filepath.Clean(root), root)
		distinctRoots[root] = true
	}
	require.Len(t, volumes, 3)
	require.Len(t, distinctRoots, 3)

	config, err := litt.DefaultConfig(volumes...)
	require.NoError(t, err)
	config.Fsync = false // Disable fsync for faster tests
	config.TargetSegmentFileSize = 100

	db, err := littbuilder.NewDB(config)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.ElementsMatch(t, volumes, config.Paths)

	tableConfig := litt.DefaultTableConfig("test_table")
	tableConfig.ShardingFactor = 3
	table, err := db.BuildTable(tableConfig)
	require.NoError(t, err)

	expectedData := make(map[string][]byte)

	// Write some data
	for i := 0; i < 100; i++ {
		key := rand.PrintableBytes(32)
		value := rand.PrintableVariableBytes(1, 100)

		expectedData[string(key)] = value
		err = table.Put(key, value)
		require.NoError(t, err, "Failed to put data in table")
	}
	require.NoError(t, table.Flush())

	assertRootLocks := func(locked bool) {
		t.Helper()
		for _, root := range volumes {
			lockPath := filepath.Join(root, util.LockfileName)
			if locked {
				require.FileExists(t, lockPath)
				lock, err := util.NewFileLock(slog.Default(), lockPath, false)
				if lock != nil {
					lock.Release()
				}
				require.Error(t, err, "root must reject another lock owner: %s", root)
			} else {
				require.NoFileExists(t, lockPath)
				lock, err := util.NewFileLock(slog.Default(), lockPath, false)
				require.NoError(t, err, "root must be available after close: %s", root)
				lock.Release()
				require.NoFileExists(t, lockPath)
			}
		}
	}
	assertRootLocks(true)

	// Opening a second instance of the database should fail due to existing locks.
	_, err = littbuilder.NewDB(config)
	require.Error(t, err, "Expected error when opening a second instance of the database with existing locks")

	lockRecords := make(map[string][]byte)
	for _, root := range volumes {
		lockPath := filepath.Join(root, util.LockfileName)
		lockRecords[lockPath], err = os.ReadFile(lockPath)
		require.NoError(t, err)
	}
	err = db.Close()
	require.NoError(t, err, "Failed to close the first instance of the database")
	assertRootLocks(false)

	// Replay the real DB's metadata without retaining a live advisory-lock owner.
	for lockPath, record := range lockRecords {
		require.NoError(t, os.WriteFile(lockPath, record, 0600))
	}
	assertRootLocks(true)
	_, err = littbuilder.NewDB(config)
	require.Error(t, err, "Recorded lock metadata must require an explicit purge")

	purgeConfig := *config
	purgeConfig.PurgeLocks = true
	db2, err := littbuilder.NewDB(&purgeConfig)
	require.NoError(t, err, "Failed to reopen the database with lock purging enabled")
	t.Cleanup(func() { require.NoError(t, db2.Close()) })
	assertRootLocks(true)

	verifyData := func(database litt.DB) {
		t.Helper()
		recovered, err := database.BuildTable(tableConfig)
		require.NoError(t, err)
		require.Equal(t, uint64(len(expectedData)), recovered.KeyCount())
		for key, expectedValue := range expectedData {
			value, exists, err := recovered.Get([]byte(key))
			require.NoError(t, err, key)
			require.True(t, exists, key)
			require.Equal(t, expectedValue, value, key)
		}
	}
	verifyData(db2)
	_, err = littbuilder.NewDB(config)
	require.Error(t, err, "Reopened database must retain exclusive access")
	require.NoError(t, db2.Close())
	assertRootLocks(false)

	db3, err := littbuilder.NewDB(config)
	require.NoError(t, err, "All roots must reopen normally after the purged owner closes")
	t.Cleanup(func() { require.NoError(t, db3.Close()) })
	assertRootLocks(true)
	verifyData(db3)
	require.NoError(t, db3.Close())
	assertRootLocks(false)
}
