package storage

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/internaltest"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestDurableMediaLedgerBindsOperationsAndVisibility(t *testing.T) {
	rdb, _ := internaltest.NewRedis(t)
	t.Cleanup(func() { _ = rdb.Close() })
	if _, err := NewLedger(nil, "owned"); err == nil {
		t.Fatal("ledger without durable Redis was admitted")
	}
	ledger, err := NewLedger(rdb, "gateway-"+uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApprovedKey(nil, "256"); err == nil {
		t.Fatal("empty media bytes received an approved key")
	}
	key, err := ApprovedKey([]byte("owned fixture bytes"), "256")
	if err != nil || !ValidApprovedKey(key) {
		t.Fatalf("approved key=%q err=%v", key, err)
	}
	hash := strings.Split(key, "/")[1]
	ctx := context.Background()
	if _, err := ledger.Begin(ctx, "invalid operation", hash, uploadKind); err == nil {
		t.Fatal("invalid operation identity was admitted")
	}
	if state, err := ledger.Begin(ctx, "owned-operation", hash, uploadKind); err != nil || state != requestedState {
		t.Fatalf("durable operation state=%q err=%v", state, err)
	}
	if winner, err := ledger.ClaimExecution(ctx, "owned-operation", "worker-a"); err != nil || !winner {
		t.Fatalf("first execution claim: winner=%v err=%v", winner, err)
	}
	if winner, err := ledger.ClaimExecution(ctx, "owned-operation", "worker-b"); err != nil || winner {
		t.Fatalf("second execution claim: winner=%v err=%v", winner, err)
	}
	if err := ledger.Advance(ctx, "owned-operation", requestedState, acceptedState, "provider-accepted"); err != nil {
		t.Fatalf("provider acknowledgement: %v", err)
	}
	if err := ledger.Advance(ctx, "owned-operation", acceptedState, completeState, "provider-complete"); err != nil {
		t.Fatalf("provider completion: %v", err)
	}
	if err := ledger.Advance(ctx, "owned-operation", requestedState, completeState, ""); !errors.Is(err, ErrConflict) {
		t.Fatalf("unproven completion: %v", err)
	}
	if err := ledger.Restrict(ctx, hash, 1, approvedState); err != nil {
		t.Fatalf("approved restriction: %v", err)
	}
	if err := ledger.Allowed(ctx, key); err != nil {
		t.Fatalf("approved delivery: %v", err)
	}
	if err := ledger.AuthorizeDownload(ctx, key, time.Minute); err != nil {
		t.Fatalf("approved signed delivery: %v", err)
	}
	if err := ledger.Restrict(ctx, hash, 2, blockedState); err != nil {
		t.Fatalf("blocked restriction: %v", err)
	}
	if err := ledger.Allowed(ctx, key); !errors.Is(err, ErrRestricted) {
		t.Fatalf("blocked delivery: %v", err)
	}
	if err := ledger.Restrict(ctx, hash, 1, approvedState); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale approval: %v", err)
	}
	if _, err := ledger.ClaimExecution(ctx, "invalid operation", "worker-a"); !errors.Is(err, ErrConflict) {
		t.Fatalf("invalid execution claim: %v", err)
	}
}

func TestConnectedAuthorityReportsUnavailableDatabase(t *testing.T) {
	dsn := os.Getenv("L41_POSTGRES_URL")
	if dsn == "" {
		t.Fatal("gateway database fixture is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	pool.Close()
	a := &Authority{Pool: pool, Schema: "gateway", MetadataSchema: "metadata", SafetySchema: "chat", productionViews: true}
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte("owned fixture bytes")))
	key := "approved/" + hash + "/256/" + hash + ".png"
	if err := a.Allowed(ctx, key); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("closed database delivery: %v", err)
	}
	if _, err := a.Restrict(ctx, hash, 2, blockedState); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("closed database restriction: %v", err)
	}
	if _, err := a.Operation(ctx, uuid.NewString()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("closed database operation: %v", err)
	}
	if err := a.ImportApprovedLineage(ctx, uuid.NewString()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("closed database lineage import: %v", err)
	}
	if err := a.RegisterLineage(ctx, LineageObject{Hash: hash, Key: "private/" + hash, Kind: originalKind, RetainUntil: time.Now().Add(31 * 24 * time.Hour)}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("closed database lineage registration: %v", err)
	}
	if count, err := a.ImportApprovedBatch(ctx); count != 0 || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("closed database approved intake: count=%d err=%v", count, err)
	}
	if count, err := a.IntakeDeletedAssets(ctx); count != 0 || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("closed database deletion intake: count=%d err=%v", count, err)
	}
	if _, err := a.PurgeBacklogAge(ctx); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("closed database purge backlog: %v", err)
	}
	if err := a.RetryPurge(ctx, uuid.NewString()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("closed database purge retry: %v", err)
	}
	if _, _, err := a.MediaBacklogAges(ctx); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("closed database health: %v", err)
	}
	if err := a.PersistInventory(ctx, "approved", approvedPrefix, InventoryReport{GeneratedAt: time.Now().UTC()}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("closed database inventory record: %v", err)
	}
	provider, err := NewHTTPPurgeProvider("https://localhost", strings.Repeat("p", 32), nil)
	if err != nil {
		t.Fatal(err)
	}
	if worked, err := (PurgeWorker{Authority: a, Provider: provider}).RunOne(ctx); worked || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("closed database purge worker: worked=%v err=%v", worked, err)
	}
	observer := NewMediaObserver()
	if health, err := observer.Health(ctx, a, time.Now().UTC(), true); !errors.Is(err, ErrUnavailable) || health.ReadReady {
		t.Fatalf("closed database health decision: health=%+v err=%v", health, err)
	}
}

func TestMediaObserverRejectsInvalidMeasurements(t *testing.T) {
	observer := NewMediaObserver()
	if err := observer.RecordUpload("unknown"); !errors.Is(err, ErrConflict) {
		t.Fatalf("unknown upload outcome: %v", err)
	}
	if err := observer.RunScannerProbes(context.Background(), sharedhttp.ScanOptions{}, 0); !errors.Is(err, ErrConflict) {
		t.Fatalf("unbounded scanner interval: %v", err)
	}
	accumulator := NewEnvelopeAccumulator()
	if err := accumulator.Observe("unknown", time.Millisecond, http.StatusOK, 1, true); err == nil {
		t.Fatal("unknown measured request class was admitted")
	}
	if err := accumulator.Observe(originServiceName, -time.Millisecond, http.StatusOK, 1, true); !errors.Is(err, ErrConflict) {
		t.Fatalf("negative request latency: %v", err)
	}
	if quantile := envelopeQuantile(nil, 0, 95); quantile != -1 {
		t.Fatalf("empty latency quantile = %d", quantile)
	}
}

func TestPurgeProviderRejectsUnboundOperations(t *testing.T) {
	provider, err := NewHTTPPurgeProvider("https://localhost", strings.Repeat("p", 32), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Submit(context.Background(), "invalid operation", strings.Repeat("a", 64)); !errors.Is(err, ErrConflict) {
		t.Fatalf("invalid purge submission: %v", err)
	}
	if _, err := provider.Status(context.Background(), "owned-operation", "invalid provider"); !errors.Is(err, ErrConflict) {
		t.Fatalf("invalid purge status: %v", err)
	}
	if worked, err := (PurgeWorker{}).RunOne(context.Background()); worked || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unbound purge worker: worked=%v err=%v", worked, err)
	}
}
