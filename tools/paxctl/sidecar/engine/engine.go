package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/sei-protocol/seilog"
)

var log = seilog.NewLogger("seictl", "engine")

// ErrInvalidTaskID is returned when a caller-provided task ID is not a valid UUID.
var ErrInvalidTaskID = fmt.Errorf("task ID must be a valid UUID")

// validateTaskID checks that a non-empty ID string is a valid UUID.
func validateTaskID(id string) error {
	if id == "" {
		return nil
	}
	if _, err := uuid.Parse(id); err != nil {
		return fmt.Errorf("%w: %q", ErrInvalidTaskID, id)
	}
	return nil
}

// Engine is the task executor. Every submitted task runs in its own
// goroutine. The store is the single source of truth for all task state.
// The engine context propagates to all handlers — on SIGTERM the
// context is cancelled and handlers observe ctx.Done() to stop gracefully.
type Engine struct {
	handlers map[TaskType]TaskHandler
	ctx      context.Context
	ready    atomic.Bool
	store    ResultStore
	mu       sync.Mutex

	// cancels holds the cancel func of every currently running task, keyed by
	// task ID, so RemoveResult can stop a task's goroutine. Each entry carries
	// the generation that registered it so cleanup is a compare-and-delete: a
	// resubmit under the same ID registers a fresh entry with a newer generation
	// and overwrites the prior one, and a superseded registration's late cleanup
	// must not touch the newer entry. Guarded by mu.
	cancels map[string]cancelEntry

	// gen mints a strictly-increasing, process-lifetime-unique generation for
	// every newTaskContext registration. It discriminates a superseded
	// registration's late cleanup from a newer registration's entry even when
	// both share a row's run number — run is scoped to a row's lifetime and
	// resets to 1 after a delete, so it collides across a DELETE-then-resubmit;
	// gen never repeats.
	gen atomic.Int64

	// Config is set once during single-threaded startup before Submit
	// is reachable; read-only thereafter. No synchronization.
	Config ExecutionConfig
}

// cancelEntry is a registered task's cancel func tagged with the generation that
// registered it, so clearCancel and RemoveResult can distinguish "my
// registration's entry" from "a newer registration that overwrote mine".
type cancelEntry struct {
	cancel context.CancelFunc
	gen    int64
}

// NewEngine creates a new Engine. The engine runs until ctx is cancelled.
// Callers MUST install handler dependencies on e.Config before
// RehydrateStaleTasks, else a rehydrated handler races with the write.
func NewEngine(ctx context.Context, handlers map[TaskType]TaskHandler, store ResultStore) *Engine {
	e := &Engine{
		handlers: handlers,
		ctx:      ctx,
		store:    store,
		cancels:  make(map[string]cancelEntry),
	}
	// Restore before probes can observe the engine, even with no stale tasks.
	_ = e.restoreReadiness()
	return e
}

func (e *Engine) restoreReadiness() error {
	ready, err := e.store.Readiness()
	if err != nil {
		e.ready.Store(false)
		log.Error("failed to restore readiness; keeping gate closed", "err", err)
		return err
	}
	e.ready.Store(ready)
	return nil
}

// RehydrateStaleTasks re-executes tasks left in "running" state by a
// previous process that exited before completing them. Run count is
// NOT incremented — rehydration is crash recovery of an incomplete
// run, not a new run. Must be called only after Config is installed.
func (e *Engine) RehydrateStaleTasks() {
	if err := e.restoreReadiness(); err != nil {
		return
	}
	stale, err := e.store.ListStaleTasks()
	if err != nil {
		log.Error("failed to list stale tasks", "err", err)
		return
	}

	// A node hold must win crash recovery deterministically. Run any stale
	// mark-not-ready SYNCHRONOUSLY FIRST: its handler purges stranded mark-ready
	// rows from the store and its completion hook flips readiness false. This
	// removes the Store(true)/Store(false) race that goroutine dispatch would
	// otherwise create between a stranded mark-ready and a stranded
	// mark-not-ready — a race that could transiently release a held seid.
	//
	// CONTRACT: a hold handler run here must stay bounded (a single store op).
	// This executes before the HTTP server's ListenAndServe, so livez does not
	// serve yet — a slow handler here silently delays startup with no signal.
	holdSeen := false
	for _, tr := range stale {
		if TaskType(tr.Type) != TaskMarkNotReady {
			continue
		}
		holdSeen = true
		if handler, ok := e.resolveStaleHandler(tr); ok {
			log.Info("rehydrating hold synchronously before other tasks",
				"type", tr.Type, "id", tr.ID, "run", tr.Run)
			e.mu.Lock()
			ctx, gen := e.newTaskContext(tr.ID)
			e.mu.Unlock()
			e.runTaskSync(ctx, tr.ID, TaskType(tr.Type), handler, tr.Params, tr.SubmittedAt, tr.Run, gen, tr.Generation)
		}
	}

	// Re-read the source of truth after the purge so the mark-ready rows it
	// deleted (and the now-terminal mark-not-ready) are not dispatched.
	if holdSeen {
		stale, err = e.store.ListStaleTasks()
		if err != nil {
			log.Error("failed to re-list stale tasks after hold", "err", err)
			return
		}
	}

	for _, tr := range stale {
		switch TaskType(tr.Type) {
		case TaskMarkNotReady:
			continue // handled synchronously above
		case TaskMarkReady:
			// Durable hold supersession: never rehydrate a stranded mark-ready
			// while a hold at least as new as it exists in the store. Keyed on
			// the persisted mark-not-ready record (any status), this survives
			// restarts — a failed purge persists the hold Failed, which the
			// in-run holdSeen flag (used only for the re-list above) cannot see
			// on a later boot.
			if e.markReadySuperseded(tr) {
				continue
			}
		}
		if handler, ok := e.resolveStaleHandler(tr); ok {
			log.Info("rehydrating stale task", "type", tr.Type, "id", tr.ID, "run", tr.Run)
			e.mu.Lock()
			ctx, gen := e.newTaskContext(tr.ID)
			e.mu.Unlock()
			e.runTask(ctx, tr.ID, TaskType(tr.Type), handler, tr.Params, tr.SubmittedAt, tr.Run, gen, tr.Generation)
		}
	}
}

