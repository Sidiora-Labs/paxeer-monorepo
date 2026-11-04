package litt_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/dbcache"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/disktable"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/disktable/keymap"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/disktable/segment"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/littbuilder"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/util"
	"github.com/stretchr/testify/require"
)

func repair1165Text(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(name)
	require.NoError(t, err)
	return string(data)
}

func repair1165Contains(t *testing.T, text string, claims ...string) {
	t.Helper()
	for _, claim := range claims {
		require.Contains(t, text, claim)
	}
}

func repair1165Config(t *testing.T, roots ...string) *litt.Config {
	t.Helper()
	config, err := litt.DefaultConfig(roots...)
	require.NoError(t, err)
	config.KeymapType = keymap.MemKeymapType
	config.GCPeriod = time.Hour
	return config
}

func repair1165Open(t *testing.T, config *litt.Config, tc litt.TableConfig, clock func() time.Time) (litt.DB, litt.Table) {
	t.Helper()
	runtime := litt.DefaultRuntimeConfig()
	runtime.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	if clock != nil {
		runtime.Clock = clock
	}
	db, err := littbuilder.NewDB(config, runtime)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	table, err := db.BuildTable(tc)
	require.NoError(t, err)
	return db, table
}

func repair1165Get(t *testing.T, table litt.Table, key, want []byte) {
	t.Helper()
	value, found, err := table.Get(key)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, want, value)
}

// Consume the DB's real files through the same scanner and segment reader used by CLI table-info.
// The database is closed before this helper is called; no subprocess or replacement CLI is used.
func repair1165Segments(t *testing.T, config *litt.Config, name string) map[uint32]*segment.Segment {
	t.Helper()
	paths, err := segment.BuildSegmentPaths(config.Paths, "", name)
	require.NoError(t, err)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	monitor := util.NewErrorMonitor(t.Context(), logger, nil)
	_, _, segments, err := segment.GatherSegmentFiles(logger, monitor, paths, false, time.Now(), false, config.Fsync)
	require.NoError(t, err)
	require.NotEmpty(t, segments)
	ok, err := monitor.IsOk()
	require.True(t, ok)
	require.NoError(t, err)
	return segments
}

func TestRepairM_1165_KeyLimit(t *testing.T) {
	doc := repair1165Text(t, "README.md")
	features := strings.Split(strings.Split(doc, "## Features\n")[1], "## Consistency Guarantees")[0]
	repair1165Contains(t, features, "65,535 bytes (2^16 - 1)", "4,294,967,295 bytes", "math.MaxUint32")
	repair1165Contains(t, doc, "Both primary and secondary keys must not exceed 65,535 bytes", "bytes.Repeat([]byte{'k'}, 65535)", "bytes.Repeat([]byte{'k'}, 65536)")

	config := repair1165Config(t, t.TempDir())
	tc := litt.DefaultTableConfig("limits")
	tc.ShardingFactor = 1
	db, table := repair1165Open(t, config, tc, nil)
	primary := bytes.Repeat([]byte{'k'}, math.MaxUint16)
	alias := bytes.Repeat([]byte{'s'}, math.MaxUint16)
	value := []byte("payload")
	require.NoError(t, table.Put(primary, value, &types.SecondaryKey{Key: alias, Offset: 1, Length: 5}))
	require.Error(t, table.Put(bytes.Repeat([]byte{'x'}, math.MaxUint16+1), value))
	require.Error(t, table.Put([]byte("rejected"), value, &types.SecondaryKey{
		Key: bytes.Repeat([]byte{'y'}, math.MaxUint16+1), Length: uint32(len(value)),
	}))
	require.NoError(t, db.Close()) // No explicit Flush: Close must publish the accepted group.
	_, table = repair1165Open(t, config, tc, nil)
	repair1165Get(t, table, primary, value)
	repair1165Get(t, table, alias, []byte("ayloa"))
	require.Equal(t, uint64(2), table.KeyCount(), "rejected requests must leave no persisted keys")

	// The real codec preserves the exact maximum length without a 4 GiB allocation.
	address := types.NewAddress(0, 0, 0, math.MaxUint32)
	recovered, err := types.DeserializeAddress(address.Serialize())
	require.NoError(t, err)
	require.Equal(t, address, recovered)
	require.Equal(t, uint32(math.MaxUint32), recovered.ValueSize())
}

