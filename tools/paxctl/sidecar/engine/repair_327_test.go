package engine

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func repair327Wait(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for engine transition")
}

func repair327Receive(t *testing.T, started <-chan context.Context) context.Context {
	t.Helper()
	select {
	case ctx := <-started:
		return ctx
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not start")
		return nil
	}
}

func repair327Release(t *testing.T) (<-chan struct{}, func()) {
	t.Helper()
	ch := make(chan struct{})
	var once sync.Once
	release := func() { once.Do(func() { close(ch) }) }
	t.Cleanup(release)
	return ch, release
}

func repair327Tombstones(t *testing.T, store *SQLiteStore, id, generation string) int {
	t.Helper()
	var count int
	if err := store.db.QueryRow("SELECT count(*) FROM task_cancellations WHERE id=? AND generation=?", id, generation).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func repair327Absent(t *testing.T, store *SQLiteStore, id string) {
	t.Helper()
	r, err := store.Get(id)
	if err != nil || r != nil {
		t.Fatalf("deleted task recovered: result=%+v err=%v", r, err)
	}
	listed, err := store.List(100)
	if err != nil {
		t.Fatal(err)
	}
	stale, err := store.ListStaleTasks()
	if err != nil {
		t.Fatal(err)
	}
	latest, err := store.LatestByType(string(TaskConfigPatch))
	if err != nil || len(listed) != 0 || len(stale) != 0 || latest != nil {
		t.Fatalf("deleted task visible in recovery queries: list=%+v stale=%+v latest=%+v err=%v", listed, stale, latest, err)
	}
}

func TestRepair_327_DeleteLateCompletionRestart(t *testing.T) {
	for _, outcome := range []string{"success", "failure"} {
		t.Run(outcome, func(t *testing.T) {
			store, path := newFileStore(t)
			ctx, stop := context.WithCancel(context.Background())
			t.Cleanup(stop)
			started := make(chan context.Context, 1)
			release, finish := repair327Release(t)
			var calls atomic.Int32
			handlers := map[TaskType]TaskHandler{
				TaskConfigPatch: func(ctx context.Context, params map[string]any) (json.RawMessage, error) {
					calls.Add(1)
					started <- ctx
					<-release
					result, err := json.Marshal(params)
					if outcome == "failure" {
						err = errors.New("completion after cancellation")
					}
					return result, err
				},
			}
			eng := NewEngine(ctx, handlers, store)
			id, err := eng.Submit(Task{Type: TaskConfigPatch, Params: map[string]any{"value": "deleted"}})
			if err != nil {
				t.Fatal(err)
			}
			runningCtx := repair327Receive(t, started)
			old := mustGetResult(t, eng, id)
			if old == nil || old.Generation == "" {
				t.Fatal("submission did not persist a generation")
			}
			if found, err := eng.RemoveResult(id); err != nil || !found {
				t.Fatalf("delete: found=%v err=%v", found, err)
			}
			if !errors.Is(runningCtx.Err(), context.Canceled) {
				t.Fatal("DELETE did not cancel its handler")
			}
			repair327Absent(t, store, id)
			if repair327Tombstones(t, store, id, old.Generation) != 1 {
				t.Fatal("cancellation tombstone missing while handler is still running")
			}
			// Open the durable state through a fresh store while completion is blocked.
			reader, err := NewSQLiteStore(path)
			if err != nil {
				t.Fatal(err)
			}
			if repair327Tombstones(t, reader, id, old.Generation) != 1 {
				t.Fatal("cancellation tombstone was not durable")
			}
			repair327Absent(t, reader, id)
			if err := reader.Close(); err != nil {
				t.Fatal(err)
			}
			if found, err := eng.RemoveResult(id); err != nil || found {
				t.Fatalf("repeat delete: found=%v err=%v", found, err)
			}
			finish()
			repair327Wait(t, func() bool { return repair327Tombstones(t, store, id, old.Generation) == 0 })
			repair327Absent(t, store, id)
			stop()
			store = reopenStore(t, store, path)
			restarted := NewEngine(context.Background(), handlers, store)
			restarted.RehydrateStaleTasks()
			repair327Absent(t, store, id)
			if calls.Load() != 1 {
				t.Fatalf("deleted handler replayed after restart: calls=%d", calls.Load())
			}
		})
	}
}

func TestRepair_327_DeleteResubmitInterleavings(t *testing.T) {
	for _, oldFirst := range []bool{true, false} {
		name := "new-completes-first"
		if oldFirst {
			name = "old-completes-first"
		}
		t.Run(name, func(t *testing.T) {
			store, path := newFileStore(t)
			ctx, stop := context.WithCancel(context.Background())
			t.Cleanup(stop)
			oldStarted := make(chan context.Context, 1)
			newStarted := make(chan context.Context, 1)
			oldRelease, finishOld := repair327Release(t)
			newRelease, finishNew := repair327Release(t)
			handlers := map[TaskType]TaskHandler{
				TaskConfigPatch: func(ctx context.Context, params map[string]any) (json.RawMessage, error) {
					if params["version"] == "old" {
						oldStarted <- ctx
						<-oldRelease
					} else {
						newStarted <- ctx
						<-newRelease
					}
					return json.Marshal(params)
				},
			}
			eng := NewEngine(ctx, handlers, store)
			id, err := eng.Submit(Task{Type: TaskConfigPatch, Params: map[string]any{"version": "old"}})
			if err != nil {
				t.Fatal(err)
			}
			repair327Receive(t, oldStarted)
			old := mustGetResult(t, eng, id)
			if found, err := eng.RemoveResult(id); err != nil || !found {
				t.Fatalf("delete: found=%v err=%v", found, err)
			}
			if _, err := eng.Submit(Task{ID: id, Type: TaskConfigPatch, Params: map[string]any{"version": "new"}}); err != nil {
				t.Fatal(err)
			}
			newCtx := repair327Receive(t, newStarted)
			current := mustGetResult(t, eng, id)
			if current == nil || current.Generation == old.Generation || current.Generation == "" || current.Run != 1 || old.Run != 1 {
				t.Fatalf("resubmission reused generation: old=%+v new=%+v", old, current)
			}
			if oldFirst {
				finishOld()
				repair327Wait(t, func() bool { return repair327Tombstones(t, store, id, old.Generation) == 0 })
				stillRunning := mustGetResult(t, eng, id)
				if stillRunning == nil || stillRunning.Status != TaskStatusRunning || stillRunning.Generation != current.Generation || newCtx.Err() != nil {
					t.Fatalf("obsolete completion changed or cancelled the live resubmission: %+v, ctx=%v", stillRunning, newCtx.Err())
				}
				finishNew()
			} else {
				finishNew()
				waitForResult(t, eng, id)
				finishOld()
				repair327Wait(t, func() bool { return repair327Tombstones(t, store, id, old.Generation) == 0 })
			}
			result := waitForResult(t, eng, id)
			if result.Generation != current.Generation || result.Status != TaskStatusCompleted || string(result.Result) != `{"version":"new"}` || result.Params["version"] != "new" {
				t.Fatalf("obsolete completion overwrote new result: %+v", result)
			}
			repair327Wait(t, func() bool { return cancelRegistrySize(eng) == 0 })
			stop()
			store = reopenStore(t, store, path)
			restarted := NewEngine(context.Background(), handlers, store)
			restarted.RehydrateStaleTasks()
			recovered := mustGetResult(t, restarted, id)
			if recovered == nil || recovered.Generation != current.Generation || string(recovered.Result) != `{"version":"new"}` || recovered.Status != TaskStatusCompleted {
				t.Fatalf("incorrect recovered resubmission: %+v", recovered)
			}
		})
	}
}

func TestRepair_327_PersistedGenerationRejectsObsoleteResult(t *testing.T) {
	store, path := newFileStore(t)
	ctx, stop := context.WithCancel(context.Background())
	t.Cleanup(stop)
	handlers := map[TaskType]TaskHandler{
		TaskConfigPatch: func(_ context.Context, params map[string]any) (json.RawMessage, error) {
			return json.Marshal(params)
		},
	}
	eng := NewEngine(ctx, handlers, store)
	id, err := eng.Submit(Task{Type: TaskConfigPatch, Params: map[string]any{"version": "old"}})
	if err != nil {
		t.Fatal(err)
	}
	old := waitForResult(t, eng, id)
	repair327Wait(t, func() bool { return cancelRegistrySize(eng) == 0 })
	if found, err := eng.RemoveResult(id); err != nil || !found {
		t.Fatalf("delete: found=%v err=%v", found, err)
	}
	stop()
	store = reopenStore(t, store, path)
	if err := store.Save(old); !errors.Is(err, ErrTaskGeneration) {
		t.Fatalf("obsolete result recreated deleted state after restart: %v", err)
	}
	repair327Absent(t, store, id)
	restarted := NewEngine(context.Background(), handlers, store)
	if _, err := restarted.Submit(Task{ID: id, Type: TaskConfigPatch, Params: map[string]any{"version": "new"}}); err != nil {
		t.Fatal(err)
	}
	current := waitForResult(t, restarted, id)
	repair327Wait(t, func() bool { return cancelRegistrySize(restarted) == 0 })
	if current.Generation == old.Generation {
		t.Fatal("generation repeated after store and engine restart")
	}
	if err := store.Save(old); !errors.Is(err, ErrTaskGeneration) {
		t.Fatalf("obsolete result replaced resubmitted state: %v", err)
	}
	recovered := mustGetResult(t, restarted, id)
	if recovered == nil || recovered.Generation != current.Generation || string(recovered.Result) != `{"version":"new"}` {
		t.Fatalf("persisted generation guard lost new output: %+v", recovered)
	}
}

func TestRepair_327_ShutdownRecoveryPreservesGeneration(t *testing.T) {
	for _, successful := range []bool{false, true} {
		name := "interrupted"
		if successful {
			name = "successful"
		}
		t.Run(name, func(t *testing.T) {
			store, path := newFileStore(t)
			ctx, stop := context.WithCancel(context.Background())
			t.Cleanup(stop)
			started := make(chan context.Context, 1)
			eng := NewEngine(ctx, map[TaskType]TaskHandler{
				TaskConfigPatch: func(ctx context.Context, params map[string]any) (json.RawMessage, error) {
					started <- ctx
					<-ctx.Done()
					if !successful {
						return nil, ctx.Err()
					}
					return json.Marshal(params)
				},
			}, store)
			id, err := eng.Submit(Task{Type: TaskConfigPatch, Params: map[string]any{"value": "durable"}})
			if err != nil {
				t.Fatal(err)
			}
			repair327Receive(t, started)
			before := mustGetResult(t, eng, id)
			stop()
			repair327Wait(t, func() bool { return cancelRegistrySize(eng) == 0 })
			store = reopenStore(t, store, path)
			var calls atomic.Int32
			restarted := NewEngine(context.Background(), map[TaskType]TaskHandler{
				TaskConfigPatch: func(_ context.Context, params map[string]any) (json.RawMessage, error) {
					calls.Add(1)
					return json.Marshal(params)
				},
			}, store)
			restarted.RehydrateStaleTasks()
			recovered := waitForResult(t, restarted, id)
			repair327Wait(t, func() bool { return cancelRegistrySize(restarted) == 0 })
			if recovered.Generation != before.Generation || recovered.Run != before.Run || recovered.Status != TaskStatusCompleted || string(recovered.Result) != `{"value":"durable"}` {
				t.Fatalf("shutdown recovery lost generation or output: before=%+v after=%+v", before, recovered)
			}
			wantCalls := int32(1)
			if successful {
				wantCalls = 0
			}
			if calls.Load() != wantCalls {
				t.Fatalf("recovery executions=%d want=%d", calls.Load(), wantCalls)
			}
		})
	}
}

func TestRepair_327_FailedRetryUsesNewGeneration(t *testing.T) {
	store, path := newFileStore(t)
	ctx, stop := context.WithCancel(context.Background())
	t.Cleanup(stop)
	eng := NewEngine(ctx, map[TaskType]TaskHandler{
		TaskConfigPatch: func(_ context.Context, params map[string]any) (json.RawMessage, error) {
			if params["fail"] == true {
				return nil, errors.New("first execution failed")
			}
			return json.Marshal(params)
		},
	}, store)
	id, err := eng.Submit(Task{Type: TaskConfigPatch, Params: map[string]any{"fail": true}})
	if err != nil {
		t.Fatal(err)
	}
	failed := waitForResult(t, eng, id)
	repair327Wait(t, func() bool { return cancelRegistrySize(eng) == 0 })
	if failed.Status != TaskStatusFailed {
		t.Fatalf("expected failed execution, got %+v", failed)
	}
	if _, err := eng.Submit(Task{ID: id, Type: TaskConfigPatch, Params: map[string]any{"value": "retry"}}); err != nil {
		t.Fatal(err)
	}
	retried := waitForResult(t, eng, id)
	repair327Wait(t, func() bool { return cancelRegistrySize(eng) == 0 })
	if retried.Status != TaskStatusCompleted || retried.Run != 2 || retried.Generation == failed.Generation || retried.Generation == "" {
		t.Fatalf("retry did not receive a fresh generation: failed=%+v retried=%+v", failed, retried)
	}
	if err := store.Save(failed); !errors.Is(err, ErrTaskGeneration) {
		t.Fatalf("previous execution overwrote retry: %v", err)
	}
	stop()
	store = reopenStore(t, store, path)
	recovered, err := store.Get(id)
	if err != nil || recovered == nil || recovered.Generation != retried.Generation || recovered.Status != TaskStatusCompleted || recovered.Run != 2 || string(recovered.Result) != `{"value":"retry"}` {
		t.Fatalf("retry recovery incorrect: result=%+v err=%v", recovered, err)
	}
}
