package multi

import (
	"io"

	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/whitelist/kv"
)

type multiStore = storetypes.MultiStore

type Store struct {
	multiStore

	storeKeyToWriteWhitelist map[string][]string
}

func NewStore(parent storetypes.MultiStore, storeKeyToWriteWhitelist map[string][]string) storetypes.MultiStore {
	whitelist := make(map[string][]string, len(storeKeyToWriteWhitelist))
	for key, prefixes := range storeKeyToWriteWhitelist {
		whitelist[key] = append([]string(nil), prefixes...)
	}
	return &Store{
		multiStore:               parent,
		storeKeyToWriteWhitelist: whitelist,
	}
}

func (cms Store) CacheMultiStore() storetypes.CacheMultiStore {
	return cms.wrapMultiStore(cms.multiStore.CacheMultiStore()).(storetypes.CacheMultiStore)
}

func (cms Store) CacheMultiStoreWithVersion(version int64) (storetypes.CacheMultiStore, error) {
	branch, err := cms.multiStore.CacheMultiStoreWithVersion(version)
	if err != nil {
		return nil, err
	}
	return cms.wrapMultiStore(branch).(storetypes.CacheMultiStore), nil
}

func (cms Store) CacheMultiStoreForExport(version int64) (storetypes.CacheMultiStore, error) {
	branch, err := cms.multiStore.CacheMultiStoreForExport(version)
	if err != nil {
		return nil, err
	}
	return cms.wrapMultiStore(branch).(storetypes.CacheMultiStore), nil
}

func (cms Store) CacheWrap(key storetypes.StoreKey) storetypes.CacheWrap {
	return cms.wrapMultiStore(cms.multiStore.CacheWrap(key).(storetypes.CacheMultiStore)).(storetypes.CacheWrap)
}

func (cms Store) CacheWrapWithTrace(key storetypes.StoreKey, w io.Writer, tc storetypes.TraceContext) storetypes.CacheWrap {
	return cms.wrapMultiStore(cms.multiStore.CacheWrapWithTrace(key, w, tc).(storetypes.CacheMultiStore)).(storetypes.CacheWrap)
}

func (cms Store) GetStore(key storetypes.StoreKey) storetypes.Store {
	return cms.wrapKVStore(key, cms.multiStore.GetStore(key).(storetypes.KVStore))
}

func (cms Store) GetKVStore(key storetypes.StoreKey) storetypes.KVStore {
	return cms.wrapKVStore(key, cms.multiStore.GetKVStore(key))
}

func (cms Store) SetTracer(w io.Writer) storetypes.MultiStore {
	return cms.wrapMultiStore(cms.multiStore.SetTracer(w))
}

func (cms Store) SetTracingContext(tc storetypes.TraceContext) storetypes.MultiStore {
	return cms.wrapMultiStore(cms.multiStore.SetTracingContext(tc))
}

func (cms Store) SetKVStores(handler func(storetypes.StoreKey, storetypes.KVStore) storetypes.CacheWrap) storetypes.MultiStore {
	parent := cms.multiStore.SetKVStores(func(key storetypes.StoreKey, s storetypes.KVStore) storetypes.CacheWrap {
		return cms.wrapKVCache(key, handler(key, cms.wrapKVStore(key, s)))
	})
	return cms.wrapMultiStore(parent)
}

func (cms Store) wrapMultiStore(parent storetypes.MultiStore) storetypes.MultiStore {
	wrapped := &Store{multiStore: parent, storeKeyToWriteWhitelist: cms.storeKeyToWriteWhitelist}
	if cache, ok := parent.(storetypes.CacheMultiStore); ok {
		return &restrictedCacheMultiStore{Store: wrapped, cache: cache}
	}
	return wrapped
}

type restrictedCacheMultiStore struct {
	*Store
	cache storetypes.CacheMultiStore
}

func (cms *restrictedCacheMultiStore) Write() { cms.cache.Write() }
func (cms *restrictedCacheMultiStore) Close() { cms.cache.Close() }

type kvStore = storetypes.KVStore

type restrictedKVStore struct {
	kvStore
	prefixes []string
}

func (cms Store) wrapKVStore(key storetypes.StoreKey, parent storetypes.KVStore) storetypes.KVStore {
	prefixes := cms.storeKeyToWriteWhitelist[key.Name()]
	if cache, ok := parent.(storetypes.CacheWrap); ok {
		return newRestrictedKVCache(cache, prefixes)
	}
	return newRestrictedKVStore(parent, prefixes)
}

func newRestrictedKVStore(parent storetypes.KVStore, prefixes []string) *restrictedKVStore {
	return &restrictedKVStore{kvStore: kv.NewStore(parent, prefixes), prefixes: prefixes}
}

func (store *restrictedKVStore) CacheWrap(key storetypes.StoreKey) storetypes.CacheWrap {
	return newRestrictedKVCache(store.kvStore.CacheWrap(key), store.prefixes)
}

func (store *restrictedKVStore) CacheWrapWithTrace(key storetypes.StoreKey, w io.Writer, tc storetypes.TraceContext) storetypes.CacheWrap {
	return newRestrictedKVCache(store.kvStore.CacheWrapWithTrace(key, w, tc), store.prefixes)
}

type restrictedKVCache struct {
	*restrictedKVStore
	cache storetypes.CacheWrap
}

func newRestrictedKVCache(parent storetypes.CacheWrap, prefixes []string) *restrictedKVCache {
	return &restrictedKVCache{
		restrictedKVStore: newRestrictedKVStore(parent.(storetypes.KVStore), prefixes),
		cache:             parent,
	}
}

func (cms Store) wrapKVCache(key storetypes.StoreKey, parent storetypes.CacheWrap) storetypes.CacheWrap {
	return newRestrictedKVCache(parent, cms.storeKeyToWriteWhitelist[key.Name()])
}

func (store *restrictedKVCache) Write() { store.cache.Write() }
