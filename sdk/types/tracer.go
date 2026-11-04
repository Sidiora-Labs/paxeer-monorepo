package types

import (
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"
	"sync"
	"time"
)

// Per-tx tracer caps. Bound memory and wire size so a pathological tx (huge
// iterator scans, many opened iterators) can't make the profile response
// unbounded. Values picked to comfortably cover normal EVM txs while
// capping worst-case overhead at a few MB per module.
const (
	maxStoreTraceIterators    = 16
	maxStoreTraceIteratorKeys = 64
	maxStoreTraceModules      = 64
	maxStoreTraceModuleBytes  = 256
	maxStoreTraceAccesses     = 4096
	maxStoreTraceBytes        = 1 << 20
	maxStoreTraceWireEntries  = 1024
	maxStoreTraceWireBytes    = 2 << 20
)

// StoreTracer collects bounded KVStore access samples (Get/Has/Set/Delete/iterator)
// performed under a debug_traceTransactionProfile call, grouped by module.
// One tracer is attached to the sdk.Context for the duration of the traced
// tx; callers must treat it as single-tx-scoped. All methods are safe for
// concurrent use.
type StoreTracer struct {
	Modules          map[string]*ModuleTrace
	nextIteratorID   int
	retainedAccesses int
	retainedBytes    int
	accessStopped    bool
	truncated        bool
	stats            map[string]OperationSummary
	mu               *sync.Mutex
}

// ModuleTrace holds a bounded prefix of access events for a module within a trace,
// plus a per-iterator roll-up.
type ModuleTrace struct {
	Accesses        []Access
	Iterators       []*IteratorTrace
	iteratorIndexBy map[int]int
	stats           map[string]OperationSummary
	Truncated       bool
}

// IteratorTrace aggregates one opened iterator: its bounds, direction, the
// keys it surfaced to the tx (capped at maxStoreTraceIteratorKeys; Truncated
// flags overflow), and cumulative Next() count + time.
type IteratorTrace struct {
	Start         []byte
	End           []byte
	Ascending     bool
	Keys          [][]byte
	NextCount     int
	DurationNanos int64
	Truncated     bool
}

// Access is a single access event in a module's chronological log. Value is
// only populated for Get/Set/IteratorValue; other ops leave it nil.
type Access struct {
	Op            OpType
	Key           []byte
	Value         []byte
	DurationNanos int64
}

// OpType tags an Access with the operation the tx performed.
type OpType int

const (
	Get OpType = iota
	Has
	Set
	Delete
	IteratorOpen
	IteratorNext
	IteratorValue
)

func (o OpType) String() string {
	switch o {
	case Get:
		return "get"
	case Has:
		return "has"
	case Set:
		return "set"
	case Delete:
		return "delete"
	case IteratorOpen:
		return "iterator"
	case IteratorNext:
		return "iteratorNext"
	case IteratorValue:
		return "iteratorValue"
	default:
		return "unknown"
	}
}

// NewStoreTracer returns an empty StoreTracer ready to record per-module
// access events for a single debug_traceTransactionProfile call.
func NewStoreTracer() *StoreTracer {
	return &StoreTracer{
		Modules: map[string]*ModuleTrace{},
		stats:   map[string]OperationSummary{},
		mu:      &sync.Mutex{},
	}
}

func (st *StoreTracer) Get(key []byte, value []byte, module string, duration time.Duration) {
	st.recordAccess(module, Access{
		Op:            Get,
		Key:           key,
		Value:         value,
		DurationNanos: duration.Nanoseconds(),
	})
}

func (st *StoreTracer) Set(key []byte, value []byte, module string, duration time.Duration) {
	st.recordAccess(module, Access{
		Op:            Set,
		Key:           key,
		Value:         value,
		DurationNanos: duration.Nanoseconds(),
	})
}

func (st *StoreTracer) Has(key []byte, module string, duration time.Duration) {
	st.recordAccess(module, Access{
		Op:            Has,
		Key:           key,
		DurationNanos: duration.Nanoseconds(),
	})
}

func (st *StoreTracer) Delete(key []byte, module string, duration time.Duration) {
	st.recordAccess(module, Access{
		Op:            Delete,
		Key:           key,
		DurationNanos: duration.Nanoseconds(),
	})
}

// StartIterator records the opening of an iterator over [start, end) and
// allocates a tracer-scoped iteratorID the caller uses to tag subsequent
// Next/Value events. Past maxStoreTraceIterators the IteratorTrace record is
// dropped and truncation is exposed. The returned ID lets later calls keep
// updating operation totals without retaining an iterator.
func (st *StoreTracer) StartIterator(start, end []byte, ascending bool, module string, duration time.Duration) int {
	st.mu.Lock()
	defer st.mu.Unlock()

	mt := st.getOrSetModuleTrace(module)
	st.nextIteratorID++
	iteratorID := st.nextIteratorID

	st.recordAccessLocked(mt, Access{
		Op: IteratorOpen, Key: start, Value: end, DurationNanos: duration.Nanoseconds(),
	})
	if mt == nil {
		return iteratorID
	}
	if len(mt.Iterators) >= maxStoreTraceIterators || !st.retainBytes(len(start), len(end)) {
		st.markTruncated(mt)
		return iteratorID
	}

	idx := len(mt.Iterators)
	mt.Iterators = append(mt.Iterators, &IteratorTrace{
		Start:         slices.Clone(start),
		End:           slices.Clone(end),
		Ascending:     ascending,
		DurationNanos: duration.Nanoseconds(),
	})
	mt.iteratorIndexBy[iteratorID] = idx
	return iteratorID
}

