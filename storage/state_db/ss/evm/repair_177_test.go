package evm

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	commonevm "github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/proto"
)

func repair177Key(kind EVMStoreType, first byte) []byte {
	key := bytes.Repeat([]byte{first}, commonevm.InternalKeyLen(kind))
	return commonevm.BuildEVMKey(kind, key)
}

func repair177Open(t *testing.T, separate bool, versions [][]*proto.KVPair) types.StateStore {
	t.Helper()
	dir := t.TempDir()
	cfg := testConfig()
	cfg.SeparateEVMSubDBs = separate
	store, err := NewEVMStateStore(dir, cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		if store != nil {
			require.NoError(t, store.Close())
		}
	})
	for i, pairs := range versions {
		version := int64(i + 1)
		require.NoError(t, store.ApplyChangesetSync(version, []*proto.NamedChangeSet{{
			Name: EVMStoreKey, Changeset: proto.ChangeSet{Pairs: pairs},
		}}))
		require.NoError(t, store.SetLatestVersion(version))
	}
	require.NoError(t, store.Close())
	store = nil
	store, err = NewEVMStateStore(dir, cfg)
	require.NoError(t, err)
	require.Equal(t, int64(len(versions)), store.GetLatestVersion())
	return store
}

func repair177Collect(t *testing.T, store types.StateStore, version int64, start, end []byte, reverse bool, maxKeys int) []*proto.KVPair {
	t.Helper()
	var itr dbm.Iterator
	var err error
	if reverse {
		itr, err = store.ReverseIterator(EVMStoreKey, version, start, end)
	} else {
		itr, err = store.Iterator(EVMStoreKey, version, start, end)
	}
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, itr.Close()) })
	actualStart, actualEnd := itr.Domain()
	require.Equal(t, start, actualStart)
	require.Equal(t, end, actualEnd)
	var pairs []*proto.KVPair
	for ; itr.Valid(); itr.Next() {
		require.Less(t, len(pairs), maxKeys, "iterator must advance without duplicates")
		key := bytes.Clone(itr.Key())
		if len(pairs) > 0 {
			cmp := bytes.Compare(pairs[len(pairs)-1].Key, key)
			if reverse {
				require.Positive(t, cmp, "reverse iterator must return keys in decreasing byte order")
			} else {
				require.Negative(t, cmp, "forward iterator must return keys in increasing byte order")
			}
		}
		pairs = append(pairs, &proto.KVPair{Key: key, Value: bytes.Clone(itr.Value())})
	}
	require.NoError(t, itr.Error())
	require.False(t, itr.Valid(), "exhaustion must be permanent")
	require.NoError(t, itr.Close())
	require.NoError(t, itr.Close(), "closing an exhausted iterator must be idempotent")
	return pairs
}

