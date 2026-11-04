package disktable

import (
	"context"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/disktable/keymap"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/types"
	"github.com/stretchr/testify/require"
)

func repair111Open(t *testing.T, root string, shards uint8) *DiskTable {
	t.Helper()
	config, err := litt.DefaultConfig(root)
	require.NoError(t, err)
	require.Equal(t, uint32(math.MaxUint32), config.TargetSegmentFileSize)
	config.GCPeriod = time.Hour
	config.Fsync = false
	runtime := litt.DefaultRuntimeConfig()
	ctx, cancel := context.WithCancel(runtime.CTX)
	runtime.CTX = ctx
	t.Cleanup(cancel)
	keys, _, err := keymap.NewMemKeymap(runtime.Logger, "", true)
	require.NoError(t, err)
	keymapPath := filepath.Join(root, "rollover", keymap.KeymapDirectoryName)
	keymapType, err := setupKeymapTypeFile(keymapPath, keymap.MemKeymapType)
	require.NoError(t, err)
	tableConfig := litt.DefaultTableConfig("rollover")
	tableConfig.ShardingFactor = shards
	table, err := NewDiskTable(config, runtime, "rollover", tableConfig, keys,
		keymapPath, keymapType, []string{root}, true, nil)
	require.NoError(t, err)
	disk := table.(*DiskTable)
	t.Cleanup(func() { require.NoError(t, disk.Close()) })
	return disk
}

// Sparse padding positions the real writer near the format's limit without allocating or writing
// gigabytes. Only private byte cursors are adjusted; every key, value and address under assertion is
// subsequently produced by Table.Put and consumed by the real segment, keymap and disk reader.
func repair111SparseShard(t *testing.T, table *DiskTable, shard int, size uint64) {
	t.Helper()
	require.NoError(t, table.Flush())
	seg := table.controlLoop.segments[table.controlLoop.threadsafeHighestSegmentIndex.Load()]
	state := reflect.ValueOf(seg).Elem()
	valueFile := state.FieldByName("shards").Index(shard).Elem()
	field := func(v reflect.Value) reflect.Value {
		return reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem()
	}
	file := field(valueFile.FieldByName("file")).Interface().(*os.File)
	require.NoError(t, file.Truncate(int64(size)))
	position, err := file.Seek(int64(size), io.SeekStart)
	require.NoError(t, err)
	require.Equal(t, int64(size), position)
	field(valueFile.FieldByName("size")).SetUint(size)
	field(valueFile.FieldByName("flushedSize")).Addr().Interface().(*atomic.Uint64).Store(size)
	field(state.FieldByName("shardSizes")).Index(shard).SetUint(size)
	if size > seg.GetMaxShardSize() {
		field(state.FieldByName("maxShardSize")).SetUint(size)
	}
	info, err := file.Stat()
	require.NoError(t, err)
	require.Equal(t, int64(size), info.Size())
}

func repair111Read(t *testing.T, table *DiskTable, expected map[string]string) {
	t.Helper()
	require.NoError(t, table.Flush())
	for key, want := range expected {
		_, cached := table.unflushedDataCache.Load(key)
		require.False(t, cached, "lookup must read the segment for %s", key)
		got, exists, err := table.Get([]byte(key))
		require.NoError(t, err, key)
		require.True(t, exists, key)
		require.Equal(t, want, string(got), key)
	}
	require.Equal(t, uint64(len(expected)), table.KeyCount())
	ok, err := table.errorMonitor.IsOk()
	require.True(t, ok)
	require.NoError(t, err)
}

func repair111Address(t *testing.T, table *DiskTable, key string, index uint32, shard uint8,
	offset uint32, length uint32) {
	t.Helper()
	address, exists, err := table.keymap.Get([]byte(key))
	require.NoError(t, err)
	require.True(t, exists, key)
	require.Equal(t, types.NewAddress(index, offset, shard, length), address, key)
}

