package kv

import (
	"fmt"
	"io"
	"strings"

	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
)

type Store struct {
	parent         storetypes.KVStore
	writeWhitelist map[string]struct{}
}

var _ storetypes.KVStore = (*Store)(nil)

type cacheStore struct {
	*Store
	cache storetypes.CacheWrap
}

var _ storetypes.CacheKVStore = (*cacheStore)(nil)

func NewStore(parent storetypes.KVStore, writeWhitelistKeys []string) storetypes.KVStore {
	writeWhitelist := map[string]struct{}{}
	for _, writeWhitelistKey := range writeWhitelistKeys {
		writeWhitelist[writeWhitelistKey] = struct{}{}
	}
	return &Store{
		parent:         parent,
		writeWhitelist: writeWhitelist,
	}
}

func (store *Store) Set(key []byte, value []byte) {
	store.validateKeyForWrite(key)
	store.parent.Set(key, value)
}

func (store *Store) Delete(key []byte) {
	store.validateKeyForWrite(key)
	store.parent.Delete(key)
}

func (store *Store) DeleteAll(start, end []byte) error {
	keys := store.parent.GetAllKeyStrsInRange(start, end)
	// Check the entire range before deleting anything so a rejected range
	// cannot leave permitted keys partially deleted.
	for _, key := range keys {
		store.validateKeyForWrite([]byte(key))
	}
	for _, key := range keys {
		store.Delete([]byte(key))
	}
	return nil
}

func (store *Store) CacheWrap(storeKey storetypes.StoreKey) storetypes.CacheWrap {
	return store.wrapCache(store.parent.CacheWrap(storeKey))
}

func (store *Store) CacheWrapWithTrace(storeKey storetypes.StoreKey, w io.Writer, tc storetypes.TraceContext) storetypes.CacheWrap {
	return store.wrapCache(store.parent.CacheWrapWithTrace(storeKey, w, tc))
}

func (store *Store) wrapCache(cache storetypes.CacheWrap) storetypes.CacheWrap {
	return &cacheStore{
		Store: &Store{parent: cache.(storetypes.KVStore), writeWhitelist: store.writeWhitelist},
		cache: cache,
	}
}

func (store *cacheStore) Write() {
	store.cache.Write()
}

func (store *Store) Get(key []byte) []byte {
	return store.parent.Get(key)
}

func (store *Store) Has(key []byte) bool {
	return store.parent.Has(key)
}

func (store *Store) Iterator(start, end []byte) storetypes.Iterator {
	return store.parent.Iterator(start, end)
}

func (store *Store) ReverseIterator(start, end []byte) storetypes.Iterator {
	return store.parent.ReverseIterator(start, end)
}

func (store *Store) GetStoreType() storetypes.StoreType {
	return store.parent.GetStoreType()
}

func (store *Store) GetWorkingHash() ([]byte, error) {
	return store.parent.GetWorkingHash()
}

func (store *Store) VersionExists(version int64) bool {
	return store.parent.VersionExists(version)
}

func (store *Store) GetAllKeyStrsInRange(start, end []byte) []string {
	return store.parent.GetAllKeyStrsInRange(start, end)
}

func (store *Store) validateKeyForWrite(key []byte) {
	keyStr := string(key)
	for whitelist := range store.writeWhitelist {
		if strings.HasPrefix(keyStr, whitelist) {
			return
		}
	}
	// Panic since the Store interface does not return error on Set. Can be
	// intercepted by calling routine through `err := recover()`
	panic(fmt.Sprintf("Setting disallowed key %s in whitelisted KV store", keyStr))
}
