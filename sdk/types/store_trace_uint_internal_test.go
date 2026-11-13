package types

import "math/big"

const (
	StoreTraceMaxIterators    = maxStoreTraceIterators
	StoreTraceMaxIteratorKeys = maxStoreTraceIteratorKeys
	StoreTraceMaxModules      = maxStoreTraceModules
	StoreTraceMaxModuleBytes  = maxStoreTraceModuleBytes
	StoreTraceMaxAccesses     = maxStoreTraceAccesses
	StoreTraceMaxBytes        = maxStoreTraceBytes
	StoreTraceMaxWireEntries  = maxStoreTraceWireEntries
	StoreTraceMaxWireBytes    = maxStoreTraceWireBytes
)

type StoreTraceSnapshot struct {
	Accesses            int
	Bytes               int
	Truncated           bool
	IteratorIndexCounts map[string]int
}

func StoreTraceState(st *StoreTracer) StoreTraceSnapshot {
	st.mu.Lock()
	defer st.mu.Unlock()
	state := StoreTraceSnapshot{
		Accesses: st.retainedAccesses, Bytes: st.retainedBytes,
		Truncated: st.truncated, IteratorIndexCounts: map[string]int{},
	}
	for name, module := range st.Modules {
		state.IteratorIndexCounts[name] = len(module.iteratorIndexBy)
	}
	return state
}

func UintPointerForTest(u Uint) *big.Int { return u.i }