func TestRepair_111_SegmentCapacityRollover(t *testing.T) {
	root := t.TempDir()
	table := repair111Open(t, root, 1)
	repair111SparseShard(t, table, 0, math.MaxUint32-8)
	require.NoError(t, table.Put([]byte("prior"), []byte("old!")))
	require.NoError(t, table.Put([]byte("edge"), []byte("edge"),
		&types.SecondaryKey{Key: []byte("edge-alias"), Length: 4},
		&types.SecondaryKey{Key: []byte("edge-end"), Offset: 4}))
	require.NoError(t, table.Put([]byte("empty"), []byte{},
		&types.SecondaryKey{Key: []byte("empty-alias")}))
	require.NoError(t, table.Flush())
	require.Equal(t, uint32(0), table.controlLoop.threadsafeHighestSegmentIndex.Load())
	repair111Address(t, table, "prior", 0, 0, math.MaxUint32-8, 4)
	repair111Address(t, table, "edge", 0, 0, math.MaxUint32-4, 4)
	repair111Address(t, table, "edge-end", 0, 0, math.MaxUint32, 0)
	repair111Address(t, table, "empty", 0, 0, math.MaxUint32, 0)

	require.NoError(t, table.PutBatch([]*types.PutRequest{
		{Key: []byte("group"), Value: []byte("abcdef"), SecondaryKeys: []*types.SecondaryKey{
			{Key: []byte("suffix"), Offset: 2, Length: 4},
			{Key: []byte("group-end"), Offset: 6},
		}},
		{Key: []byte("tail"), Value: []byte("tail")},
		{Key: []byte("final"), Value: []byte("final")},
	}))
	expected := map[string]string{
		"prior": "old!", "edge": "edge", "edge-alias": "edge", "edge-end": "",
		"empty": "", "empty-alias": "", "group": "abcdef", "suffix": "cdef",
		"group-end": "", "tail": "tail", "final": "final",
	}
	repair111Read(t, table, expected)
	require.Equal(t, uint32(1), table.controlLoop.threadsafeHighestSegmentIndex.Load())
	repair111Address(t, table, "group", 1, 0, 0, 6)
	repair111Address(t, table, "suffix", 1, 0, 2, 4)
	repair111Address(t, table, "group-end", 1, 0, 6, 0)
	repair111Address(t, table, "tail", 1, 0, 6, 4)
	old := table.controlLoop.segments[0]
	require.True(t, old.IsSealed())
	keys, err := old.GetKeys()
	require.NoError(t, err)
	require.Len(t, keys, 6, "the retried group must leave no partial keys in the old segment")
	require.Equal(t, uint64(math.MaxUint32), old.GetMaxShardSize())
	require.NoError(t, table.Close())
	newKeys, err := table.controlLoop.segments[1].GetKeys()
	require.NoError(t, err)
	require.Len(t, newKeys, 5)
	require.Equal(t, []byte("group"), newKeys[0].Key)
	require.Equal(t, types.KeyKindPrimary, newKeys[0].Kind)
	require.Equal(t, []byte("suffix"), newKeys[1].Key)
	require.Equal(t, types.KeyKindSecondary, newKeys[1].Kind)
	require.Equal(t, []byte("group-end"), newKeys[2].Key)
	require.Equal(t, types.KeyKindFinalSecondary, newKeys[2].Kind)

	reopened := repair111Open(t, root, 1)
	repair111Read(t, reopened, expected)
	repair111Address(t, reopened, "suffix", 1, 0, 2, 4)
	require.NoError(t, reopened.Put([]byte("after-restart"), []byte("live")))
	expected["after-restart"] = "live"
	repair111Read(t, reopened, expected)
}

func TestRepair_111_SelectedShardRollover(t *testing.T) {
	table := repair111Open(t, t.TempDir(), 3)
	repair111SparseShard(t, table, 0, math.MaxUint32-4)
	repair111SparseShard(t, table, 1, math.MaxUint32-6)
	require.NoError(t, table.Put([]byte("full-shard"), []byte("full")))
	require.NoError(t, table.Put([]byte("near-full"), []byte("near")))
	require.NoError(t, table.Put([]byte("roomy-shard"), []byte("roomy")))
	require.NoError(t, table.Flush())
	require.Equal(t, uint32(0), table.controlLoop.threadsafeHighestSegmentIndex.Load(),
		"a full unrelated shard must not force rotation")
	repair111Address(t, table, "full-shard", 0, 0, math.MaxUint32-4, 4)
	repair111Address(t, table, "near-full", 0, 1, math.MaxUint32-6, 4)
	repair111Address(t, table, "roomy-shard", 0, 2, 0, 5)
	require.NoError(t, table.Put([]byte("rotated"), []byte("rotate"),
		&types.SecondaryKey{Key: []byte("alias"), Offset: 1, Length: 5}))
	require.NoError(t, table.Put([]byte("next-shard"), []byte("next")))
	repair111Read(t, table, map[string]string{
		"full-shard": "full", "near-full": "near", "roomy-shard": "roomy",
		"rotated": "rotate", "alias": "otate", "next-shard": "next",
	})
	require.Equal(t, uint32(1), table.controlLoop.threadsafeHighestSegmentIndex.Load())
	repair111Address(t, table, "rotated", 1, 0, 0, 6)
	repair111Address(t, table, "alias", 1, 0, 1, 5)
	repair111Address(t, table, "next-shard", 1, 1, 0, 4)
	keys, err := table.controlLoop.segments[0].GetKeys()
	require.NoError(t, err)
	require.Len(t, keys, 3)
}

