package multi_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/rootmulti"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/whitelist/multi"
	versioned "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func repairM644Seed(parent storetypes.MultiStore, keys []storetypes.StoreKey) {
	for _, key := range keys {
		s := parent.GetKVStore(key)
		s.Set([]byte("deny/existing"), []byte("protected"))
		s.Set([]byte("allow/delete"), []byte("removable"))
	}
}

func repairM644Open(t *testing.T) (storetypes.MultiStore, storetypes.MultiStore, []storetypes.StoreKey) {
	t.Helper()
	parent := rootmulti.NewStore(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, parent.Close()) })
	keys := []storetypes.StoreKey{storetypes.NewKVStoreKey("bank"), storetypes.NewKVStoreKey("blocked")}
	for _, key := range keys {
		parent.MountStoreWithDB(key, storetypes.StoreTypeDB, nil)
	}
	require.NoError(t, parent.LoadLatestVersion())
	repairM644Seed(parent, keys)
	return parent, multi.NewStore(parent, map[string][]string{"bank": {"allow/", "exact"}}), keys
}

func repairM644Policy(t *testing.T, s storetypes.KVStore, allowed bool) {
	t.Helper()
	require.Equal(t, []byte("protected"), s.Get([]byte("deny/existing")))
	denied := []string{"deny/existing", "deny/new", "allow", "allowish/key", "xallow/key"}
	if !allowed {
		denied = append(denied, "allow/new", "allow/delete", "exact-suffix")
	}
	for _, key := range denied {
		want := fmt.Sprintf("Setting disallowed key %s in whitelisted KV store", key)
		require.PanicsWithValue(t, want, func() { s.Set([]byte(key), []byte("attack")) })
		require.PanicsWithValue(t, want, func() { s.Delete([]byte(key)) })
	}
	require.Equal(t, []byte("protected"), s.Get([]byte("deny/existing")))
	require.False(t, s.Has([]byte("deny/new")))
	if allowed {
		s.Set([]byte("allow/new"), []byte("accepted"))
		s.Set([]byte("exact-suffix"), []byte("prefix"))
		s.Delete([]byte("allow/delete"))
		require.Equal(t, []byte("accepted"), s.Get([]byte("allow/new")))
		require.Equal(t, []byte("prefix"), s.Get([]byte("exact-suffix")))
		require.False(t, s.Has([]byte("allow/delete")))
	}
}

func repairM644Result(t *testing.T, parent storetypes.MultiStore, keys []storetypes.StoreKey) {
	t.Helper()
	for i, key := range keys {
		s := parent.GetKVStore(key)
		require.Equal(t, []byte("protected"), s.Get([]byte("deny/existing")))
		require.False(t, s.Has([]byte("deny/new")))
		if i == 0 {
			require.Equal(t, []byte("accepted"), s.Get([]byte("allow/new")))
			require.Equal(t, []byte("prefix"), s.Get([]byte("exact-suffix")))
			require.False(t, s.Has([]byte("allow/delete")))
		} else {
			require.False(t, s.Has([]byte("allow/new")))
			require.False(t, s.Has([]byte("exact-suffix")))
			require.Equal(t, []byte("removable"), s.Get([]byte("allow/delete")))
		}
	}
}

