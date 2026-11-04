package cachemulti_test

import (
	"bytes"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/cachekv"
	basecachemulti "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/cachemulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/dbadapter"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/whitelist/cachemulti"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func repairM643Stores(t *testing.T) (storetypes.CacheMultiStore, basecachemulti.Store, []storetypes.StoreKey, map[storetypes.StoreKey]storetypes.KVStore) {
	t.Helper()
	keys := []storetypes.StoreKey{storetypes.NewKVStoreKey("allowed"), storetypes.NewKVStoreKey("denied")}
	parents := make(map[storetypes.StoreKey]storetypes.CacheWrapper)
	backing := make(map[storetypes.StoreKey]storetypes.KVStore)
	names := make(map[string]storetypes.StoreKey)
	for _, key := range keys {
		db := dbm.NewMemDB()
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		parent := dbadapter.Store{DB: db}
		parent.Set([]byte("forbidden/existing"), []byte("original"))
		parent.Set([]byte("ok/existing"), []byte("original"))
		parents[key] = parent
		backing[key] = parent
		names[key.Name()] = key
	}
	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	raw := basecachemulti.NewStore(db, parents, names, nil, nil, nil, 7)
	return cachemulti.NewStore(raw, map[string][]string{keys[0].Name(): {"ok/"}}), raw, keys, backing
}

func repairM643CheckWrites(t *testing.T, s storetypes.KVStore, allowed bool) {
	t.Helper()
	require.Equal(t, []byte("original"), s.Get([]byte("forbidden/existing")))
	require.PanicsWithValue(t, "Setting disallowed key forbidden/new in whitelisted KV store", func() {
		s.Set([]byte("forbidden/new"), []byte("unauthorized"))
	})
	require.PanicsWithValue(t, "Setting disallowed key forbidden/existing in whitelisted KV store", func() {
		s.Delete([]byte("forbidden/existing"))
	})
	require.Nil(t, s.Get([]byte("forbidden/new")))
	require.Equal(t, []byte("original"), s.Get([]byte("forbidden/existing")))
	if allowed {
		s.Set([]byte("ok/new"), []byte("permitted"))
		require.Equal(t, []byte("permitted"), s.Get([]byte("ok/new")))
		s.Delete([]byte("ok/existing"))
		require.Nil(t, s.Get([]byte("ok/existing")))
	} else {
		require.PanicsWithValue(t, "Setting disallowed key ok/new in whitelisted KV store", func() {
			s.Set([]byte("ok/new"), []byte("unauthorized"))
		})
		require.PanicsWithValue(t, "Setting disallowed key ok/existing in whitelisted KV store", func() {
			s.Delete([]byte("ok/existing"))
		})
		require.Nil(t, s.Get([]byte("ok/new")))
		require.Equal(t, []byte("original"), s.Get([]byte("ok/existing")))
	}
}

func TestRepairM_643_Accessors(t *testing.T) {
	for _, accessor := range []string{"GetKVStore", "GetStore"} {
		for _, eager := range []bool{false, true} {
			t.Run(accessor+map[bool]string{false: "/lazy", true: "/materialized"}[eager], func(t *testing.T) {
				cms, raw, keys, backing := repairM643Stores(t)
				for i, key := range keys {
					if eager {
						raw.GetKVStore(key)
					}
					var s storetypes.KVStore
					if accessor == "GetStore" {
						s = cms.GetStore(key).(storetypes.KVStore)
					} else {
						s = cms.GetKVStore(key)
					}
					repairM643CheckWrites(t, s, i == 0)
				}
				cms.Write()
				require.Equal(t, []byte("permitted"), backing[keys[0]].Get([]byte("ok/new")))
				require.Nil(t, backing[keys[0]].Get([]byte("ok/existing")))
				for _, key := range keys {
					require.Equal(t, []byte("original"), backing[key].Get([]byte("forbidden/existing")))
					require.Nil(t, backing[key].Get([]byte("forbidden/new")))
				}
			})
		}
	}
}

