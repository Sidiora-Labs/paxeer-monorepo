package prefix

import (
	"bytes"
	"reflect"
	"sort"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/mem"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
)

func TestRepair_291_EnumerationKeyspace(t *testing.T) {
	for _, tc := range []struct {
		name     string
		prefixes [][]byte
	}{
		{name: "nil", prefixes: [][]byte{nil}},
		{name: "empty", prefixes: [][]byte{{}}},
		{name: "flat", prefixes: [][]byte{[]byte("tenant/")}},
		{name: "nested", prefixes: [][]byte{[]byte("tenant/"), []byte("accounts/")}},
		{name: "binary", prefixes: [][]byte{{0x00, 0xff}}},
		{name: "unbounded_prefix_end", prefixes: [][]byte{{0xff, 0xff}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, bounds := range []struct {
				name       string
				start, end []byte
				want       []string
			}{
				{name: "all", want: []string{"a", "b", "c", "d", "\xff"}},
				{name: "upper_bound", end: []byte("c"), want: []string{"a", "b"}},
				{name: "bounded", start: []byte("b"), end: []byte("d"), want: []string{"b", "c"}},
				{name: "lower_bound", start: []byte("d"), want: []string{"d", "\xff"}},
				{name: "empty_range", start: []byte("b"), end: []byte("b")},
			} {
				t.Run(bounds.name, func(t *testing.T) {
					root := mem.NewStore()
					t.Cleanup(func() {
						if err := root.DB.Close(); err != nil {
							t.Error(err)
						}
					})
					var store types.KVStore = NewStore(root, []byte("root/"))
					for _, p := range tc.prefixes {
						store = NewStore(store, p)
					}
					for _, key := range []string{"a", "b", "c", "d", "\xff"} {
						store.Set([]byte(key), []byte("value:"+key))
					}
					keys := store.GetAllKeyStrsInRange(bounds.start, bounds.end)
					sort.Strings(keys)
					if !reflect.DeepEqual(keys, bounds.want) {
						t.Fatalf("enumeration = %q, want relative keys %q", keys, bounds.want)
					}
					iter := store.Iterator(bounds.start, bounds.end)
					var iterKeys []string
					for ; iter.Valid(); iter.Next() {
						iterKeys = append(iterKeys, string(iter.Key()))
					}
					if err := iter.Close(); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(keys, iterKeys) {
						t.Fatalf("enumeration = %q, iterator = %q", keys, iterKeys)
					}
					for _, key := range keys {
						if got := store.Get([]byte(key)); !bytes.Equal(got, []byte("value:"+key)) {
							t.Fatalf("Get(enumerated key %q) = %q", key, got)
						}
						store.Delete([]byte(key))
						if store.Has([]byte(key)) {
							t.Fatalf("Delete(enumerated key %q) left the value present", key)
						}
					}
					for _, key := range []string{"a", "b", "c", "d", "\xff"} {
						deleted := sort.SearchStrings(keys, key)
						if deleted < len(keys) && keys[deleted] == key {
							continue
						}
						if got := store.Get([]byte(key)); !bytes.Equal(got, []byte("value:"+key)) {
							t.Fatalf("out-of-range key %q changed: %q", key, got)
						}
					}
				})
			}
		})
	}
}

func TestRepair_291_CacheDeleteAllWrite(t *testing.T) {
	for _, nested := range []bool{false, true} {
		name := "flat"
		if nested {
			name = "nested"
		}
		t.Run(name, func(t *testing.T) {
			for _, bounds := range []struct {
				name       string
				start, end []byte
				deleted    []string
			}{
				{name: "all", deleted: []string{"a", "b", "c", "d"}},
				{name: "bounded", start: []byte("b"), end: []byte("d"), deleted: []string{"b", "c"}},
			} {
				t.Run(bounds.name, func(t *testing.T) {
					root := mem.NewStore()
					t.Cleanup(func() {
						if err := root.DB.Close(); err != nil {
							t.Error(err)
						}
					})
					outer := NewStore(root, []byte("tenant/"))
					store := outer
					physicalPrefix := "tenant/"
					if nested {
						store = NewStore(outer, []byte("accounts/"))
						physicalPrefix += "accounts/"
					}
					want := make(map[string]string)
					for _, key := range []string{"a", "b", "c", "d"} {
						value := "target:" + key
						store.Set([]byte(key), []byte(value))
						want[physicalPrefix+key] = value
					}
					for _, p := range []string{"tenans/", "tenant0/", "other/"} {
						sibling := NewStore(root, []byte(p))
						for _, key := range []string{"a", "b", "c", "d"} {
							value := "sibling:" + p + key
							sibling.Set([]byte(key), []byte(value))
							want[p+key] = value
						}
					}
					if nested {
						for _, p := range []string{"account/", "accounts0/", "settings/"} {
							sibling := NewStore(outer, []byte(p))
							sibling.Set([]byte("b"), []byte("nested sibling"))
							want["tenant/"+p+"b"] = "nested sibling"
						}
					}
					cache := store.CacheWrap(nil).(types.CacheKVStore)
					for _, key := range cache.GetAllKeyStrsInRange(bounds.start, bounds.end) {
						if got := cache.Get([]byte(key)); !bytes.Equal(got, []byte("target:"+key)) {
							t.Fatalf("cache Get(enumerated key %q) = %q", key, got)
						}
					}
					if err := cache.DeleteAll(bounds.start, bounds.end); err != nil {
						t.Fatal(err)
					}
					for _, key := range bounds.deleted {
						if cache.Get([]byte(key)) != nil {
							t.Fatalf("cache still exposes deleted key %q", key)
						}
						if got := store.Get([]byte(key)); !bytes.Equal(got, []byte("target:"+key)) {
							t.Fatalf("parent changed before Write for %q: %q", key, got)
						}
						delete(want, physicalPrefix+key)
					}
					cache.Write()
					for _, key := range bounds.deleted {
						if store.Get([]byte(key)) != nil || cache.Get([]byte(key)) != nil {
							t.Fatalf("Write left deleted key %q present", key)
						}
					}
					got := make(map[string]string)
					iter := root.Iterator(nil, nil)
					for ; iter.Valid(); iter.Next() {
						got[string(iter.Key())] = string(iter.Value())
					}
					if err := iter.Close(); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("physical store after Write = %q, want %q", got, want)
					}
				})
			}
		})
	}
}
