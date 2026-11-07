package receipt_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	dbutils "github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/utils"
	dbconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/pebbledb/mvcc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/ledger_db/receipt"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/wal"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	tidwal "github.com/tidwall/wal"
)

func TestRepairM_1146_SingletonRecovery(t *testing.T) {
	cfg, key, records := repairM1146CrashTail(t, 0, 1)
	repairM1146AssertReopens(t, cfg, key, records)
}

func TestRepairM_1146_MultiEntryRecovery(t *testing.T) {
	cfg, key, records := repairM1146CrashTail(t, 1, 3)
	repairM1146AssertReopens(t, cfg, key, records)
}

func TestRepairM_1146_SingletonAfterAppliedPrefix(t *testing.T) {
	cfg, key, records := repairM1146CrashTail(t, 1, 1)
	repairM1146AssertReopens(t, cfg, key, records)
}

func TestRepairM_1146_HandleLifecycle(t *testing.T) {
	baseline := repairM1146WALWorkers(t)
	cfg, key, records := repairM1146CrashTail(t, 0, 1)
	repairM1146AssertResources(t, cfg.DBDirectory, baseline)
	for i := 0; i < 5; i++ {
		store, err := receipt.NewReceiptStore(cfg, key)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, store.Close()) })
		require.Eventually(t, func() bool { return repairM1146WALWorkers(t) == baseline+1 }, 2*time.Second, time.Millisecond)
		require.Positive(t, repairM1146OpenFiles(t, cfg.DBDirectory))
		repairM1146AssertReceipts(t, store, records)
		// A second constructor must not open a recovery handle over a live owner.
		other, err := receipt.NewReceiptStore(cfg, key)
		if other != nil {
			t.Cleanup(func() { _ = other.Close() })
		}
		require.ErrorContains(t, err, "lock receipt store")
		require.Nil(t, other)
		require.Equal(t, baseline+1, repairM1146WALWorkers(t))
		require.NoError(t, store.Close())
		require.NoError(t, store.Close())
		repairM1146AssertResources(t, cfg.DBDirectory, baseline)
	}
}

func TestRepairM_1146_EmptyRecovery(t *testing.T) {
	baseline := repairM1146WALWorkers(t)
	cfg := dbconfig.DefaultReceiptStoreConfig()
	cfg.DBDirectory = t.TempDir()
	key := storetypes.NewKVStoreKey("evm")
	for i := 0; i < 4; i++ {
		store, err := receipt.NewReceiptStore(cfg, key)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, store.Close()) })
		require.Zero(t, store.LatestVersion())
		require.NoError(t, store.Close())
		repairM1146AssertResources(t, cfg.DBDirectory, baseline)
	}
}

func TestRepairM_1146_OffsetErrors(t *testing.T) {
	baseline := repairM1146WALWorkers(t)
	dir := t.TempDir()
	stream, err := wal.NewChangelogWAL(dir, wal.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, stream.Close()) })
	entries, err := receipt.ReadReceiptChangelog(stream)
	require.NoError(t, err)
	require.Empty(t, entries)
	require.NoError(t, stream.Close())
	// A real closed WAL returns zero offsets AND ErrClosed. Zero alone must
	// never make recovery succeed; both offset failures retain their causes.
	first, errFirst := stream.FirstOffset()
	last, errLast := stream.LastOffset()
	require.Zero(t, first)
	require.Zero(t, last)
	require.ErrorIs(t, errFirst, tidwal.ErrClosed)
	require.ErrorIs(t, errLast, tidwal.ErrClosed)
	entries, err = receipt.ReadReceiptChangelog(stream)
	require.Nil(t, entries)
	require.ErrorIs(t, err, tidwal.ErrClosed)
	require.ErrorContains(t, err, "first offset")
	require.ErrorContains(t, err, "last offset")
	repairM1146AssertResources(t, dir, baseline)
}

func TestRepairM_1146_UnreadableRecoveryMetadata(t *testing.T) {
	baseline := repairM1146WALWorkers(t)
	cfg, key, records := repairM1146CrashTail(t, 0, 1)
	path := dbutils.GetChangelogPath(cfg.DBDirectory)
	saved := filepath.Join(t.TempDir(), "saved-changelog")
	require.NoError(t, os.Rename(path, saved))
	require.NoError(t, os.WriteFile(path, []byte("not a directory"), 0o600))
	for i := 0; i < 3; i++ {
		store, err := receipt.NewReceiptStore(cfg, key)
		if store != nil {
			t.Cleanup(func() { _ = store.Close() })
		}
		require.Nil(t, store)
		require.ErrorContains(t, err, "open receipt recovery changelog")
		var pathErr *os.PathError
		require.ErrorAs(t, err, &pathErr)
		require.Equal(t, path, pathErr.Path)
		repairM1146AssertResources(t, cfg.DBDirectory, baseline)
	}
	require.NoError(t, os.Remove(path))
	require.NoError(t, os.Rename(saved, path))
	repairM1146AssertReopens(t, cfg, key, records)
	repairM1146AssertResources(t, cfg.DBDirectory, baseline)
}

