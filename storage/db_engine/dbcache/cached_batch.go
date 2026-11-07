package dbcache

import (
	"fmt"
	"sync"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/types"
)

// cachedBatch wraps a types.Batch and applies pending mutations to the cache
// after a successful commit.
type cachedBatch struct {
	inner   types.Batch
	cache   Cache
	pending []CacheUpdate
	writeMu *sync.Mutex
}

var _ types.Batch = (*cachedBatch)(nil)

func newCachedBatch(inner types.Batch, cache Cache, writeMu *sync.Mutex) *cachedBatch {
	return &cachedBatch{inner: inner, cache: cache, writeMu: writeMu}
}

func (cb *cachedBatch) Set(key, value []byte) error {
	if err := cb.inner.Set(key, value); err != nil {
		return err
	}
	cb.pending = append(cb.pending, CacheUpdate{Key: key, Value: value})
	return nil
}

func (cb *cachedBatch) Delete(key []byte) error {
	if err := cb.inner.Delete(key); err != nil {
		return err
	}
	cb.pending = append(cb.pending, CacheUpdate{Key: key, Value: nil})
	return nil
}

func (cb *cachedBatch) Commit(opts types.WriteOptions) error {
	cb.writeMu.Lock()
	defer cb.writeMu.Unlock()

	if err := cb.inner.Commit(opts); err != nil {
		return err
	}
	if err := cb.cache.BatchSet(cb.pending); err != nil {
		// A cache write can only fail during a shutdown when the cache's context is cancelled,
		// or when the cache's work pools have their contexts cancelled. Continuing to use the
		// cache after shutdown is not permissible, and so this method must return an error.
		return fmt.Errorf("failed to update cache after commit: %w", err)
	}
	cb.pending = nil
	return nil
}

func (cb *cachedBatch) Len() int {
	return cb.inner.Len()
}

func (cb *cachedBatch) Reset() {
	cb.inner.Reset()
	cb.pending = nil
}

func (cb *cachedBatch) Close() error {
	return cb.inner.Close()
}
