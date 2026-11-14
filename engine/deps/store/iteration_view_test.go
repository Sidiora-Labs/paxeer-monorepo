package store_test

import (
	"bytes"
	"fmt"
	"testing"

	gigastore "github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/cachekv"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	"github.com/stretchr/testify/require"
)

func iterationPairs(t *testing.T, store types.KVStore, start, end []byte, reverse bool) [][2][]byte {
	t.Helper()
	var iter types.Iterator
	if reverse {
		iter = store.ReverseIterator(start, end)
	} else {
		iter = store.Iterator(start, end)
	}
	gotStart, gotEnd := iter.Domain()
	require.Equal(t, start, gotStart)
	require.Equal(t, end, gotEnd)
	var pairs [][2][]byte
	for ; iter.Valid(); iter.Next() {
		pairs = append(pairs, [2][]byte{bytes.Clone(iter.Key()), bytes.Clone(iter.Value())})
	}
	require.NoError(t, iter.Error())
	require.NoError(t, iter.Close())
	return pairs
}

func TestIterationView(t *testing.T) {
	for _, pending := range []bool{false, true} {
		for _, nested := range []bool{false, true} {
			t.Run(fmt.Sprintf("pending=%t/nested=%t", pending, nested), func(t *testing.T) {
				initial := map[string]string{"a": "base-a", "d": "base-d", "m": "base-m", "t": "base-t", "z": "base-z"}
				backing := newNestedCacheBacking(t, initial)
				key := types.NewKVStoreKey("iteration")
				plain := gigastore.NewStore(backing, key, 0)
				reference := cachekv.NewStore(newNestedCacheBacking(t, initial), key, 0)
				mutate := func(store types.KVStore) {
					store.Set([]byte("b"), []byte("added"))
					store.Set([]byte("m"), []byte("overwritten"))
					store.Delete([]byte("d"))
					store.Set([]byte("c"), []byte("temporary"))
					store.Delete([]byte("c"))
					store.Delete([]byte("absent"))
					store.Delete([]byte("t"))
					store.Set([]byte("t"), []byte{})
					store.Set([]byte{0xff}, []byte("high"))
				}
				if pending {
					mutate(plain)
					mutate(reference)
				}
				if nested {
					plain = plain.CacheWrap(key).(*gigastore.Store)
					plain.Delete([]byte("a"))
					plain.Set([]byte("m"), []byte("child"))
					reference.Delete([]byte("a"))
					reference.Set([]byte("m"), []byte("child"))
				}
				view := gigastore.NewIterationView(plain)
				check := func() {
					for _, reverse := range []bool{false, true} {
						for _, domain := range [][2][]byte{{nil, nil}, {nil, []byte("m")}, {[]byte("m"), nil}, {[]byte("b"), []byte("z")}, {[]byte("e"), []byte("f")}, {[]byte{0xff}, nil}} {
							require.Equal(t, iterationPairs(t, reference, domain[0], domain[1], reverse), iterationPairs(t, view, domain[0], domain[1], reverse))
						}
					}
				}
				check()
				require.PanicsWithValue(t, gigastore.ErrIteratorUnsupported, func() { plain.Iterator(nil, nil) })
				require.PanicsWithValue(t, gigastore.ErrIteratorUnsupported, func() { plain.ReverseIterator(nil, nil) })
				plain.Write()
				check()
				view.Set([]byte("new"), []byte("visible"))
				reference.Set([]byte("new"), []byte("visible"))
				view.Delete([]byte("z"))
				reference.Delete([]byte("z"))
				check()
				wrapped := view.CacheWrap(key).(types.CacheKVStore)
				require.Equal(t, iterationPairs(t, view, nil, nil, false), iterationPairs(t, wrapped, nil, nil, false))
			})
		}
	}
}