func TestRepairM_1146_ReplayReadErrorClosesHandle(t *testing.T) {
	baseline := repairM1146WALWorkers(t)
	cfg, key, records := repairM1146CrashTail(t, 0, 1)
	path := dbutils.GetChangelogPath(cfg.DBDirectory)
	// Keep the real producer's receipt, but append an undecodable protobuf
	// record using the same on-disk WAL transport.
	log, err := tidwal.Open(path, nil)
	require.NoError(t, err)
	last, err := log.LastIndex()
	require.NoError(t, err)
	require.NoError(t, log.Write(last+1, []byte{0xff}))
	require.NoError(t, log.Close())
	for i := 0; i < 3; i++ {
		store, err := receipt.NewReceiptStore(cfg, key)
		if store != nil {
			t.Cleanup(func() { _ = store.Close() })
		}
		require.Nil(t, store)
		require.ErrorContains(t, err, "read receipt recovery changelog")
		require.ErrorContains(t, err, "unmarshal log failed")
		repairM1146AssertResources(t, cfg.DBDirectory, baseline)
	}
	log, err = tidwal.Open(path, nil)
	require.NoError(t, err)
	require.NoError(t, log.TruncateBack(last))
	require.NoError(t, log.Close())
	repairM1146AssertReopens(t, cfg, key, records)
	repairM1146AssertResources(t, cfg.DBDirectory, baseline)
}

// Produce all WAL bytes through the public receipt writer. Restoring only
// older Pebble files models the durable WAL surviving unapplied database writes.
func repairM1146CrashTail(t *testing.T, prefix, tail int) (dbconfig.ReceiptStoreConfig, storetypes.StoreKey, []receipt.ReceiptRecord) {
	t.Helper()
	cfg := dbconfig.DefaultReceiptStoreConfig()
	cfg.DBDirectory = t.TempDir()
	cfg.KeepRecent = 0
	key := storetypes.NewKVStoreKey("evm")
	records := make([]receipt.ReceiptRecord, prefix+tail)
	for i := range records {
		hash := common.Hash{}
		hash[31] = byte(i + 1)
		r := makeReceipt(hash, common.HexToAddress("0x400"), []common.Hash{common.HexToHash("0x22")}, uint32(i))
		r.BlockNumber = uint64(i + 1)
		r.TxType = 2
		r.Status = 1
		r.GasUsed = uint64(21000 + i)
		r.CumulativeGasUsed = uint64(42000 + i)
		r.EffectiveGasPrice = 17
		r.From = common.HexToAddress("0x123").Hex()
		r.To = common.HexToAddress("0x456").Hex()
		r.Logs[0].Data = []byte{0x11, byte(i), 0xff}
		records[i] = receipt.ReceiptRecord{TxHash: hash, Receipt: r}
	}
	store, err := receipt.NewReceiptStore(cfg, key)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	for i := 0; i < prefix; i++ {
		require.NoError(t, store.SetReceipts(sdk.Context{}.WithBlockHeight(int64(i+1)), records[i:i+1]))
	}
	require.NoError(t, store.Close())
	snapshot := t.TempDir()
	repairM1146CopyDB(t, cfg.DBDirectory, snapshot)
	store, err = receipt.NewReceiptStore(cfg, key)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	for i := prefix; i < len(records); i++ {
		require.NoError(t, store.SetReceipts(sdk.Context{}.WithBlockHeight(int64(i+1)), records[i:i+1]))
	}
	require.NoError(t, store.Close())
	stream, err := wal.NewChangelogWAL(dbutils.GetChangelogPath(cfg.DBDirectory), wal.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, stream.Close()) })
	entries, err := receipt.ReadReceiptChangelog(stream)
	require.NoError(t, err)
	require.Len(t, entries, len(records))
	for i, entry := range entries {
		require.Equal(t, int64(i+1), entry.Version)
		require.Len(t, entry.Changesets, 1)
		require.Equal(t, evmtypes.ReceiptStoreKey, entry.Changesets[0].Name)
		require.Len(t, entry.Changesets[0].Changeset.Pairs, 1)
		pair := entry.Changesets[0].Changeset.Pairs[0]
		require.Equal(t, evmtypes.ReceiptKey(records[i].TxHash), pair.Key)
		expected, err := records[i].Receipt.Marshal()
		require.NoError(t, err)
		require.Equal(t, expected, pair.Value)
	}
	require.NoError(t, stream.Close())
	files, err := os.ReadDir(cfg.DBDirectory)
	require.NoError(t, err)
	for _, file := range files {
		if file.Name() != "changelog" {
			require.NoError(t, os.RemoveAll(filepath.Join(cfg.DBDirectory, file.Name())))
		}
	}
	repairM1146CopyDB(t, snapshot, cfg.DBDirectory)
	ssConfig := dbconfig.DefaultStateStoreConfig()
	ssConfig.DBDirectory = cfg.DBDirectory
	ssConfig.KeepRecent = 0
	db, err := mvcc.OpenDB(cfg.DBDirectory, ssConfig)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.Equal(t, int64(prefix), db.GetLatestVersion(), "fixture must actually roll back the database head")
	for i, record := range records {
		data, err := db.Get(evmtypes.ReceiptStoreKey, db.GetLatestVersion(), evmtypes.ReceiptKey(record.TxHash))
		require.NoError(t, err)
		if i < prefix {
			expected, err := record.Receipt.Marshal()
			require.NoError(t, err)
			require.Equal(t, expected, data)
		} else {
			require.Nil(t, data, "tail must be absent from the database before recovery")
		}
	}
	require.NoError(t, db.Close())
	return cfg, key, records
}

