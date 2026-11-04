package prefix

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	"github.com/syndtr/goleveldb/leveldb"
	dbm "github.com/tendermint/tm-db"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/dbadapter"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/mem"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
)

func repairM639MemoryStore(t *testing.T) *mem.Store {
	t.Helper()
	root := mem.NewStore()
	t.Cleanup(func() {
		if err := root.DB.Close(); err != nil {
			t.Error(err)
		}
	})
	return root
}

func repairM639Close(t *testing.T, iter types.Iterator) {
	t.Helper()
	t.Cleanup(func() {
		if err := iter.Close(); err != nil {
			t.Error(err)
		}
	})
}

func repairM639InvalidGuards(t *testing.T, iter types.Iterator) {
	t.Helper()
	if iter.Valid() {
		t.Fatal("expected an invalid iterator")
	}
	for _, operation := range []struct {
		name string
		call func()
	}{
		{"Next", iter.Next},
		{"Key", func() { iter.Key() }},
		{"Value", func() { iter.Value() }},
	} {
		t.Run(operation.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("positional operation on an invalid iterator must panic")
				}
			}()
			operation.call()
		})
	}
}

func TestRepairM_639_EmptyRanges(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		for _, tc := range []struct {
			name       string
			prefix     []byte
			seed       bool
			start, end []byte
		}{
			{name: "empty_database", prefix: []byte("tenant/")},
			{name: "absent_namespace", prefix: []byte("missing/"), seed: true},
			{name: "empty_domain", prefix: []byte("tenant/"), seed: true, start: []byte("a"), end: []byte("a")},
			{name: "nil_prefix", prefix: nil},
			{name: "empty_prefix", prefix: []byte{}},
		} {
			direction := "forward/"
			if reverse {
				direction = "reverse/"
			}
			t.Run(direction+tc.name, func(t *testing.T) {
				root := repairM639MemoryStore(t)
				outer := NewStore(root, []byte("root/"))
				if tc.seed {
					outer.Set([]byte("tenant/a"), []byte("value:a"))
					outer.Set([]byte("other/a"), []byte("sibling"))
				}
				store := NewStore(outer, tc.prefix)
				var iter types.Iterator
				if reverse {
					iter = store.ReverseIterator(tc.start, tc.end)
				} else {
					iter = store.Iterator(tc.start, tc.end)
				}
				repairM639Close(t, iter)
				if iter.Valid() {
					t.Fatal("empty scan returned a key")
				}
				if err := iter.Error(); err != nil {
					t.Fatalf("empty scan reported a storage error: %v", err)
				}
				repairM639InvalidGuards(t, iter)
			})
		}
	}
}

func TestRepairM_639_Traversal(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		for _, tc := range []struct {
			name   string
			prefix []byte
		}{
			{"namespace", []byte("tenant/")},
			{"nil_prefix", nil},
			{"empty_prefix", []byte{}},
		} {
			direction := "forward/"
			if reverse {
				direction = "reverse/"
			}
			t.Run(direction+tc.name, func(t *testing.T) {
				root := repairM639MemoryStore(t)
				root.Set([]byte("q/sibling"), []byte("before"))
				root.Set([]byte("s/sibling"), []byte("after"))
				store := NewStore(NewStore(root, []byte("root/")), tc.prefix)
				for _, key := range []string{"a", "b", "c"} {
					store.Set([]byte(key), []byte("value:"+key))
				}
				var iter types.Iterator
				want := []string{"a", "b", "c"}
				if reverse {
					iter = store.ReverseIterator(nil, nil)
					want = []string{"c", "b", "a"}
				} else {
					iter = store.Iterator(nil, nil)
				}
				repairM639Close(t, iter)
				start, end := iter.Domain()
				if start != nil || end != nil {
					t.Fatalf("relative domain = [%q, %q), want [nil, nil)", start, end)
				}
				var got []string
				for ; iter.Valid(); iter.Next() {
					if len(got) >= len(want) {
						t.Fatal("iterator escaped its namespace or did not advance")
					}
					key := iter.Key()
					got = append(got, string(key))
					if !bytes.Equal(iter.Value(), store.Get(key)) || !bytes.Equal(iter.Value(), []byte("value:"+string(key))) {
						t.Fatalf("relative key %q did not round-trip through the store", key)
					}
					if err := iter.Error(); err != nil {
						t.Fatalf("healthy position reported an error: %v", err)
					}
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("keys = %q, want %q", got, want)
				}
				if err := iter.Error(); err != nil {
					t.Fatalf("normal exhaustion reported a storage error: %v", err)
				}
				repairM639InvalidGuards(t, iter)
			})
		}
	}
}

func TestRepairM_639_PrefixBoundary(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		name := "forward"
		if reverse {
			name = "reverse"
		}
		t.Run(name, func(t *testing.T) {
			root := repairM639MemoryStore(t)
			for _, key := range []string{"tenant-previous", "tenant/a", "tenant/b", "tenant0-next"} {
				root.Set([]byte(key), []byte("value:"+key))
			}
			p := []byte("tenant/")
			var parent types.Iterator
			want := []string{"a", "b"}
			boundary := "tenant0-next"
			if reverse {
				parent = root.ReverseIterator(nil, cpIncr(p))
				want = []string{"b", "a"}
				boundary = "tenant-previous"
			} else {
				parent = root.Iterator(p, nil)
			}
			repairM639Close(t, parent)
			iter := newPrefixIterator(p, nil, nil, parent)
			for _, key := range want {
				if !iter.Valid() || string(iter.Key()) != key || string(iter.Value()) != "value:tenant/"+key {
					t.Fatalf("expected prefix key %q and its value", key)
				}
				iter.Next()
			}
			if iter.Valid() || !parent.Valid() || string(parent.Key()) != boundary {
				t.Fatal("prefix must end while the backing iterator remains on the adjacent namespace")
			}
			if err := parent.Error(); err != nil {
				t.Fatalf("healthy parent returned %v", err)
			}
			if err := iter.Error(); err != nil {
				t.Fatalf("prefix boundary reported a storage error: %v", err)
			}
			repairM639InvalidGuards(t, iter)
		})
	}
}

func TestRepairM_639_BackendFailure(t *testing.T) {
	db, err := dbm.NewGoLevelDB("prefix-error", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parent := dbadapter.Store{DB: db}
	store := NewStore(parent, []byte("tenant/"))
	store.Set([]byte("a"), []byte("value:a"))
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	for _, reverse := range []bool{false, true} {
		name := "forward"
		if reverse {
			name = "reverse"
		}
		t.Run(name, func(t *testing.T) {
			var iter types.Iterator
			if reverse {
				iter = store.ReverseIterator(nil, nil)
			} else {
				iter = store.Iterator(nil, nil)
			}
			repairM639Close(t, iter)
			if iter.Valid() {
				t.Fatal("closed backend returned a valid iterator")
			}
			backingError := iter.(*prefixIterator).iter.Error()
			if !errors.Is(backingError, leveldb.ErrClosed) {
				t.Fatalf("backend error = %v, want leveldb.ErrClosed", backingError)
			}
			if got := iter.Error(); got != backingError {
				t.Fatalf("wrapper error = %v, want original backend error %v", got, backingError)
			}
			repairM639InvalidGuards(t, iter)
			if got := iter.Error(); got != backingError {
				t.Fatalf("positional guards changed backend error to %v", got)
			}
		})
	}
}
