package util

import (
	"runtime"
	"time"
)

var _ Cache[string, string] = &FIFOCache[string, string]{}

// FIFOCache is a cache that evicts the least recently added item when the cache is full. Useful for situations
// where time of addition is a better predictor of future access than time of most recent access.
type FIFOCache[K comparable, V any] struct {
	weightCalculator WeightCalculator[K, V]

	currentWeight uint64
	maxWeight     uint64
	data          map[K]V
	evictionQueue *Queue[*insertionRecord]
	metrics       *CacheMetrics
	occupancy     *cacheOccupancy
}

// cacheOccupancy outlives its FIFO just long enough to release the aggregate
// contribution when the cache is collected. It must never reference the FIFO.
type cacheOccupancy struct {
	metrics *CacheMetrics
	size    int
	weight  uint64
}

// insertionRecord is a record of when a key was inserted into the cache, and is used to decide when it should be
// evicted.
type insertionRecord struct {
	// The key that was added to the cache.
	key any
	// The time at which the key was added to the cache.
	timestamp time.Time
}

// NewFIFOCache creates a new FIFOCache. If the calculator is nil, the weight of each key-value pair will be 1.
func NewFIFOCache[K comparable, V any](
	maxWeight uint64,
	calculator WeightCalculator[K, V],
	metrics *CacheMetrics) Cache[K, V] {

	if calculator == nil {
		calculator = func(K, V) uint64 { return 1 }
	}

	fifo := &FIFOCache[K, V]{
		maxWeight:        maxWeight,
		data:             make(map[K]V),
		weightCalculator: calculator,
		evictionQueue:    NewQueue[*insertionRecord](1024),
		metrics:          metrics,
		occupancy:        &cacheOccupancy{metrics: metrics},
	}
	if metrics != nil {
		runtime.AddCleanup(fifo, func(state *cacheOccupancy) { state.metrics.reportSizeChange(state.size, state.weight, 0, 0) }, fifo.occupancy)
	}
	return fifo
}

func (f *FIFOCache[K, V]) Get(key K) (V, bool) {
	val, ok := f.data[key]
	return val, ok
}

func (f *FIFOCache[K, V]) Put(key K, value V) {
	weight := f.weightCalculator(key, value)
	if weight > f.maxWeight {
		// this item won't fit in the cache no matter what we evict
		return
	}

	oldSize, oldWeight := len(f.data), f.currentWeight
	old, ok := f.data[key]
	f.currentWeight += weight
	f.data[key] = value
	if ok {
		oldWeight := f.weightCalculator(key, old)
		f.currentWeight -= oldWeight
	} else {
		f.evictionQueue.Push(&insertionRecord{
			key:       key,
			timestamp: time.Now(),
		})
	}

	if f.currentWeight > f.maxWeight {
		f.evict()
	}

	f.metrics.reportInsertion(weight)
	f.metrics.reportSizeChange(oldSize, oldWeight, len(f.data), f.currentWeight)
	f.occupancy.size, f.occupancy.weight = len(f.data), f.currentWeight
	runtime.KeepAlive(f)
}

func (f *FIFOCache[K, V]) evict() {
	now := time.Now()

	for f.currentWeight > f.maxWeight {
		next := f.evictionQueue.Pop()
		keyToEvict := next.key.(K)
		weightToEvict := f.weightCalculator(keyToEvict, f.data[keyToEvict])
		delete(f.data, keyToEvict)
		f.currentWeight -= weightToEvict
		f.metrics.reportEviction(now.Sub(next.timestamp))
	}
}

func (f *FIFOCache[K, V]) Size() int {
	return len(f.data)
}

func (f *FIFOCache[K, V]) Weight() uint64 {
	return f.currentWeight
}

func (f *FIFOCache[K, V]) SetMaxWeight(capacity uint64) {
	oldSize, oldWeight := len(f.data), f.currentWeight
	f.maxWeight = capacity
	f.evict()
	f.metrics.reportSizeChange(oldSize, oldWeight, len(f.data), f.currentWeight)
	f.occupancy.size, f.occupancy.weight = len(f.data), f.currentWeight
	runtime.KeepAlive(f)
}