// markReadySuperseded reports whether a stranded mark-ready must not be
// rehydrated because a hold supersedes it, and if so records that outcome.
//
// The store is the durable source of truth: a mark-not-ready record of ANY
// status (a failed purge persists the hold Failed, so this must not be limited
// to running rows) submitted no earlier than the mark-ready means the hold
// attempt supersedes the stale release. The superseded mark-ready is marked
// Failed ("superseded by hold") so a controller polling it terminates, rather
// than being left running across restarts. A lone stranded mark-ready with no
// newer hold is not superseded — that init-time crash recovery path is
// load-bearing and must still rehydrate. On a store read error it fails closed
// (treats the mark-ready as superseded) rather than risk releasing a hold.
func (e *Engine) markReadySuperseded(mr TaskResult) bool {
	hold, err := e.store.LatestByType(string(TaskMarkNotReady))
	if err != nil {
		log.Error("checking for a superseding hold; refusing to rehydrate mark-ready", "id", mr.ID, "err", err)
		return true
	}
	if hold == nil || hold.SubmittedAt.Before(mr.SubmittedAt) {
		return false
	}
	log.Warn("stranded mark-ready superseded by a hold; marking failed instead of rehydrating",
		"markReadyID", mr.ID, "holdID", hold.ID, "holdStatus", hold.Status)
	t := time.Now().UTC()
	mr.Status = TaskStatusFailed
	mr.Error = "superseded by hold"
	mr.CompletedAt = &t
	if err := e.store.Save(&mr); err != nil {
		log.Error("failed to persist superseded mark-ready", "id", mr.ID, "err", err)
	}
	return true
}

