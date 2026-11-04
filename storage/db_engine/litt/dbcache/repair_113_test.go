package dbcache

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/disktable"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/disktable/keymap"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/util"
	"github.com/stretchr/testify/require"
)

func TestRepair_113_CacheExpiryVisibility(t *testing.T) {
	for _, cache := range []struct {
		name  string
		write uint64
		read  uint64
	}{
		{name: "write", write: 64},
		{name: "read", read: 64},
		{name: "both", write: 64, read: 64},
		{name: "disk"},
	} {
		for _, batch := range []bool{false, true} {
			for _, baseGC := range []bool{false, true} {
				name := cache.name + "/put"
				if batch {
					name += "batch"
				}
				if baseGC {
					name += "/base_gc"
				} else {
					name += "/wrapper_gc"
				}
				t.Run(name, func(t *testing.T) {
					repair113ExpiryVisibility(t, cache.write, cache.read, batch, baseGC)
				})
			}
		}
	}
}

func repair113ExpiryVisibility(t *testing.T, writeSize, readSize uint64, batch, baseGC bool) {
	t.Helper()
	directory := t.TempDir()
	start := time.Unix(1_700_000_000, 0)
	var elapsed atomic.Int64
	clock := func() time.Time { return start.Add(time.Duration(elapsed.Load())) }

	open := func() (*cachedTable, litt.ManagedTable) {
		t.Helper()
		config, err := litt.DefaultConfig(directory)
		require.NoError(t, err)
		config.Fsync = false
		config.GCPeriod = time.Hour
		config.MaxSegmentKeyCount = 3
		runtimeConfig := litt.DefaultRuntimeConfig()
		runtimeConfig.Clock = clock
		tableConfig := litt.DefaultTableConfig("repair113")
		tableConfig.ShardingFactor = 1
		tableConfig.TTL = 10 * time.Second
		keymapPath := filepath.Join(directory, tableConfig.Name, keymap.KeymapDirectoryName)
		require.NoError(t, os.MkdirAll(keymapPath, 0750))
		typeFileExists, err := keymap.KeymapFileExists(keymapPath)
		require.NoError(t, err)
		var typeFile *keymap.KeymapTypeFile
		if typeFileExists {
			typeFile, err = keymap.LoadKeymapTypeFile(keymapPath)
			require.NoError(t, err)
			require.Equal(t, keymap.KeymapType(keymap.MemKeymapType), typeFile.Type())
		} else {
			typeFile = keymap.NewKeymapTypeFile(keymapPath, keymap.MemKeymapType)
			require.NoError(t, typeFile.Write())
		}
		keys, reload, err := keymap.NewMemKeymap(runtimeConfig.Logger, "", true)
		require.NoError(t, err)
		base, err := disktable.NewDiskTable(config, runtimeConfig, tableConfig.Name, tableConfig,
			keys, keymapPath, typeFile, config.Paths, reload, nil)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, base.Close()) })
		writeCache := util.NewThreadSafeCache(util.NewFIFOCache[string, []byte](writeSize, nil, nil))
		readCache := util.NewThreadSafeCache(util.NewFIFOCache[string, []byte](readSize, nil, nil))
		cached := NewCachedTable(base, writeCache, readCache, nil).(*cachedTable)
		return cached, base
	}

	put := func(table litt.ManagedTable, prefix string) map[string][]byte {
		t.Helper()
		value := []byte("0123456789")
		request := &types.PutRequest{
			Key: []byte(prefix), Value: value,
			SecondaryKeys: []*types.SecondaryKey{
				{Key: []byte(prefix + "-whole"), Length: uint32(len(value))},
				{Key: []byte(prefix + "-slice"), Offset: 2, Length: 4},
			},
		}
		if batch {
			require.NoError(t, table.PutBatch([]*types.PutRequest{request}))
		} else {
			require.NoError(t, table.Put(request.Key, request.Value, request.SecondaryKeys...))
		}
		require.NoError(t, table.Flush())
		return map[string][]byte{prefix: value, prefix + "-whole": value, prefix + "-slice": value[2:6]}
	}

	table, base := open()
	expired := put(table, "expired")
	repair113Present(t, table, expired, writeSize > 0, readSize > 0)
	elapsed.Store(int64(5 * time.Second))
	live := put(table, "live")
	repair113Present(t, table, live, writeSize > 0, readSize > 0)
	require.Equal(t, uint64(6), table.KeyCount())
	elapsed.Store(int64(11 * time.Second))

	// Expiration is lazy: an elapsed TTL alone must not hide retained disk keys.
	repair113Present(t, table, expired, writeSize > 0 || readSize > 0, readSize > 0)
	if baseGC {
		require.NoError(t, base.RunGC())
	} else {
		require.NoError(t, table.RunGC())
	}
	if writeSize > 0 {
		require.Equal(t, 6, table.writeCache.Size(), "the idle cache still holds expired entries")
	} else if readSize > 0 {
		require.Equal(t, 6, table.readCache.Size(), "the idle cache still holds expired entries")
	}
	repair113Absent(t, base, expired)
	repair113Absent(t, table, expired)
	repair113Present(t, table, live, writeSize > 0 || readSize > 0, readSize > 0)
	repair113Iteration(t, base, live)
	repair113Iteration(t, table, live)
	require.NoError(t, table.Close())

	table, base = open()
	repair113Absent(t, base, expired)
	repair113Absent(t, table, expired)
	repair113Present(t, table, live, false, readSize > 0)
	repair113Iteration(t, table, live)

	// Recovered keys enter the read cache and must disappear when GC removes them too.
	elapsed.Store(int64(21 * time.Second))
	require.NoError(t, table.RunGC())
	repair113Absent(t, base, live)
	repair113Absent(t, table, live)
	repair113Iteration(t, table, nil)
}

