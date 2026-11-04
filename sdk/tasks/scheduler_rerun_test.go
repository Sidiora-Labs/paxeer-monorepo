package tasks

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/utils/tracing"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func newRerunTestScheduler(t *testing.T) (*scheduler, sdk.Context, map[int]int) {
	t.Helper()
	ctx := initTestCtx(true)
	ctx.MultiStore().GetKVStore(testStoreKey).Set(itemKey, []byte("before"))
	executions := make(map[int]int)
	ti := tracing.NewTracingInfo(trace.NewNoopTracerProvider().Tracer("scheduler-rerun-test"), true)
	s := NewScheduler(1, ti, func(ctx sdk.Context, req types.RequestDeliverTxV2, tx sdk.Tx, checksum [32]byte) types.ResponseDeliverTx {
		executions[ctx.TxIndex()]++
		kv := ctx.MultiStore().GetKVStore(testStoreKey)
		if ctx.TxIndex() == 0 {
			kv.Set(itemKey, req.Tx)
			return types.ResponseDeliverTx{Info: string(req.Tx)}
		}

		value := kv.Get(itemKey)
		kv.Set(req.Tx, value)
		iterator := kv.Iterator(nil, nil)
		for ; iterator.Valid(); iterator.Next() {
			_ = iterator.Value()
		}
		require.NoError(t, iterator.Close())
		return types.ResponseDeliverTx{Info: string(value)}
	}).(*scheduler)
	s.allTasks, s.allTasksMap = toTasks(requestList(2))
	s.conflictKeyCounts = make(map[string]int)
	s.tryInitMultiVersionStore(ctx)
	for _, task := range s.allTasks {
		task.Ctx = ctx
	}
	return s, ctx, executions
}

func TestShouldRerunInvalidatesValidatedTaskOnNewConflict(t *testing.T) {
	s, ctx, executions := newRerunTestScheduler(t)
	earlier, task := s.allTasks[0], s.allTasks[1]
	mv := s.multiVersionStores[testStoreKey]

	// Validate the reader before an earlier transaction publishes its new write.
	s.executeTask(task)
	require.Equal(t, "before", task.Response.Info)
	require.False(t, s.shouldRerun(task))
	require.True(t, task.IsStatus(statusValidated))
	require.NotEmpty(t, mv.GetReadset(task.AbsoluteIndex))
	require.NotEmpty(t, mv.GetIterateset(task.AbsoluteIndex))
	write := mv.GetLatest(task.Request.Tx)
	require.NotNil(t, write)
	require.False(t, write.IsEstimate())

	s.executeTask(earlier)
	require.False(t, s.shouldRerun(earlier))
	require.True(t, earlier.IsStatus(statusValidated))
	valid, conflicts, keys := s.findConflicts(task)
	require.False(t, valid)
	require.Equal(t, []int{earlier.AbsoluteIndex}, conflicts)
	require.Contains(t, keys, testStoreKey.Name()+"/"+string(itemKey))

	require.True(t, s.shouldRerun(task))
	require.Contains(t, task.Dependencies, earlier.AbsoluteIndex)
	write = mv.GetLatest(task.Request.Tx)
	require.NotNil(t, write)
	require.True(t, write.IsEstimate(), "the stale writeset must be invalidated before rerunning")
	require.Equal(t, task.AbsoluteIndex, write.Index())
	require.Equal(t, task.Incarnation, write.Incarnation())
	require.Nil(t, mv.GetReadset(task.AbsoluteIndex))
	require.Nil(t, mv.GetIterateset(task.AbsoluteIndex))
	require.Equal(t, 1, s.conflictKeyCounts[testStoreKey.Name()+"/"+string(itemKey)])

	task.Reset()
	task.Increment()
	task.Ctx = ctx
	s.executeTask(task)
	require.False(t, s.shouldRerun(task))
	require.True(t, task.IsStatus(statusValidated))
	require.Equal(t, "0", task.Response.Info)
	require.Equal(t, 2, executions[task.AbsoluteIndex])
	require.Equal(t, []byte("0"), mv.GetLatest(task.Request.Tx).Value())
}

func TestShouldRerunKeepsValidatedTaskWithoutConflict(t *testing.T) {
	s, _, executions := newRerunTestScheduler(t)
	earlier, task := s.allTasks[0], s.allTasks[1]
	mv := s.multiVersionStores[testStoreKey]

	s.executeTask(earlier)
	require.False(t, s.shouldRerun(earlier))
	require.True(t, earlier.IsStatus(statusValidated))
	s.executeTask(task)
	require.False(t, s.shouldRerun(task))
	require.True(t, task.IsStatus(statusValidated))
	response, incarnation := task.Response, task.Incarnation
	readset, iterateset := mv.GetReadset(task.AbsoluteIndex), mv.GetIterateset(task.AbsoluteIndex)
	require.NotEmpty(t, readset)
	require.NotEmpty(t, iterateset)
	valid, conflicts, keys := s.findConflicts(task)
	require.True(t, valid)
	require.Empty(t, conflicts)
	require.Empty(t, keys)

	require.False(t, s.shouldRerun(task), "a validated task with unchanged inputs must not rerun")
	require.True(t, task.IsStatus(statusValidated))
	require.Same(t, response, task.Response)
	require.Equal(t, "0", task.Response.Info)
	require.Equal(t, incarnation, task.Incarnation)
	require.Empty(t, task.Dependencies)
	require.Equal(t, readset, mv.GetReadset(task.AbsoluteIndex))
	require.Equal(t, iterateset, mv.GetIterateset(task.AbsoluteIndex))
	write := mv.GetLatest(task.Request.Tx)
	require.NotNil(t, write)
	require.False(t, write.IsEstimate())
	require.Equal(t, []byte("0"), write.Value())
	require.Empty(t, s.conflictKeyCounts)
	require.Equal(t, map[int]int{0: 1, 1: 1}, executions)
}

