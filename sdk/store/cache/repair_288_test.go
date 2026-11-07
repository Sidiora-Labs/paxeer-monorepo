package cache

import (
	"bytes"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/mem"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	"github.com/stretchr/testify/require"
)

func TestRepair_288_RangeRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name       string
		start, end []byte
	}{
		{name: "partial", start: []byte("b"), end: []byte("d")},
		{name: "unbounded_start", end: []byte("c")},
		{name: "unbounded_end", start: []byte("c")},
		{name: "all"},
		{name: "empty", start: []byte("c"), end: []byte("c")},
		{name: "absent", start: []byte("f"), end: []byte("x")},
		{name: "binary", start: []byte{0x80}, end: []byte{0xff}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parent := mem.NewStore()
			t.Cleanup(func() { require.NoError(t, parent.DB.Close()) })
			cached := NewCommitKVStoreCache(parent, 64, types.DefaultCacheSizeLimit)
			var store types.CommitKVStore = cached
			keys := []string{"a", "b", "c", "d", "e", "\x80", "\xff"}
			misses := []string{"bb", "cc", "y"}
			for _, key := range keys {
				parent.Set([]byte(key), []byte("value:"+key))
				require.Equal(t, parent.Get([]byte(key)), store.Get([]byte(key)))
			}
			for _, key := range misses {
				require.Nil(t, store.Get([]byte(key)))
			}
			for _, key := range append(append([]string{}, keys...), misses...) {
				require.True(t, cached.cache.Contains(key), "read must populate cache: %q", key)
			}

			require.NoError(t, store.DeleteAll(tc.start, tc.end))
			inRange := func(key string) bool {
				return (tc.start == nil || bytes.Compare([]byte(key), tc.start) >= 0) &&
					(tc.end == nil || bytes.Compare([]byte(key), tc.end) < 0)
			}
			for _, key := range append(append([]string{}, keys...), misses...) {
				require.Equal(t, !inRange(key), cached.cache.Contains(key), "range invalidation: %q", key)
			}
			want := make(map[string]string)
			for _, key := range keys {
				if !inRange(key) {
					want[key] = "value:" + key
				}
			}
			repair288AssertState(t, parent, want, keys, misses)
			repair288AssertState(t, store, want, keys, misses)

			// Repeated deletion must also evict the negative entries read above.
			require.NoError(t, store.DeleteAll(tc.start, tc.end))
			for _, key := range misses {
				if inRange(key) {
					require.False(t, cached.cache.Contains(key))
					parent.Set([]byte(key), []byte("new:"+key))
					want[key] = "new:" + key
					require.Equal(t, []byte(want[key]), store.Get([]byte(key)))
				}
			}
			for _, key := range keys {
				if inRange(key) {
					store.Set([]byte(key), []byte("replacement:"+key))
					want[key] = "replacement:" + key
				}
			}
			repair288AssertState(t, parent, want, keys, misses)
			repair288AssertState(t, store, want, keys, misses)
		})
	}
}

func TestRepair_288_ResetRoundTrip(t *testing.T) {
	parent := mem.NewStore()
	t.Cleanup(func() { require.NoError(t, parent.DB.Close()) })
	manager := NewCommitKVStoreCacheManager(64, types.DefaultCacheSizeLimit)
	key := types.NewKVStoreKey("repair288")
	store := manager.GetStoreCache(key, parent)
	store.Set([]byte("a"), []byte("keep"))
	store.Set([]byte("b"), []byte("remove"))
	require.Equal(t, []byte("remove"), store.Get([]byte("b")))
	require.Nil(t, store.Get([]byte("c")))
	require.NoError(t, store.DeleteAll([]byte("b"), []byte("c")))
	repair288AssertState(t, store, map[string]string{"a": "keep"}, []string{"a", "b", "c"})

	parent.Set([]byte("a"), []byte("updated"))
	parent.Set([]byte("c"), []byte("created"))
	manager.Reset()
	require.Zero(t, store.(*CommitKVStoreCache).cache.Len())
	require.Same(t, parent, manager.Unwrap(key))
	require.Same(t, store, manager.GetStoreCache(key, parent))
	want := map[string]string{"a": "updated", "c": "created"}
	repair288AssertState(t, store, want, []string{"a", "b", "c"})
	repair288AssertState(t, parent, want, []string{"a", "b", "c"})

	require.NoError(t, store.DeleteAll(nil, nil))
	store.(*CommitKVStoreCache).Reset()
	repair288AssertState(t, store, map[string]string{}, []string{"a", "b", "c"})
	repair288AssertState(t, parent, map[string]string{}, []string{"a", "b", "c"})
}

func repair288AssertState(t *testing.T, store types.KVStore, want map[string]string, keySets ...[]string) {
	t.Helper()
	for _, keys := range keySets {
		for _, key := range keys {
			value, exists := want[key]
			require.Equal(t, exists, store.Has([]byte(key)), "Has(%q)", key)
			if exists {
				require.Equal(t, []byte(value), store.Get([]byte(key)), "Get(%q)", key)
			} else {
				require.Nil(t, store.Get([]byte(key)), "Get(%q)", key)
			}
		}
	}
	for _, reverse := range []bool{false, true} {
		var iter types.Iterator
		if reverse {
			iter = store.ReverseIterator(nil, nil)
		} else {
			iter = store.Iterator(nil, nil)
		}
		got := make(map[string]string)
		var previous []byte
		for ; iter.Valid(); iter.Next() {
			key := iter.Key()
			if previous != nil {
				if reverse {
					require.Positive(t, bytes.Compare(previous, key))
				} else {
					require.Negative(t, bytes.Compare(previous, key))
				}
			}
			previous = append([]byte(nil), key...)
			got[string(key)] = string(iter.Value())
		}
		require.NoError(t, iter.Close())
		require.Equal(t, want, got)
	}
	keys := make([]string, 0, len(want))
	for key := range want {
		keys = append(keys, key)
	}
	require.ElementsMatch(t, keys, store.GetAllKeyStrsInRange(nil, nil))
}
