package storage_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/serve"
	storage "github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/storage"
	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/storage/testdata/reconciliation"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
	sharedstorage "github.com/Sidiora-Technologies/KindleLaunch/shared/storage"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func repairM993Store(t *testing.T, path string) *reconciliation.Store {
	t.Helper()
	store, err := reconciliation.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func repairM993Update(t *testing.T, store *reconciliation.Store, update func(*reconciliation.State)) {
	t.Helper()
	if err := store.Update(context.Background(), update); err != nil {
		t.Fatal(err)
	}
}

func TestRepairM_993_tombstone_replay_batch(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "committed.json")
	keeper := repairM993Store(t, path)
	const total = 769
	repairM993Update(t, keeper, func(state *reconciliation.State) {
		for i := 0; i < total; i++ {
			hash := fmt.Sprintf("%064x", i+1)
			state.Tombstones[hash] = storage.Tombstone{Hash: hash, Revision: 2}
		}
	})
	// Use the production reconciler to produce the initial operation identities.
	if err := storage.ReplayTombstones(ctx, keeper); err != nil {
		t.Fatal(err)
	}
	before := keeper.Snapshot()
	if len(before.Operations) != 2*total {
		t.Fatalf("initial operations=%d", len(before.Operations))
	}
	repairM993Update(t, keeper, func(state *reconciliation.State) {
		for key, op := range state.Operations {
			if op.Kind == "purge" || strings.HasSuffix(op.Hash, "1") {
				delete(state.Operations, key)
			}
		}
		for hash := range state.Restrictions {
			state.Restrictions[hash] = reconciliation.Restriction{Revision: 1, State: "approved"}
		}
		// One existing completed operation must retain both identity and proof.
		for key, op := range state.Operations {
			op.State = "complete"
			op.ProviderID = "persisted-provider"
			op.Proof = "persisted-completion"
			state.Operations[key] = op
		}
	})
	retained := keeper.Snapshot().Operations
	if err := keeper.Close(); err != nil {
		t.Fatal(err)
	}
	if err := storage.ReplayTombstones(ctx, keeper); !errors.Is(err, storage.ErrUnavailable) {
		t.Fatalf("closed keeper accepted work: %v", err)
	}
	keeper = repairM993Store(t, path)
	defer func() { _ = keeper.Close() }()
	if err := storage.ReplayTombstones(ctx, keeper); err != nil {
		t.Fatal(err)
	}
	recovered := keeper.Snapshot()
	if keeper.MaxBatch != 256 || keeper.Commits != 5 {
		t.Fatalf("batch bound/progress: maximum=%d commits=%d", keeper.MaxBatch, keeper.Commits)
	}
	if len(recovered.Operations) != 2*total {
		t.Fatalf("tail work lost: operations=%d", len(recovered.Operations))
	}
	seen := map[string]bool{}
	for key, op := range recovered.Operations {
		if _, err := uuid.Parse(op.ID); err != nil || seen[op.ID] {
			t.Fatalf("invalid/duplicate operation identity: %+v", op)
		}
		seen[op.ID] = true
		if prior, exists := retained[key]; exists {
			if !reflect.DeepEqual(op, prior) {
				t.Fatalf("existing work changed: before=%+v after=%+v", prior, op)
			}
		} else if op.State != "requested" || op.Proof != "" || op.ProviderID != "" {
			t.Fatalf("recovery manufactured completion: %+v", op)
		}
	}
	for hash, r := range recovered.Restrictions {
		if r.State != "deleted" || r.Revision != 2 {
			t.Fatalf("restored restriction %s: %+v", hash, r)
		}
	}
	if err := storage.ReplayTombstones(ctx, keeper); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(recovered, keeper.Snapshot()) {
		t.Fatal("duplicate replay changed durable identities")
	}
	if err := keeper.Close(); err != nil {
		t.Fatal(err)
	}
	keeper = repairM993Store(t, path)
	if !reflect.DeepEqual(recovered, keeper.Snapshot()) {
		t.Fatal("committed recovery lost on second restart")
	}
}

func repairM993Objects(t *testing.T) (*reconciliation.Objects, *s3.Client, *sharedstorage.Client) {
	t.Helper()
	objects := reconciliation.NewObjects()
	server := httptest.NewServer(objects)
	t.Cleanup(server.Close)
	client := s3.New(s3.Options{
		Region: "us-east-1", BaseEndpoint: aws.String(server.URL), UsePathStyle: true,
		Credentials: credentials.NewStaticCredentialsProvider("fixture-key", "fixture-secret", ""),
		HTTPClient:  server.Client(), RetryMaxAttempts: 1,
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	})
	return objects, client, sharedstorage.NewFromS3(client, "media-fixture")
}