func TestRepairM_1165_CloseName(t *testing.T) {
	doc := repair1165Text(t, "README.md")
	require.NotContains(t, doc, "Stop()")
	repair1165Contains(t, doc, "if err := db.Close(); err != nil", "errors.Join(err, db.Close())", "Writes overlapping `Close()` are not guaranteed durable", "stop and join writers before closing")
	config := repair1165Config(t, t.TempDir())
	tc := litt.DefaultTableConfig("shutdown")
	tc.ShardingFactor = 1
	db, table := repair1165Open(t, config, tc, nil)
	var writers sync.WaitGroup
	errors := make(chan error, 8)
	for i := 0; i < 8; i++ {
		writers.Add(1)
		go func(i int) {
			defer writers.Done()
			errors <- table.Put([]byte(fmt.Sprintf("key-%d", i)), []byte(fmt.Sprintf("value-%d", i)))
		}(i)
	}
	writers.Wait() // The documented durability boundary requires writers to finish before Close.
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	require.NoError(t, db.Close())
	_, table = repair1165Open(t, config, tc, nil)
	for i := 0; i < 8; i++ {
		repair1165Get(t, table, []byte(fmt.Sprintf("key-%d", i)), []byte(fmt.Sprintf("value-%d", i)))
	}
	require.Equal(t, uint64(8), table.KeyCount())
}

// As in repair_111_test.go, sparse padding positions the real writer at the format boundary.
// Flush is a barrier, and GC is disabled for an hour. Only byte cursors are adjusted; Put,
// preflight, rotation, key encoding, flushing, reads and reopen all use production code.
func repair1165Private(value reflect.Value) reflect.Value {
	return reflect.NewAt(value.Type(), unsafe.Pointer(value.UnsafeAddr())).Elem()
}

func repair1165Sparse(t *testing.T, table litt.Table, size uint64) *segment.Segment {
	t.Helper()
	require.NoError(t, table.Flush())
	// BuildTable returns ownedTable -> cachedTable -> DiskTable, even with zero cache sizes.
	owned := reflect.ValueOf(table).Elem().FieldByName("ManagedTable").Interface().(litt.ManagedTable)
	base := repair1165Private(reflect.ValueOf(owned).Elem().FieldByName("base")).Interface().(*disktable.DiskTable)
	control := reflect.ValueOf(base).Elem().FieldByName("controlLoop").Elem()
	segments := repair1165Private(control.FieldByName("segments")).Interface().(map[uint32]*segment.Segment)
	seg := segments[0]
	require.NotNil(t, seg)
	state := reflect.ValueOf(seg).Elem()
	shard := state.FieldByName("shards").Index(0).Elem()
	file := repair1165Private(shard.FieldByName("file")).Interface().(*os.File)
	require.NoError(t, file.Truncate(int64(size)))
	position, err := file.Seek(int64(size), io.SeekStart)
	require.NoError(t, err)
	require.Equal(t, int64(size), position)
	repair1165Private(shard.FieldByName("size")).SetUint(size)
	repair1165Private(shard.FieldByName("flushedSize")).Addr().Interface().(*atomic.Uint64).Store(size)
	repair1165Private(state.FieldByName("shardSizes")).Index(0).SetUint(size)
	repair1165Private(state.FieldByName("maxShardSize")).SetUint(size)
	return seg
}

func TestRepairM_1165_BoundedValue(t *testing.T) {
	doc := repair1165Text(t, "README.md")
	repair1165Contains(t, doc, "uint32 offset", "uint32 length", "Values cannot be arbitrarily large", "Before writing a primary value and its aliases", "`Segment.HasCapacity`", "fresh segment")
	config := repair1165Config(t, t.TempDir())
	tc := litt.DefaultTableConfig("capacity")
	tc.ShardingFactor = 1
	db, table := repair1165Open(t, config, tc, nil)
	old := repair1165Sparse(t, table, math.MaxUint32-8)
	require.NoError(t, table.Put([]byte("anchor"), []byte("keep")))
	require.NoError(t, table.Flush())
	request := &types.PutRequest{Key: []byte("whole"), Value: []byte("abcdef"), SecondaryKeys: []*types.SecondaryKey{
		{Key: []byte("suffix"), Offset: 2, Length: 4}, {Key: []byte("end"), Offset: 6},
	}}
	require.False(t, old.HasCapacity(request), "even though the first byte fits, the whole extent must fit")
	_, _, err := old.Write(request)
	require.Error(t, err)
	require.Equal(t, uint32(1), old.KeyCount(), "preflight rejection must not write a partial group")
	require.NoError(t, table.PutBatch([]*types.PutRequest{request}))
	require.NoError(t, db.Close())
	segments := repair1165Segments(t, config, tc.Name)
	require.Len(t, segments, 2)
	keys, err := segments[0].GetKeys()
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.Equal(t, types.NewAddress(0, math.MaxUint32-8, 0, 4), keys[0].Address)
	keys, err = segments[1].GetKeys()
	require.NoError(t, err)
	require.Len(t, keys, 3)
	for i, want := range []types.Address{types.NewAddress(1, 0, 0, 6), types.NewAddress(1, 2, 0, 4), types.NewAddress(1, 6, 0, 0)} {
		address, err := types.DeserializeAddress(keys[i].Address.Serialize())
		require.NoError(t, err)
		require.Equal(t, want, address)
	}
	_, table = repair1165Open(t, config, tc, nil)
	for key, value := range map[string]string{"anchor": "keep", "whole": "abcdef", "suffix": "cdef", "end": ""} {
		repair1165Get(t, table, []byte(key), []byte(value))
	}
}

