package engine

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

func readinessRecoveryHandlers(calls *atomic.Int32) map[TaskType]TaskHandler {
	return map[TaskType]TaskHandler{
		TaskMarkReady: TypedHandler(func(context.Context, struct{}) error {
			calls.Add(1)
			return nil
		}),
		// Leave the release history present so the engine must enforce ordering
		// itself, independently of the hold handler's usual history purge.
		TaskMarkNotReady: TypedHandler(func(context.Context, struct{}) error { return nil }),
	}
}

func readinessRecoveryEngine(t *testing.T, store ResultStore, handlers map[TaskType]TaskHandler) (*Engine, context.CancelFunc) {
	t.Helper()
	ctx, stop := context.WithCancel(context.Background())
	t.Cleanup(stop)
	return NewEngine(ctx, handlers, store), stop
}

func readinessRecoveryDrain(t *testing.T, eng *Engine) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cancelRegistrySize(eng) == 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("task handlers did not stop")
}

func readinessRecoverySubmit(t *testing.T, eng *Engine, task Task, status TaskStatus) *TaskResult {
	t.Helper()
	id, err := eng.Submit(task)
	if err != nil {
		t.Fatalf("submit %s: %v", task.Type, err)
	}
	result := waitForResult(t, eng, id)
	if result.Status != status {
		t.Fatalf("%s status=%s error=%q, want %s", task.Type, result.Status, result.Error, status)
	}
	readinessRecoveryDrain(t, eng)
	return result
}

func readinessRecoveryReady(t *testing.T, eng *Engine, want bool) {
	t.Helper()
	wantStatus := "Initializing"
	if want {
		wantStatus = "Ready"
	}
	if eng.Healthz() != want || eng.Status().Status != wantStatus {
		t.Fatalf("readiness=%v status=%s, want %v/%s", eng.Healthz(), eng.Status().Status, want, wantStatus)
	}
}

func TestReadinessRecoveryCompletedReleaseRestartRepeatedSubmission(t *testing.T) {
	store, path := newFileStore(t)
	var calls atomic.Int32
	handlers := readinessRecoveryHandlers(&calls)
	eng, stop := readinessRecoveryEngine(t, store, handlers)
	readinessRecoveryReady(t, eng, false)
	task := Task{ID: uuid.NewString(), Type: TaskMarkReady}
	original := readinessRecoverySubmit(t, eng, task, TaskStatusCompleted)
	readinessRecoveryReady(t, eng, true)

	for boot := 0; boot < 2; boot++ {
		stop()
		store = reopenStore(t, store, path)
		eng, stop = readinessRecoveryEngine(t, store, handlers)
		// Probes must be correct immediately, before stale-task replay and
		// before a controller redelivers the already completed release.
		readinessRecoveryReady(t, eng, true)
		stale, err := store.ListStaleTasks()
		if err != nil || len(stale) != 0 {
			t.Fatalf("completed-only restart: stale=%+v err=%v", stale, err)
		}
		eng.RehydrateStaleTasks()
		for delivery := 0; delivery < 3; delivery++ {
			replayed := readinessRecoverySubmit(t, eng, task, TaskStatusCompleted)
			if replayed.ID != original.ID || replayed.Run != 1 || replayed.Generation != original.Generation ||
				!replayed.CompletedAt.Equal(*original.CompletedAt) {
				t.Fatalf("completed release was re-executed: before=%+v after=%+v", original, replayed)
			}
			readinessRecoveryReady(t, eng, true)
		}
		if calls.Load() != 1 {
			t.Fatalf("release handler ran %d times, want exactly once", calls.Load())
		}
	}
}