func repairM1146CopyDB(t *testing.T, source, target string) {
	t.Helper()
	require.NoError(t, filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "changelog" {
			return filepath.SkipDir
		}
		if relative == "RECEIPT-LOCK" {
			return nil
		}
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o750)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(destination, data, 0o600)
	}))
}

func repairM1146AssertReopens(t *testing.T, cfg dbconfig.ReceiptStoreConfig, key storetypes.StoreKey, records []receipt.ReceiptRecord) {
	t.Helper()
	for i := 0; i < 3; i++ {
		store, err := receipt.NewReceiptStore(cfg, key)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, store.Close()) })
		repairM1146AssertReceipts(t, store, records)
		require.NoError(t, store.Close())
	}
}

func repairM1146AssertReceipts(t *testing.T, store receipt.ReceiptStore, records []receipt.ReceiptRecord) {
	t.Helper()
	require.Equal(t, int64(len(records)), store.LatestVersion())
	for _, record := range records {
		got, err := store.GetReceiptFromStore(sdk.Context{}, record.TxHash)
		require.NoError(t, err)
		require.Equal(t, record.Receipt, got)
		expectedBytes, err := record.Receipt.Marshal()
		require.NoError(t, err)
		gotBytes, err := got.Marshal()
		require.NoError(t, err)
		require.Equal(t, expectedBytes, gotBytes)
		public, err := store.GetReceipt(sdk.Context{}, record.TxHash)
		require.NoError(t, err)
		require.Equal(t, record.Receipt, public)
	}
}

func repairM1146WALWorkers(t *testing.T) int {
	t.Helper()
	buffer := make([]byte, 1<<20)
	n := runtime.Stack(buffer, true)
	require.Less(t, n, len(buffer), "resource accounting must include every goroutine")
	count := 0
	for _, line := range strings.Split(string(buffer[:n]), "\n") {
		if strings.Contains(line, "/storage/wal.") && strings.Contains(line, ".mainLoop(") {
			count++
		}
	}
	return count
}

func repairM1146OpenFiles(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	require.NoError(t, err)
	count := 0
	for _, entry := range entries {
		path, err := os.Readlink(filepath.Join("/proc/self/fd", entry.Name()))
		if os.IsNotExist(err) {
			continue
		}
		require.NoError(t, err)
		if strings.HasPrefix(path, dir+string(os.PathSeparator)) {
			count++
		}
	}
	return count
}

func repairM1146AssertResources(t *testing.T, dir string, workers int) {
	t.Helper()
	require.Eventually(t, func() bool {
		return repairM1146WALWorkers(t) == workers && repairM1146OpenFiles(t, dir) == 0
	}, 2*time.Second, time.Millisecond, "all recovery/owner workers and file descriptors must close")
}