func repairM993Put(t *testing.T, client *sharedstorage.Client, key string, body []byte) {
	t.Helper()
	if err := client.Put(context.Background(), key, bytes.NewReader(body), int64(len(body)), "image/png"); err != nil {
		t.Fatal(err)
	}
}

func TestRepairM_993_inventory_cursor(t *testing.T) {
	ctx := context.Background()
	objects, client, publisher := repairM993Objects(t)
	path := filepath.Join(t.TempDir(), "inventory.json")
	keeper := repairM993Store(t, path)
	const total = 7
	keys := make([]string, total)
	bodies := make([][]byte, total)
	for i := 0; i < total; i++ {
		keys[i] = fmt.Sprintf("private/owned-object-%03d", i)
		bodies[i] = []byte(fmt.Sprintf("owned immutable content %d", i))
		repairM993Put(t, publisher, keys[i], bodies[i])
	}
	repairM993Update(t, keeper, func(state *reconciliation.State) {
		for i, key := range keys {
			state.Objects[key] = reconciliation.Lineage{Hash: fmt.Sprintf("%x", sha256.Sum256(bodies[i])), Kind: "original"}
		}
	})
	report, err := storage.ReconcileInventory(ctx, keeper, client, "media-fixture", "private/", 3)
	if err != nil || report.Checked != 3 || !report.Truncated || len(report.Orphans) != 0 {
		t.Fatalf("first bounded pass: %+v err=%v", report, err)
	}
	firstCursor := keeper.Snapshot().Cursors
	if len(firstCursor) != 1 {
		t.Fatalf("missing cursor: %v", firstCursor)
	}
	for _, token := range firstCursor {
		if token == "" {
			t.Fatal("truncated traversal reset cursor")
		}
	}
	if err = keeper.Close(); err != nil {
		t.Fatal(err)
	}
	// Mutate an object beyond the first provider page while the consumer is stopped.
	repairM993Put(t, publisher, keys[6], []byte("provider object was mutated"))
	keeper = repairM993Store(t, path)
	report, err = storage.ReconcileInventory(ctx, keeper, client, "media-fixture", "private/", 3)
	if err != nil || report.Checked != 3 || !report.Truncated {
		t.Fatalf("resumed pass: %+v err=%v", report, err)
	}
	secondCursor := keeper.Snapshot().Cursors
	if reflect.DeepEqual(firstCursor, secondCursor) {
		t.Fatal("cursor did not advance after restart")
	}
	if err = keeper.Close(); err != nil {
		t.Fatal(err)
	}
	keeper = repairM993Store(t, path)
	report, err = storage.ReconcileInventory(ctx, keeper, client, "media-fixture", "private/", 3)
	if !errors.Is(err, storage.ErrConflict) || report.Checked != 1 {
		t.Fatalf("tail mutation escaped detection: %+v err=%v", report, err)
	}
	if !reflect.DeepEqual(secondCursor, keeper.Snapshot().Cursors) {
		t.Fatal("failed page advanced durable continuation")
	}
	repairM993Put(t, publisher, keys[6], bodies[6])
	report, err = storage.ReconcileInventory(ctx, keeper, client, "media-fixture", "private/", 3)
	if err != nil || report.Checked != 1 || report.Truncated {
		t.Fatalf("complete traversal: %+v err=%v", report, err)
	}
	for _, token := range keeper.Snapshot().Cursors {
		if token != "" {
			t.Fatal("complete traversal did not reset continuation")
		}
	}
	if err = keeper.Close(); err != nil {
		t.Fatal(err)
	}
	keeper = repairM993Store(t, path)
	defer keeper.Close()
	report, err = storage.ReconcileInventory(ctx, keeper, client, "media-fixture", "private/", 3)
	if err != nil || report.Checked != 3 || !report.Truncated {
		t.Fatalf("new traversal: %+v err=%v", report, err)
	}
	lists, gets := objects.Requests()
	if len(lists) != 5 || lists[0].Token != "" || lists[1].Token == "" || lists[2].Token == "" || lists[2].Token != lists[3].Token || lists[4].Token != "" {
		t.Fatalf("provider continuation path: %+v", lists)
	}
	for _, request := range lists {
		if request.MaxKeys != 3 || request.Prefix != "private/" {
			t.Fatalf("unbounded/wrong provider request: %+v", request)
		}
	}
	want := append(append(append([]string{}, keys...), keys[6]), keys[:3]...)
	if !reflect.DeepEqual(gets, want) {
		t.Fatalf("producer -> list -> hash verification: got=%v want=%v", gets, want)
	}
}