func TestReadinessRecoveryNewerHoldWinsOnRestartAndRedelivery(t *testing.T) {
	for _, holdStatus := range []TaskStatus{TaskStatusCompleted, TaskStatusFailed, TaskStatusRunning} {
		t.Run(string(holdStatus), func(t *testing.T) {
			store, path := newFileStore(t)
			var calls atomic.Int32
			handlers := readinessRecoveryHandlers(&calls)
			started := make(chan struct{})
			handlers[TaskMarkNotReady] = TypedHandler(func(ctx context.Context, _ struct{}) error {
				if holdStatus == TaskStatusFailed {
					return errors.New("hold could not complete")
				}
				if holdStatus == TaskStatusRunning {
					close(started)
					<-ctx.Done()
					return ctx.Err()
				}
				return nil
			})
			eng, stop := readinessRecoveryEngine(t, store, handlers)
			release := Task{ID: uuid.NewString(), Type: TaskMarkReady}
			original := readinessRecoverySubmit(t, eng, release, TaskStatusCompleted)
			holdID, err := eng.Submit(Task{Type: TaskMarkNotReady})
			if err != nil {
				t.Fatal(err)
			}
			if holdStatus == TaskStatusRunning {
				select {
				case <-started:
				case <-time.After(2 * time.Second):
					t.Fatal("hold handler did not start")
				}
			} else if hold := waitForResult(t, eng, holdID); hold.Status != holdStatus {
				t.Fatalf("hold=%+v, want %s", hold, holdStatus)
			}
			stop()
			readinessRecoveryDrain(t, eng)
			store = reopenStore(t, store, path)
			handlers = readinessRecoveryHandlers(&calls)
			eng, _ = readinessRecoveryEngine(t, store, handlers)
			readinessRecoveryReady(t, eng, false)
			eng.RehydrateStaleTasks()
			if holdStatus == TaskStatusRunning {
				waitForResult(t, eng, holdID)
			}
			replayed := readinessRecoverySubmit(t, eng, release, TaskStatusCompleted)
			if replayed.Generation != original.Generation || calls.Load() != 1 {
				t.Fatal("old release was re-executed instead of deduplicated")
			}
			readinessRecoveryReady(t, eng, false)
		})
	}
}

func TestReadinessRecoveryReleaseAfterHoldRestoresReady(t *testing.T) {
	store, path := newFileStore(t)
	var calls atomic.Int32
	handlers := readinessRecoveryHandlers(&calls)
	eng, stop := readinessRecoveryEngine(t, store, handlers)
	readinessRecoverySubmit(t, eng, Task{Type: TaskMarkReady}, TaskStatusCompleted)
	readinessRecoverySubmit(t, eng, Task{Type: TaskMarkNotReady}, TaskStatusCompleted)
	readinessRecoveryReady(t, eng, false)
	release := Task{ID: uuid.NewString(), Type: TaskMarkReady}
	readinessRecoverySubmit(t, eng, release, TaskStatusCompleted)
	readinessRecoveryReady(t, eng, true)
	stop()
	store = reopenStore(t, store, path)
	eng, _ = readinessRecoveryEngine(t, store, handlers)
	readinessRecoveryReady(t, eng, true)
	eng.RehydrateStaleTasks()
	readinessRecoverySubmit(t, eng, release, TaskStatusCompleted)
	if calls.Load() != 2 {
		t.Fatalf("release calls=%d, want 2", calls.Load())
	}
	readinessRecoveryReady(t, eng, true)
}

func TestReadinessRecoveryFailedReleasePreservesPriorCompletedRelease(t *testing.T) {
	store, path := newFileStore(t)
	handlers := map[TaskType]TaskHandler{
		TaskMarkReady: TypedHandler(func(_ context.Context, params struct{ Fail bool }) error {
			if params.Fail {
				return errors.New("release failed")
			}
			return nil
		}),
	}
	eng, stop := readinessRecoveryEngine(t, store, handlers)
	readinessRecoverySubmit(t, eng, Task{Type: TaskMarkReady}, TaskStatusCompleted)
	readinessRecoverySubmit(t, eng, Task{Type: TaskMarkReady, Params: map[string]any{"Fail": true}}, TaskStatusFailed)
	readinessRecoveryReady(t, eng, true)
	stop()
	store = reopenStore(t, store, path)
	eng, _ = readinessRecoveryEngine(t, store, handlers)
	readinessRecoveryReady(t, eng, true)
}

