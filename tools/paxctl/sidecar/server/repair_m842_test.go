package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	sidecar "github.com/sei-protocol/seictl/sidecar/client"
	"github.com/sei-protocol/seictl/sidecar/engine"
	"github.com/sei-protocol/seictl/sidecar/rpc"
	"github.com/sei-protocol/seictl/sidecar/tasks"
)

type repair842Node struct {
	store  *TaskStore
	eng    *engine.Engine
	client *sidecar.ClientWithResponses
	http   *httptest.Server
	cancel context.CancelFunc
	once   sync.Once
}

func repair842Open(t *testing.T, path, home string, handlers map[engine.TaskType]engine.TaskHandler) *repair842Node {
	t.Helper()
	store, err := NewTaskStore(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	eng := engine.NewEngine(ctx, handlers, store)
	eng.Config.Checkpointer = store
	eng.RehydrateStaleTasks()
	srv := NewServer("127.0.0.1:0", eng, home, AuthnModeTrustedHeader)
	if srv.configErr != nil {
		cancel()
		store.Close()
		t.Fatal(srv.configErr)
	}
	transport := httptest.NewServer(srv.handler)
	client, err := sidecar.NewClientWithResponses(transport.URL,
		sidecar.WithHTTPClient(&http.Client{Timeout: 3 * time.Second}),
		sidecar.WithRequestEditorFn(func(_ context.Context, r *http.Request) error {
			r.Header.Set(remoteUserHeader, "repair-operator")
			return nil
		}))
	if err != nil {
		cancel()
		transport.Close()
		store.Close()
		t.Fatal(err)
	}
	n := &repair842Node{store: store, eng: eng, client: client, http: transport, cancel: cancel}
	t.Cleanup(n.close)
	return n
}

func (n *repair842Node) close() {
	n.once.Do(func() { n.http.Close(); n.cancel(); _ = n.store.Close() })
}

func repair842Eventually(t *testing.T, description string, f func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if f() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out: " + description)
}

func repair842Submit(t *testing.T, n *repair842Node, id uuid.UUID, request sidecar.TaskRequest, status int) {
	t.Helper()
	request.Id = &id
	response, err := n.client.SubmitTaskWithResponse(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode() != status {
		t.Fatalf("submit %s: status=%d body=%s, want %d", id, response.StatusCode(), response.Body, status)
	}
	if status == http.StatusCreated {
		if response.JSON201 == nil || response.JSON201.Id != id {
			t.Fatalf("submission ID changed: %+v", response)
		}
	} else {
		var failure ErrorResponse
		if err := json.Unmarshal(response.Body, &failure); err != nil || failure.Error == "" {
			t.Fatalf("missing explicit error: %s (%v)", response.Body, err)
		}
		if (status == http.StatusTooManyRequests || status == http.StatusServiceUnavailable) && response.HTTPResponse.Header.Get("Retry-After") == "" {
			t.Fatal("overload/unavailable response has no Retry-After")
		}
	}
}

func repair842Completed(t *testing.T, n *repair842Node, id uuid.UUID, run int, status engine.TaskStatus) *engine.TaskResult {
	t.Helper()
	var result engine.TaskResult
	repair842Eventually(t, "HTTP terminal result for "+id.String(), func() bool {
		response, err := n.client.GetTaskWithResponse(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode() != http.StatusOK {
			t.Fatalf("GET task: %d %s", response.StatusCode(), response.Body)
		}
		if err = json.Unmarshal(response.Body, &result); err != nil {
			t.Fatal(err)
		}
		return result.Status == status && result.Run == run && result.CompletedAt != nil
	})
	return &result
}

func repair842RPC(t *testing.T) (*httptest.Server, *atomic.Int64, <-chan struct{}) {
	t.Helper()
	height := &atomic.Int64{}
	seen := make(chan struct{})
	var once sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/status" {
			t.Errorf("unexpected RPC path %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		once.Do(func() { close(seen) })
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"jsonrpc":"2.0","result":{"sync_info":{"latest_block_height":"%d","catching_up":false}}}`, height.Load())
	}))
	t.Cleanup(server.Close)
	return server, height, seen
}

func repair842WaitSeen(t *testing.T, seen <-chan struct{}) {
	t.Helper()
	select {
	case <-seen:
	case <-time.After(5 * time.Second):
		t.Fatal("real condition consumer did not poll RPC")
	}
}

func repair842Patch(moniker string) sidecar.TaskRequest {
	return (sidecar.ConfigPatchTask{Files: map[string]map[string]any{"config.toml": {"moniker": moniker}}}).ToTaskRequest()
}

func TestRepairM_842_RequestSize(t *testing.T) {
	n := repair842Open(t, filepath.Join(t.TempDir(), "sidecar.db"), t.TempDir(), map[engine.TaskType]engine.TaskHandler{engine.TaskMarkReady: tasks.MarkReadyHandler()})
	base := `{"type":"mark-ready"}`
	exact := base + strings.Repeat(" ", MaxTaskRequestBytes-len(base))
	for _, test := range []struct {
		name, body string
		status     int
		chunked    bool
	}{
		{"exact limit", exact, http.StatusCreated, false},
		{"over limit", exact + " ", http.StatusRequestEntityTooLarge, false},
		{"chunked over limit", exact + " ", http.StatusRequestEntityTooLarge, true},
		{"large JSON string", `{"type":"mark-ready","params":{"value":"` + strings.Repeat("x", MaxTaskRequestBytes) + `"}}`, http.StatusRequestEntityTooLarge, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodPost, n.http.URL+"/v0/tasks", io.NopCloser(strings.NewReader(test.body)))
			if err != nil {
				t.Fatal(err)
			}
			if test.chunked {
				request.ContentLength = -1
			} else {
				request.ContentLength = int64(len(test.body))
			}
			request.Header.Set(remoteUserHeader, "repair-operator")
			response, err := n.client.ClientInterface.(*sidecar.Client).Client.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != test.status {
				t.Fatalf("status=%d want=%d body=%s", response.StatusCode, test.status, body)
			}
			if test.status == http.StatusRequestEntityTooLarge {
				var e ErrorResponse
				if json.Unmarshal(body, &e) != nil || e.Error == "" {
					t.Fatalf("missing size error: %s", body)
				}
			}
		})
	}
	var retained int
	if err := n.store.admission.db.QueryRow(retainedTaskCount).Scan(&retained); err != nil || retained != 1 {
		t.Fatalf("oversize request admitted: retained=%d err=%v", retained, err)
	}
}

func TestRepairM_842_BoundedQueueAndNodeExclusion(t *testing.T) {
	rpcServer, height, seen := repair842RPC(t)
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "config"), 0700); err != nil {
		t.Fatal(err)
	}
	n := repair842Open(t, filepath.Join(t.TempDir(), "sidecar.db"), home, map[engine.TaskType]engine.TaskHandler{
		engine.TaskAwaitCondition: tasks.NewConditionWaiter(rpc.NewStatusClient(rpcServer.URL, nil)).Handler(),
		engine.TaskConfigPatch:    tasks.NewConfigPatcher(home).Handler(), engine.TaskMarkReady: tasks.MarkReadyHandler(),
	})
	active := uuid.New()
	repair842Submit(t, n, active, (sidecar.AwaitConditionTask{Condition: sidecar.ConditionHeight, TargetHeight: 10}).ToTaskRequest(), http.StatusCreated)
	repair842WaitSeen(t, seen)
	patchID := uuid.New()
	repair842Submit(t, n, patchID, repair842Patch("after-exclusive-work"), http.StatusCreated)
	const producers = 2 * MaxQueuedTasks
	results := make(chan int, producers)
	var wg sync.WaitGroup
	for i := 0; i < producers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := uuid.New()
			req := (sidecar.MarkReadyTask{}).ToTaskRequest()
			req.Id = &id
			response, err := n.client.SubmitTaskWithResponse(context.Background(), req)
			if err != nil {
				results <- 0
				return
			}
			if response.StatusCode() == http.StatusTooManyRequests && response.HTTPResponse.Header.Get("Retry-After") == "" {
				results <- 0
				return
			}
			results <- response.StatusCode()
		}()
	}
	wg.Wait()
	close(results)
	accepted, rejected := 0, 0
	for code := range results {
		switch code {
		case http.StatusCreated:
			accepted++
		case http.StatusTooManyRequests:
			rejected++
		default:
			t.Fatalf("unexpected concurrent admission result: %d", code)
		}
	}
	if accepted != MaxQueuedTasks-2 || rejected != producers-accepted {
		t.Fatalf("queue bound: accepted=%d rejected=%d", accepted, rejected)
	}
	repair842Submit(t, n, active, (sidecar.AwaitConditionTask{Condition: sidecar.ConditionHeight, TargetHeight: 10}).ToTaskRequest(), http.StatusCreated)
	if _, err := os.Stat(filepath.Join(home, "config", "config.toml")); !os.IsNotExist(err) {
		t.Fatalf("queued node mutation executed concurrently: %v", err)
	}
	if n.eng.Healthz() {
		t.Fatal("queued mark-ready ran while the node slot was held")
	}
	response, err := n.client.GetTaskWithResponse(context.Background(), patchID)
	if err != nil || response.StatusCode() != http.StatusOK {
		t.Fatalf("queued GET: %+v %v", response, err)
	}
	var queued engine.TaskResult
	if err = json.Unmarshal(response.Body, &queued); err != nil || queued.Status != TaskStatusQueued {
		t.Fatalf("queued result: %s %v", response.Body, err)
	}
	var count, running int
	if err = n.store.admission.db.QueryRow("SELECT count(*) FROM task_admission").Scan(&count); err != nil || count != MaxQueuedTasks {
		t.Fatalf("persistent queue count=%d err=%v", count, err)
	}
	if err = n.store.admission.db.QueryRow("SELECT count(*) FROM task_results WHERE status='running'").Scan(&running); err != nil || running != 1 {
		t.Fatalf("running=%d err=%v", running, err)
	}
	height.Store(10)
	repair842Completed(t, n, active, 1, engine.TaskStatusCompleted)
	repair842Completed(t, n, patchID, 1, engine.TaskStatusCompleted)
	data, err := os.ReadFile(filepath.Join(home, "config", "config.toml"))
	if err != nil || !bytes.Contains(data, []byte("after-exclusive-work")) {
		t.Fatalf("real queued consumer did not patch config: %s %v", data, err)
	}
	repair842Eventually(t, "drain all accepted tasks", func() bool {
		var queued int
		return n.store.admission.db.QueryRow("SELECT count(*) FROM task_admission").Scan(&queued) == nil && queued == 0
	})
}

func TestRepairM_842_PersistentQueueRecovery(t *testing.T) {
	rpcServer, height, seen := repair842RPC(t)
	home := t.TempDir()
	path := filepath.Join(t.TempDir(), "sidecar.db")
	if err := os.MkdirAll(filepath.Join(home, "config"), 0700); err != nil {
		t.Fatal(err)
	}
	stopped := make(chan struct{})
	realWaiter := tasks.NewConditionWaiter(rpc.NewStatusClient(rpcServer.URL, nil)).Handler()
	handlers := map[engine.TaskType]engine.TaskHandler{
		engine.TaskAwaitCondition: func(ctx context.Context, p map[string]any) (json.RawMessage, error) {
			defer close(stopped)
			return realWaiter(ctx, p)
		},
		engine.TaskConfigPatch: tasks.NewConfigPatcher(home).Handler(),
	}
	first := repair842Open(t, path, home, handlers)
	active, pending := uuid.New(), uuid.New()
	repair842Submit(t, first, active, (sidecar.AwaitConditionTask{Condition: sidecar.ConditionHeight, TargetHeight: 10}).ToTaskRequest(), http.StatusCreated)
	repair842WaitSeen(t, seen)
	repair842Submit(t, first, pending, repair842Patch("recovered-from-durable-queue"), http.StatusCreated)
	before, err := first.store.Get(active.String())
	if err != nil || before == nil || before.Generation == "" {
		t.Fatalf("active recovery generation: %+v %v", before, err)
	}
	first.cancel()
	repair842WaitSeen(t, stopped)
	first.close()
	height.Store(10)
	handlers[engine.TaskAwaitCondition] = realWaiter
	reopened := repair842Open(t, path, home, handlers)
	recovered := repair842Completed(t, reopened, active, 1, engine.TaskStatusCompleted)
	if recovered.ID != active.String() {
		t.Fatal("recovery changed task identity")
	}
	repair842Completed(t, reopened, pending, 1, engine.TaskStatusCompleted)
	after, err := reopened.store.Get(active.String())
	if err != nil || after.Generation != before.Generation {
		t.Fatalf("recovery replaced generation: before=%+v after=%+v err=%v", before, after, err)
	}
	data, err := os.ReadFile(filepath.Join(home, "config", "config.toml"))
	if err != nil || !bytes.Contains(data, []byte("recovered-from-durable-queue")) {
		t.Fatalf("recovered producer -> consumer lost params: %s %v", data, err)
	}
}

func TestRepairM_842_DeleteKeepsNodeSlotUntilHandlerExit(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "config"), 0700); err != nil {
		t.Fatal(err)
	}
	patched, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	consumer := tasks.NewConfigPatcher(home).Handler()
	n := repair842Open(t, filepath.Join(t.TempDir(), "sidecar.db"), home, map[engine.TaskType]engine.TaskHandler{
		engine.TaskConfigPatch: func(ctx context.Context, p map[string]any) (json.RawMessage, error) {
			result, err := consumer(ctx, p)
			close(patched)
			<-release
			return result, err
		}, engine.TaskMarkReady: tasks.MarkReadyHandler(),
	})
	active, next := uuid.New(), uuid.New()
	repair842Submit(t, n, active, repair842Patch("real-consumer-completed-before-delayed-return"), http.StatusCreated)
	repair842WaitSeen(t, patched)
	repair842Submit(t, n, next, (sidecar.MarkReadyTask{}).ToTaskRequest(), http.StatusCreated)
	deleted, err := n.client.DeleteTaskWithResponse(context.Background(), active)
	if err != nil || deleted.StatusCode() != http.StatusNoContent {
		t.Fatalf("DELETE: %+v %v", deleted, err)
	}
	repair842Submit(t, n, active, repair842Patch("cannot-overlap-cancelled-handler"), http.StatusTooManyRequests)
	var state string
	var cancellations int
	if err = n.store.admission.db.QueryRow("SELECT state FROM task_admission WHERE id=?", active.String()).Scan(&state); err != nil || state != "cancelled" {
		t.Fatalf("cancelled claim disappeared: %q %v", state, err)
	}
	if err = n.store.admission.db.QueryRow("SELECT count(*) FROM task_cancellations WHERE id=?", active.String()).Scan(&cancellations); err != nil || cancellations != 1 {
		t.Fatalf("cancellation recovery marker lost: %d %v", cancellations, err)
	}
	if n.eng.Healthz() {
		t.Fatal("DELETE released the node slot before handler exit")
	}
	releaseOnce.Do(func() { close(release) })
	repair842Completed(t, n, next, 1, engine.TaskStatusCompleted)
	response, err := n.client.GetTaskWithResponse(context.Background(), active)
	if err != nil || response.StatusCode() != http.StatusNotFound {
		t.Fatalf("deleted task resurrected: %+v %v", response, err)
	}
	if err = n.store.admission.db.QueryRow("SELECT count(*) FROM task_cancellations WHERE id=?", active.String()).Scan(&cancellations); err != nil || cancellations != 0 {
		t.Fatalf("finished cancellation not retired: %d %v", cancellations, err)
	}
}

func TestRepairM_842_RetentionAndRecoveryMarkers(t *testing.T) {
	path, home := filepath.Join(t.TempDir(), "sidecar.db"), t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "config"), 0700); err != nil {
		t.Fatal(err)
	}
	n := repair842Open(t, path, home, map[engine.TaskType]engine.TaskHandler{engine.TaskMarkReady: tasks.MarkReadyHandler(), engine.TaskConfigPatch: tasks.NewConfigPatcher(home).Handler()})
	ids := make([]uuid.UUID, MaxRetainedTasks)
	for i := range ids {
		ids[i] = uuid.New()
		if i == len(ids)-1 {
			repair842Submit(t, n, ids[i], sidecar.TaskRequest{Type: string(engine.TaskConfigPatch)}, http.StatusCreated)
			repair842Completed(t, n, ids[i], 1, engine.TaskStatusFailed)
		} else {
			repair842Submit(t, n, ids[i], (sidecar.MarkReadyTask{}).ToTaskRequest(), http.StatusCreated)
			repair842Completed(t, n, ids[i], 1, engine.TaskStatusCompleted)
		}
	}
	repair842Submit(t, n, ids[len(ids)-1], repair842Patch("retry-at-retention-capacity"), http.StatusCreated)
	repair842Completed(t, n, ids[len(ids)-1], 2, engine.TaskStatusCompleted)
	marker := &engine.TxMarker{TaskID: ids[0].String(), TxHash: "retained-recovery-hash", TxBytes: []byte("retained-signed-transaction"), ChainID: "repair-chain", AccountNumber: 7, Sequence: 9}
	if err := n.store.SaveTxMarker(marker); err != nil {
		t.Fatal(err)
	}
	repair842Submit(t, n, uuid.New(), (sidecar.MarkReadyTask{}).ToTaskRequest(), http.StatusTooManyRequests)
	repair842Submit(t, n, ids[1], (sidecar.MarkReadyTask{}).ToTaskRequest(), http.StatusCreated)
	deleted, err := n.client.DeleteTaskWithResponse(context.Background(), ids[0])
	if err != nil || deleted.StatusCode() != http.StatusNoContent {
		t.Fatalf("delete pinned result: %+v %v", deleted, err)
	}
	repair842Submit(t, n, uuid.New(), (sidecar.MarkReadyTask{}).ToTaskRequest(), http.StatusTooManyRequests)
	deleted, err = n.client.DeleteTaskWithResponse(context.Background(), ids[1])
	if err != nil || deleted.StatusCode() != http.StatusNoContent {
		t.Fatalf("delete unpinned terminal result: %+v %v", deleted, err)
	}
	replacement := uuid.New()
	repair842Submit(t, n, replacement, (sidecar.MarkReadyTask{}).ToTaskRequest(), http.StatusCreated)
	repair842Completed(t, n, replacement, 1, engine.TaskStatusCompleted)
	n.close()
	reopened, err := NewTaskStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	recovered, err := reopened.GetTxMarker(marker.TaskID)
	if err != nil || recovered == nil || recovered.TxHash != marker.TxHash || !bytes.Equal(recovered.TxBytes, marker.TxBytes) || recovered.Sequence != marker.Sequence || recovered.AccountNumber != marker.AccountNumber || recovered.ChainID != marker.ChainID {
		t.Fatalf("retention discarded recovery marker: %+v %v", recovered, err)
	}
	var count int
	if err = reopened.admission.db.QueryRow(retainedTaskCount).Scan(&count); err != nil || count != MaxRetainedTasks {
		t.Fatalf("retained state bound=%d err=%v", count, err)
	}
}

func TestRepairM_842_FailedTaskRetry(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "config"), 0700); err != nil {
		t.Fatal(err)
	}
	n := repair842Open(t, filepath.Join(t.TempDir(), "sidecar.db"), home, map[engine.TaskType]engine.TaskHandler{engine.TaskConfigPatch: tasks.NewConfigPatcher(home).Handler()})
	id := uuid.New()
	repair842Submit(t, n, id, sidecar.TaskRequest{Type: string(engine.TaskConfigPatch)}, http.StatusCreated)
	failed := repair842Completed(t, n, id, 1, engine.TaskStatusFailed)
	if failed.Error == "" {
		t.Fatal("real consumer failure disappeared")
	}
	repair842Submit(t, n, id, repair842Patch("second-attempt"), http.StatusCreated)
	repair842Completed(t, n, id, 2, engine.TaskStatusCompleted)
	data, err := os.ReadFile(filepath.Join(home, "config", "config.toml"))
	if err != nil || !bytes.Contains(data, []byte("second-attempt")) {
		t.Fatalf("retry did not reach real consumer: %s %v", data, err)
	}
}

func TestRepairM_842_AdmissionUnavailable(t *testing.T) {
	n := repair842Open(t, filepath.Join(t.TempDir(), "sidecar.db"), t.TempDir(), map[engine.TaskType]engine.TaskHandler{engine.TaskMarkReady: tasks.MarkReadyHandler()})
	if err := n.store.Close(); err != nil {
		t.Fatal(err)
	}
	repair842Submit(t, n, uuid.New(), (sidecar.MarkReadyTask{}).ToTaskRequest(), http.StatusServiceUnavailable)
}

func TestRepairM_842_ExclusiveStoreOwner(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sidecar.db")
	first, err := NewTaskStore(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewTaskStore(path)
	if err == nil {
		second.Close()
		first.Close()
		t.Fatal("two sidecars acquired the same node queue")
	}
	if err = first.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewTaskStore(path)
	if err != nil {
		t.Fatalf("store owner lock survived close: %v", err)
	}
	if err = reopened.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRepairM_842_DeletePendingTask(t *testing.T) {
	rpcServer, height, seen := repair842RPC(t)
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "config"), 0700); err != nil {
		t.Fatal(err)
	}
	n := repair842Open(t, filepath.Join(t.TempDir(), "sidecar.db"), home, map[engine.TaskType]engine.TaskHandler{
		engine.TaskAwaitCondition: tasks.NewConditionWaiter(rpc.NewStatusClient(rpcServer.URL, nil)).Handler(),
		engine.TaskConfigPatch:    tasks.NewConfigPatcher(home).Handler(),
	})
	active, pending := uuid.New(), uuid.New()
	repair842Submit(t, n, active, (sidecar.AwaitConditionTask{Condition: sidecar.ConditionHeight, TargetHeight: 10}).ToTaskRequest(), http.StatusCreated)
	repair842WaitSeen(t, seen)
	repair842Submit(t, n, pending, repair842Patch("must-not-run"), http.StatusCreated)
	deleted, err := n.client.DeleteTaskWithResponse(context.Background(), pending)
	if err != nil || deleted.StatusCode() != http.StatusNoContent {
		t.Fatalf("delete pending: %+v %v", deleted, err)
	}
	response, err := n.client.GetTaskWithResponse(context.Background(), pending)
	if err != nil || response.StatusCode() != http.StatusNotFound {
		t.Fatalf("pending task survived DELETE: %+v %v", response, err)
	}
	height.Store(10)
	repair842Completed(t, n, active, 1, engine.TaskStatusCompleted)
	repair842Eventually(t, "active claim cleanup", func() bool {
		var count int
		return n.store.admission.db.QueryRow("SELECT count(*) FROM task_admission").Scan(&count) == nil && count == 0
	})
	if _, err = os.Stat(filepath.Join(home, "config", "config.toml")); !os.IsNotExist(err) {
		t.Fatalf("deleted queued mutation executed: %v", err)
	}
}
