package kv_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/cachekv"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/dbadapter"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/whitelist/kv"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func repairM642Parent(t *testing.T, cached bool) (storetypes.KVStore, storetypes.KVStore, func()) {
	t.Helper()
	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	base := dbadapter.Store{DB: db}
	if cached {
		parent := cachekv.NewStore(base, storetypes.NewKVStoreKey("parent"), storetypes.DefaultCacheSizeLimit)
		return parent, base, parent.Write
	}
	return base, base, func() {}
}

func repairM642NoParentEscape(t *testing.T, guarded storetypes.KVStore) {
	t.Helper()
	_, exposed := reflect.TypeOf(guarded).Elem().FieldByName("KVStore")
	require.False(t, exposed, "the raw parent must not be an exported embedded field")
	_, exposed = guarded.(interface{ GetParent() storetypes.KVStore })
	require.False(t, exposed, "a cache parent accessor must not escape the policy")
	_, exposed = guarded.(interface{ Parent() storetypes.KVStore })
	require.False(t, exposed, "a tracing parent accessor must not escape the policy")
	_, exposed = guarded.(*cachekv.Store)
	require.False(t, exposed, "a raw cache must not escape the policy")
}

func TestRepairM_642_MutationPaths(t *testing.T) {
	paths := []struct {
		name  string
		trace []bool
	}{
		{name: "direct"},
		{name: "cache", trace: []bool{false}},
		{name: "trace", trace: []bool{true}},
		{name: "cache_cache", trace: []bool{false, false}},
		{name: "cache_trace", trace: []bool{false, true}},
		{name: "trace_cache", trace: []bool{true, false}},
		{name: "trace_trace", trace: []bool{true, true}},
	}
	for _, cached := range []bool{false, true} {
		for _, path := range paths {
			t.Run(fmt.Sprintf("cached_parent=%t/%s", cached, path.name), func(t *testing.T) {
				parent, base, flushParent := repairM642Parent(t, cached)
				seed := map[string]string{
					"allow/set": "original", "allow/delete": "original",
					"allow/range/1": "one", "allow/range/2": "two", "deny/key": "protected",
				}
				for key, value := range seed {
					parent.Set([]byte(key), []byte(value))
				}
				flushParent()
				prefixes := []string{"allow/"}
				guarded := kv.NewStore(parent, prefixes)
				prefixes[0] = "deny/"
				repairM642NoParentEscape(t, guarded)
				var branches []storetypes.CacheKVStore
				var trace bytes.Buffer
				for _, traced := range path.trace {
					var branch storetypes.CacheWrap
					if traced {
						branch = guarded.CacheWrapWithTrace(storetypes.NewKVStoreKey("branch"), &trace, storetypes.TraceContext{"task": "m642"})
					} else {
						branch = guarded.CacheWrap(storetypes.NewKVStoreKey("branch"))
					}
					cache, ok := branch.(storetypes.CacheKVStore)
					require.True(t, ok, "branches must retain the KVStore and Write interfaces")
					branches = append(branches, cache)
					guarded = cache
					repairM642NoParentEscape(t, guarded)
				}

				for _, key := range []string{"deny/key", "allow", "allow0/key", "xallow/key"} {
					require.Panics(t, func() { guarded.Set([]byte(key), []byte("forbidden")) }, key)
					require.Panics(t, func() { guarded.Delete([]byte(key)) }, key)
				}
				require.Panics(t, func() { _ = guarded.DeleteAll(nil, nil) })
				require.Panics(t, func() { _ = guarded.DeleteAll([]byte("deny/"), []byte("deny0")) })
				for key, value := range seed {
					require.Equal(t, []byte(value), guarded.Get([]byte(key)), "rejected deletion changed %q", key)
					require.Equal(t, []byte(value), parent.Get([]byte(key)))
				}

				guarded.Set([]byte("allow/set"), []byte("changed"))
				guarded.Set([]byte("allow/new"), []byte("created"))
				guarded.Delete([]byte("allow/delete"))
				require.NoError(t, guarded.DeleteAll([]byte("allow/range/"), []byte("allow/range0")))
				require.NoError(t, guarded.DeleteAll([]byte("empty/"), []byte("empty0")))
				require.Equal(t, []byte("changed"), guarded.Get([]byte("allow/set")))
				require.Equal(t, []byte("created"), guarded.Get([]byte("allow/new")))
				for _, key := range []string{"allow/delete", "allow/range/1", "allow/range/2"} {
					require.False(t, guarded.Has([]byte(key)))
				}
				if len(branches) > 0 {
					for key, value := range seed {
						require.Equal(t, []byte(value), parent.Get([]byte(key)), "branch leaked before Write")
					}
					require.False(t, parent.Has([]byte("allow/new")))
				}
				for i := len(branches) - 1; i >= 0; i-- {
					branches[i].Write()
				}
				flushParent()
				require.Equal(t, []byte("changed"), base.Get([]byte("allow/set")))
				require.Equal(t, []byte("created"), base.Get([]byte("allow/new")))
				require.Equal(t, []byte("protected"), base.Get([]byte("deny/key")))
				for _, key := range []string{"allow/delete", "allow/range/1", "allow/range/2", "allow", "allow0/key", "xallow/key"} {
					require.False(t, base.Has([]byte(key)), key)
				}
				for _, traced := range path.trace {
					if traced {
						require.NotEmpty(t, trace.String(), "traced branches must preserve tracing")
					}
				}
			})
		}
	}
}