// RecordIteratorValue records that the tx read the current key/value from
// the iterator identified by iteratorID. Beyond maxStoreTraceIteratorKeys the
// iterator is flagged Truncated and further keys are dropped from the
// per-iterator sample. Operation totals continue after retention fills.
func (st *StoreTracer) RecordIteratorValue(iteratorID int, key []byte, value []byte, module string) {
	st.mu.Lock()
	defer st.mu.Unlock()

	mt := st.getOrSetModuleTrace(module)
	st.recordAccessLocked(mt, Access{Op: IteratorValue, Key: key, Value: value})
	if mt == nil {
		return
	}
	idx, ok := mt.iteratorIndexBy[iteratorID]
	if !ok {
		return
	}
	it := mt.Iterators[idx]
	if len(it.Keys) >= maxStoreTraceIteratorKeys || !st.retainBytes(len(key), 0) {
		it.Truncated = true
		st.markTruncated(mt)
		return
	}
	it.Keys = append(it.Keys, slices.Clone(key))
}

// RecordIteratorNext records a Next() advance on the iterator identified by
// iteratorID, adding to its cumulative step count and stepping time.
func (st *StoreTracer) RecordIteratorNext(iteratorID int, module string, duration time.Duration) {
	st.mu.Lock()
	defer st.mu.Unlock()

	mt := st.getOrSetModuleTrace(module)
	st.recordAccessLocked(mt, Access{
		Op: IteratorNext, DurationNanos: duration.Nanoseconds(),
	})
	if mt == nil {
		return
	}
	idx, ok := mt.iteratorIndexBy[iteratorID]
	if !ok {
		return
	}
	it := mt.Iterators[idx]
	it.NextCount++
	it.DurationNanos += duration.Nanoseconds()
}

func (st *StoreTracer) getOrSetModuleTrace(module string) *ModuleTrace {
	if len(module) > maxStoreTraceModuleBytes {
		st.truncated = true
		return nil
	}
	if mt, ok := st.Modules[module]; ok {
		return mt
	}
	if len(st.Modules) >= maxStoreTraceModules || !st.retainBytes(len(module), 0) {
		st.truncated = true
		return nil
	}
	mt := &ModuleTrace{
		Accesses: []Access{}, Iterators: []*IteratorTrace{},
		iteratorIndexBy: map[int]int{}, stats: map[string]OperationSummary{},
	}
	st.Modules[strings.Clone(module)] = mt
	return mt
}

func (st *StoreTracer) markTruncated(mt *ModuleTrace) {
	st.truncated = true
	if mt != nil {
		mt.Truncated = true
	}
}

// retainBytes reserves all payload bytes before any cloning, including
// iterator samples and module names. Subtractions avoid length overflow.
func (st *StoreTracer) retainBytes(keyBytes, valueBytes int) bool {
	remaining := maxStoreTraceBytes - st.retainedBytes
	if keyBytes > remaining || valueBytes > remaining-keyBytes {
		return false
	}
	st.retainedBytes += keyBytes + valueBytes
	return true
}

func (st *StoreTracer) recordAccessLocked(mt *ModuleTrace, access Access) {
	updateSummary(st.stats, access.Op, access.DurationNanos)
	if mt == nil {
		return
	}
	updateSummary(mt.stats, access.Op, access.DurationNanos)
	if st.accessStopped || st.retainedAccesses >= maxStoreTraceAccesses ||
		!st.retainBytes(len(access.Key), len(access.Value)) {
		// Retain a prefix: resuming after an omitted write could mislabel a
		// later read of that key as prestate.
		st.accessStopped = true
		st.markTruncated(mt)
		return
	}
	access.Key = slices.Clone(access.Key)
	access.Value = slices.Clone(access.Value)
	mt.Accesses = append(mt.Accesses, access)
	st.retainedAccesses++
}

func (st *StoreTracer) recordAccess(module string, access Access) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.recordAccessLocked(st.getOrSetModuleTrace(module), access)
}

// Clear resets the tracer to its empty state so a single StoreTracer can be
// reused across successive trace requests on the same connection.
func (st *StoreTracer) Clear() {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.Modules = map[string]*ModuleTrace{}
	st.nextIteratorID = 0
	st.retainedAccesses = 0
	st.retainedBytes = 0
	st.accessStopped = false
	st.truncated = false
	st.stats = map[string]OperationSummary{}
}

type OperationSummary struct {
	Count      int   `json:"count"`
	TotalNanos int64 `json:"totalNanos"`
}