func repair113Present(t *testing.T, table litt.ManagedTable, expected map[string][]byte, initiallyHot, readCache bool) {
	t.Helper()
	for key, want := range expected {
		exists, err := table.Exists([]byte(key))
		require.NoError(t, err)
		require.True(t, exists, key)
		value, exists, hot, err := table.CacheAwareGet([]byte(key), true)
		require.NoError(t, err)
		require.True(t, exists, key)
		require.Equal(t, initiallyHot, hot, key)
		if initiallyHot {
			require.Equal(t, want, value, key)
		} else {
			require.Nil(t, value, key)
		}
		value, exists, hot, err = table.CacheAwareGet([]byte(key), false)
		require.NoError(t, err)
		require.True(t, exists, key)
		require.Equal(t, initiallyHot, hot, key)
		require.Equal(t, want, value, key)
		value, exists, hot, err = table.CacheAwareGet([]byte(key), true)
		require.NoError(t, err)
		require.True(t, exists, key)
		require.Equal(t, initiallyHot || readCache, hot, key)
		if hot {
			require.Equal(t, want, value, key)
		} else {
			require.Nil(t, value, key)
		}
	}
}

func repair113Absent(t *testing.T, table litt.ManagedTable, expected map[string][]byte) {
	t.Helper()
	for key := range expected {
		exists, err := table.Exists([]byte(key))
		require.NoError(t, err)
		require.False(t, exists, key)
		value, exists, err := table.Get([]byte(key))
		require.NoError(t, err)
		require.False(t, exists, key)
		require.Nil(t, value, key)
		for _, cacheOnly := range []bool{true, false} {
			value, exists, hot, err := table.CacheAwareGet([]byte(key), cacheOnly)
			require.NoError(t, err)
			require.False(t, exists, key)
			require.False(t, hot, key)
			require.Nil(t, value, key)
		}
	}
}

func repair113Iteration(t *testing.T, table litt.ManagedTable, expected map[string][]byte) {
	t.Helper()
	require.Equal(t, uint64(len(expected)), table.KeyCount())
	for _, reverse := range []bool{false, true} {
		iterator, err := table.Iterator(reverse)
		require.NoError(t, err)
		func() {
			defer func() { require.NoError(t, iterator.Close()) }()
			seen := make(map[string]bool)
			for {
				next, err := iterator.Next()
				require.NoError(t, err)
				if !next {
					break
				}
				key, primary := iterator.GetKey()
				want, exists := expected[string(key)]
				require.True(t, exists, string(key))
				require.False(t, seen[string(key)], "duplicate key")
				require.Equal(t, string(key) == "live", primary)
				value, err := iterator.GetValue()
				require.NoError(t, err)
				require.Equal(t, want, value)
				seen[string(key)] = true
			}
			require.Len(t, seen, len(expected))
		}()
	}
}