func TestRepair_111_GroupRolloverWithRemainingCapacity(t *testing.T) {
	table := repair111Open(t, t.TempDir(), 1)
	repair111SparseShard(t, table, 0, math.MaxUint32-8)
	require.NoError(t, table.Put([]byte("anchor"), []byte("keep")))
	require.NoError(t, table.Flush())
	require.Equal(t, uint64(math.MaxUint32-4), table.controlLoop.segments[0].GetMaxShardSize())
	require.NoError(t, table.Put([]byte("whole"), []byte("abcdef"),
		&types.SecondaryKey{Key: []byte("prefix"), Length: 3},
		&types.SecondaryKey{Key: []byte("last-byte"), Offset: 5, Length: 1}))
	repair111Read(t, table, map[string]string{
		"anchor": "keep", "whole": "abcdef", "prefix": "abc", "last-byte": "f",
	})
	require.Equal(t, uint32(1), table.controlLoop.threadsafeHighestSegmentIndex.Load())
	repair111Address(t, table, "whole", 1, 0, 0, 6)
	repair111Address(t, table, "prefix", 1, 0, 0, 3)
	repair111Address(t, table, "last-byte", 1, 0, 5, 1)
	keys, err := table.controlLoop.segments[0].GetKeys()
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.Equal(t, []byte("anchor"), keys[0].Key)
	require.Equal(t, uint64(math.MaxUint32-4), table.controlLoop.segments[0].GetMaxShardSize())
}

func TestRepair_111_CapacityPreflight(t *testing.T) {
	table := repair111Open(t, t.TempDir(), 2)
	repair111SparseShard(t, table, 0, math.MaxUint32-4)
	seg := table.controlLoop.segments[0]
	primaryOverflow := &types.PutRequest{Key: []byte("too-long"), Value: []byte("12345")}
	secondaryOverflow := &types.PutRequest{Key: []byte("bad-range"), Value: []byte("1234"),
		SecondaryKeys: []*types.SecondaryKey{{Key: []byte("bad-alias"), Offset: 4, Length: 1}}}
	for _, request := range []*types.PutRequest{primaryOverflow, secondaryOverflow} {
		for i := 0; i < 2; i++ {
			require.False(t, seg.HasCapacity(request))
		}
		_, _, err := seg.Write(request)
		require.Error(t, err)
		require.Equal(t, uint32(0), seg.KeyCount())
		require.Equal(t, uint64(math.MaxUint32-4), seg.GetMaxShardSize())
	}
	valid := &types.PutRequest{Key: []byte("exact-fit"), Value: []byte("fits"),
		SecondaryKeys: []*types.SecondaryKey{{Key: []byte("end"), Offset: 4}}}
	require.True(t, seg.HasCapacity(valid))
	require.True(t, seg.HasCapacity(valid))
	require.NoError(t, table.Put(valid.Key, valid.Value, valid.SecondaryKeys...))
	repair111Read(t, table, map[string]string{"exact-fit": "fits", "end": ""})
	repair111Address(t, table, "exact-fit", 0, 0, math.MaxUint32-4, 4)
	repair111Address(t, table, "end", 0, 0, math.MaxUint32, 0)
	require.NoError(t, table.Close())
	require.False(t, seg.HasCapacity(valid), "sealed segments cannot accept writes")
	keys, err := seg.GetKeys()
	require.NoError(t, err)
	require.Len(t, keys, 2, "rejected groups must not produce key records")
}