func TestRepairM_644_Accessors(t *testing.T) {
	accessors := []struct {
		name string
		get  func(storetypes.MultiStore) storetypes.MultiStore
	}{
		{"direct", func(s storetypes.MultiStore) storetypes.MultiStore { return s }},
		{"cache", func(s storetypes.MultiStore) storetypes.MultiStore { return s.CacheMultiStore() }},
		{"cache-wrap", func(s storetypes.MultiStore) storetypes.MultiStore {
			return s.CacheWrap(nil).(storetypes.CacheMultiStore)
		}},
		{"traced-cache-wrap", func(s storetypes.MultiStore) storetypes.MultiStore {
			return s.CacheWrapWithTrace(nil, &bytes.Buffer{}, storetypes.TraceContext{"test": "multi"}).(storetypes.CacheMultiStore)
		}},
		{"tracer", func(s storetypes.MultiStore) storetypes.MultiStore { return s.SetTracer(&bytes.Buffer{}) }},
		{"tracing-context", func(s storetypes.MultiStore) storetypes.MultiStore {
			return s.SetTracingContext(storetypes.TraceContext{"test": "multi"})
		}},
	}
	for _, cached := range []bool{false, true} {
		for _, accessor := range accessors {
			for _, generic := range []bool{false, true} {
				t.Run(fmt.Sprintf("cached=%t/%s/generic=%t", cached, accessor.name, generic), func(t *testing.T) {
					parent, guarded, keys := repairM644Open(t)
					var outer storetypes.CacheMultiStore
					if cached {
						outer = guarded.CacheMultiStore()
						guarded = outer
					}
					branch := accessor.get(guarded)
					for i, key := range keys {
						s := branch.GetKVStore(key)
						if generic {
							s = branch.GetStore(key).(storetypes.KVStore)
						}
						repairM644Policy(t, s, i == 0)
					}
					if cache, ok := branch.(storetypes.CacheMultiStore); ok {
						cache.Write()
						cache.Close()
					}
					if outer != nil {
						outer.Write()
						outer.Close()
					}
					repairM644Result(t, parent, keys)
				})
			}
		}
	}
}

func TestRepairM_644_KVBranches(t *testing.T) {
	for _, generic := range []bool{false, true} {
		for _, traced := range []bool{false, true} {
			t.Run(fmt.Sprintf("generic=%t/traced=%t", generic, traced), func(t *testing.T) {
				parent, guarded, keys := repairM644Open(t)
				outer := guarded.CacheMultiStore()
				for i, key := range keys {
					s := outer.GetKVStore(key)
					if generic {
						s = outer.GetStore(key).(storetypes.KVStore)
					}
					first := s.CacheWrap(key)
					if traced {
						first = s.CacheWrapWithTrace(key, &bytes.Buffer{}, storetypes.TraceContext{"test": "kv"})
					}
					repairM644Policy(t, first.(storetypes.KVStore), i == 0)
					second := first.CacheWrapWithTrace(key, &bytes.Buffer{}, nil)
					repairM644Policy(t, second.(storetypes.KVStore), i == 0)
					third := second.CacheWrap(key)
					repairM644Policy(t, third.(storetypes.KVStore), i == 0)
					third.Write()
					second.Write()
					first.Write()
				}
				require.False(t, parent.GetKVStore(keys[0]).Has([]byte("allow/new")), "branches must remain isolated until Write")
				outer.Write()
				outer.Close()
				repairM644Result(t, parent, keys)
			})
		}
	}
}

func TestRepairM_644_SetKVStores(t *testing.T) {
	for _, cached := range []bool{false, true} {
		t.Run(fmt.Sprintf("cached=%t", cached), func(t *testing.T) {
			parent, guarded, keys := repairM644Open(t)
			var outer storetypes.CacheMultiStore
			if cached {
				outer = guarded.CacheMultiStore()
				guarded = outer
			}
			seen := map[string]bool{}
			captured := map[string]storetypes.KVStore{}
			transformed := guarded.SetKVStores(func(key storetypes.StoreKey, s storetypes.KVStore) storetypes.CacheWrap {
				seen[key.Name()] = true
				captured[key.Name()] = s
				repairM644Policy(t, s, key == keys[0])
				cache := s.CacheWrapWithTrace(key, &bytes.Buffer{}, nil)
				repairM644Policy(t, cache.(storetypes.KVStore), key == keys[0])
				cache.Write()
				return s.(storetypes.CacheWrap)
			})
			require.Equal(t, map[string]bool{"bank": true, "blocked": true}, seen)
			for i, key := range keys {
				repairM644Policy(t, captured[key.Name()], i == 0)
				repairM644Policy(t, transformed.GetStore(key).(storetypes.KVStore), i == 0)
				repairM644Policy(t, transformed.GetKVStore(key), i == 0)
			}
			branch := transformed.SetTracer(&bytes.Buffer{}).SetTracingContext(nil).CacheMultiStore()
			for i, key := range keys {
				repairM644Policy(t, branch.GetStore(key).(storetypes.KVStore), i == 0)
			}
			branch.Write()
			branch.Close()
			transformed.(storetypes.CacheMultiStore).Write()
			if outer != nil {
				outer.Write()
				outer.Close()
			}
			repairM644Result(t, parent, keys)
		})
	}
}

