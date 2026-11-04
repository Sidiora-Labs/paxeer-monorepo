package types

import "math/big"

const (
	RepairM646MaxIterators    = maxStoreTraceIterators
	RepairM646MaxIteratorKeys = maxStoreTraceIteratorKeys
	RepairM646MaxModules      = maxStoreTraceModules
	RepairM646MaxModuleBytes  = maxStoreTraceModuleBytes
	RepairM646MaxAccesses     = maxStoreTraceAccesses
	RepairM646MaxBytes        = maxStoreTraceBytes
	RepairM646MaxWireEntries  = maxStoreTraceWireEntries
	RepairM646MaxWireBytes    = maxStoreTraceWireBytes
)

type RepairM646TraceSnapshot struct {
	Accesses            int
	Bytes               int
	Truncated           bool
	IteratorIndexCounts map[string]int
}

func RepairM646TraceState(st *StoreTracer) RepairM646TraceSnapshot {
	st.mu.Lock()
	defer st.mu.Unlock()
	state := RepairM646TraceSnapshot{
		Accesses: st.retainedAccesses, Bytes: st.retainedBytes,
		Truncated: st.truncated, IteratorIndexCounts: map[string]int{},
	}
	for name, module := range st.Modules {
		state.IteratorIndexCounts[name] = len(module.iteratorIndexBy)
	}
	return state
}

func RepairM646UintPointer(u Uint) *big.Int { return u.i }