func TestRepairM_993_inventory_multi_page_limit(t *testing.T) {
	objects, client, publisher := repairM993Objects(t)
	keeper := repairM993Store(t, filepath.Join(t.TempDir(), "inventory.json"))
	defer keeper.Close()
	for i := 0; i < 1003; i++ {
		repairM993Put(t, publisher, fmt.Sprintf("temporary/%04d", i), []byte("owned orphan"))
	}
	report, err := storage.ReconcileInventory(context.Background(), keeper, client, "media-fixture", "temporary/", 1001)
	if err != nil || report.Checked != 1001 || len(report.Orphans) != 1001 || !report.Truncated {
		t.Fatalf("multi-page bounded pass: %+v %v", report, err)
	}
	report, err = storage.ReconcileInventory(context.Background(), keeper, client, "media-fixture", "temporary/", 1001)
	if err != nil || report.Checked != 2 || report.Truncated || !reflect.DeepEqual(report.Orphans, []string{"temporary/1001", "temporary/1002"}) {
		t.Fatalf("provider tail was skipped: %+v %v", report, err)
	}
	lists, _ := objects.Requests()
	if len(lists) != 3 || lists[0].MaxKeys != 1000 || lists[1].MaxKeys != 1 || lists[2].MaxKeys != 1000 || lists[1].Token == "" || lists[2].Token == "" {
		t.Fatalf("remaining page budget: %+v", lists)
	}
}

func TestRepairM_993_inventory_cursor_scope_and_page_budget(t *testing.T) {
	objects, client, publisher := repairM993Objects(t)
	keeper := repairM993Store(t, filepath.Join(t.TempDir(), "inventory.json"))
	defer keeper.Close()
	// Unknown keys are actual listed orphans, never implicitly approved.
	for i := 0; i < 4; i++ {
		repairM993Put(t, publisher, fmt.Sprintf("temporary/%d", i), []byte("owned orphan"))
	}
	report, err := storage.ReconcileInventory(context.Background(), keeper, client, "media-fixture", "temporary/", 2)
	if err != nil || report.Checked != 2 || len(report.Orphans) != 2 || !report.Truncated {
		t.Fatalf("orphan pass: %+v %v", report, err)
	}
	report, err = storage.ReconcileInventory(context.Background(), keeper, client, "another-bucket", "temporary/", 2)
	if err != nil || report.Checked != 0 || report.Truncated {
		t.Fatalf("bucket isolation: %+v %v", report, err)
	}
	report, err = storage.ReconcileInventory(context.Background(), keeper, client, "media-fixture", "quarantine/", 2)
	if err != nil || report.Checked != 0 {
		t.Fatalf("prefix isolation: %+v %v", report, err)
	}
	report, err = storage.ReconcileInventory(context.Background(), keeper, client, "media-fixture", "temporary/", 2)
	if err != nil || report.Checked != 2 || report.Truncated || !reflect.DeepEqual(report.Orphans, []string{"temporary/2", "temporary/3"}) {
		t.Fatalf("tail/reset: %+v %v", report, err)
	}
	lists, _ := objects.Requests()
	if len(lists) != 4 || lists[1].Token != "" || lists[2].Token != "" || lists[3].Token == "" {
		t.Fatalf("scoped tokens: %+v", lists)
	}
}

func repairM993Scanner(t *testing.T, address string) (net.Listener, <-chan error) {
	t.Helper()
	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				done <- nil
				return
			}
			_ = conn.SetDeadline(time.Now().Add(time.Second))
			request, err := bufio.NewReader(conn).ReadString(0)
			if err == nil && request != "zVERSION\x00" {
				err = fmt.Errorf("unexpected scanner command %q", request)
			}
			if err == nil {
				_, err = io.WriteString(conn, "ClamAV 1.4.0/12345/owned transcript\x00")
			}
			_ = conn.Close()
			if err != nil {
				done <- err
				return
			}
		}
	}()
	return listener, done
}

func repairM993Await(t *testing.T, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for scanner observation")
}

