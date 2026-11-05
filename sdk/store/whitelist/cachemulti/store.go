package cachemulti

import (
	"io"

	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/whitelist/kv"
)

// Since `CacheMultiStore` has a method with the same name, we have to
// type alias here or otherwise we won't be able to inherit or implement
// `CacheMultiStore` the method.
type sdkCacheMultiStore = storetypes.CacheMultiStore

type Store struct {
	sdkCacheMultiStore

	storeKeyToWriteWhitelist map[string][]string
}

func NewStore(parent storetypes.CacheMultiStore, storeKeyToWriteWhitelist map[string][]string) storetypes.CacheMultiStore {
	return &Store{
		sdkCacheMultiStore:       parent,
		storeKeyToWriteWhitelist: storeKeyToWriteWhitelist,
	}
}

func (cms Store) CacheMultiStore() storetypes.CacheMultiStore {
	return NewStore(cms.sdkCacheMultiStore.CacheMultiStore(), cms.storeKeyToWriteWhitelist)
}

func (cms Store) GetKVStore(key storetypes.StoreKey) storetypes.KVStore {
	return cms.wrapKVStore(key, cms.sdkCacheMultiStore.GetKVStore(key))
}

func (cms Store) GetStore(key storetypes.StoreKey) storetypes.Store {
	return cms.wrapKVStore(key, cms.sdkCacheMultiStore.GetStore(key).(storetypes.KVStore))
}

func (cms Store) wrapKVStore(key storetypes.StoreKey, parent storetypes.KVStore) storetypes.KVStore {
	return kv.NewStore(parent, cms.storeKeyToWriteWhitelist[key.Name()])
}

type transformedKVStore struct {
	storetypes.KVStore
	cacheWrap storetypes.CacheWrap
}

func (s transformedKVStore) Write() {
	s.cacheWrap.Write()
}

func (cms Store) SetKVStores(handler func(storetypes.StoreKey, storetypes.KVStore) storetypes.CacheWrap) storetypes.MultiStore {
	parent := cms.sdkCacheMultiStore.SetKVStores(func(key storetypes.StoreKey, parent storetypes.KVStore) storetypes.CacheWrap {
		transformed := handler(key, cms.wrapKVStore(key, parent))
		return transformedKVStore{
			KVStore:   cms.wrapKVStore(key, transformed.(storetypes.KVStore)),
			cacheWrap: transformed,
		}
	})
	return NewStore(parent.(storetypes.CacheMultiStore), cms.storeKeyToWriteWhitelist)
}

func (cms Store) CacheWrap(key storetypes.StoreKey) storetypes.CacheWrap {
	return NewStore(cms.sdkCacheMultiStore.CacheWrap(key).(storetypes.CacheMultiStore), cms.storeKeyToWriteWhitelist).(storetypes.CacheWrap)
}

func (cms Store) CacheWrapWithTrace(key storetypes.StoreKey, w io.Writer, tc storetypes.TraceContext) storetypes.CacheWrap {
	return NewStore(cms.sdkCacheMultiStore.CacheWrapWithTrace(key, w, tc).(storetypes.CacheMultiStore), cms.storeKeyToWriteWhitelist).(storetypes.CacheWrap)
}

func (cms Store) CacheMultiStoreWithVersion(version int64) (storetypes.CacheMultiStore, error) {
	parent, err := cms.sdkCacheMultiStore.CacheMultiStoreWithVersion(version)
	if err != nil {
		return nil, err
	}
	return NewStore(parent, cms.storeKeyToWriteWhitelist), nil
}

func (cms Store) CacheMultiStoreForExport(version int64) (storetypes.CacheMultiStore, error) {
	parent, err := cms.sdkCacheMultiStore.CacheMultiStoreForExport(version)
	if err != nil {
		return nil, err
	}
	return NewStore(parent, cms.storeKeyToWriteWhitelist), nil
}

func (cms Store) SetTracer(w io.Writer) storetypes.MultiStore {
	return NewStore(cms.sdkCacheMultiStore.SetTracer(w).(storetypes.CacheMultiStore), cms.storeKeyToWriteWhitelist)
}

func (cms Store) SetTracingContext(tc storetypes.TraceContext) storetypes.MultiStore {
	return NewStore(cms.sdkCacheMultiStore.SetTracingContext(tc).(storetypes.CacheMultiStore), cms.storeKeyToWriteWhitelist)
}
