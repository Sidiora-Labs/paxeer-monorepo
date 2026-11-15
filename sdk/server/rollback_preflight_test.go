package server

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"runtime"
	"testing"

	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func rollbackCopyTestConfig(t *testing.T) *tmconfig.Config {
	t.Helper()
	cfg := tmconfig.TestConfig()
	cfg.SetRoot(t.TempDir())
	cfg.BaseConfig.DBBackend = "goleveldb"
	return cfg
}

func writeRollbackCopyRecords(t *testing.T, cfg *tmconfig.Config, name string, records [][2][]byte) {
	t.Helper()
	db, err := dbm.NewDB(name, dbm.BackendType(cfg.DBBackend), cfg.DBDir())
	require.NoError(t, err)
	batch := db.NewBatch()
	for _, record := range records {
		require.NoError(t, batch.Set(record[0], record[1]))
	}
	require.NoError(t, batch.WriteSync())
	require.NoError(t, batch.Close())
	require.NoError(t, db.Close())
}

func readRollbackCopyRecords(t *testing.T, cfg *tmconfig.Config, name string) [][2][]byte {
	t.Helper()
	db, err := dbm.NewDB(name, dbm.GoLevelDBBackend, cfg.DBDir())
	require.NoError(t, err)
	defer func() { require.NoError(t, db.Close()) }()
	iter, err := db.Iterator(nil, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, iter.Close()) }()
	var records [][2][]byte
	for iter.Valid() {
		records = append(records, [2][]byte{append([]byte(nil), iter.Key()...), append([]byte(nil), iter.Value()...)})
		iter.Next()
	}
	require.NoError(t, iter.Error())
	return records
}

func captureRollbackCopyOutput(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	previous := os.Stdout
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = writer
	callErr := fn()
	require.NoError(t, writer.Close())
	os.Stdout = previous
	data, readErr := io.ReadAll(reader)
	require.NoError(t, reader.Close())
	require.NoError(t, readErr)
	return string(data), callErr
}

func TestRollbackPreflightCopyParts(t *testing.T) {
	setRollbackDBCopyPartLimit(t, 4*1024)

	source := rollbackCopyTestConfig(t)
	destination := rollbackCopyTestConfig(t)
	records := make([][2][]byte, 0, 2000)
	for i := 0; i < 2000; i++ {
		size := 32 + (i % 257)
		if i == 137 {
			size = rollbackDBCopyPartLimit * 2
		}
		records = append(records, [2][]byte{[]byte(fmt.Sprintf("key-%05d", i)), bytes.Repeat([]byte{byte(i)}, size)})
	}
	writeRollbackCopyRecords(t, source, "state", records)

	output, err := captureRollbackCopyOutput(t, func() error {
		return snapshotRollbackDB(source, destination, "state")
	})
	require.NoError(t, err)
	require.Contains(t, output, "copying store state")
	require.Contains(t, output, "copied store state")
	require.Regexp(t, `parts=[2-9][0-9]*`, output)
	require.Equal(t, records, readRollbackCopyRecords(t, destination, "state"))
}

func TestRollbackPreflightCopyEmpty(t *testing.T) {
	source := rollbackCopyTestConfig(t)
	destination := rollbackCopyTestConfig(t)
	writeRollbackCopyRecords(t, source, "state", nil)

	_, err := captureRollbackCopyOutput(t, func() error {
		return snapshotRollbackDB(source, destination, "state")
	})
	require.NoError(t, err)
	require.Empty(t, readRollbackCopyRecords(t, destination, "state"))
}

func TestRollbackPreflightCopyMemory(t *testing.T) {
	setRollbackDBCopyPartLimit(t, 1<<20)

	const valueSize = 1 << 20
	const valueCount = 64
	source := rollbackCopyTestConfig(t)
	destination := rollbackCopyTestConfig(t)
	records := make([][2][]byte, 0, valueCount)
	for i := 0; i < valueCount; i++ {
		records = append(records, [2][]byte{[]byte(fmt.Sprintf("key-%03d", i)), bytes.Repeat([]byte{byte(i)}, valueSize)})
	}
	writeRollbackCopyRecords(t, source, "state", records)

	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err := captureRollbackCopyOutput(t, func() error {
		return snapshotRollbackDB(source, destination, "state")
	})
	require.NoError(t, err)
	runtime.GC()
	runtime.ReadMemStats(&after)
	heapDelta := uint64(0)
	if after.HeapInuse > before.HeapInuse {
		heapDelta = after.HeapInuse - before.HeapInuse
	}
	require.Less(t, heapDelta, uint64(valueSize*valueCount/4))
}

func TestRollbackPreflightCopyCommand(t *testing.T) {
	setRollbackDBCopyPartLimit(t, 1)

	t.Run("same height", func(t *testing.T) {
		TestRollbackScenario1_BothAtSameHeight(t)
	})
	t.Run("app ahead", func(t *testing.T) {
		TestRollbackScenario2_AppAheadOfTendermint(t)
	})
	t.Run("tendermint ahead", func(t *testing.T) {
		TestRollbackScenario3_TendermintAheadOfApp(t)
	})
	t.Run("error cases", func(t *testing.T) {
		TestRollbackErrorCases(t)
	})
	t.Run("num blocks", func(t *testing.T) {
		TestRollbackWithNumBlocks(t)
	})
}
