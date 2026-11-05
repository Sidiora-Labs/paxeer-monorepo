package store_test

import (
	"testing"

	gigastore "github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/dbadapter"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func repair94Backing(t *testing.T, values map[string]string) types.KVStore {
	t.Helper()
	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	backing := dbadapter.Store{DB: db}
	for key, value := range values {
		backing.Set([]byte(key), []byte(value))
	}
	return backing
}

func repair94AssertState(t *testing.T, store types.KVStore, values map[string]string) {
	t.Helper()
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	require.ElementsMatch(t, keys, store.GetAllKeyStrsInRange(nil, nil))
	for _, key := range []string{"a", "b", "c", "d", "m", "t", "z", "\xff"} {
		if value, ok := values[key]; ok {
			require.Equal(t, []byte(value), store.Get([]byte(key)), "key %q", key)
			require.True(t, store.Has([]byte(key)), "key %q", key)
		} else {
			require.Nil(t, store.Get([]byte(key)), "key %q", key)
			require.False(t, store.Has([]byte(key)), "key %q", key)
		}
	}
}

func repair94Nested(t *testing.T) (types.KVStore, *gigastore.Store, types.CacheKVStore) {
	t.Helper()
	backing := repair94Backing(t, map[string]string{
		"a": "base-a", "d": "base-d", "m": "base-m", "t": "base-t", "z": "base-z",
	})
	key := types.NewKVStoreKey("repair94")
	outer := gigastore.NewStore(backing, key, 0)
	outer.Set([]byte("b"), []byte("outer-b"))
	outer.Set([]byte("m"), []byte("outer-m"))
	outer.Delete([]byte("d"))
	child := outer.CacheWrap(key).(types.CacheKVStore)
	child.Set([]byte("c"), []byte("child-c"))
	child.Set([]byte("m"), []byte("child-m"))
	child.Set([]byte{0xff}, []byte("child-high"))
	child.Delete([]byte("b"))
	child.Delete([]byte("t"))
	return backing, outer, child
}

func TestRepair_94_NestedRange(t *testing.T) {
	backing, outer, child := repair94Nested(t)
	initial := map[string]string{
		"a": "base-a", "d": "base-d", "m": "base-m", "t": "base-t", "z": "base-z",
	}
	merged := map[string]string{
		"a": "base-a", "c": "child-c", "m": "child-m", "z": "base-z", "\xff": "child-high",
	}
	reference := repair94Backing(t, merged)
	checkRanges := func(t *testing.T, store types.KVStore) {
		t.Helper()
		for _, tc := range []struct {
			name       string
			start, end []byte
			want       []string
		}{
			{"unbounded", nil, nil, []string{"a", "c", "m", "z", "\xff"}},
			{"tail", []byte("m"), nil, []string{"m", "z", "\xff"}},
			{"head", nil, []byte("m"), []string{"a", "c"}},
			{"bounded", []byte("c"), []byte("z"), []string{"c", "m"}},
			{"high", []byte{0xff}, nil, []string{"\xff"}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				require.ElementsMatch(t, tc.want, reference.GetAllKeyStrsInRange(tc.start, tc.end))
				require.ElementsMatch(t, tc.want, store.GetAllKeyStrsInRange(tc.start, tc.end))
			})
		}
		repair94AssertState(t, store, merged)
	}

	repair94AssertState(t, backing, initial)
	repair94AssertState(t, outer, map[string]string{
		"a": "base-a", "b": "outer-b", "m": "outer-m", "t": "base-t", "z": "base-z",
	})
	t.Run("pending", func(t *testing.T) { checkRanges(t, child) })
	child.Write()
	t.Run("child-written", func(t *testing.T) {
		checkRanges(t, child)
		checkRanges(t, outer)
	})
	repair94AssertState(t, backing, initial)
	outer.Write()
	t.Run("outer-written", func(t *testing.T) {
		checkRanges(t, backing)
		checkRanges(t, outer)
		checkRanges(t, outer.CacheWrap(types.NewKVStoreKey("fresh")).(types.CacheKVStore))
	})
}

func TestRepair_94_NestedDeleteAll(t *testing.T) {
	for _, tc := range []struct {
		name       string
		start, end []byte
		remaining  map[string]string
	}{
		{"unbounded", nil, nil, map[string]string{}},
		{"tail", []byte("m"), nil, map[string]string{"a": "base-a", "c": "child-c"}},
		{"head", nil, []byte("m"), map[string]string{"m": "child-m", "z": "base-z", "\xff": "child-high"}},
		{"bounded", []byte("m"), []byte("z"), map[string]string{"a": "base-a", "c": "child-c", "z": "base-z", "\xff": "child-high"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			backing, outer, child := repair94Nested(t)
			merged := map[string]string{
				"a": "base-a", "c": "child-c", "m": "child-m", "z": "base-z", "\xff": "child-high",
			}
			reference := repair94Backing(t, merged)
			repair94AssertState(t, child, merged)
			require.NoError(t, reference.DeleteAll(tc.start, tc.end))
			require.NoError(t, child.DeleteAll(tc.start, tc.end))
			repair94AssertState(t, reference, tc.remaining)
			repair94AssertState(t, child, tc.remaining)
			repair94AssertState(t, outer, map[string]string{
				"a": "base-a", "b": "outer-b", "m": "outer-m", "t": "base-t", "z": "base-z",
			})
			child.Write()
			repair94AssertState(t, child, tc.remaining)
			repair94AssertState(t, outer, tc.remaining)
			repair94AssertState(t, backing, map[string]string{
				"a": "base-a", "d": "base-d", "m": "base-m", "t": "base-t", "z": "base-z",
			})
			outer.Write()
			repair94AssertState(t, backing, tc.remaining)
			repair94AssertState(t, outer, tc.remaining)
		})
	}
}
