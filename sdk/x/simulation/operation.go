package simulation

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"sort"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/simulation"
)

// entry kinds for use within OperationEntry
const (
	BeginBlockEntryKind = "begin_block"
	EndBlockEntryKind   = "end_block"
	MsgEntryKind        = "msg"
	QueuedMsgEntryKind  = "queued_msg"
)

// OperationEntry - an operation entry for logging (ex. BeginBlock, EndBlock, XxxMsg, etc)
type OperationEntry struct {
	EntryKind string          `json:"entry_kind" yaml:"entry_kind"`
	Height    int64           `json:"height" yaml:"height"`
	Order     int64           `json:"order" yaml:"order"`
	Operation json.RawMessage `json:"operation" yaml:"operation"`
}

// NewOperationEntry creates a new OperationEntry instance
func NewOperationEntry(entry string, height, order int64, op json.RawMessage) OperationEntry {
	return OperationEntry{
		EntryKind: entry,
		Height:    height,
		Order:     order,
		Operation: op,
	}
}

// BeginBlockEntry - operation entry for begin block
func BeginBlockEntry(height int64) OperationEntry {
	return NewOperationEntry(BeginBlockEntryKind, height, -1, nil)
}

// EndBlockEntry - operation entry for end block
func EndBlockEntry(height int64) OperationEntry {
	return NewOperationEntry(EndBlockEntryKind, height, -1, nil)
}

// MsgEntry - operation entry for standard msg
func MsgEntry(height, order int64, opMsg simulation.OperationMsg) OperationEntry {
	return NewOperationEntry(MsgEntryKind, height, order, opMsg.MustMarshal())
}

// QueuedMsgEntry creates an operation entry for a given queued message.
func QueuedMsgEntry(height int64, opMsg simulation.OperationMsg) OperationEntry {
	return NewOperationEntry(QueuedMsgEntryKind, height, -1, opMsg.MustMarshal())
}

// MustMarshal marshals the operation entry, panic on error.
func (oe OperationEntry) MustMarshal() json.RawMessage {
	out, err := json.Marshal(oe)
	if err != nil {
		panic(err)
	}

	return out
}

// OperationQueue defines an object for a queue of operations
type OperationQueue map[int][]simulation.Operation

// NewOperationQueue creates a new OperationQueue instance.
func NewOperationQueue() OperationQueue {
	return make(OperationQueue)
}

// queueOperations adds all future operations into the operation queue.
func queueOperations(queuedOps OperationQueue, queuedTimeOps []simulation.FutureOperation, futureOps []simulation.FutureOperation) []simulation.FutureOperation {
	if futureOps == nil {
		return queuedTimeOps
	}

	for _, futureOp := range futureOps {
		futureOp := futureOp
		if futureOp.BlockHeight != 0 {
			if val, ok := queuedOps[futureOp.BlockHeight]; ok {
				queuedOps[futureOp.BlockHeight] = append(val, futureOp.Op)
			} else {
				queuedOps[futureOp.BlockHeight] = []simulation.Operation{futureOp.Op}
			}

			continue
		}

		// TODO: Replace with proper sorted data structure, so don't have the
		// copy entire slice
		index := sort.Search(
			len(queuedTimeOps),
			func(i int) bool {
				return queuedTimeOps[i].BlockTime.After(futureOp.BlockTime)
			},
		)

		queuedTimeOps = append(queuedTimeOps, simulation.FutureOperation{})
		copy(queuedTimeOps[index+1:], queuedTimeOps[index:])
		queuedTimeOps[index] = futureOp
	}
	return queuedTimeOps
}

// WeightedOperation is an operation with associated weight.
// This is used to bias the selection operation within the simulator.
type WeightedOperation struct {
	weight int
	op     simulation.Operation
}

func (w WeightedOperation) Weight() int {
	return w.weight
}

func (w WeightedOperation) Op() simulation.Operation {
	return w.op
}

// NewWeightedOperation creates a new WeightedOperation instance
func NewWeightedOperation(weight int, op simulation.Operation) WeightedOperation {
	return WeightedOperation{
		weight: weight,
		op:     op,
	}
}

// WeightedOperations is the group of all weighted operations to simulate.
type WeightedOperations []simulation.WeightedOperation

// getSelectOpFn validates and snapshots all weights before admitting a run.
func (ops WeightedOperations) getSelectOpFn() (simulation.SelectOpFn, error) {
	total := 0
	maxInt := int(^uint(0) >> 1)
	admitted := make([]WeightedOperation, len(ops))
	for i, weighted := range ops {
		if weighted == nil || (reflect.ValueOf(weighted).Kind() == reflect.Ptr && reflect.ValueOf(weighted).IsNil()) {
			return nil, fmt.Errorf("operation %d: nil weighted operation", i)
		}
		weight := weighted.Weight()
		if weight < 0 {
			return nil, fmt.Errorf("operation %d: negative weight %d", i, weight)
		}
		if weight > maxInt-total {
			return nil, fmt.Errorf("operation %d: total weight overflows int", i)
		}
		op := weighted.Op()
		if weight > 0 && op == nil {
			return nil, fmt.Errorf("operation %d: positive weight %d has nil operation", i, weight)
		}
		total += weight
		admitted[i] = NewWeightedOperation(weight, op)
	}
	if total == 0 {
		return nil, fmt.Errorf("operation weights: total must be positive (empty or all disabled)")
	}
	return func(r *rand.Rand) simulation.Operation {
		x := r.Intn(total)
		for _, op := range admitted {
			if x < op.Weight() {
				return op.Op()
			}
			x -= op.Weight()
		}
		panic("validated operation weights exhausted")
	}, nil
}
