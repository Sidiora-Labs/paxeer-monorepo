package store

import (
	"io"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/cachekv"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/tracekv"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	dbm "github.com/tendermint/tm-db"
)

type iterationView struct {
	*Store
}

func NewIterationView(parent types.KVStore) types.KVStore {
	if store, ok := parent.(*Store); ok {
		return &iterationView{Store: store}
	}
	return parent
}

func (view *iterationView) Iterator(start, end []byte) types.Iterator {
	return view.iterator(start, end, true)
}

func (view *iterationView) ReverseIterator(start, end []byte) types.Iterator {
	return view.iterator(start, end, false)
}

func (view *iterationView) iterator(start, end []byte, ascending bool) types.Iterator {
	view.mtx.RLock()
	defer view.mtx.RUnlock()
	merged := cachekv.NewStore(NewIterationView(view.parent), view.storeKey, view.cacheSize)
	view.cache.Range(func(key, value any) bool {
		keyBytes := []byte(key.(string))
		if dbm.IsKeyInDomain(keyBytes, start, end) {
			if cached := value.(*types.CValue).Value(); cached != nil {
				merged.Set(keyBytes, cached)
			} else {
				merged.Delete(keyBytes)
			}
		}
		return true
	})
	if ascending {
		return merged.Iterator(start, end)
	}
	return merged.ReverseIterator(start, end)
}

func (view *iterationView) CacheWrap(key types.StoreKey) types.CacheWrap {
	return cachekv.NewStore(view, key, view.cacheSize)
}

func (view *iterationView) CacheWrapWithTrace(key types.StoreKey, w io.Writer, tc types.TraceContext) types.CacheWrap {
	return cachekv.NewStore(tracekv.NewStore(view, w, tc), key, view.cacheSize)
}