func TestRepair_177_SeparateEVMIteratorRangeReopen(t *testing.T) {
	var initial, updates, future []*proto.KVPair
	for _, kind := range AllEVMStoreTypes() {
		if kind == StoreLegacy {
			continue
		}
		first, second := repair177Key(kind, 0x20), repair177Key(kind, 0x40)
		initial = append(initial,
			&proto.KVPair{Key: first, Value: []byte("initial-first")},
			&proto.KVPair{Key: second, Value: []byte("initial-second")},
			&proto.KVPair{Key: repair177Key(kind, 0xa0), Value: []byte("live-after-future")},
		)
		prefix, ok := commonevm.EVMKeyPrefixByte(kind)
		require.True(t, ok)
		// All three are successfully written Legacy keys sharing a family prefix.
		for _, key := range [][]byte{{prefix}, {prefix, 0x30}, append(bytes.Clone(first), 0x01)} {
			parsed, _ := commonevm.ParseEVMKey(key)
			require.Equal(t, StoreLegacy, parsed)
			initial = append(initial, &proto.KVPair{Key: key, Value: []byte("legacy-in-family")})
		}
		updates = append(updates,
			&proto.KVPair{Key: first, Value: []byte("updated-first")},
			&proto.KVPair{Key: second, Delete: true},
			&proto.KVPair{Key: repair177Key(kind, 0x60), Value: []byte("inserted-second-version")},
		)
		future = append(future, &proto.KVPair{Key: repair177Key(kind, 0x80), Value: []byte("future")})
	}
	for _, key := range [][]byte{{0x01, 0x20}, {0x05, 0x20}, {0x09, 0x20}, {0xf0, 0x20}} {
		initial = append(initial, &proto.KVPair{Key: key, Value: []byte("legacy")})
	}
	updates = append(updates, &proto.KVPair{Key: []byte{0x05, 0x20}, Delete: true})
	versions := [][]*proto.KVPair{initial, updates, future}
	unified := repair177Open(t, false, versions)
	separate := repair177Open(t, true, versions)

	ranges := []struct {
		name       string
		start, end []byte
	}{
		{"unbounded", nil, nil},
		{"nil-start", nil, []byte{0x08}},
		{"nil-end", []byte{0x07}, nil},
		{"cross-prefix", []byte{0x03}, []byte{0x0b}},
		{"cross-full-key", repair177Key(StoreStorage, 0x20), repair177Key(StoreNonce, 0x40)},
		{"legacy", []byte{0x05}, []byte{0x06}},
		{"empty-before", []byte{0x00}, []byte{0x01}},
		{"empty-between", []byte{0x06}, []byte{0x07}},
		{"empty-after", []byte{0xfe}, nil},
		{"equal-bounds", []byte{0x07}, []byte{0x07}},
	}
	for _, kind := range AllEVMStoreTypes() {
		prefix, ok := commonevm.EVMKeyPrefixByte(kind)
		if !ok {
			continue
		}
		ranges = append(ranges, struct {
			name       string
			start, end []byte
		}{StoreTypeName(kind) + "-prefix", []byte{prefix}, []byte{prefix + 1}})
		ranges = append(ranges, struct {
			name       string
			start, end []byte
		}{StoreTypeName(kind) + "-partial-address", []byte{prefix, 0x20}, []byte{prefix, 0x21}})
	}

	state := make(map[string][]byte)
	for i, changes := range versions {
		for _, pair := range changes {
			if pair.Delete {
				delete(state, string(pair.Key))
			} else {
				state[string(pair.Key)] = pair.Value
			}
		}
		version := int64(i + 1)
		var ordered []*proto.KVPair
		for key, value := range state {
			ordered = append(ordered, &proto.KVPair{Key: []byte(key), Value: value})
		}
		sort.Slice(ordered, func(i, j int) bool {
			return bytes.Compare(ordered[i].Key, ordered[j].Key) < 0
		})
		for _, bounds := range ranges {
			for _, reverse := range []bool{false, true} {
				t.Run(fmt.Sprintf("v%d/%s/reverse=%t", version, bounds.name, reverse), func(t *testing.T) {
					var expected []*proto.KVPair
					for _, pair := range ordered {
						if (bounds.start == nil || bytes.Compare(pair.Key, bounds.start) >= 0) &&
							(bounds.end == nil || bytes.Compare(pair.Key, bounds.end) < 0) {
							expected = append(expected, pair)
						}
					}
					if reverse {
						for left, right := 0, len(expected)-1; left < right; left, right = left+1, right-1 {
							expected[left], expected[right] = expected[right], expected[left]
						}
					}
					for _, backend := range []struct {
						name  string
						store types.StateStore
					}{{"unified", unified}, {"separate", separate}} {
						t.Run(backend.name, func(t *testing.T) {
							actual := repair177Collect(t, backend.store, version, bounds.start, bounds.end, reverse, len(state)+1)
							require.Equal(t, expected, actual)
							for _, pair := range actual {
								value, err := backend.store.Get(EVMStoreKey, version, pair.Key)
								require.NoError(t, err)
								require.Equal(t, pair.Value, value, "iteration and point reads must agree")
							}
						})
					}
				})
			}
		}
	}
}