func TestRepairM_642_RangeBoundsAndPolicies(t *testing.T) {
	cases := []struct {
		name       string
		prefixes   []string
		start, end []byte
		denied     bool
		removed    []string
	}{
		{name: "mixed_full_range", prefixes: []string{"a/"}, denied: true},
		{name: "mixed_open_start", prefixes: []string{"z/"}, end: []byte("z0"), denied: true},
		{name: "mixed_open_end", prefixes: []string{"a/"}, start: []byte("a/"), denied: true},
		{name: "bounded", prefixes: []string{"a/"}, start: []byte("a/1"), end: []byte("a/2"), removed: []string{"a/1"}},
		{name: "open_start", prefixes: []string{"a/"}, end: []byte("a/2"), removed: []string{"a/1"}},
		{name: "open_end", prefixes: []string{"z/"}, start: []byte("z/"), removed: []string{"z/1"}},
		{name: "multiple_prefixes", prefixes: []string{"a/", "z/"}, removed: []string{"a/1", "a/2", "z/1"}},
		{name: "explicit_allow_all", prefixes: []string{""}, removed: []string{"a/1", "a/2", "z/1"}},
		{name: "deny_all", denied: true},
		{name: "empty_range_deny_all", start: []byte("m"), end: []byte("n")},
	}
	for _, cached := range []bool{false, true} {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("cached_parent=%t/%s", cached, tc.name), func(t *testing.T) {
				parent, base, flush := repairM642Parent(t, cached)
				keys := []string{"a/1", "a/2", "z/1"}
				for _, key := range keys {
					parent.Set([]byte(key), []byte(key))
				}
				// Leave parent cache entries dirty to cover its merged range enumeration.
				guarded := kv.NewStore(parent, tc.prefixes)
				if tc.denied {
					require.Panics(t, func() { _ = guarded.DeleteAll(tc.start, tc.end) })
				} else {
					require.NoError(t, guarded.DeleteAll(tc.start, tc.end))
				}
				for _, key := range keys {
					if containsM642(tc.removed, key) {
						require.False(t, guarded.Has([]byte(key)), key)
					} else {
						require.Equal(t, []byte(key), guarded.Get([]byte(key)), key)
					}
				}
				flush()
				for _, key := range keys {
					require.Equal(t, !containsM642(tc.removed, key), base.Has([]byte(key)), key)
				}
			})
		}
	}
}

func containsM642(keys []string, key string) bool {
	for _, candidate := range keys {
		if candidate == key {
			return true
		}
	}
	return false
}

func TestRepairM_642_ReadAndTracePreservation(t *testing.T) {
	parent, _, _ := repairM642Parent(t, false)
	parent.Set([]byte("allow/a"), []byte("allowed"))
	parent.Set([]byte("deny/z"), []byte("protected"))
	guarded := kv.NewStore(parent, []string{"allow/"})
	require.Equal(t, parent.GetStoreType(), guarded.GetStoreType())
	hash, err := parent.GetWorkingHash()
	require.NoError(t, err)
	guardedHash, err := guarded.GetWorkingHash()
	require.NoError(t, err)
	require.Equal(t, hash, guardedHash)
	// The real DB adapter has no versioning; the wrapper must retain its panic.
	require.Panics(t, func() { guarded.VersionExists(1) })
	require.True(t, guarded.Has([]byte("deny/z")))
	require.Equal(t, []byte("protected"), guarded.Get([]byte("deny/z")))
	require.Equal(t, []string{"allow/a", "deny/z"}, guarded.GetAllKeyStrsInRange(nil, nil))
	for _, reverse := range []bool{false, true} {
		var iter storetypes.Iterator
		if reverse {
			iter = guarded.ReverseIterator(nil, nil)
		} else {
			iter = guarded.Iterator(nil, nil)
		}
		var keys []string
		for ; iter.Valid(); iter.Next() {
			keys = append(keys, string(iter.Key()))
			require.Equal(t, parent.Get(iter.Key()), iter.Value())
		}
		require.NoError(t, iter.Error())
		require.NoError(t, iter.Close())
		if reverse {
			require.Equal(t, []string{"deny/z", "allow/a"}, keys)
		} else {
			require.Equal(t, []string{"allow/a", "deny/z"}, keys)
		}
	}

	var trace bytes.Buffer
	branch := guarded.CacheWrapWithTrace(storetypes.NewKVStoreKey("trace"), &trace, storetypes.TraceContext{"task": "m642"}).(storetypes.CacheKVStore)
	require.Panics(t, func() { branch.Set([]byte("deny/z"), []byte("forbidden")) })
	require.Panics(t, func() { branch.Delete([]byte("deny/z")) })
	require.Panics(t, func() { _ = branch.DeleteAll(nil, nil) })
	branch.Set([]byte("allow/a"), []byte("traced"))
	branch.Write()
	require.Equal(t, []byte("traced"), parent.Get([]byte("allow/a")))
	require.Equal(t, []byte("protected"), parent.Get([]byte("deny/z")))
	decoder := json.NewDecoder(&trace)
	var event struct {
		Operation string                 `json:"operation"`
		Key       string                 `json:"key"`
		Value     string                 `json:"value"`
		Metadata  map[string]interface{} `json:"metadata"`
	}
	require.NoError(t, decoder.Decode(&event))
	require.Equal(t, "write", event.Operation)
	require.Equal(t, base64.StdEncoding.EncodeToString([]byte("allow/a")), event.Key)
	require.Equal(t, base64.StdEncoding.EncodeToString([]byte("traced")), event.Value)
	require.Equal(t, "m642", event.Metadata["task"])
	require.False(t, decoder.More(), "forbidden mutations must not be traced or queued")
}
