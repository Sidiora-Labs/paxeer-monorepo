package tasks

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/utils/tracing"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
)

func repairM631Tracing() *tracing.Info {
	return tracing.NewTracingInfo(trace.NewNoopTracerProvider().Tracer("repair-m631"), true)
}

func TestRepairM_631_CancelBeforeDispatch(t *testing.T) {
	for _, empty := range []bool{false, true} {
		for _, expired := range []bool{false, true} {
			t.Run(fmt.Sprintf("empty=%t/deadline=%t", empty, expired), func(t *testing.T) {
				reader := oteltest.Install()
				beforeRetries, beforeIncarnations := retries(reader), incarnations(reader)
				batchCtx, cancel := context.WithCancel(context.Background())
				wantErr := context.Canceled
				if expired {
					cancel()
					batchCtx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
					wantErr = context.DeadlineExceeded
				}
				cancel()
				ctx := initTestCtx(true).WithContext(batchCtx)
				kv := ctx.MultiStore().GetKVStore(testStoreKey)
				kv.Set(itemKey, []byte("before"))
				var calls atomic.Int32
				s := NewScheduler(2, repairM631Tracing(), func(ctx sdk.Context, req types.RequestDeliverTxV2, tx sdk.Tx, checksum [32]byte) types.ResponseDeliverTx {
					calls.Add(1)
					ctx.MultiStore().GetKVStore(testStoreKey).Set(itemKey, req.Tx)
					return types.ResponseDeliverTx{Info: string(req.Tx)}
				})
				n := 4
				if empty {
					n = 0
				}
				responses, err := s.ProcessAll(ctx, requestList(n))
				require.ErrorIs(t, err, wantErr)
				require.Nil(t, responses)
				require.Zero(t, calls.Load())
				require.Equal(t, []byte("before"), kv.Get(itemKey))
				require.Equal(t, beforeRetries, retries(reader))
				require.Equal(t, beforeIncarnations, incarnations(reader))
			})
		}
	}
}

func TestRepairM_631_CancelDuringExecutionJoinsWorkers(t *testing.T) {
	for _, synchronous := range []bool{false, true} {
		t.Run(fmt.Sprintf("synchronous=%t", synchronous), func(t *testing.T) {
			batchCtx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ctx := initTestCtx(true).WithContext(batchCtx)
			kv := ctx.MultiStore().GetKVStore(testStoreKey)
			kv.Set(itemKey, []byte("before"))
			started, cancelled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var active, calls atomic.Int32
			s := NewScheduler(1, repairM631Tracing(), func(ctx sdk.Context, req types.RequestDeliverTxV2, tx sdk.Tx, checksum [32]byte) types.ResponseDeliverTx {
				active.Add(1)
				defer active.Add(-1)
				calls.Add(1)
				store := ctx.MultiStore().GetKVStore(testStoreKey)
				store.Set(req.Tx, req.Tx)
				store.Set(itemKey, req.Tx)
				if ctx.TxIndex() == 1 {
					close(started)
					<-ctx.Context().Done()
					close(cancelled)
					<-release
				}
				return types.ResponseDeliverTx{Info: string(req.Tx)}
			}).(*scheduler)
			s.synchronous = synchronous
			type result struct {
				responses []types.ResponseDeliverTx
				err       error
			}
			finished := make(chan result, 1)
			go func() {
				responses, err := s.ProcessAll(ctx, requestList(64))
				finished <- result{responses, err}
			}()
			t.Cleanup(func() { cancel(); close(release) })
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("delivery did not start")
			}
			other := initTestCtx(true)
			other.MultiStore().GetKVStore(testStoreKey).Set(itemKey, []byte("other"))
			responses, err := s.ProcessAll(other, requestList(1))
			var reuse *SchedulerReuseError
			require.ErrorAs(t, err, &reuse, "a concurrent caller must receive the typed single-use refusal")
			require.Nil(t, responses)
			require.Equal(t, []byte("other"), other.MultiStore().GetKVStore(testStoreKey).Get(itemKey))
			cancel()
			select {
			case <-cancelled:
			case <-time.After(5 * time.Second):
				t.Fatal("delivery context did not receive cancellation")
			}
			select {
			case <-finished:
				t.Fatal("ProcessAll returned before joining the active delivery")
			case <-time.After(25 * time.Millisecond):
			}
			require.Equal(t, []byte("before"), kv.Get(itemKey), "speculative writes must remain private")
			// Release the active handler, then wait for all queued jobs and workers.
			release <- struct{}{}
			select {
			case got := <-finished:
				require.ErrorIs(t, got.err, context.Canceled)
				require.Nil(t, got.responses)
			case <-time.After(5 * time.Second):
				t.Fatal("cancelled batch left a completion waiter blocked")
			}
			require.Zero(t, active.Load())
			require.Equal(t, int32(2), calls.Load(), "queued deliveries must not run after cancellation")
			require.Equal(t, []byte("before"), kv.Get(itemKey))
			for _, req := range requestList(64) {
				require.Nil(t, kv.Get(req.Request.Tx))
			}
		})
	}
}

