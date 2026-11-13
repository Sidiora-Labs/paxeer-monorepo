package dbcache

import (
	"sync"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/types"
)

// NewCachedBatchForTest exposes the production wrapper to external-package tests.
func NewCachedBatchForTest(inner types.Batch, cache Cache) types.Batch {
	return newCachedBatch(inner, cache, &sync.Mutex{})
}
