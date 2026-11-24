package delivery

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestMeasuredRunwayFailsUnavailableInputs(t *testing.T) {
	if v, err := DiskRunwaySeconds(100, 1000, 100, 50, 20); err != nil || v != 30 {
		t.Fatal(v, err)
	}
	for _, rates := range [][2]int64{{0, 0}, {20, 20}, {10, 20}, {20, -1}} {
		if _, err := DiskRunwaySeconds(100, 1000, 100, rates[0], rates[1]); err == nil {
			t.Fatal("unavailable growth accepted", rates)
		}
	}
}

// This separately admitted integration test uses the existing private
// PostgreSQL/object-store fixture. The bounded repair gate uses loopback peers.
func TestReplayPostgresLifecycle(t *testing.T) {
	ctx, pool, archive, engine, _, digest := activeRebuildFixture(t)
	if err := engine.Start(ctx, digest); err != nil {
		t.Fatal(err)
	}
	rival, err := NewReplayEngine(archive, engine.consumer, engine.generation, "rival", activeRebuildProjection{archive}, verifyWebhookFixtureAdmin, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err = rival.Start(ctx, digest); !errors.Is(err, ErrLease) {
		t.Fatal("live replay lease stolen", err)
	}
	if _, err = pool.Exec(ctx, "UPDATE "+archive.store.table("delivery_rebuilds")+" SET lease_until=clock_timestamp()-interval '1 second'"); err != nil {
		t.Fatal(err)
	}
	if err = rival.Start(ctx, digest); err != nil {
		t.Fatal(err)
	}
	if err = engine.RunBlock(ctx); !errors.Is(err, ErrLease) {
		t.Fatal("stale worker applied archive", err)
	}
	activeRebuildRebuild(t, ctx, rival, digest)
	schema := strings.Trim(archive.store.schema, `"`)
	before := activeRebuildSnapshot(t, ctx, archive)
	// Stop the runtime connections and reopen after promotion; the fixture
	// keeps its separate housekeeping connection for schema cleanup.
	workerCtx, workerPool, _, worker := activeRebuildOpen(t, schema)
	activeRebuildRebuild(t, workerCtx, worker, digest)
	workerPool.Close()
	reopenedCtx, _, reopenedArchive, reopened := activeRebuildOpen(t, schema)
	activeRebuildRebuild(t, reopenedCtx, reopened, digest)
	if after := activeRebuildSnapshot(t, reopenedCtx, reopenedArchive); after != before {
		t.Fatal("duplicate startup changed persisted generation")
	}
}
