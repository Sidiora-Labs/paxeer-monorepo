package tasks

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/cachemulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/dbadapter"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/utils/tracing"
	dbm "github.com/tendermint/tm-db"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

type repairM1124Spans struct {
	*tracetest.SpanRecorder
	onStart func(sdktrace.ReadWriteSpan)
}

func (p *repairM1124Spans) OnStart(ctx context.Context, span sdktrace.ReadWriteSpan) {
	p.SpanRecorder.OnStart(ctx, span)
	p.onStart(span)
}

func repairM1124Tracing(t *testing.T, onStart func(sdktrace.ReadWriteSpan)) *tracing.Info {
	t.Helper()
	processor := &repairM1124Spans{SpanRecorder: tracetest.NewSpanRecorder(), onStart: onStart}
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(processor))
	t.Cleanup(func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Error(err)
		}
	})
	return tracing.NewTracingInfo(provider.Tracer("repair-m1124"), true)
}

func repairM1124Context(t *testing.T, parent context.Context, giga bool) (sdk.Context, *storetypes.KVStoreKey) {
	t.Helper()
	backing := dbm.NewMemDB()
	t.Cleanup(func() {
		if err := backing.Close(); err != nil {
			t.Error(err)
		}
	})
	key := sdk.NewKVStoreKey("repair-m1124")
	var gigaKeys []storetypes.StoreKey
	if giga {
		gigaKeys = []storetypes.StoreKey{key}
	}
	ms := cachemulti.NewFromKVStore(dbadapter.Store{DB: backing},
		map[storetypes.StoreKey]storetypes.CacheWrapper{key: dbadapter.Store{DB: backing}},
		nil, map[string]storetypes.StoreKey{key.Name(): key}, gigaKeys, nil, nil, 0)
	return sdk.NewContext(ms, tmproto.Header{Height: 1}, false).WithContext(parent), key
}

func repairM1124Store(ctx sdk.Context, key storetypes.StoreKey, giga bool) sdk.KVStore {
	if giga {
		return ctx.GigaKVStore(key)
	}
	return ctx.KVStore(key)
}

func repairM1124Requests(n int) []*sdk.DeliverTxEntry {
	reqs := make([]*sdk.DeliverTxEntry, n)
	for i := range reqs {
		reqs[i] = &sdk.DeliverTxEntry{AbsoluteIndex: i, Request: abci.RequestDeliverTxV2{Tx: []byte(fmt.Sprintf("tx-%03d", i))}}
	}
	return reqs
}

type repairM1124Result struct {
	responses []abci.ResponseDeliverTx
	err       error
}

func repairM1124Run(s Scheduler, ctx sdk.Context, reqs []*sdk.DeliverTxEntry) <-chan repairM1124Result {
	done := make(chan repairM1124Result, 1)
	go func() {
		responses, err := s.ProcessAll(ctx, reqs)
		done <- repairM1124Result{responses: responses, err: err}
	}()
	return done
}

func repairM1124Await(t *testing.T, done <-chan repairM1124Result) repairM1124Result {
	t.Helper()
	select {
	case result := <-done:
		return result
	case <-time.After(5 * time.Second):
		t.Fatal("ProcessAll stranded accepted work after cancellation")
		return repairM1124Result{}
	}
}

func repairM1124Canceled(t *testing.T, result repairM1124Result, want error) {
	t.Helper()
	if !errors.Is(result.err, want) || result.responses != nil {
		t.Fatalf("responses=%v error=%v; want no responses and %v", result.responses, result.err, want)
	}
}

func repairM1124Drained(t *testing.T, s *scheduler) {
	t.Helper()
	for name, ch := range map[string]chan func(){"execute": s.executeCh, "validate": s.validateCh} {
		select {
		case _, open := <-ch:
			if open {
				t.Fatalf("%s queue retained accepted work", name)
			}
		default:
			t.Fatalf("%s workers were not shut down", name)
		}
	}
}

func TestRepairM_1124_BeforeExecution(t *testing.T) {
	for _, synchronous := range []bool{false, true} {
		for _, deadline := range []bool{false, true} {
			t.Run(fmt.Sprintf("sync=%t/deadline=%t", synchronous, deadline), func(t *testing.T) {
				parent, cancel := context.WithCancel(context.Background())
				want := error(context.Canceled)
				if deadline {
					cancel()
					parent, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
					want = context.DeadlineExceeded
				}
				cancel()
				ctx, _ := repairM1124Context(t, parent, false)
				var calls atomic.Int32
				s := NewScheduler(1, tracing.NewTracingInfo(nil, false), func(sdk.Context, abci.RequestDeliverTxV2, sdk.Tx, [32]byte) abci.ResponseDeliverTx {
					calls.Add(1)
					return abci.ResponseDeliverTx{}
				}).(*scheduler)
				s.synchronous = synchronous
				repairM1124Canceled(t, repairM1124Await(t, repairM1124Run(s, ctx, repairM1124Requests(32))), want)
				if calls.Load() != 0 || s.multiVersionStores != nil {
					t.Fatal("canceled batch initialized stores or executed a transaction")
				}
			})
		}
	}
}

