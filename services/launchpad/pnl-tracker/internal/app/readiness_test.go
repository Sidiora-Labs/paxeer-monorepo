package app

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/config"
	sharedconfig "github.com/Sidiora-Technologies/KindleLaunch/shared/config"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestDatabaseReadinessSurvivesProbeCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db := coordinatortest.Bootstrap(t, os.Getenv("PNL_TEST_POSTGRES_DSN"), "indexer_runtime", "pnl_runtime")
	admin, err := pgxpool.New(ctx, db.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	options, err := pgxpool.ParseConfig(db.Runtimes["pnl_runtime"].DSN)
	if err != nil {
		t.Fatal(err)
	}
	options.ConnConfig.RuntimeParams["jit"] = "off"
	pool, err := pgxpool.NewWithConfig(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	cfg := config.Config{BaseEnv: sharedconfig.BaseEnv{NodeEnv: "production"}}
	cfg.ExpectedSchemaFile, cfg.ExpectedSchemaSHA256 = db.ExpectedSchemaFile(t, "pnl_runtime")
	var calls atomic.Int32
	service, stop := context.WithCancel(ctx)
	defer stop()
	check := serviceReadiness(service, func(c context.Context) error {
		calls.Add(1)
		return cfg.CheckRuntimeSchema(c, pool)
	})
	lock := func() pgx.Tx {
		t.Helper()
		tx, err := admin.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
		if _, err := tx.Exec(ctx, "LOCK TABLE migration_control.runtime_readiness IN ACCESS EXCLUSIVE MODE"); err != nil {
			t.Fatal(err)
		}
		return tx
	}
	waitForCheck := func() {
		t.Helper()
		bounded, stop := context.WithTimeout(ctx, time.Second)
		defer stop()
		for {
			var waiting bool
			err := admin.QueryRow(bounded, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND cardinality(pg_blocking_pids(pid))>0)`).Scan(&waiting)
			if err != nil {
				t.Fatal(err)
			}
			if waiting {
				return
			}
			select {
			case <-bounded.Done():
				t.Fatal("schema verification did not reach the deployed attestation")
			case <-time.After(time.Millisecond):
			}
		}
	}
	probe, stopProbe := context.WithCancel(ctx)
	stopProbe()
	if err := check(probe); !errors.Is(err, context.Canceled) || calls.Load() != 0 {
		t.Fatalf("canceled probe started verification: err=%v checks=%d", err, calls.Load())
	}
	blocked := lock()
	probe, stopProbe = context.WithCancel(ctx)
	first := make(chan error, 1)
	go func() { first <- check(probe) }()
	waitForCheck()
	stopProbe()
	if err := <-first; !errors.Is(err, context.Canceled) {
		t.Fatalf("disconnected probe = %v", err)
	}
	if acquired := pool.Stat().AcquiredConns(); acquired != 1 {
		t.Fatalf("disconnected probe interrupted schema verification: acquired=%d", acquired)
	}
	second := make(chan error, 1)
	go func() { second <- check(ctx) }()
	if err := blocked.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-second; err != nil {
		t.Fatalf("subsequent probe rejected the deployed schema: %v", err)
	}
	completed := calls.Load()
	if _, err := admin.Exec(ctx, "ALTER TABLE pnl.l18_requests ALTER COLUMN response TYPE text USING response::text"); err != nil {
		t.Fatal(err)
	}
	if err := check(ctx); err == nil || !strings.Contains(err.Error(), "structural fingerprint mismatch") {
		t.Fatalf("subsequent probe reused readiness across schema changes: %v", err)
	}
	if calls.Load() != completed+1 {
		t.Fatalf("completed schema verification was cached: checks=%d", calls.Load())
	}
	blocked = lock()
	done := make(chan error, 1)
	go func() { done <- check(ctx) }()
	waitForCheck()
	stop()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("service shutdown did not cancel schema verification: %v", err)
	}
	if err := blocked.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
}
