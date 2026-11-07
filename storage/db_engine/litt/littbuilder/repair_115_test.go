package littbuilder_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/littbuilder"
	"github.com/stretchr/testify/require"
)

func repair115OpenDB(t *testing.T, config *litt.Config) litt.DB {
	t.Helper()
	database, err := littbuilder.NewDB(config)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close()) })
	return database
}

func repair115RejectTable(t *testing.T, database litt.DB, name string) {
	t.Helper()
	table, err := database.BuildTable(litt.DefaultTableConfig(name))
	require.ErrorContains(t, err, "database is closing or closed")
	require.Nil(t, table, "a stopped database must not return a writable handle")
}

func TestRepair_115_BuildAfterClose(t *testing.T) {
	config, err := litt.DefaultConfig(t.TempDir(), t.TempDir())
	require.NoError(t, err)
	database := repair115OpenDB(t, config)
	table, err := database.BuildTable(litt.DefaultTableConfig("existing"))
	require.NoError(t, err)
	require.NoError(t, table.Put([]byte("key"), []byte("value")))
	require.NoError(t, table.Flush())
	require.NoError(t, database.Close())

	repair115RejectTable(t, database, "existing")
	repair115RejectTable(t, database, "after_close")
	for _, root := range config.Paths {
		_, err := os.Stat(filepath.Join(root, "after_close"))
		require.ErrorIs(t, err, os.ErrNotExist, "rejected builds must not create table files")
	}
	require.NoError(t, database.Close())

	reopened := repair115OpenDB(t, config)
	recovered, err := reopened.BuildTable(litt.DefaultTableConfig("existing"))
	require.NoError(t, err)
	value, exists, err := recovered.Get([]byte("key"))
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, []byte("value"), value)
}

func TestRepair_115_BuildAfterCloseWithReopenedDB(t *testing.T) {
	config, err := litt.DefaultConfig(t.TempDir(), t.TempDir())
	require.NoError(t, err)
	old := repair115OpenDB(t, config)
	table, err := old.BuildTable(litt.DefaultTableConfig("existing"))
	require.NoError(t, err)
	require.NoError(t, table.Put([]byte("before"), []byte("original")))
	require.NoError(t, table.Flush())
	require.NoError(t, old.Close())

	owner := repair115OpenDB(t, config)
	// Even before the new owner opens this table, the old handle cannot reopen it.
	repair115RejectTable(t, old, "existing")
	active, err := owner.BuildTable(litt.DefaultTableConfig("existing"))
	require.NoError(t, err)
	value, exists, err := active.Get([]byte("before"))
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, []byte("original"), value)

	repair115RejectTable(t, old, "after_reopen")
	for _, root := range config.Paths {
		_, err := os.Stat(filepath.Join(root, "after_reopen"))
		require.ErrorIs(t, err, os.ErrNotExist)
	}
	require.NoError(t, old.Close())
	require.NoError(t, old.Close())
	for _, root := range config.Paths {
		contenderConfig, err := litt.DefaultConfig(root)
		require.NoError(t, err)
		contender, err := littbuilder.NewDB(contenderConfig)
		if contender != nil {
			t.Cleanup(func() { require.NoError(t, contender.Close()) })
		}
		require.Error(t, err, "repeated Close on the old handle must preserve the new owner's locks")
		require.Nil(t, contender)
	}

	require.NoError(t, active.Put([]byte("after"), []byte("new owner")))
	require.NoError(t, active.Flush())
	require.NoError(t, owner.Close())
	recoveredDB := repair115OpenDB(t, config)
	recovered, err := recoveredDB.BuildTable(litt.DefaultTableConfig("existing"))
	require.NoError(t, err)
	for key, expected := range map[string]string{"before": "original", "after": "new owner"} {
		value, exists, err := recovered.Get([]byte(key))
		require.NoError(t, err)
		require.True(t, exists)
		require.Equal(t, []byte(expected), value)
	}
}
