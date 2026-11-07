package tasks

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/multiversion"
	store "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/occ"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/utils/tracing"
	"github.com/paxeer-network/paxlog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var logger = paxlog.NewLogger("cosmos", "tasks")

type status string

const (
	// statusPending tasks are ready for execution
	// all executing tasks are in pending state
	statusPending status = "pending"
	// statusExecuted tasks are ready for validation
	// these tasks did not abort during execution
	statusExecuted status = "executed"
	// statusAborted means the task has been aborted
	// these tasks transition to pending upon next execution
	statusAborted status = "aborted"
	// statusValidated means the task has been validated
	// tasks in this status can be reset if an earlier task fails validation
	statusValidated status = "validated"
	// statusWaiting tasks are waiting for another tx to complete
	statusWaiting status = "waiting"
	// maximumIterations before we revert to sequential (for high conflict rates)
	maximumIterations = 10
)

type deliverTxTask struct {
	Ctx     sdk.Context
	AbortCh chan occ.Abort

	mx            sync.RWMutex
	Status        status
	Dependencies  map[int]struct{}
	Abort         *occ.Abort
	Incarnation   int
	Request       types.RequestDeliverTxV2
	SdkTx         sdk.Tx
	Checksum      [32]byte
	AbsoluteIndex int
	Response      *types.ResponseDeliverTx
	VersionStores map[sdk.StoreKey]*multiversion.VersionIndexedStore
	TxTracer      sdk.TxTracer
}

// AppendDependencies appends the given indexes to the task's dependencies
func (dt *deliverTxTask) AppendDependencies(deps []int) {
	dt.mx.Lock()
	defer dt.mx.Unlock()
	for _, taskIdx := range deps {
		dt.Dependencies[taskIdx] = struct{}{}
	}
}

func (dt *deliverTxTask) IsStatus(s status) bool {
	dt.mx.RLock()
	defer dt.mx.RUnlock()
	return dt.Status == s
}

func (dt *deliverTxTask) SetStatus(s status) {
	dt.mx.Lock()
	defer dt.mx.Unlock()
	dt.Status = s
}

func (dt *deliverTxTask) Reset() {
	dt.SetStatus(statusPending)
	dt.Response = nil
	dt.Abort = nil
	dt.AbortCh = nil
	dt.VersionStores = nil

	if dt.TxTracer != nil {
		dt.TxTracer.Reset()
	}
}

func (dt *deliverTxTask) Increment() {
	dt.Incarnation++
}

// Scheduler processes one batch concurrently. Create a new scheduler for each batch.
type Scheduler interface {
	ProcessAll(ctx sdk.Context, reqs []*sdk.DeliverTxEntry) ([]types.ResponseDeliverTx, error)
}

// SchedulerReuseError refuses a second ProcessAll call, including concurrent calls.
type SchedulerReuseError struct{}

func (*SchedulerReuseError) Error() string {
	return "scheduler has already been used for a batch"
}

type scheduler struct {
	used               atomic.Bool
	deliverTx          func(ctx sdk.Context, req types.RequestDeliverTxV2, tx sdk.Tx, checksum [32]byte) (res types.ResponseDeliverTx)
	workers            int
	multiVersionStores map[sdk.StoreKey]multiversion.MultiVersionStore
	tracingInfo        *tracing.Info
	allTasksMap        map[int]*deliverTxTask
	allTasks           []*deliverTxTask
	executeCh          chan func()
	metrics            *schedulerMetrics
	synchronous        bool           // true if maxIncarnation exceeds threshold
	maxIncarnation     int            // current highest incarnation
	conflictKeyCounts  map[string]int // per-key conflict counts accumulated over the block
	conflictKeyMu      sync.Mutex
}