func TestReadinessRecoveryNewestHoldAndTies(t *testing.T) {
	base := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name    string
		release time.Time
		hold    time.Time
		ready   bool
	}{
		{"newer hold with fractional seconds", base, base.Add(time.Nanosecond), false},
		{"newer release with fractional seconds", base.Add(time.Nanosecond), base, true},
		{"equal timestamps keep hold", base, base, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, path := newFileStore(t)
			var calls atomic.Int32
			handlers := readinessRecoveryHandlers(&calls)
			eng, stop := readinessRecoveryEngine(t, store, handlers)
			oldHold := readinessRecoverySubmit(t, eng, Task{Type: TaskMarkNotReady}, TaskStatusCompleted)
			release := readinessRecoverySubmit(t, eng, Task{Type: TaskMarkReady}, TaskStatusCompleted)
			newHold := readinessRecoverySubmit(t, eng, Task{Type: TaskMarkNotReady}, TaskStatusCompleted)
			// Give real producer-emitted rows exact boundary times. Ordering
			// must use time values, including RFC3339Nano's optional fraction.
			for id, submitted := range map[string]time.Time{
				oldHold.ID: base.Add(-time.Second),
				release.ID: tc.release,
				newHold.ID: tc.hold,
			} {
				if _, err := store.db.Exec("UPDATE task_results SET submitted_at = ? WHERE id = ?", submitted.Format(time.RFC3339Nano), id); err != nil {
					t.Fatal(err)
				}
			}
			stop()
			store = reopenStore(t, store, path)
			eng, _ = readinessRecoveryEngine(t, store, handlers)
			readinessRecoveryReady(t, eng, tc.ready)
			eng.RehydrateStaleTasks()
			readinessRecoverySubmit(t, eng, Task{ID: release.ID, Type: TaskMarkReady}, TaskStatusCompleted)
			readinessRecoveryReady(t, eng, tc.ready)
		})
	}
}