func TestRepairM_644_VersionAndExport(t *testing.T) {
	sc := config.DefaultStateCommitConfig()
	sc.WriteMode = config.MemiavlOnly
	sc.MemIAVLConfig.AsyncCommitBuffer = 0
	parent := versioned.NewStore(t.TempDir(), sc, config.StateStoreConfig{}, nil)
	t.Cleanup(func() { require.NoError(t, parent.Close()) })
	keys := []storetypes.StoreKey{storetypes.NewKVStoreKey("bank"), storetypes.NewKVStoreKey("blocked")}
	for _, key := range keys {
		parent.MountStoreWithDB(key, storetypes.StoreTypeIAVL, nil)
	}
	require.NoError(t, parent.LoadLatestVersion())
	repairM644Seed(parent, keys)
	commit := parent.Commit(true)
	require.Equal(t, int64(1), commit.Version)
	guarded := multi.NewStore(parent, map[string][]string{"bank": {"allow/", "exact"}})
	for name, load := range map[string]func(int64) (storetypes.CacheMultiStore, error){
		"version": guarded.CacheMultiStoreWithVersion,
		"export":  guarded.CacheMultiStoreForExport,
	} {
		t.Run(name, func(t *testing.T) {
			branch, err := load(commit.Version)
			require.NoError(t, err)
			defer branch.Close()
			for i, key := range keys {
				repairM644Policy(t, branch.GetKVStore(key), i == 0)
				repairM644Policy(t, branch.GetStore(key).(storetypes.KVStore), i == 0)
			}
			nested := branch.CacheMultiStore()
			for i, key := range keys {
				repairM644Policy(t, nested.GetStore(key).(storetypes.KVStore), i == 0)
			}
			nested.Write()
			nested.Close()
			branch.Write()
			next := parent.Commit(true)
			require.Equal(t, commit.Version+1, next.Version)
			commit = next
			repairM644Result(t, parent, keys)
		})
	}
	branch, err := guarded.CacheMultiStoreWithVersion(commit.Version + 1)
	require.ErrorContains(t, err, "unable to load historical state with SS disabled")
	require.Nil(t, branch)
	_, legacyGuard, _ := repairM644Open(t)
	for _, load := range []func(int64) (storetypes.CacheMultiStore, error){legacyGuard.CacheMultiStoreWithVersion, legacyGuard.CacheMultiStoreForExport} {
		branch, err := load(1)
		require.ErrorIs(t, err, rootmulti.ErrVersionedStateUnsupported)
		require.Nil(t, branch)
	}
}

func TestRepairM_644_PolicySnapshot(t *testing.T) {
	parent, _, keys := repairM644Open(t)
	prefixes := []string{"allow/", "exact"}
	policy := map[string][]string{"bank": prefixes}
	guarded := multi.NewStore(parent, policy)
	prefixes[0] = "deny/"
	policy["blocked"] = []string{""}
	delete(policy, "bank")
	branch := guarded.CacheMultiStore()
	for i, key := range keys {
		repairM644Policy(t, guarded.GetKVStore(key), i == 0)
		repairM644Policy(t, branch.GetStore(key).(storetypes.KVStore), i == 0)
	}
	branch.Write()
	branch.Close()
	repairM644Result(t, parent, keys)
}