// NewScheduler creates a single-use scheduler; even a cancelled batch consumes it.
func NewScheduler(workers int, tracingInfo *tracing.Info, deliverTxFunc func(ctx sdk.Context, req types.RequestDeliverTxV2, tx sdk.Tx, checksum [32]byte) (res types.ResponseDeliverTx)) Scheduler {
	return &scheduler{
		workers:     workers,
		deliverTx:   deliverTxFunc,
		tracingInfo: tracingInfo,
		metrics:     &schedulerMetrics{},
	}
}

func (s *scheduler) invalidateTask(task *deliverTxTask) {
	for _, mv := range s.multiVersionStores {
		mv.InvalidateWriteset(task.AbsoluteIndex, task.Incarnation)
		mv.ClearReadset(task.AbsoluteIndex)
		mv.ClearIterateset(task.AbsoluteIndex)
	}
}

// Workers drain submitted jobs so every job accounts for its completion, even
// after cancellation. The caller closes the queue and joins them before returning.
func start(ch chan func(), workers int) *sync.WaitGroup {
	wg := &sync.WaitGroup{}
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for work := range ch {
				work()
			}
		}()
	}
	return wg
}

func (s *scheduler) DoExecute(ctx context.Context, work func()) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.synchronous {
		work()
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case s.executeCh <- work:
		return nil
	}
}

func (s *scheduler) findConflicts(task *deliverTxTask) (bool, []int, []string) {
	var conflicts []int
	conflictKeys := make([]string, 0, len(s.multiVersionStores))
	uniq := make(map[int]struct{})
	valid := true
	for storeKey, mv := range s.multiVersionStores {
		ok, mvConflicts, mvKeys := mv.ValidateTransactionStateWithKeys(task.AbsoluteIndex)
		for _, c := range mvConflicts {
			if _, ok := uniq[c]; !ok {
				conflicts = append(conflicts, c)
				uniq[c] = struct{}{}
			}
		}
		for _, k := range mvKeys {
			conflictKeys = append(conflictKeys, storeKey.Name()+"/"+k)
		}
		// any non-ok value makes valid false
		valid = valid && ok
	}
	sort.Ints(conflicts)
	return valid, conflicts, conflictKeys
}

func toTasks(reqs []*sdk.DeliverTxEntry) ([]*deliverTxTask, map[int]*deliverTxTask) {
	tasksMap := make(map[int]*deliverTxTask)
	allTasks := make([]*deliverTxTask, 0, len(reqs))
	for _, r := range reqs {
		task := &deliverTxTask{
			Request:       r.Request,
			SdkTx:         r.SdkTx,
			Checksum:      r.Checksum,
			AbsoluteIndex: r.AbsoluteIndex,
			Status:        statusPending,
			Dependencies:  map[int]struct{}{},
			TxTracer:      r.TxTracer,
		}

		tasksMap[r.AbsoluteIndex] = task
		allTasks = append(allTasks, task)
	}
	return allTasks, tasksMap
}

func (s *scheduler) collectResponses(tasks []*deliverTxTask) []types.ResponseDeliverTx {
	res := make([]types.ResponseDeliverTx, 0, len(tasks))
	for _, t := range tasks {
		res = append(res, *t.Response)

		if t.TxTracer != nil {
			t.TxTracer.Commit()
		}
	}
	return res
}

func (s *scheduler) tryInitMultiVersionStore(ctx sdk.Context) {
	if s.multiVersionStores != nil {
		return
	}
	mvs := make(map[sdk.StoreKey]multiversion.MultiVersionStore)
	keys := ctx.MultiStore().StoreKeys()
	for _, sk := range keys {
		mvs[sk] = multiversion.NewMultiVersionStore(ctx.MultiStore().GetKVStore(sk))
	}
	s.multiVersionStores = mvs
}

func dependenciesValidated(tasksMap map[int]*deliverTxTask, deps map[int]struct{}) bool {
	for i := range deps {
		// because idx contains absoluteIndices, we need to fetch from map
		task := tasksMap[i]
		if !task.IsStatus(statusValidated) {
			return false
		}
	}
	return true
}