type StoreTraceDump struct {
	Truncated bool                        `json:"truncated,omitempty"`
	Modules   map[string]ModuleTraceDump  `json:"modules"`
	Stats     map[string]OperationSummary `json:"stats,omitempty"`
}

type ModuleTraceDump struct {
	Truncated bool                        `json:"truncated,omitempty"`
	Reads     map[string]string           `json:"reads"`
	Has       []string                    `json:"has"`
	Stats     map[string]OperationSummary `json:"stats,omitempty"`
	Iterators []IteratorTraceDump         `json:"iterators,omitempty"`
}

type IteratorTraceDump struct {
	Start      string   `json:"start,omitempty"`
	End        string   `json:"end,omitempty"`
	Ascending  bool     `json:"ascending"`
	Keys       []string `json:"keys,omitempty"`
	NextCount  int      `json:"nextCount"`
	TotalNanos int64    `json:"totalNanos"`
	Truncated  bool     `json:"truncated,omitempty"`
}

// Dump materializes the tracer's accumulated per-module accesses into a
// wire-shaped StoreTraceDump. Reads that were later overwritten by a Set or
// Delete during the same tx are excluded so the Reads map reflects the
// pre-state the tx observed.
func (st *StoreTracer) Dump() StoreTraceDump {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.dumpLocked()
}

// traceWireBudget bounds materialization before hex encoding or map insertion.
// The payload cap plus fixed caps on modules, entries and iterator records also
// bounds JSON framing and escaping independently of transaction size.
type traceWireBudget struct {
	entries int
	bytes   int
}

func (b *traceWireBudget) take(keyBytes, valueBytes int) bool {
	remaining := (maxStoreTraceWireBytes - b.bytes) / 2
	if b.entries >= maxStoreTraceWireEntries || keyBytes > remaining || valueBytes > remaining-keyBytes {
		return false
	}
	b.entries++
	b.bytes += 2 * (keyBytes + valueBytes)
	return true
}

func cloneTraceStats(stats map[string]OperationSummary) map[string]OperationSummary {
	result := make(map[string]OperationSummary, len(stats))
	for op, summary := range stats {
		result[op] = summary
	}
	return result
}

func (st *StoreTracer) dumpLocked() StoreTraceDump {
	d := StoreTraceDump{
		Modules: make(map[string]ModuleTraceDump, len(st.Modules)),
		Stats:   cloneTraceStats(st.stats), Truncated: st.truncated,
	}
	budget := traceWireBudget{}
	// Stable selection when the shared response budget fills.
	names := make([]string, 0, len(st.Modules))
	for name := range st.Modules {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		module := st.Modules[name]
		mtd := ModuleTraceDump{
			Reads: make(map[string]string), Has: []string{},
			Stats: cloneTraceStats(module.stats), Truncated: module.Truncated,
			Iterators: make([]IteratorTraceDump, 0, len(module.Iterators)),
		}
		writtenKey := map[string]struct{}{}
		hasMap := map[string]struct{}{}
		for _, a := range module.Accesses {
			switch a.Op {
			case Get, IteratorValue, Has:
				if _, ok := writtenKey[string(a.Key)]; ok {
					continue
				}
				if !budget.take(len(a.Key), len(a.Value)) {
					mtd.Truncated = true
					continue
				}
				if a.Op == Has {
					hasMap[hex.EncodeToString(a.Key)] = struct{}{}
				} else {
					mtd.Reads[hex.EncodeToString(a.Key)] = hex.EncodeToString(a.Value)
				}
			case Set, Delete:
				writtenKey[string(a.Key)] = struct{}{}
			}
		}
		for k := range hasMap {
			mtd.Has = append(mtd.Has, k)
		}
		slices.Sort(mtd.Has)
		for _, it := range module.Iterators {
			if !budget.take(len(it.Start), len(it.End)) {
				mtd.Truncated = true
				continue
			}
			itd := IteratorTraceDump{
				Start: hex.EncodeToString(it.Start), End: hex.EncodeToString(it.End),
				Ascending: it.Ascending, NextCount: it.NextCount,
				TotalNanos: it.DurationNanos, Truncated: it.Truncated,
			}
			for _, key := range it.Keys {
				if !budget.take(len(key), 0) {
					itd.Truncated = true
					mtd.Truncated = true
					break
				}
				itd.Keys = append(itd.Keys, hex.EncodeToString(key))
			}
			mtd.Iterators = append(mtd.Iterators, itd)
		}
		if mtd.Truncated {
			d.Truncated = true
		}
		d.Modules[name] = mtd
	}
	return d
}

func updateSummary(stats map[string]OperationSummary, op OpType, durationNanos int64) {
	key := op.String()
	summary := stats[key]
	summary.Count++
	summary.TotalNanos += durationNanos
	stats[key] = summary
}

// DerivePrestateToJson returns a JSON encoding of the current trace state,
// used by debug_traceTransaction to attach AppState to the response.
func (st *StoreTracer) DerivePrestateToJson() []byte {
	st.mu.Lock()
	defer st.mu.Unlock()
	d := st.dumpLocked()
	bz, err := json.Marshal(&d)
	if err != nil {
		panic(err)
	}
	return bz
}
