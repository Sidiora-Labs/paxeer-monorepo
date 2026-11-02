package tasks

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/utils/tracing"
	"github.com/stretchr/testify/require"
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