func TestRepairM_993_media_observer_unwired(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	keeper := repairM993Store(t, filepath.Join(t.TempDir(), "health.json"))
	defer keeper.Close()
	_, _, publisher := repairM993Objects(t)
	body := []byte("owned approved image bytes")
	hash := fmt.Sprintf("%x", sha256.Sum256(body))
	key := "approved/" + hash + "/256"
	repairM993Put(t, publisher, key, body)
	repairM993Update(t, keeper, func(state *reconciliation.State) {
		state.Objects[key] = reconciliation.Lineage{Hash: hash, Kind: "variant"}
		state.Restrictions[hash] = reconciliation.Restriction{Revision: 1, State: "approved"}
		state.QuarantinedAt = time.Now().Add(-2 * time.Minute)
		state.Operations["pending-purge"] = storage.Operation{ID: uuid.NewString(), Kind: "purge", State: "requested", RequestedAt: time.Now().Add(-time.Minute)}
	})
	observer := storage.NewMediaObserver()
	reader := serve.NewApproved(serve.Deps{Buckets: map[string]serve.Reader{"approved": publisher}, Observer: observer}, keeper)
	router := chi.NewRouter()
	reader.RegisterRoutes(router)
	read := func() {
		t.Helper()
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/media/approved/"+key, nil))
		if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), body) {
			t.Fatalf("permitted read: status=%d body=%q", response.Code, response.Body.Bytes())
		}
	}
	listener, scannerDone := repairM993Scanner(t, "127.0.0.1:0")
	defer listener.Close()
	options := sharedhttp.ScanOptions{Host: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port, Timeout: time.Second}
	probeDone := make(chan error, 1)
	go func() { probeDone <- observer.RunScannerProbes(ctx, options, 10*time.Millisecond) }()
	defer func() {
		cancel()
		select {
		case err := <-probeDone:
			if !errors.Is(err, context.Canceled) {
				t.Errorf("probe shutdown: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Error("probe loop did not stop")
		}
	}()
	repairM993Await(t, func() bool {
		h, err := observer.HealthFrom(ctx, keeper, time.Now(), true)
		return err == nil && h.WriteReady && h.ReadReady && h.ScannerUnavailableSince.IsZero()
	})
	read()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-scannerDone; err != nil {
		t.Fatal(err)
	}
	repairM993Await(t, func() bool {
		h, err := observer.HealthFrom(ctx, keeper, time.Now(), true)
		return err == nil && !h.WriteReady && h.ReadReady && !h.ScannerUnavailableSince.IsZero()
	})
	health, err := observer.HealthFrom(ctx, keeper, time.Now().Add(61*time.Second), true)
	if err != nil || !health.ScanAlert || health.WriteReady || !health.ReadReady || !health.PurgeAlert || health.QuarantineOldestSeconds < 120 {
		t.Fatalf("actual scanner outage health: %+v err=%v", health, err)
	}
	read()
	snapshot := observer.Snapshot()
	if snapshot["scanCount"].(uint64) < 2 || snapshot["originHitRatio"].(float64) != 1 {
		t.Fatalf("runtime measurements: %+v", snapshot)
	}
	failure := health.ScannerUnavailableSince
	repairM993Await(t, func() bool { return observer.Snapshot()["scanCount"].(uint64) >= snapshot["scanCount"].(uint64)+2 })
	health, err = observer.HealthFrom(ctx, keeper, time.Now().Add(62*time.Second), true)
	if err != nil || !health.ScannerUnavailableSince.Equal(failure) || !health.ScanAlert {
		t.Fatalf("outage timeline reset: %+v %v", health, err)
	}
	health, err = observer.HealthFrom(ctx, keeper, time.Now(), false)
	if err != nil || !health.WriteReady || !health.ReadReady || health.ScanAlert {
		t.Fatalf("disabled upload readiness: %+v %v", health, err)
	}
	recoveredScanner, recoveredDone := repairM993Scanner(t, listener.Addr().String())
	defer func() {
		_ = recoveredScanner.Close()
		if err := <-recoveredDone; err != nil {
			t.Error(err)
		}
	}()
	repairM993Await(t, func() bool {
		h, err := observer.HealthFrom(ctx, keeper, time.Now(), true)
		return err == nil && h.WriteReady && h.ReadReady && !h.ScanAlert && h.ScannerUnavailableSince.IsZero()
	})
	read()
	// Storage authority failure still denies a read regardless of scanner status.
	repairM993Update(t, keeper, func(state *reconciliation.State) {
		state.Restrictions[hash] = reconciliation.Restriction{Revision: 2, State: "blocked"}
	})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/media/approved/"+key, nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("scanner independence bypassed restrictions: %d", response.Code)
	}
}