func allValidated(tasks []*deliverTxTask) bool {
	for _, t := range tasks {
		if !t.IsStatus(statusValidated) {
			return false
		}
	}
	return true
}

// schedulerMetrics contains metrics for the scheduler
type schedulerMetrics struct {
	// maxIncarnation is the highest incarnation seen in this set
	maxIncarnation int
	// retries is the number of tx attempts beyond the first attempt
	retries int
}

func (s *scheduler) emitMetrics() {
	taskMetrics.retries.Add(context.Background(), int64(s.metrics.retries))
	taskMetrics.incarnations.Add(context.Background(), int64(s.metrics.maxIncarnation))
}

func (s *scheduler) ProcessAll(ctx sdk.Context, reqs []*sdk.DeliverTxEntry) ([]types.ResponseDeliverTx, error) {
	if !s.used.CompareAndSwap(false, true) {
		return nil, &SchedulerReuseError{}
	}
	if err := ctx.Context().Err(); err != nil {
		return nil, err
	}
	startTime := time.Now()
	var iterations int
	// initialize mutli-version stores if they haven't been initialized yet
	s.tryInitMultiVersionStore(ctx)
	responseTasks, tasksMap := toTasks(reqs)
	tasks := append([]*deliverTxTask(nil), responseTasks...)
	sort.SliceStable(tasks, func(i, j int) bool {
		return tasks[i].AbsoluteIndex < tasks[j].AbsoluteIndex
	})
	s.allTasks = tasks
	s.allTasksMap = tasksMap
	s.conflictKeyCounts = make(map[string]int)
	s.executeCh = make(chan func(), len(tasks))
	defer s.emitMetrics()

	// default to number of tasks if workers is negative or 0 by this point
	workers := s.workers
	if s.workers < 1 || len(tasks) < s.workers {
		workers = len(tasks)
	}

	workerWg := start(s.executeCh, workers)
	defer func() {
		close(s.executeCh)
		workerWg.Wait()
	}()

	toExecute := tasks
	for !allValidated(tasks) {
		// if the max incarnation >= x, we should revert to synchronous
		if iterations >= maximumIterations {
			// process synchronously
			s.synchronous = true
			startIdx, anyLeft := s.findFirstNonValidated()
			if !anyLeft {
				break
			}
			toExecute = tasks[startIdx:]
		}

		// execute sets statuses of tasks to either executed or aborted
		if err := s.executeAll(ctx, toExecute); err != nil {
			return nil, err
		}

		// validate returns any that should be re-executed
		// note this processes ALL tasks, not just those recently executed
		var err error
		toExecute, err = s.validateAll(ctx, tasks)
		if err != nil {
			return nil, err
		}
		// these are retries which apply to metrics
		s.metrics.retries += len(toExecute)
		iterations++
	}

	if err := ctx.Context().Err(); err != nil {
		return nil, err
	}
	for _, mv := range s.multiVersionStores {
		mv.WriteLatestToStore()
	}
	s.metrics.maxIncarnation = s.maxIncarnation

	if s.metrics.retries > 0 && len(s.conflictKeyCounts) > 0 {
		encoded := make(map[string]int, len(s.conflictKeyCounts))
		for k, v := range s.conflictKeyCounts {
			storeName, rawKey, _ := strings.Cut(k, "/")
			var encodedKey string
			if rawKey == "globalAccountNumber" {
				encodedKey = storeName + "/" + rawKey
			} else {
				encodedKey = storeName + "/" + hex.EncodeToString([]byte(rawKey))
			}
			encoded[encodedKey] = v
		}
		logger.Info("occ scheduler key conflicts", "height", ctx.BlockHeight(), "counts", encoded)
	}

	logger.Info("occ scheduler", "height", ctx.BlockHeight(), "txs", len(tasks), "latency_ms", time.Since(startTime).Milliseconds(), "retries", s.metrics.retries, "maxIncarnation", s.maxIncarnation, "iterations", iterations, "sync", s.synchronous, "workers", s.workers)

	return s.collectResponses(responseTasks), nil
}