func TestReadinessRecoveryLateReleaseCannotOverrideNewerHold(t *testing.T) {
	store, path := newFileStore(t)
	started := make(chan struct{})
	finish := make(chan struct{})
	var calls atomic.Int32
	handlers := readinessRecoveryHandlers(&calls)
	handlers[TaskMarkReady] = TypedHandler(func(ctx context.Context, _ struct{}) error {
		close(started)
		select {
		case <-finish:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	eng, stop := readinessRecoveryEngine(t, store, handlers)
	id, err := eng.Submit(Task{Type: TaskMarkReady})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("release handler did not start")
	}
	holdID, err := eng.Submit(Task{Type: TaskMarkNotReady})
	if err != nil {
		t.Fatal(err)
	}
	if hold := waitForResult(t, eng, holdID); hold.Status != TaskStatusCompleted {
		t.Fatalf("hold failed: %+v", hold)
	}
	close(finish)
	if release := waitForResult(t, eng, id); release.Status != TaskStatusCompleted {
		t.Fatalf("late release failed: %+v", release)
	}
	readinessRecoveryDrain(t, eng)
	readinessRecoveryReady(t, eng, false)
	stop()
	store = reopenStore(t, store, path)
	eng, _ = readinessRecoveryEngine(t, store, readinessRecoveryHandlers(&calls))
	readinessRecoveryReady(t, eng, false)
}

func TestReadinessRecoveryReleaseRequiresDurableCompletion(t *testing.T) {
	store, path := newFileStore(t)
	if _, err := store.db.Exec(`CREATE TRIGGER reject_release_completion
		BEFORE UPDATE OF status ON task_results
		WHEN NEW.type = 'mark-ready' AND NEW.status = 'completed'
		BEGIN SELECT RAISE(ABORT, 'release completion unavailable'); END`); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	handlers := readinessRecoveryHandlers(&calls)
	eng, stop := readinessRecoveryEngine(t, store, handlers)
	id, err := eng.Submit(Task{Type: TaskMarkReady})
	if err != nil {
		t.Fatal(err)
	}
	readinessRecoveryDrain(t, eng)
	result := mustGetResult(t, eng, id)
	if result == nil || result.Status != TaskStatusRunning || calls.Load() != 1 {
		t.Fatalf("failed completion should remain recoverable: result=%+v calls=%d", result, calls.Load())
	}
	readinessRecoveryReady(t, eng, false)
	stop()
	store = reopenStore(t, store, path)
	eng, _ = readinessRecoveryEngine(t, store, handlers)
	readinessRecoveryReady(t, eng, false)
	if _, err := store.db.Exec("DROP TRIGGER reject_release_completion"); err != nil {
		t.Fatal(err)
	}
	eng.RehydrateStaleTasks()
	if recovered := waitForResult(t, eng, id); recovered.Status != TaskStatusCompleted || recovered.Run != 1 {
		t.Fatalf("release did not recover its incomplete run: %+v", recovered)
	}
	readinessRecoveryDrain(t, eng)
	readinessRecoveryReady(t, eng, true)
}

func TestReadinessRecoveryQueryStoreFailureDiffersFromAbsence(t *testing.T) {
	store := newTestStore(t)
	eng, _ := readinessRecoveryEngine(t, store, map[TaskType]TaskHandler{
		TaskConfigPatch: TypedHandlerWithResult(func(_ context.Context, params struct{ Value string }) (struct{ Value string }, error) {
			return params, nil
		}),
	})
	missing := uuid.NewString()
	if result, err := eng.GetResult(missing); result != nil || err != nil {
		t.Fatalf("confirmed absence: result=%+v err=%v", result, err)
	}
	if results, err := eng.RecentResults(); len(results) != 0 || err != nil {
		t.Fatalf("confirmed empty history: results=%+v err=%v", results, err)
	}
	produced := readinessRecoverySubmit(t, eng, Task{Type: TaskConfigPatch, Params: map[string]any{"Value": "persisted"}}, TaskStatusCompleted)
	result, err := eng.GetResult(produced.ID)
	if err != nil || result == nil || string(result.Result) != `{"Value":"persisted"}` {
		t.Fatalf("producer/store/query result=%+v err=%v", result, err)
	}
	if results, err := eng.RecentResults(); err != nil || len(results) != 1 || results[0].ID != produced.ID || string(results[0].Result) != string(result.Result) {
		t.Fatalf("producer/store/list results=%+v err=%v", results, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{produced.ID, missing} {
		_, sourceErr := store.Get(id)
		result, err := eng.GetResult(id)
		if sourceErr == nil || err == nil || !errors.Is(err, sourceErr) || result != nil {
			t.Fatalf("unavailable lookup must retain store error: result=%+v err=%v source=%v", result, err, sourceErr)
		}
	}
	_, sourceErr := store.List(100)
	results, err := eng.RecentResults()
	if sourceErr == nil || err == nil || !errors.Is(err, sourceErr) || results != nil {
		t.Fatalf("unavailable history must retain store error: results=%+v err=%v source=%v", results, err, sourceErr)
	}
}

func TestReadinessRecoveryQueryDecodeErrorsReachConsumer(t *testing.T) {
	store := newTestStore(t)
	eng, _ := readinessRecoveryEngine(t, store, map[TaskType]TaskHandler{
		TaskConfigPatch: TypedHandler(func(context.Context, struct{}) error { return nil }),
	})
	produced := readinessRecoverySubmit(t, eng, Task{Type: TaskConfigPatch}, TaskStatusCompleted)
	if _, err := store.db.Exec("UPDATE task_results SET params = ? WHERE id = ?", "{", produced.ID); err != nil {
		t.Fatal(err)
	}
	result, getErr := eng.GetResult(produced.ID)
	results, listErr := eng.RecentResults()
	var getSyntax, listSyntax *json.SyntaxError
	if result != nil || !errors.As(getErr, &getSyntax) {
		t.Fatalf("get hid persisted decode failure: result=%+v err=%v", result, getErr)
	}
	if results != nil || !errors.As(listErr, &listSyntax) {
		t.Fatalf("list hid persisted decode failure: results=%+v err=%v", results, listErr)
	}
}

func TestReadinessRecoveryUnavailableRecoveryKeepsGateClosed(t *testing.T) {
	store, path := newFileStore(t)
	var calls atomic.Int32
	handlers := readinessRecoveryHandlers(&calls)
	eng, stop := readinessRecoveryEngine(t, store, handlers)
	readinessRecoverySubmit(t, eng, Task{Type: TaskMarkReady}, TaskStatusCompleted)
	stop()
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	eng, _ = readinessRecoveryEngine(t, store, handlers)
	readinessRecoveryReady(t, eng, false)
	eng.RehydrateStaleTasks()
	readinessRecoveryReady(t, eng, false)
	if calls.Load() != 1 {
		t.Fatal("recovery dispatched a task with an unavailable store")
	}
	store, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	eng, _ = readinessRecoveryEngine(t, store, handlers)
	readinessRecoveryReady(t, eng, true)
}
