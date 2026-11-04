package multiversion

import (
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/dbadapter"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/occ"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func TestRepair_290_RemovedVersion(t *testing.T) {
	for _, ascending := range []bool{true, false} {
		for _, parentValue := range [][]byte{nil, []byte("parent"), {}} {
			name := "forward"
			if !ascending {
				name = "reverse"
			}
			if parentValue == nil {
				name += "/absent"
			} else if len(parentValue) == 0 {
				name += "/empty"
			} else {
				name += "/present"
			}
			t.Run(name, func(t *testing.T) {
				parent := dbadapter.Store{DB: dbm.NewMemDB()}
				parent.Set([]byte("a"), []byte("first"))
				parent.Set([]byte("c"), []byte("last"))
				if parentValue != nil {
					parent.Set([]byte("b"), parentValue)
				}
				store := NewMultiVersionStore(parent)
				predecessor := store.VersionedIndexedStore(1, 0, make(chan occ.Abort, 1))
				predecessor.Set([]byte("b"), []byte("speculative"))
				predecessor.WriteToMultiVersionStore()

				iterate := func(vis *VersionIndexedStore) ([]string, []string) {
					var iter types.Iterator
					if ascending {
						iter = vis.Iterator([]byte("a"), []byte("d"))
					} else {
						iter = vis.ReverseIterator([]byte("a"), []byte("d"))
					}
					defer func() { require.NoError(t, iter.Close()) }()
					var keys, values []string
					for ; iter.Valid(); iter.Next() {
						keys = append(keys, string(iter.Key()))
						values = append(values, string(iter.Value()))
					}
					return keys, values
				}
				consumer := store.VersionedIndexedStore(2, 0, make(chan occ.Abort, 1))
				oldKeys, oldValues := iterate(consumer)
				require.Contains(t, oldKeys, "b")
				require.Contains(t, oldValues, "speculative")
				consumer.Set([]byte("result"), []byte(strings.Join(oldValues, ",")))
				consumer.WriteToMultiVersionStore()

				// Freeze exactly the key collection used by validateIterator, then
				// let a real predecessor incarnation remove its published write.
				items := store.CollectIteratorItems(2)
				hasKey, err := items.Has([]byte("b"))
				require.NoError(t, err)
				require.True(t, hasKey)
				removed := make(chan struct{})
				type observation struct {
					value, cached []byte
					keys, values  []string
					err           error
				}
				observed := make(chan observation, 1)
				go func() {
					<-removed
					iter := store.newMVSValidationIterator(2, []byte("a"), []byte("d"), items, ascending, nil, parent, make(chan occ.Abort, 1))
					result := observation{value: iter.Value()}
					// Repeated merge-iterator probes must keep the parent read,
					// including a cached nil, even if another incarnation writes.
					replacement := store.VersionedIndexedStore(1, 2, make(chan occ.Abort, 1))
					replacement.Set([]byte("b"), []byte("replacement"))
					replacement.WriteToMultiVersionStore()
					result.cached = iter.Value()
					store.VersionedIndexedStore(1, 3, make(chan occ.Abort, 1)).WriteToMultiVersionStore()
					var parentIter types.Iterator
					if ascending {
						parentIter = parent.Iterator([]byte("a"), []byte("d"))
					} else {
						parentIter = parent.ReverseIterator([]byte("a"), []byte("d"))
					}
					merged := NewMVSMergeIterator(parentIter, iter, ascending, NoOpHandler{})
					for ; merged.Valid(); merged.Next() {
						result.keys = append(result.keys, string(merged.Key()))
						result.values = append(result.values, string(merged.Value()))
					}
					result.err = merged.Close()
					observed <- result
				}()
				store.InvalidateWriteset(1, 0)
				store.VersionedIndexedStore(1, 1, make(chan occ.Abort, 1)).WriteToMultiVersionStore()
				require.Nil(t, store.GetLatestBeforeIndex(2, []byte("b")))
				close(removed)
				result := <-observed
				require.NoError(t, result.err)
				require.Equal(t, parentValue, result.value)
				require.Equal(t, parentValue, result.cached)

				expectedKeys, expectedValues := []string{"a", "c"}, []string{"first", "last"}
				if parentValue != nil {
					expectedKeys = []string{"a", "b", "c"}
					expectedValues = []string{"first", string(parentValue), "last"}
				}
				if !ascending {
					for i, j := 0, len(expectedKeys)-1; i < j; i, j = i+1, j-1 {
						expectedKeys[i], expectedKeys[j] = expectedKeys[j], expectedKeys[i]
						expectedValues[i], expectedValues[j] = expectedValues[j], expectedValues[i]
					}
				}
				require.Equal(t, expectedKeys, result.keys)
				require.Equal(t, expectedValues, result.values)
				valid, conflicts := store.ValidateTransactionState(2)
				require.False(t, valid, "a consumer of the removed value must retry")
				require.Empty(t, conflicts)

				retried := store.VersionedIndexedStore(2, 1, make(chan occ.Abort, 1))
				keys, values := iterate(retried)
				require.Equal(t, expectedKeys, keys)
				require.Equal(t, expectedValues, values)
				retried.Set([]byte("result"), []byte(strings.Join(values, ",")))
				retried.WriteToMultiVersionStore()
				valid, conflicts = store.ValidateTransactionState(2)
				require.True(t, valid)
				require.Empty(t, conflicts)
				store.WriteLatestToStore()
				require.Equal(t, parentValue, parent.Get([]byte("b")))
				require.Equal(t, []byte(strings.Join(expectedValues, ",")), parent.Get([]byte("result")))
			})
		}
	}
}

func TestRepair_290_VisibleVersionPrecedence(t *testing.T) {
	parent := dbadapter.Store{DB: dbm.NewMemDB()}
	parent.Set([]byte("key"), []byte("parent"))
	store := NewMultiVersionStore(parent)
	store.SetWriteset(0, 0, WriteSet{"key": []byte("earlier")})
	store.SetWriteset(1, 0, WriteSet{"key": []byte("removed")})
	items := store.CollectIteratorItems(2)
	store.SetWriteset(1, 1, WriteSet{})
	store.SetWriteset(3, 0, WriteSet{"key": []byte("later")})

	for _, tc := range []struct {
		name     string
		writeset WriteSet
		want     []byte
	}{
		{"earlier-version", nil, []byte("earlier")},
		{"own-write", WriteSet{"key": []byte("own")}, []byte("own")},
		{"own-deletion", WriteSet{"key": nil}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			iter := store.newMVSValidationIterator(2, nil, nil, items, true, tc.writeset, parent, make(chan occ.Abort, 1))
			defer func() { require.NoError(t, iter.Close()) }()
			require.True(t, iter.Valid())
			require.Equal(t, tc.want, iter.Value())
		})
	}
	store.SetWriteset(0, 1, WriteSet{"key": nil})
	iter := store.newMVSValidationIterator(2, nil, nil, items, true, nil, parent, make(chan occ.Abort, 1))
	require.Nil(t, iter.Value(), "an explicit deletion must still shadow the parent")
	require.NoError(t, iter.Close())
	store.InvalidateWriteset(0, 1)
	aborts := make(chan occ.Abort, 1)
	iter = store.newMVSValidationIterator(2, nil, nil, items, true, nil, parent, aborts)
	require.PanicsWithValue(t, occ.NewEstimateAbort(0), func() { iter.Value() })
	require.Equal(t, 0, (<-aborts).DependentTxIdx)
	require.NoError(t, iter.Close())
}
