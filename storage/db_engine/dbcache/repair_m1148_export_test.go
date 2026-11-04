package dbcache

import (
	"sync"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/types"
)

// RepairM1148CachedBatch exposes the production wrapper to external-package tests.
func RepairM1148CachedBatch(inner types.Batch, cache Cache) types.Batch {
	return newCachedBatch(inner, cache, &sync.Mutex{})
}