func TestRepairM_1124_QueueAdmission(t *testing.T) {
	for _, validation := range []bool{false, true} {
		for _, synchronous := range []bool{false, true} {
			t.Run(fmt.Sprintf("validate=%t/sync=%t", validation, synchronous), func(t *testing.T) {
				parent, cancel := context.WithCancel(context.Background())
				defer cancel()
				s := &scheduler{executeCh: make(chan func(), 1), validateCh: make(chan func(), 1), synchronous: synchronous}
				var calls atomic.Int32
				work := func() { calls.Add(1) }
				s.executeCh <- work
				s.validateCh <- work
				submit := s.DoExecute
				if validation {
					submit = s.DoValidate
				}
				if synchronous {
					cancel()
				}
				started := make(chan struct{})
				done := make(chan error, 1)
				go func() {
					close(started)
					done <- submit(parent, work)
				}()
				<-started
				cancel()
				select {
				case err := <-done:
					if !errors.Is(err, context.Canceled) {
						t.Fatalf("admission error=%v", err)
					}
				case <-time.After(5 * time.Second):
					t.Fatal("cancellation did not unblock a full queue")
				}
				if calls.Load() != 0 || len(s.executeCh) != 1 || len(s.validateCh) != 1 {
					t.Fatal("rejected closure executed or changed the queues")
				}
			})
		}
	}
}

func TestRepairM_1124_MidBatch(t *testing.T) {
	for _, giga := range []bool{false, true} {
		t.Run(fmt.Sprintf("giga=%t", giga), func(t *testing.T) {
			parent, cancel := context.WithCancel(context.Background())
			defer cancel()
			ctx, key := repairM1124Context(t, parent, giga)
			started, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			defer once.Do(func() { close(release) })
			var calls atomic.Int32
			s := NewScheduler(1, tracing.NewTracingInfo(nil, false), func(ctx sdk.Context, req abci.RequestDeliverTxV2, _ sdk.Tx, _ [32]byte) abci.ResponseDeliverTx {
				if calls.Add(1) == 1 {
					close(started)
					<-release
				}
				repairM1124Store(ctx, key, giga).Set(req.Tx, []byte("tentative"))
				return abci.ResponseDeliverTx{Data: req.Tx}
			}).(*scheduler)
			reqs := repairM1124Requests(64)
			done := repairM1124Run(s, ctx, reqs)
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("first transaction never started")
			}
			timer := time.NewTimer(5 * time.Second)
			defer timer.Stop()
			ticker := time.NewTicker(time.Millisecond)
			defer ticker.Stop()
			for len(s.executeCh) != len(reqs)-1 {
				select {
				case <-ticker.C:
				case <-timer.C:
					t.Fatal("remaining transactions were not admitted")
				}
			}
			cancel()
			once.Do(func() { close(release) })
			repairM1124Canceled(t, repairM1124Await(t, done), context.Canceled)
			if calls.Load() != 1 {
				t.Fatalf("executed %d transactions after cancellation; want only the running transaction", calls.Load())
			}
			for i, req := range reqs {
				if repairM1124Store(ctx, key, giga).Get(req.Request.Tx) != nil {
					t.Fatal("canceled batch published a write")
				}
				if i > 0 && !s.allTasks[i].IsStatus(statusPending) {
					t.Fatalf("queued task %d executed after cancellation", i)
				}
			}
			repairM1124Drained(t, s)
		})
	}
}