func TestRepairM_643_SetKVStores(t *testing.T) {
	for _, eager := range []bool{false, true} {
		t.Run(map[bool]string{false: "lazy", true: "materialized"}[eager], func(t *testing.T) {
			cms, raw, keys, _ := repairM643Stores(t)
			if eager {
				for _, key := range keys {
					raw.GetKVStore(key)
				}
			}
			replacements := make(map[storetypes.StoreKey]storetypes.KVStore)
			calls := make(map[storetypes.StoreKey]int)
			transformed := cms.SetKVStores(func(key storetypes.StoreKey, s storetypes.KVStore) storetypes.CacheWrap {
				calls[key]++
				repairM643CheckWrites(t, s, key == keys[0])
				db := dbm.NewMemDB()
				t.Cleanup(func() { require.NoError(t, db.Close()) })
				replacement := dbadapter.Store{DB: db}
				replacement.Set([]byte("forbidden/existing"), []byte("original"))
				replacement.Set([]byte("ok/existing"), []byte("original"))
				replacements[key] = replacement
				return cachekv.NewStore(replacement, key, storetypes.DefaultCacheSizeLimit)
			})
			for i, key := range keys {
				require.Equal(t, 1, calls[key])
				repairM643CheckWrites(t, transformed.GetStore(key).(storetypes.KVStore), i == 0)
				repairM643CheckWrites(t, cms.GetKVStore(key), i == 0)
				// The replacement installed in the real parent must itself retain the policy.
				repairM643CheckWrites(t, raw.GetKVStore(key), i == 0)
			}
			transformed.(storetypes.CacheMultiStore).Write()
			require.Equal(t, []byte("permitted"), replacements[keys[0]].Get([]byte("ok/new")))
			require.Nil(t, replacements[keys[0]].Get([]byte("ok/existing")))
			for _, key := range keys {
				require.Equal(t, []byte("original"), replacements[key].Get([]byte("forbidden/existing")))
				require.Nil(t, replacements[key].Get([]byte("forbidden/new")))
			}
		})
	}
}

func TestRepairM_643_BranchesAndTransformations(t *testing.T) {
	transforms := []struct {
		name   string
		apply  func(storetypes.CacheMultiStore, *bytes.Buffer) storetypes.CacheMultiStore
		branch bool
	}{
		{"CacheMultiStore", func(s storetypes.CacheMultiStore, _ *bytes.Buffer) storetypes.CacheMultiStore {
			return s.CacheMultiStore()
		}, true},
		{"CacheWrap", func(s storetypes.CacheMultiStore, _ *bytes.Buffer) storetypes.CacheMultiStore {
			return s.CacheWrap(nil).(storetypes.CacheMultiStore)
		}, true},
		{"CacheWrapWithTrace", func(s storetypes.CacheMultiStore, b *bytes.Buffer) storetypes.CacheMultiStore {
			return s.CacheWrapWithTrace(nil, b, storetypes.TraceContext{"task": "m643"}).(storetypes.CacheMultiStore)
		}, true},
		{"SetTracer", func(s storetypes.CacheMultiStore, b *bytes.Buffer) storetypes.CacheMultiStore {
			return s.SetTracer(b).(storetypes.CacheMultiStore)
		}, false},
		{"SetTracingContext", func(s storetypes.CacheMultiStore, _ *bytes.Buffer) storetypes.CacheMultiStore {
			return s.SetTracingContext(storetypes.TraceContext{"task": "m643"}).(storetypes.CacheMultiStore)
		}, false},
	}
	for _, transform := range transforms {
		t.Run(transform.name, func(t *testing.T) {
			cms, _, keys, backing := repairM643Stores(t)
			var trace bytes.Buffer
			transformed := transform.apply(cms, &trace)
			if transform.name == "SetTracer" {
				require.True(t, transformed.TracingEnabled())
			}
			for i, key := range keys {
				repairM643CheckWrites(t, transformed.GetKVStore(key), i == 0)
				repairM643CheckWrites(t, transformed.GetStore(key).(storetypes.KVStore), i == 0)
			}
			transformed.Write()
			if transform.branch {
				require.Nil(t, backing[keys[0]].Get([]byte("ok/new")), "a branch must not write through its parent")
			}
			cms.Write()
			require.Equal(t, []byte("permitted"), backing[keys[0]].Get([]byte("ok/new")))
			for _, key := range keys {
				require.Equal(t, []byte("original"), backing[key].Get([]byte("forbidden/existing")))
				require.Nil(t, backing[key].Get([]byte("forbidden/new")))
			}
			if transform.name == "SetTracer" {
				require.Contains(t, trace.String(), `"key":"b2svbmV3"`)
			}
		})
	}
}

func TestRepairM_643_UnsupportedBranches(t *testing.T) {
	cms, _, keys, _ := repairM643Stores(t)
	require.PanicsWithValue(t, "cannot branch cached multi-store with a version", func() { cms.CacheMultiStoreWithVersion(3) })
	branch, err := cms.CacheMultiStoreForExport(3)
	require.Nil(t, branch)
	require.ErrorIs(t, err, basecachemulti.ErrCacheMultiStoreExportUnsupported)
	unknown := storetypes.NewKVStoreKey("unknown")
	require.Panics(t, func() { cms.GetStore(unknown) })
	require.Panics(t, func() { cms.GetKVStore(unknown) })
	repairM643CheckWrites(t, cms.GetKVStore(keys[0]), true)
}
