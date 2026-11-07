package cachekv_test

import (
	"testing"

	enginestore "github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/cachekv"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/dbadapter"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/prefix"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func repair289Backing(t *testing.T) types.KVStore {
	t.Helper()
	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	return dbadapter.Store{DB: db}
}

func repair289Cache(parent types.KVStore, engine bool) types.CacheKVStore {
	key := types.NewKVStoreKey("repair289")
	if engine {
		return enginestore.NewStore(parent, key, types.DefaultCacheSizeLimit)
	}
	return cachekv.NewStore(parent, key, types.DefaultCacheSizeLimit)
}

func repair289Seed(parent types.KVStore) {
	for _, key := range []string{"a", "b", "d", "m", "z"} {
		parent.Set([]byte(key), []byte("parent-"+key))
	}
}

func repair289Mutate(store types.KVStore) {
	store.Delete([]byte("b"))
	store.Set([]byte("c"), []byte("insert-c"))
	store.Set([]byte("d"), []byte("update-d"))
	store.Delete([]byte("m"))
	store.Set([]byte("m"), []byte("restore-m"))
	store.Set([]byte("t"), []byte("insert-t"))
	store.Delete([]byte("t"))
	store.Set([]byte{0xff}, []byte("high"))
	store.Set([]byte{0xff, 0xff}, []byte("higher"))
}

func repair289Values() map[string]string {
	return map[string]string{
		"a": "parent-a", "c": "insert-c", "d": "update-d", "m": "restore-m",
		"z": "parent-z", "\xff": "high", "\xff\xff": "higher",
	}
}

func repair289AssertState(t *testing.T, store types.KVStore, want map[string]string) {
	t.Helper()
	keys := make([]string, 0, len(want))
	for key := range want {
		keys = append(keys, key)
	}
	require.ElementsMatch(t, keys, store.GetAllKeyStrsInRange(nil, nil))
	for _, key := range []string{"a", "b", "c", "d", "m", "t", "z", "\xff", "\xff\xff", "missing"} {
		value, exists := want[key]
		require.Equal(t, exists, store.Has([]byte(key)), "key %q", key)
		if exists {
			require.Equal(t, []byte(value), store.Get([]byte(key)), "key %q", key)
		} else {
			require.Nil(t, store.Get([]byte(key)), "key %q", key)
		}
	}
}

func repair289AssertRange(t *testing.T, store types.KVStore, start, end []byte, want []string) {
	t.Helper()
	require.ElementsMatch(t, want, store.GetAllKeyStrsInRange(start, end))
	// Engine caches explicitly do not support iterators. SDK caches and the real
	// backing store must agree with both iterator directions, including values.
	if _, engine := store.(*enginestore.Store); engine {
		return
	}
	for _, iterator := range []func([]byte, []byte) types.Iterator{store.Iterator, store.ReverseIterator} {
		iter := iterator(start, end)
		keys := make([]string, 0)
		for ; iter.Valid(); iter.Next() {
			keys = append(keys, string(iter.Key()))
			require.Equal(t, store.Get(iter.Key()), iter.Value())
		}
		require.NoError(t, iter.Close())
		require.ElementsMatch(t, want, keys)
	}
}

func TestRepair_289_RangeMerge(t *testing.T) {
	for _, implementation := range []string{"sdk", "engine"} {
		t.Run(implementation, func(t *testing.T) {
			parent := repair289Backing(t)
			repair289Seed(parent)
			store := repair289Cache(parent, implementation == "engine")
			repair289Mutate(store)
			check := func(t *testing.T, view types.KVStore) {
				t.Helper()
				repair289AssertState(t, view, repair289Values())
				for _, tc := range []struct {
					name       string
					start, end []byte
					want       []string
				}{
					{"unbounded", nil, nil, []string{"a", "c", "d", "m", "z", "\xff", "\xff\xff"}},
					{"tail", []byte("m"), nil, []string{"m", "z", "\xff", "\xff\xff"}},
					{"head", nil, []byte("d"), []string{"a", "c"}},
					{"bounded", []byte("c"), []byte("z"), []string{"c", "d", "m"}},
					{"high-tail", []byte{0xff}, nil, []string{"\xff", "\xff\xff"}},
					{"high-bounded", []byte{0xff}, []byte{0xff, 0xff}, []string{"\xff"}},
					{"empty", []byte("n"), []byte("s"), nil},
				} {
					t.Run(tc.name, func(t *testing.T) {
						repair289AssertRange(t, view, tc.start, tc.end, tc.want)
					})
				}
			}
			t.Run("pending", func(t *testing.T) { check(t, store) })
			require.Equal(t, []byte("parent-b"), parent.Get([]byte("b")))
			require.Nil(t, parent.Get([]byte("c")))
			store.Write()
			t.Run("written", func(t *testing.T) { check(t, parent); check(t, store) })
			t.Run("fresh-cache", func(t *testing.T) { check(t, repair289Cache(parent, implementation == "engine")) })
		})
	}
}

func TestRepair_289_DeleteAll(t *testing.T) {
	for _, implementation := range []string{"sdk", "engine"} {
		t.Run(implementation, func(t *testing.T) {
			for _, tc := range []struct {
				name       string
				start, end []byte
				remaining  map[string]string
			}{
				{"unbounded", nil, nil, map[string]string{}},
				{"tail", []byte("m"), nil, map[string]string{"a": "parent-a", "c": "insert-c", "d": "update-d"}},
				{"head", nil, []byte("d"), map[string]string{"d": "update-d", "m": "restore-m", "z": "parent-z", "\xff": "high", "\xff\xff": "higher"}},
				{"bounded", []byte("c"), []byte("z"), map[string]string{"a": "parent-a", "z": "parent-z", "\xff": "high", "\xff\xff": "higher"}},
				{"high-tail", []byte{0xff}, nil, map[string]string{"a": "parent-a", "c": "insert-c", "d": "update-d", "m": "restore-m", "z": "parent-z"}},
			} {
				t.Run(tc.name, func(t *testing.T) {
					parent := repair289Backing(t)
					repair289Seed(parent)
					outer := repair289Cache(parent, implementation == "engine")
					store := outer.CacheWrap(types.NewKVStoreKey("child")).(types.CacheKVStore)
					repair289Mutate(store)
					require.NoError(t, store.DeleteAll(tc.start, tc.end))
					repair289AssertState(t, store, tc.remaining)
					repair289AssertRange(t, store, tc.start, tc.end, nil)
					initial := map[string]string{"a": "parent-a", "b": "parent-b", "d": "parent-d", "m": "parent-m", "z": "parent-z"}
					repair289AssertState(t, outer, initial)
					store.Write()
					repair289AssertState(t, outer, tc.remaining)
					repair289AssertState(t, store, tc.remaining)
					repair289AssertState(t, parent, initial)
					outer.Write()
					repair289AssertState(t, parent, tc.remaining)
					repair289AssertState(t, store, tc.remaining)
					repair289AssertRange(t, parent, tc.start, tc.end, nil)
				})
			}
		})
	}
}

func TestRepair_289_PrefixDeleteAll(t *testing.T) {
	for _, implementation := range []string{"sdk", "engine"} {
		t.Run(implementation, func(t *testing.T) {
			parent := repair289Backing(t)
			parent.Set([]byte("p/b"), []byte("deleted-parent"))
			parent.Set([]byte("q/a"), []byte("outside"))
			store := repair289Cache(parent, implementation == "engine")
			view := prefix.NewStore(store, []byte("p/"))
			view.Set([]byte("a"), []byte("pending"))
			view.Delete([]byte("b"))
			require.Equal(t, []byte("pending"), view.Get([]byte("a")))
			require.Nil(t, view.Get([]byte("b")))
			repair289AssertRange(t, store, []byte("p/"), []byte("p0"), []string{"p/a"})
			require.NoError(t, view.DeleteAll(nil, nil))
			require.Nil(t, view.Get([]byte("a")))
			require.Empty(t, store.GetAllKeyStrsInRange([]byte("p/"), []byte("p0")))
			require.Equal(t, []byte("deleted-parent"), parent.Get([]byte("p/b")))
			store.Write()
			require.ElementsMatch(t, []string{"q/a"}, parent.GetAllKeyStrsInRange(nil, nil))
			require.Equal(t, []byte("outside"), parent.Get([]byte("q/a")))
			require.Nil(t, parent.Get([]byte("p/a")))
			require.Nil(t, parent.Get([]byte("p/b")))
		})
	}
}