func TestRepairM_631_SchedulerReuse(t *testing.T) {
	reader := oteltest.Install()
	beforeRetries, beforeIncarnations := retries(reader), incarnations(reader)
	first, second := initTestCtx(true), initTestCtx(true)
	first.MultiStore().GetKVStore(testStoreKey).Set(itemKey, []byte("first"))
	second.MultiStore().GetKVStore(testStoreKey).Set(itemKey, []byte("second"))
	secondRead := make(chan struct{})
	var attempts [2]atomic.Int32
	s := NewScheduler(2, repairM631Tracing(), func(ctx sdk.Context, req types.RequestDeliverTxV2, tx sdk.Tx, checksum [32]byte) (res types.ResponseDeliverTx) {
		defer abortRecoveryFunc(&res)
		attempt := attempts[ctx.TxIndex()].Add(1)
		kv := ctx.MultiStore().GetKVStore(testStoreKey)
		previous := kv.Get(itemKey)
		if ctx.TxIndex() == 0 {
			<-secondRead
		} else if attempt == 1 {
			close(secondRead)
		}
		kv.Set(itemKey, req.Tx)
		return types.ResponseDeliverTx{Info: string(previous)}
	})
	responses, err := s.ProcessAll(first, requestList(2))
	require.NoError(t, err)
	require.Equal(t, "first", responses[0].Info)
	require.Equal(t, "0", responses[1].Info)
	require.Equal(t, int64(1), retries(reader)-beforeRetries)
	require.Equal(t, int64(1), incarnations(reader)-beforeIncarnations)
	afterRetries, afterIncarnations := retries(reader), incarnations(reader)
	responses, err = s.ProcessAll(second, requestList(2))
	var reuse *SchedulerReuseError
	require.True(t, errors.As(err, &reuse), "reuse must return a typed refusal: %v", err)
	require.Nil(t, responses)
	require.Equal(t, []byte("1"), first.MultiStore().GetKVStore(testStoreKey).Get(itemKey))
	require.Equal(t, []byte("second"), second.MultiStore().GetKVStore(testStoreKey).Get(itemKey))
	require.Equal(t, int32(1), attempts[0].Load())
	require.Equal(t, int32(2), attempts[1].Load())
	require.Equal(t, afterRetries, retries(reader), "refused batches must not emit stale retry counts")
	require.Equal(t, afterIncarnations, incarnations(reader))
	fresh := NewScheduler(1, repairM631Tracing(), func(ctx sdk.Context, req types.RequestDeliverTxV2, tx sdk.Tx, checksum [32]byte) types.ResponseDeliverTx {
		kv := ctx.MultiStore().GetKVStore(testStoreKey)
		previous := kv.Get(itemKey)
		kv.Set(itemKey, []byte("fresh"))
		return types.ResponseDeliverTx{Info: string(previous)}
	})
	responses, err = fresh.ProcessAll(second, requestList(1))
	require.NoError(t, err)
	require.Equal(t, "second", responses[0].Info)
	require.Equal(t, []byte("fresh"), second.MultiStore().GetKVStore(testStoreKey).Get(itemKey))
	require.Equal(t, []byte("1"), first.MultiStore().GetKVStore(testStoreKey).Get(itemKey))
	require.Equal(t, afterRetries, retries(reader))
	require.Equal(t, afterIncarnations, incarnations(reader))
}

func TestRepairM_631_ValidationOrder(t *testing.T) {
	t.Run("read_conflict", func(t *testing.T) { proveProcessAllOrderedRerun(t, false) })
	t.Run("iterator_conflict", func(t *testing.T) { proveProcessAllOrderedRerun(t, true) })
}

func TestRepairM_631_RerunDriver(t *testing.T) {
	t.Run("ProcessAll", TestProcessAllRetriesLowerIndexBeforeReader)
	t.Run("direct_invalidation", TestShouldRerunInvalidatesValidatedTaskOnNewConflict)
	t.Run("unchanged_read", TestShouldRerunKeepsValidatedTaskWithoutConflict)
}

func TestRepairM_631_MixedConflictResponses(t *testing.T) {
	ctx := initTestCtx(true)
	s := NewScheduler(12, repairM631Tracing(), func(ctx sdk.Context, req types.RequestDeliverTxV2, tx sdk.Tx, checksum [32]byte) (res types.ResponseDeliverTx) {
		defer abortRecoveryFunc(&res)
		if ctx.TxIndex()%10 != 0 {
			return types.ResponseDeliverTx{Info: "none"}
		}
		kv := ctx.MultiStore().GetKVStore(testStoreKey)
		previous := kv.Get(itemKey)
		kv.Set(itemKey, req.Tx)
		return types.ResponseDeliverTx{Info: string(previous)}
	})
	responses, err := s.ProcessAll(ctx, requestList(2000))
	require.NoError(t, err)
	require.Len(t, responses, 2000)
	assertMixedConflictResponses(t, ctx, responses)
}