// Submit starts a task in its own goroutine and returns its ID.
//
// The engine follows a cloud-API model for task lifecycle:
//   - If no task with this ID exists, create and execute it (run 1).
//   - If the task is running or completed, return its ID (idempotent no-op).
//   - If the task failed, re-execute it with an incremented run counter.
//
// The caller submits a stable key and the engine owns the execution lifecycle.
func (e *Engine) Submit(task Task) (string, error) {
	handler, ok := e.handlers[task.Type]
	if !ok {
		return "", fmt.Errorf("unknown task type: %s", task.Type)
	}

	if err := validateTaskID(task.ID); err != nil {
		return "", err
	}

	id := task.ID
	if id == "" {
		id = uuid.New().String()
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	run := 1
	if existing, _ := e.store.Get(id); existing != nil {
		switch existing.Status {
		case TaskStatusCompleted:
			if TaskType(existing.Type) == TaskMarkReady || TaskType(existing.Type) == TaskMarkNotReady {
				if err := e.restoreReadiness(); err != nil {
					return "", fmt.Errorf("restore readiness: %w", err)
				}
			}
			return id, nil
		case TaskStatusRunning:
			return id, nil
		case TaskStatusFailed:
			run = existing.Run + 1
		}
	}

	now := time.Now().UTC()
	tr := &TaskResult{
		ID:          id,
		Type:        string(task.Type),
		Status:      TaskStatusRunning,
		Run:         run,
		Params:      task.Params,
		SubmittedAt: now,
	}

	if err := e.store.Save(tr); err != nil {
		return "", fmt.Errorf("persist task: %w", err)
	}

	log.Info("task submitted", "type", task.Type, "id", id, "run", run)
	taskSubmissions.WithLabelValues(string(task.Type)).Inc()
	ctx, gen := e.newTaskContext(id)
	e.runTask(ctx, id, task.Type, handler, task.Params, now, run, gen, tr.Generation)

	return id, nil
}

// newTaskContext derives a cancellable, task-tagged context from the engine
// root and registers its cancel func under id, tagged with a freshly-minted
// generation, so RemoveResult can stop the task and clearCancel can tell this
// registration's entry from a newer one's. It returns the context and the
// generation it minted; the caller threads that generation to clearCancel so
// cleanup removes only its own entry. The task id is threaded through ctx for
// handlers that need it (e.g. sign-tx memo tagging). Callers MUST hold e.mu;
// clearCancel removes the entry when the task terminates.
func (e *Engine) newTaskContext(id string) (context.Context, int64) {
	gen := e.gen.Add(1)
	ctx, cancel := context.WithCancel(e.ctx)
	e.cancels[id] = cancelEntry{cancel: cancel, gen: gen}
	return WithTaskID(ctx, id), gen
}

// clearCancel cancels and unregisters a task's context when its run reaches a
// terminal state, so completed/failed tasks do not leak cancel funcs. It is a
// compare-and-delete on (id, gen): if a newer registration has overwritten the
// entry, this superseded registration leaves the newer entry untouched and does
// NOT cancel it — otherwise a stale run's late cleanup would kill a live
// context. Idempotent: a no-op when RemoveResult or a newer registration already
// removed this entry.
func (e *Engine) clearCancel(id string, gen int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if entry, ok := e.cancels[id]; ok && entry.gen == gen {
		entry.cancel()
		delete(e.cancels, id)
	}
}

// runTask spawns a goroutine to run the handler and persist the result.
func (e *Engine) runTask(ctx context.Context, id string, taskType TaskType, handler TaskHandler, params map[string]any, submittedAt time.Time, run int, gen int64, generation string) {
	go e.runTaskSync(ctx, id, taskType, handler, params, submittedAt, run, gen, generation)
}

// runTaskSync runs the handler and persists the result, blocking until done.
// runTask wraps it in a goroutine; RehydrateStaleTasks calls it directly for a
// stale mark-not-ready that must complete (purge + readiness flip) before any
// other stale task is dispatched. ctx is the per-task cancellable context from
// newTaskContext.
func (e *Engine) runTaskSync(ctx context.Context, id string, taskType TaskType, handler TaskHandler, params map[string]any, submittedAt time.Time, run int, gen int64, generation string) {
	defer func() {
		// The handler has returned. Retire only its durable cancellation record;
		// a resubmission may already own this ID with a different generation.
		e.mu.Lock()
		if err := e.store.ClearCancellation(id, generation); err != nil {
			log.Error("failed to retire task cancellation", "id", id, "err", err)
		}
		e.mu.Unlock()
		e.clearCancel(id, gen)
	}()
	result, err := e.executeRecovered(ctx, taskType, handler, params)

	// The task's context was cancelled — either engine shutdown (e.ctx) or an
	// explicit DELETE (RemoveResult cancelled this task's context). In both
	// cases, leave the store untouched: on shutdown the row stays 'running' so
	// RehydrateStaleTasks resumes it on restart (persisting a spurious Failed
	// would strand an in-flight sign-tx); on DELETE the handler already removed
	// the row, so writing nothing is the true-deletion outcome. Keying on a
	// cancelled context (not only a context.Canceled-wrapping error) also
	// suppresses the case where cancellation surfaces as an unrelated handler
	// error, avoiding a confusing Failed row after the row is already gone. The
	// guard matches only context.Canceled, never any ctx error: a
	// context.DeadlineExceeded (e.g. a per-task timeout added to newTaskContext)
	// falls through to the normal Failed-persistence path so a timed-out one-shot
	// task reaches Failed rather than being stranded in 'running'. A run that
	// SUCCEEDED (err == nil) is persisted after shutdown only if its durable
	// generation still exists. DELETE and resubmission invalidate that generation,
	// so a late success cannot recreate recovery state or replace a newer run.
	if err != nil && (errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled)) {
		log.Info("task cancelled; leaving store untouched",
			"type", taskType, "id", id, "run", run)
		return
	}

	t := time.Now().UTC()
	tr := &TaskResult{
		Generation:  generation,
		ID:          id,
		Type:        string(taskType),
		Run:         run,
		Params:      params,
		SubmittedAt: submittedAt,
		CompletedAt: &t,
	}
	// Stamp on both paths — a failed run may still carry a result (e.g. a
	// tx hash); a panic yields nil, so nothing partial is stamped.
	tr.Result = result
	if err != nil {
		tr.Error = err.Error()
		tr.Status = TaskStatusFailed
	} else {
		tr.Status = TaskStatusCompleted
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if storeErr := e.store.Save(tr); storeErr != nil {
		log.Error("failed to persist task result", "id", id, "err", storeErr)
		return
	}
	// Publish readiness only after its successful result is durable. A late
	// release cannot override a newer persisted hold, including a failed hold.
	if err == nil {
		switch taskType {
		case TaskMarkReady:
			_ = e.restoreReadiness()
		case TaskMarkNotReady:
			e.ready.Store(false)
		}
	}
}

// resolveStaleHandler returns the handler for a stale task. When no handler is
// registered it marks the task failed, persists that, and returns ok=false.
func (e *Engine) resolveStaleHandler(tr TaskResult) (TaskHandler, bool) {
	handler, ok := e.handlers[TaskType(tr.Type)]
	if ok {
		return handler, true
	}
	log.Warn("stale task has no handler, marking failed", "type", tr.Type, "id", tr.ID)
	t := time.Now().UTC()
	tr.Status = TaskStatusFailed
	tr.Error = "no handler registered for task type"
	tr.CompletedAt = &t
	if err := e.store.Save(&tr); err != nil {
		log.Error("failed to persist stale task failure", "id", tr.ID, "err", err)
	}
	return nil, false
}

// executeRecovered runs execute under a recover so a handler panic becomes a
// failed TaskResult instead of taking down the shared sidecar process. It
// guards only the handler goroutine; a task that spawns its own goroutines must
// recover within them (e.g. s3.streamGzip's writer).
func (e *Engine) executeRecovered(ctx context.Context, taskType TaskType, handler TaskHandler, params map[string]any) (result json.RawMessage, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Error("task handler panicked", "type", taskType, "panic", r, "stack", string(debug.Stack()))
			taskPanics.WithLabelValues(string(taskType)).Inc()
			taskFailures.WithLabelValues(string(taskType)).Inc()
			err = fmt.Errorf("task handler panicked: %v", r)
		}
	}()
	return e.execute(ctx, taskType, handler, params)
}