func TestRepairM_1124_DuringValidation(t *testing.T) {
	for _, phase := range []string{"SchedulerValidateAll", "SchedulerValidate"} {
		t.Run(phase, func(t *testing.T) {
			parent, cancel := context.WithCancel(context.Background())
			defer cancel()
			ctx, key := repairM1124Context(t, parent, false)
			var reached atomic.Bool
			info := repairM1124Tracing(t, func(span sdktrace.ReadWriteSpan) {
				if span.Name() == phase {
					reached.Store(true)
					cancel()
				}
			})
			var calls atomic.Int32
			s := NewScheduler(4, info, func(ctx sdk.Context, req abci.RequestDeliverTxV2, _ sdk.Tx, _ [32]byte) abci.ResponseDeliverTx {
				calls.Add(1)
				ctx.KVStore(key).Set(req.Tx, []byte("tentative"))
				return abci.ResponseDeliverTx{Data: req.Tx}
			}).(*scheduler)
			reqs := repairM1124Requests(64)
			repairM1124Canceled(t, repairM1124Await(t, repairM1124Run(s, ctx, reqs)), context.Canceled)
			if !reached.Load() || calls.Load() != int32(len(reqs)) {
				t.Fatalf("validation reached=%t executions=%d", reached.Load(), calls.Load())
			}
			for _, req := range reqs {
				if ctx.KVStore(key).Get(req.Request.Tx) != nil {
					t.Fatal("validation cancellation published a write")
				}
			}
			repairM1124Drained(t, s)
		})
	}
}

func TestRepairM_1124_DuringRetryValidation(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx, key := repairM1124Context(t, parent, false)
	conflictKey := []byte("shared")
	ctx.KVStore(key).Set(conflictKey, []byte("old"))
	read := make(chan struct{})
	var reads atomic.Int32
	var canceledInRetry atomic.Bool
	info := repairM1124Tracing(t, func(span sdktrace.ReadWriteSpan) {
		if span.Name() == "SchedulerValidate" && reads.Load() == 2 {
			canceledInRetry.Store(true)
			cancel()
		}
	})
	s := NewScheduler(2, info, func(ctx sdk.Context, req abci.RequestDeliverTxV2, _ sdk.Tx, _ [32]byte) abci.ResponseDeliverTx {
		if string(req.Tx) == "tx-000" {
			select {
			case <-read:
			case <-parent.Done():
				return abci.ResponseDeliverTx{}
			}
			ctx.KVStore(key).Set(conflictKey, []byte("new"))
			return abci.ResponseDeliverTx{}
		}
		value := ctx.KVStore(key).Get(conflictKey)
		if reads.Add(1) == 1 {
			close(read)
		}
		return abci.ResponseDeliverTx{Data: value}
	}).(*scheduler)
	repairM1124Canceled(t, repairM1124Await(t, repairM1124Run(s, ctx, repairM1124Requests(2))), context.Canceled)
	if !canceledInRetry.Load() || reads.Load() != 2 || s.maxIncarnation < 1 || s.metrics.retries < 1 {
		t.Fatalf("retry cancellation=%t reads=%d incarnation=%d retries=%d", canceledInRetry.Load(), reads.Load(), s.maxIncarnation, s.metrics.retries)
	}
	if got := string(ctx.KVStore(key).Get(conflictKey)); got != "old" {
		t.Fatalf("canceled retry published %q", got)
	}
	repairM1124Drained(t, s)
}

func TestRepairM_1124_CompletedBatch(t *testing.T) {
	for _, giga := range []bool{false, true} {
		for _, workers := range []int{0, 1, 4} {
			for _, synchronous := range []bool{false, true} {
				t.Run(fmt.Sprintf("giga=%t/workers=%d/sync=%t", giga, workers, synchronous), func(t *testing.T) {
					ctx, key := repairM1124Context(t, context.Background(), giga)
					var calls atomic.Int32
					s := NewScheduler(workers, tracing.NewTracingInfo(nil, false), func(ctx sdk.Context, req abci.RequestDeliverTxV2, _ sdk.Tx, _ [32]byte) abci.ResponseDeliverTx {
						calls.Add(1)
						repairM1124Store(ctx, key, giga).Set(req.Tx, req.Tx)
						return abci.ResponseDeliverTx{Data: req.Tx}
					}).(*scheduler)
					s.synchronous = synchronous
					reqs := repairM1124Requests(16)
					result := repairM1124Await(t, repairM1124Run(s, ctx, reqs))
					if result.err != nil || len(result.responses) != len(reqs) || calls.Load() != int32(len(reqs)) {
						t.Fatalf("responses=%d executions=%d error=%v", len(result.responses), calls.Load(), result.err)
					}
					for i, req := range reqs {
						if string(result.responses[i].Data) != string(req.Request.Tx) || string(repairM1124Store(ctx, key, giga).Get(req.Request.Tx)) != string(req.Request.Tx) || !s.allTasks[i].IsStatus(statusValidated) {
							t.Fatalf("task %d did not validate, preserve response order, and publish its write", i)
						}
					}
					repairM1124Drained(t, s)
				})
			}
		}
	}
}