func TestRepairM_1165_ShardLimit(t *testing.T) {
	doc := repair1165Text(t, "README.md")
	repair1165Contains(t, doc, "1 through 255 (`litt.MaxShardingFactor`)", "single byte (`uint8`)", "existing segments retain their\nfactors")
	require.Equal(t, 255, litt.MaxShardingFactor)
	config := repair1165Config(t, t.TempDir())
	tc := litt.DefaultTableConfig("shards")
	tc.ShardingFactor = 1
	db, table := repair1165Open(t, config, tc, nil)
	require.NoError(t, table.Put([]byte("old"), []byte("old-value")))
	require.Error(t, table.SetShardingFactor(0))
	require.NoError(t, table.SetShardingFactor(litt.MaxShardingFactor))
	for i := 0; i < litt.MaxShardingFactor; i++ {
		require.NoError(t, table.Put([]byte(fmt.Sprintf("shard-%d", i)), []byte{byte(i)}))
	}
	require.NoError(t, db.Close())
	segments := repair1165Segments(t, config, tc.Name)
	require.Len(t, segments, 2)
	require.Len(t, segments[0].GetValueFilePaths(), 1)
	require.Len(t, segments[1].GetValueFilePaths(), litt.MaxShardingFactor, "metadata must recover the byte-sized factor")
	keys, err := segments[1].GetKeys()
	require.NoError(t, err)
	require.Len(t, keys, litt.MaxShardingFactor)
	for i, key := range keys {
		require.Equal(t, uint8(i), key.Address.ShardID())
		value, err := segments[1].Read(key.Key, key.Address)
		require.NoError(t, err)
		require.Equal(t, []byte{byte(i)}, value)
	}
	_, table = repair1165Open(t, config, tc, nil) // New writes use 1; old metadata still uses 255.
	repair1165Get(t, table, []byte("old"), []byte("old-value"))
	for i := 0; i < litt.MaxShardingFactor; i++ {
		repair1165Get(t, table, []byte(fmt.Sprintf("shard-%d", i)), []byte{byte(i)})
	}
}

func TestRepairM_1165_RuntimeVolumeDoc(t *testing.T) {
	doc := repair1165Text(t, "db.go")
	require.NotContains(t, doc, "volume membership can be changed at runtime")
	repair1165Contains(t, doc, "check DB.Close", "offline litt rebase", "every destination root in Config.Paths", "Table.SetShardingFactor is a separate runtime operation")
	root, added := t.TempDir(), filepath.Join(t.TempDir(), "added")
	config := repair1165Config(t, root)
	tc := litt.DefaultTableConfig("roots")
	tc.ShardingFactor = 1
	db, table := repair1165Open(t, config, tc, nil)
	require.NoError(t, table.Put([]byte("before"), []byte("kept")))
	require.NoError(t, table.SetShardingFactor(2))
	_, err := os.Stat(added)
	require.ErrorIs(t, err, os.ErrNotExist, "changing shards must not add a storage root")
	require.NoError(t, db.Close())

	// Adding a root needs no file migration. Reconfigure only after the old DB is closed.
	config = repair1165Config(t, root, added)
	tc.ShardingFactor = 2
	db, table = repair1165Open(t, config, tc, nil)
	repair1165Get(t, table, []byte("before"), []byte("kept"))
	require.NoError(t, table.Put([]byte("after0"), []byte("zero")))
	require.NoError(t, table.Put([]byte("after1"), []byte("one")))
	require.NoError(t, db.Close())
	files, err := filepath.Glob(filepath.Join(added, tc.Name, "segments", "*.values"))
	require.NoError(t, err)
	require.NotEmpty(t, files)
	_, table = repair1165Open(t, config, tc, nil)
	for key, value := range map[string]string{"before": "kept", "after0": "zero", "after1": "one"} {
		repair1165Get(t, table, []byte(key), []byte(value))
	}
}