func TestRepair_177_EmptyRangesAndInvalidBounds(t *testing.T) {
	for _, separate := range []bool{false, true} {
		t.Run(fmt.Sprintf("separate=%t", separate), func(t *testing.T) {
			for _, reverse := range []bool{false, true} {
				t.Run(fmt.Sprintf("reverse=%t", reverse), func(t *testing.T) {
					t.Run("empty", func(t *testing.T) {
						store := repair177Open(t, separate, nil)
						require.Empty(t, repair177Collect(t, store, 0, nil, nil, reverse, 1))
					})
					for _, bounds := range []struct {
						name       string
						start, end []byte
					}{
						{"empty-start", []byte{}, nil}, {"empty-end", nil, []byte{}},
						{"inverted", []byte{0x0a}, []byte{0x03}},
					} {
						t.Run(bounds.name, func(t *testing.T) {
							store := repair177Open(t, separate, nil)
							var itr dbm.Iterator
							var err error
							if reverse {
								itr, err = store.ReverseIterator(EVMStoreKey, 0, bounds.start, bounds.end)
							} else {
								itr, err = store.Iterator(EVMStoreKey, 0, bounds.start, bounds.end)
							}
							require.Error(t, err)
							require.Nil(t, itr)
						})
					}
				})
			}
		})
	}
}

func TestRepair_177_OpenErrors(t *testing.T) {
	for _, separate := range []bool{false, true} {
		t.Run(fmt.Sprintf("separate=%t", separate), func(t *testing.T) {
			for _, field := range []string{"AsyncWriteBuffer", "KeepRecent"} {
				t.Run(field, func(t *testing.T) {
					cfg := testConfig()
					cfg.SeparateEVMSubDBs = separate
					if field == "AsyncWriteBuffer" {
						cfg.AsyncWriteBuffer = -1
					} else {
						cfg.KeepRecent = -1
					}
					store, err := NewEVMStateStore(t.TempDir(), cfg)
					require.ErrorContains(t, err, field+" must be non-negative")
					require.Nil(t, store)
				})
			}
			t.Run("blocked-directory", func(t *testing.T) {
				dir := t.TempDir()
				cfg := testConfig()
				cfg.SeparateEVMSubDBs = separate
				blocked := filepath.Join(dir, "database")
				if separate {
					// Fail after opening the first family, then verify its lock is released.
					blocked = filepath.Join(dir, StoreTypeName(AllEVMStoreTypes()[1]))
				} else {
					dir = blocked
				}
				require.NoError(t, os.WriteFile(blocked, []byte("not a directory"), 0600))
				store, err := NewEVMStateStore(dir, cfg)
				require.Error(t, err)
				require.Nil(t, store)
				if separate {
					require.ErrorContains(t, err, StoreTypeName(AllEVMStoreTypes()[1]))
				} else {
					require.ErrorContains(t, err, "unified EVM MVCC DB")
				}
				require.NoError(t, os.Remove(blocked))
				store, err = NewEVMStateStore(dir, cfg)
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, store.Close()) })
				require.Empty(t, repair177Collect(t, store, 0, nil, nil, false, 1))
			})
		})
	}
}

func TestRepair_177_IteratorEarlyClose(t *testing.T) {
	for _, separate := range []bool{false, true} {
		t.Run(fmt.Sprintf("separate=%t", separate), func(t *testing.T) {
			store := repair177Open(t, separate, [][]*proto.KVPair{{
				{Key: repair177Key(StoreNonce, 0x20), Value: []byte("nonce")},
				{Key: repair177Key(StoreCode, 0x20), Value: []byte("code")},
			}})
			for _, reverse := range []bool{false, true} {
				var itr dbm.Iterator
				var err error
				if reverse {
					itr, err = store.ReverseIterator(EVMStoreKey, 1, nil, nil)
				} else {
					itr, err = store.Iterator(EVMStoreKey, 1, nil, nil)
				}
				require.NoError(t, err)
				require.True(t, itr.Valid())
				require.NoError(t, itr.Close())
				require.False(t, itr.Valid())
				require.NoError(t, itr.Close())
				require.NoError(t, itr.Error())
				require.Panics(t, func() { itr.Key() })
				require.Panics(t, func() { itr.Value() })
				require.Panics(t, func() { itr.Next() })
			}
		})
	}
}
