package cachekv_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/cachekv"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/dbadapter"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	"github.com/stretchr/testify/require"
	"github.com/syndtr/goleveldb/leveldb"
	dbm "github.com/tendermint/tm-db"
)

func repairM635Cache(parent types.KVStore) *cachekv.Store {
	return cachekv.NewStore(parent, types.NewKVStoreKey("repair-m635"), types.DefaultCacheSizeLimit)
}

func repairM635Memory(t *testing.T) types.KVStore {
	t.Helper()
	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	return dbadapter.Store{DB: db}
}

func repairM635Iterator(store types.KVStore, reverse bool, start, end []byte) types.Iterator {
	if reverse {
		return store.ReverseIterator(start, end)
	}
	return store.Iterator(start, end)
}

func repairM635InvalidPosition(t *testing.T, iter types.Iterator) {
	t.Helper()
	require.False(t, iter.Valid())
	for _, operation := range []struct {
		name string
		call func()
	}{
		{"Key", func() { iter.Key() }},
		{"Value", func() { iter.Value() }},
		{"Next", iter.Next},
	} {
		t.Run(operation.name, func(t *testing.T) {
			require.PanicsWithError(t, "invalid cacheMergeIterator", operation.call)
		})
	}
	require.False(t, iter.Valid())
}

func TestRepairM_635_Empty(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		for _, scenario := range []string{"empty", "deleted-parent", "cache-tombstone", "outside-domain"} {
			name := "forward/" + scenario
			if reverse {
				name = "reverse/" + scenario
			}
			t.Run(name, func(t *testing.T) {
				parent := repairM635Memory(t)
				store := repairM635Cache(parent)
				switch scenario {
				case "deleted-parent":
					parent.Set([]byte("b"), []byte("parent-b"))
					store.Delete([]byte("b"))
				case "cache-tombstone":
					store.Delete([]byte("b"))
				case "outside-domain":
					parent.Set([]byte("a"), []byte("parent-a"))
					store.Set([]byte("z"), []byte("cache-z"))
				}
				iter := repairM635Iterator(repairM635Cache(store), reverse, []byte("b"), []byte("f"))
				t.Cleanup(func() { require.NoError(t, iter.Close()) })
				require.NoError(t, iter.Error())
				repairM635InvalidPosition(t, iter)
				require.NoError(t, iter.Error())
				require.NoError(t, iter.Error())
			})
		}
	}
}

func TestRepairM_635_Exhausted(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		for _, bounded := range []bool{false, true} {
			name := "forward"
			if reverse {
				name = "reverse"
			}
			if bounded {
				name += "/bounded"
			} else {
				name += "/unbounded"
			}
			t.Run(name, func(t *testing.T) {
				parent := repairM635Memory(t)
				for _, key := range []string{"a", "b", "d", "g"} {
					parent.Set([]byte(key), []byte("parent-"+key))
				}
				store := repairM635Cache(parent)
				store.Set([]byte("b"), []byte("cache-b"))
				store.Set([]byte("c"), []byte("cache-c"))
				store.Set([]byte("z"), []byte("cache-z"))
				store.Delete([]byte("d"))
				store.Delete([]byte("y"))
				var start, end []byte
				want := []string{"a=parent-a", "b=cache-b", "c=cache-c", "g=parent-g", "z=cache-z"}
				if bounded {
					start, end = []byte("b"), []byte("f")
					want = []string{"b=cache-b", "c=cache-c"}
				}
				if reverse {
					for i, j := 0, len(want)-1; i < j; i, j = i+1, j-1 {
						want[i], want[j] = want[j], want[i]
					}
				}
				iter := repairM635Iterator(repairM635Cache(store), reverse, start, end)
				t.Cleanup(func() { require.NoError(t, iter.Close()) })
				gotStart, gotEnd := iter.Domain()
				require.Equal(t, start, gotStart)
				require.Equal(t, end, gotEnd)
				var got []string
				for ; iter.Valid(); iter.Next() {
					key, value := string(iter.Key()), string(iter.Value())
					require.NoError(t, iter.Error())
					require.Equal(t, key, string(iter.Key()))
					require.Equal(t, value, string(iter.Value()))
					got = append(got, key+"="+value)
				}
				require.Equal(t, want, got)
				require.NoError(t, iter.Error())
				repairM635InvalidPosition(t, iter)
				require.NoError(t, iter.Error())
			})
		}
	}
}

func repairM635ClosedBackend(t *testing.T) types.KVStore {
	t.Helper()
	db, err := dbm.NewGoLevelDB("repair-m635", t.TempDir())
	require.NoError(t, err)
	require.NoError(t, db.Set([]byte("a"), []byte("persisted-a")))
	require.NoError(t, db.Close())
	return dbadapter.Store{DB: db}
}

func TestRepairM_635_FailingBackend(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		for _, side := range []string{"parent", "cache"} {
			for _, populated := range []bool{false, true} {
				name := "forward/" + side
				if reverse {
					name = "reverse/" + side
				}
				if populated {
					name += "/valid-wrapper"
				} else {
					name += "/invalid-wrapper"
				}
				t.Run(name, func(t *testing.T) {
					failed := repairM635Iterator(repairM635ClosedBackend(t), reverse, nil, nil)
					require.False(t, failed.Valid())
					backendErr := failed.Error()
					require.ErrorIs(t, backendErr, leveldb.ErrClosed)
					healthyStore := repairM635Cache(repairM635Memory(t))
					if populated {
						healthyStore.Set([]byte("b"), []byte("cache-b"))
					}
					healthy := repairM635Iterator(healthyStore, reverse, nil, nil)
					parent, cache := failed, healthy
					if side == "cache" {
						parent, cache = healthy, failed
					}
					var iter types.Iterator = cachekv.NewCacheMergeIterator(parent, cache, !reverse, types.NewKVStoreKey("repair-m635"))
					t.Cleanup(func() { require.NoError(t, iter.Close()) })
					require.Same(t, backendErr, iter.Error())
					require.Equal(t, populated, iter.Valid())
					var got []string
					for ; iter.Valid(); iter.Next() {
						require.Same(t, backendErr, iter.Error())
						got = append(got, string(iter.Key())+"="+string(iter.Value()))
					}
					if populated {
						require.Equal(t, []string{"b=cache-b"}, got)
					} else {
						require.Empty(t, got)
					}
					require.Same(t, backendErr, iter.Error())
					repairM635InvalidPosition(t, iter)
					require.Same(t, backendErr, iter.Error())
				})
			}
		}
	}
}

func TestRepairM_635_StoreBackendError(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		name := "forward"
		if reverse {
			name = "reverse"
		}
		t.Run(name, func(t *testing.T) {
			store := repairM635Cache(repairM635ClosedBackend(t))
			store.Set([]byte("b"), []byte("inner-b"))
			outer := repairM635Cache(store)
			outer.Set([]byte("c"), []byte("outer-c"))
			iter := repairM635Iterator(outer, reverse, nil, nil)
			t.Cleanup(func() { require.NoError(t, iter.Close()) })
			require.ErrorIs(t, iter.Error(), leveldb.ErrClosed)
			var got []string
			for ; iter.Valid(); iter.Next() {
				require.ErrorIs(t, iter.Error(), leveldb.ErrClosed)
				got = append(got, string(iter.Key())+"="+string(iter.Value()))
			}
			want := []string{"b=inner-b", "c=outer-c"}
			if reverse {
				want[0], want[1] = want[1], want[0]
			}
			require.Equal(t, want, got)
			require.ErrorIs(t, iter.Error(), leveldb.ErrClosed)
			repairM635InvalidPosition(t, iter)
		})
	}
}