func (s *scheduler) shouldRerun(task *deliverTxTask) bool {
	switch task.Status {

	case statusAborted, statusPending:
		return true

	// validated tasks can become unvalidated if an earlier re-run task now conflicts
	case statusExecuted, statusValidated:
		// With the current scheduler, we won't actually get to this step if a previous task has already been determined to be invalid,
		// since we choose to fail fast and mark the subsequent tasks as invalid as well.
		// Precondition: validation remains ordered and fail-fast; lower-index tasks must validate first.
		// shouldRerun relies on this ordering and does not support out-of-order validation of valid=true with conflicts.
		if valid, conflicts, conflictKeys := s.findConflicts(task); !valid {
			s.invalidateTask(task)
			task.AppendDependencies(conflicts)
			s.conflictKeyMu.Lock()
			for _, k := range conflictKeys {
				s.conflictKeyCounts[k]++
			}
			s.conflictKeyMu.Unlock()

			// if the conflicts are now validated, then rerun this task
			if dependenciesValidated(s.allTasksMap, task.Dependencies) {
				return true
			} else {
				// otherwise, wait for completion
				task.SetStatus(statusWaiting)
				return false
			}
		} else if len(conflicts) == 0 {
			// mark as validated, which will avoid re-validating unless a lower-index re-validates
			task.SetStatus(statusValidated)
			return false
		}
		// conflicts and valid, so it'll validate next time
		return false

	case statusWaiting:
		// if conflicts are done, then this task is ready to run again
		return dependenciesValidated(s.allTasksMap, task.Dependencies)
	}
	panic("unexpected status: " + task.Status)
}

func (s *scheduler) validateTask(ctx sdk.Context, task *deliverTxTask) bool {
	_, span := s.traceSpan(ctx, "SchedulerValidate", task)
	defer span.End()

	return !s.shouldRerun(task)
}

func (s *scheduler) findFirstNonValidated() (int, bool) {
	for i, t := range s.allTasks {
		if t.Status != statusValidated {
			return i, true
		}
	}
	return 0, false
}

func (s *scheduler) validateAll(ctx sdk.Context, tasks []*deliverTxTask) ([]*deliverTxTask, error) {
	ctx, span := s.traceSpan(ctx, "SchedulerValidateAll", nil)
	defer span.End()

	startIdx, anyLeft := s.findFirstNonValidated()
	if !anyLeft {
		return nil, ctx.Context().Err()
	}

	// The validated prefix is stable. Stop at the first invalid predecessor;
	// later read and iterator sets must wait until its retry has been validated.
	for _, task := range tasks[startIdx:] {
		if err := ctx.Context().Err(); err != nil {
			return nil, err
		}
		if !s.validateTask(ctx, task) {
			task.Reset()
			task.Increment()
			if task.Incarnation > s.maxIncarnation {
				s.maxIncarnation = task.Incarnation
			}
			return []*deliverTxTask{task}, nil
		}
		if !task.IsStatus(statusValidated) {
			return nil, nil
		}
	}
	return nil, ctx.Context().Err()
}

// ExecuteAll executes all tasks concurrently and accounts for every submitted job.
func (s *scheduler) executeAll(ctx sdk.Context, tasks []*deliverTxTask) error {
	ctx, span := s.traceSpan(ctx, "SchedulerExecuteAll", nil)
	span.SetAttributes(attribute.Bool("synchronous", s.synchronous))
	defer span.End()

	wg := &sync.WaitGroup{}
	var submitErr error
	for _, task := range tasks {
		t := task
		wg.Add(1)
		if err := s.DoExecute(ctx.Context(), func() {
			s.prepareAndRunTask(wg, ctx, t)
		}); err != nil {
			wg.Done()
			submitErr = err
			break
		}
	}
	wg.Wait()
	if submitErr != nil {
		return submitErr
	}
	return ctx.Context().Err()
}