func TestRepairM_1165_CacheDeletion(t *testing.T) {
	doc := repair1165Text(t, "docs/architecture.md")
	repair1165Contains(t, doc, "segment-driven visibility", "without actively evicting", "validates every cache hit with `base.Exists`", "TestRepair_113_CacheExpiryVisibility", "../dbcache/repair_113_test.go", "asynchronous file deletion")
	for _, mode := range []string{"write", "read"} {
		t.Run(mode, func(t *testing.T) {
			config := repair1165Config(t, t.TempDir())
			config.MaxSegmentKeyCount = 2
			tc := litt.DefaultTableConfig("retention")
			tc.ShardingFactor = 1
			tc.TTL = 10 * time.Second
			var elapsed atomic.Int64
			clock := func() time.Time { return time.Unix(1700000000, 0).Add(time.Duration(elapsed.Load())) }
			db, base := repair1165Open(t, config, tc, clock)
			writeSize, readSize := uint64(0), uint64(1024)
			if mode == "write" {
				writeSize, readSize = readSize, writeSize
			}
			writeCache := util.NewThreadSafeCache(util.NewFIFOCache[string, []byte](writeSize, nil, nil))
			readCache := util.NewThreadSafeCache(util.NewFIFOCache[string, []byte](readSize, nil, nil))
			table := dbcache.NewCachedTable(base.(litt.ManagedTable), writeCache, readCache, nil)
			require.NoError(t, table.Put([]byte("expired"), []byte("payload"), &types.SecondaryKey{Key: []byte("alias"), Offset: 1, Length: 5}))
			require.NoError(t, table.Flush()) // Two keys seal segment zero.
			expiredFiles := filepath.Join(config.Paths[0], tc.Name, "segments", "0*")
			files, err := filepath.Glob(expiredFiles)
			require.NoError(t, err)
			require.NotEmpty(t, files, "the producer must create the segment files before GC")
			for key, want := range map[string]string{"expired": "payload", "alias": "ayloa"} {
				repair1165Get(t, table, []byte(key), []byte(want)) // Populate the read cache when enabled.
				value, found, hot, err := table.CacheAwareGet([]byte(key), true)
				require.NoError(t, err)
				require.True(t, found)
				require.True(t, hot)
				require.Equal(t, []byte(want), value)
			}
			elapsed.Store(int64(11 * time.Second))
			require.NoError(t, table.RunGC())
			for _, key := range []string{"expired", "alias"} {
				cache := readCache
				if mode == "write" {
					cache = writeCache
				}
				_, cached := cache.Get(key)
				require.True(t, cached, "GC hides entries without eagerly reclaiming cache memory")
				found, err := base.Exists([]byte(key))
				require.NoError(t, err)
				require.False(t, found)
				for _, cacheOnly := range []bool{true, false} {
					value, found, hot, err := table.CacheAwareGet([]byte(key), cacheOnly)
					require.NoError(t, err)
					require.False(t, found)
					require.False(t, hot)
					require.Nil(t, value)
				}
			}
			// RunGC completes keymap removal; Segment.Release schedules asynchronous file deletion.
			require.Eventually(t, func() bool {
				remaining, err := filepath.Glob(expiredFiles)
				return err == nil && len(remaining) == 0
			}, 5*time.Second, time.Millisecond, "asynchronous segment deletion must finish")
			files, err = filepath.Glob(expiredFiles)
			require.NoError(t, err)
			require.Empty(t, files, "the expired segment files must be removed too")
			require.NoError(t, table.Put([]byte("live"), []byte("retained")))
			require.NoError(t, db.Close())
			_, recovered := repair1165Open(t, config, tc, clock)
			for _, key := range []string{"expired", "alias"} {
				found, err := recovered.Exists([]byte(key))
				require.NoError(t, err)
				require.False(t, found)
			}
			repair1165Get(t, recovered, []byte("live"), []byte("retained"))
		})
	}
}