func TestProcessAllRetriesLowerIndexBeforeReader(t *testing.T) {
	proveProcessAllOrderedRerun(t, false)
}

// Both scenarios run the production dispatcher and multiversion stores. The
// predecessor stays delayed until both later transactions have taken snapshots.
func proveProcessAllOrderedRerun(t *testing.T, iteratorConflict bool) {
	t.Helper()
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	t.Cleanup(func() { require.NoError(t, tp.Shutdown(context.Background())) })
	ctx := initTestCtx(true)
	kv := ctx.MultiStore().GetKVStore(testStoreKey)
	kv.Set(itemKey, []byte("before"))
	kv.Set([]byte("bridge"), []byte("before"))
	firstRead, lastRead := make(chan struct{}), make(chan struct{})
	var attempts [3]atomic.Int32
	var initial [3]string
	var prefixWasStable atomic.Bool
	var suffixUntouched atomic.Bool
	var s *scheduler
	deliver := func(ctx sdk.Context, req types.RequestDeliverTxV2, tx sdk.Tx, checksum [32]byte) (res types.ResponseDeliverTx) {
		defer abortRecoveryFunc(&res)
		idx := ctx.TxIndex()
		attempt := attempts[idx].Add(1)
		kv := ctx.MultiStore().GetKVStore(testStoreKey)
		if idx == 0 {
			<-firstRead
			<-lastRead
			kv.Set(itemKey, []byte("after"))
			kv.Set([]byte("range/new"), []byte("after"))
			return types.ResponseDeliverTx{Info: "writer"}
		}
		var value string
		if idx == 2 {
			value = string(kv.Get([]byte("bridge")))
		} else if !iteratorConflict {
			value = string(kv.Get(itemKey))
		} else {
			value = "before"
		}
		iterator := kv.Iterator([]byte("range/"), []byte("range0"))
		for ; iterator.Valid(); iterator.Next() {
			if idx == 1 && iteratorConflict {
				value = string(iterator.Value())
			}
		}
		if err := iterator.Close(); err != nil {
			panic(err)
		}
		if attempt == 1 {
			initial[idx] = value
			if idx == 1 {
				close(firstRead)
				<-lastRead
			} else {
				close(lastRead)
			}
		} else if idx == 1 {
			prefixWasStable.Store(s.allTasksMap[0].IsStatus(statusValidated))
			mv := s.multiVersionStores[testStoreKey]
			suffixUntouched.Store(s.allTasksMap[2].IsStatus(statusExecuted) &&
				len(mv.GetReadset(2)) > 0 && len(mv.GetIterateset(2)) > 0)
		}
		if idx == 1 {
			kv.Set([]byte("bridge"), []byte(value))
		} else {
			kv.Set([]byte("result"), []byte(value))
		}
		return types.ResponseDeliverTx{Info: value}
	}
	s = NewScheduler(3, tracing.NewTracingInfo(tp.Tracer("ordered-rerun"), true), deliver).(*scheduler)
	// Response order follows requests; validation follows absolute transaction indices.
	reqs := requestList(3)
	reqs[0], reqs[2] = reqs[2], reqs[0]
	responses, err := s.ProcessAll(ctx, reqs)
	require.NoError(t, err)
	require.Len(t, responses, 3)
	require.Equal(t, "before", initial[1])
	require.Equal(t, "before", initial[2])
	require.True(t, prefixWasStable.Load())
	require.True(t, suffixUntouched.Load(), "validation must stop before touching the later reader")
	require.Equal(t, int32(1), attempts[0].Load())
	require.Equal(t, int32(2), attempts[1].Load())
	require.Equal(t, int32(2), attempts[2].Load())
	require.Equal(t, "after", responses[0].Info)
	require.Equal(t, "after", responses[1].Info)
	require.Equal(t, "writer", responses[2].Info)
	for _, key := range [][]byte{itemKey, []byte("bridge"), []byte("result"), []byte("range/new")} {
		require.Equal(t, []byte("after"), kv.Get(key), "committed key %s", key)
	}
	var validations []int
	for _, span := range exporter.GetSpans() {
		if span.Name != "SchedulerValidate" {
			continue
		}
		for _, attr := range span.Attributes {
			if attr.Key == "absoluteIndex" {
				validations = append(validations, int(attr.Value.AsInt64()))
			}
		}
	}
	require.Equal(t, []int{0, 1, 1, 2, 2}, validations, "only a validated predecessor permits the next index")
	require.True(t, allValidated(s.allTasks))
}