func (s *scheduler) prepareAndRunTask(wg *sync.WaitGroup, ctx sdk.Context, task *deliverTxTask) {
	defer wg.Done() // Must be deferred to prevent deadlock on panic
	if ctx.Context().Err() != nil {
		return
	}

	eCtx, eSpan := s.traceSpan(ctx, "SchedulerExecute", task)
	defer eSpan.End()

	task.Ctx = eCtx
	s.executeTask(task)
}

func (s *scheduler) traceSpan(ctx sdk.Context, name string, task *deliverTxTask) (sdk.Context, trace.Span) {
	spanCtx, span := s.tracingInfo.StartWithContext(name, ctx.TraceSpanContext())
	if task != nil {
		span.SetAttributes(attribute.String("txHash", fmt.Sprintf("%X", sha256.Sum256(task.Request.Tx))))
		span.SetAttributes(attribute.Int("absoluteIndex", task.AbsoluteIndex))
		span.SetAttributes(attribute.Int("txIncarnation", task.Incarnation))
	}
	ctx = ctx.WithTraceSpanContext(spanCtx)
	return ctx, span
}

// prepareTask initializes the context and version stores for a task
func (s *scheduler) prepareTask(task *deliverTxTask) {
	ctx := task.Ctx.WithTxIndex(task.AbsoluteIndex)

	_, span := s.traceSpan(ctx, "SchedulerPrepare", task)
	defer span.End()

	// initialize the context
	abortCh := make(chan occ.Abort, len(s.multiVersionStores))

	// if there are no stores, don't try to wrap, because there's nothing to wrap
	if len(s.multiVersionStores) > 0 {
		// non-blocking
		cms := ctx.MultiStore().CacheMultiStore()

		// init version stores by store key
		vs := make(map[store.StoreKey]*multiversion.VersionIndexedStore)
		for storeKey, mvs := range s.multiVersionStores {
			vs[storeKey] = mvs.VersionedIndexedStore(task.AbsoluteIndex, task.Incarnation, abortCh)
		}

		// save off version store so we can ask it things later
		task.VersionStores = vs
		ms := cms.SetKVStores(func(k store.StoreKey, kvs sdk.KVStore) store.CacheWrap {
			return vs[k]
		})

		ctx = ctx.WithMultiStore(ms)
	}

	if task.TxTracer != nil {
		ctx = task.TxTracer.InjectInContext(ctx)
	}

	task.AbortCh = abortCh
	task.Ctx = ctx
}

func (s *scheduler) executeTask(task *deliverTxTask) {
	dCtx, dSpan := s.traceSpan(task.Ctx, "SchedulerExecuteTask", task)
	defer dSpan.End()
	task.Ctx = dCtx

	// in the synchronous case, we only want to re-execute tasks that need re-executing
	if s.synchronous {
		// even if already validated, it could become invalid again due to preceding
		// reruns. Make sure previous writes are invalidated before rerunning.
		if task.IsStatus(statusValidated) {
			s.invalidateTask(task)
		}

		// waiting transactions may not yet have been reset
		// this ensures a task has been reset and incremented
		if !task.IsStatus(statusPending) {
			task.Reset()
			task.Increment()
		}
	}

	s.prepareTask(task)

	resp := s.deliverTx(task.Ctx, task.Request, task.SdkTx, task.Checksum)
	// close the abort channel
	close(task.AbortCh)
	abort, ok := <-task.AbortCh
	if ok {
		// if there is an abort item that means we need to wait on the dependent tx
		task.SetStatus(statusAborted)
		task.Abort = &abort
		task.AppendDependencies([]int{abort.DependentTxIdx})
		// write from version store to multiversion stores
		for _, v := range task.VersionStores {
			v.WriteEstimatesToMultiVersionStore()
		}
		return
	}

	task.SetStatus(statusExecuted)
	task.Response = &resp

	// write from version store to multiversion stores
	for _, v := range task.VersionStores {
		v.WriteToMultiVersionStore()
	}
}