func TestRepairM_1165_CLIInstallation(t *testing.T) {
	doc := repair1165Text(t, "docs/littdb_cli.md")
	installation := strings.Split(doc, "# Sources and Destinations")[0]
	repair1165Contains(t, installation, "Paxeer monorepo", "cd storage/db_engine/litt\nmake build\n./bin/litt --help", "alias litt='/path/to/paxeer-monorepo/storage/db_engine/litt/bin/litt'", "export PATH=\"$PATH:/path/to/paxeer-monorepo/storage/db_engine/litt/bin\"", "install -m 0755 storage/db_engine/litt/bin/litt /usr/local/bin/litt")
	require.NotContains(t, installation, "Layr-Labs/eigenda")
	require.Contains(t, repair1165Text(t, "Makefile"), "build:\n\tgo build -o ./bin/litt ./cli\n")
	entry, err := parser.ParseFile(token.NewFileSet(), "cli/main.go", nil, 0)
	require.NoError(t, err)
	require.Equal(t, "main", entry.Name.Name)
	callsParser := false
	ast.Inspect(entry, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "buildCLIParser" {
			callsParser = true
		}
		return true
	})
	require.True(t, callsParser, "the documented build target must enter the real CLI parser")

	// Build target and source selection refer to this implementation's actual on-disk format.
	config := repair1165Config(t, t.TempDir())
	tc := litt.DefaultTableConfig("cli-install")
	tc.ShardingFactor = 1
	db, table := repair1165Open(t, config, tc, nil)
	require.NoError(t, table.Put([]byte("cli-key"), []byte("cli-value")))
	require.NoError(t, db.Close())
	segments := repair1165Segments(t, config, tc.Name)
	keys, err := segments[0].GetKeys()
	require.NoError(t, err)
	require.Len(t, keys, 1)
	value, err := segments[0].Read(keys[0].Key, keys[0].Address)
	require.NoError(t, err)
	require.Equal(t, []byte("cli-key"), keys[0].Key)
	require.Equal(t, []byte("cli-value"), value)
}

func TestRepairM_1165_CLIValidatorConfig(t *testing.T) {
	doc := repair1165Text(t, "docs/littdb_cli.md")
	repair1165Contains(t, doc, "map its configured storage-root list directly to `Config.Paths`", "littbuilder.NewDB(config)", "litt ls --src /data0 --src /data1 --src /data2", "`Config.SnapshotDirectory`", "roots\ncontaining table directories")
	for _, stale := range []string{"NODE_LITT_DB_STORAGE_PATHS", "NODE_DB_PATH", "chunk_v2_litt"} {
		require.NotContains(t, doc, stale)
	}
	require.Contains(t, repair1165Text(t, "cli/table_info.go"), "segment.BuildSegmentPaths(paths, \"\", tableName)")
	require.Contains(t, repair1165Text(t, "cli/ls.go"), "ctx.StringSlice(\"src\")")
	config := repair1165Config(t, t.TempDir(), t.TempDir(), t.TempDir())
	// Config is the node/library interface: deserialize the exact storage-root list before opening.
	encoded, err := json.Marshal(config)
	require.NoError(t, err)
	var nodeConfig litt.Config
	require.NoError(t, json.Unmarshal(encoded, &nodeConfig))
	require.Equal(t, config.Paths, nodeConfig.Paths)
	tc := litt.DefaultTableConfig("selected-roots")
	tc.ShardingFactor = 3
	db, table := repair1165Open(t, &nodeConfig, tc, nil)
	for i := 0; i < 3; i++ {
		require.NoError(t, table.Put([]byte(fmt.Sprintf("key-%d", i)), []byte{byte(i)}))
	}
	require.NoError(t, db.Close())
	for _, root := range nodeConfig.Paths {
		info, err := os.Stat(filepath.Join(root, tc.Name, segment.SegmentDirectory))
		require.NoError(t, err)
		require.True(t, info.IsDir())
		values, err := filepath.Glob(filepath.Join(root, tc.Name, segment.SegmentDirectory, "*.values"))
		require.NoError(t, err)
		require.Len(t, values, 1, "each configured root participates in source selection")
	}
	segments := repair1165Segments(t, &nodeConfig, tc.Name)
	keys, err := segments[0].GetKeys()
	require.NoError(t, err)
	require.Len(t, keys, 3)
	for i, key := range keys {
		value, err := segments[0].Read(key.Key, key.Address)
		require.NoError(t, err)
		require.Equal(t, []byte{byte(i)}, value)
	}
	_, recovered := repair1165Open(t, &nodeConfig, tc, nil)
	for i := 0; i < 3; i++ {
		repair1165Get(t, recovered, []byte(fmt.Sprintf("key-%d", i)), []byte{byte(i)})
	}
}