// execute runs a handler synchronously and logs the outcome.
func (e *Engine) execute(ctx context.Context, taskType TaskType, handler TaskHandler, params map[string]any) (json.RawMessage, error) {
	start := time.Now()
	result, err := handler(ctx, params)
	if err != nil {
		elapsed := time.Since(start)
		log.Error("task failed", "type", taskType, "elapsed", elapsed.Round(time.Millisecond), "err", err)
		taskDuration.WithLabelValues(string(taskType), "failed").Observe(elapsed.Seconds())
		taskFailures.WithLabelValues(string(taskType)).Inc()
		return result, err
	}
	elapsed := time.Since(start)
	log.Info("task completed", "type", taskType, "elapsed", elapsed.Round(time.Millisecond))
	taskDuration.WithLabelValues(string(taskType), "completed").Observe(elapsed.Seconds())
	return result, nil
}

// Healthz returns true after the engine has been marked ready.
// Use as a readiness check.
func (e *Engine) Healthz() bool {
	return e.ready.Load()
}

// Livez returns nil when the engine's backing store is responsive.
// Use as a liveness check — a non-nil error means the process is wedged
// (e.g., SQLite WAL corruption, PVC read-only).
func (e *Engine) Livez() error {
	return e.store.Ping()
}

// Status returns the engine's current state.
func (e *Engine) Status() StatusResponse {
	status := "Initializing"
	if e.ready.Load() {
		status = "Ready"
	}
	return StatusResponse{Status: status}
}

// RecentResults returns the most recent task results across all states.
func (e *Engine) RecentResults() ([]TaskResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	results, err := e.store.List(100)
	if err != nil {
		return nil, fmt.Errorf("list task results: %w", err)
	}
	return results, nil
}

// GetResult returns (nil, nil) only for confirmed absence. Store errors must
// reach callers so they can distinguish an unavailable task from a missing one.
func (e *Engine) GetResult(id string) (*TaskResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	r, err := e.store.Get(id)
	if err != nil {
		return nil, fmt.Errorf("get task result %q: %w", id, err)
	}
	return r, nil
}

// RemoveResult removes a task by ID. It returns (found, err): found reports
// whether a row existed, and a non-nil err means the store delete failed and
// the caller should retry the DELETE.
//
// Cancellation and durable deletion are serialized with submission. The store
// atomically replaces a running row with a cancellation tombstone; late writes
// require the removed generation and cannot recreate it. The tombstone remains
// until that handler returns, even if a new generation is submitted meanwhile.
func (e *Engine) RemoveResult(id string) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	entry, hadEntry := e.cancels[id]
	if hadEntry {
		entry.cancel()
	}

	deleted, err := e.store.Delete(id)
	if err != nil {
		log.Error("store.Delete failed; task left recoverable", "id", id, "err", err)
		return false, err
	}

	if hadEntry {
		delete(e.cancels, id)
	}
	return deleted, nil
}
