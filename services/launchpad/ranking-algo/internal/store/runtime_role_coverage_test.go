package store

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/queue"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRuntimeRolePrune(t *testing.T) {
	const coordinatorDSNEnv = "COORDINATOR_TEST_POSTGRES_DSN"
	admin := os.Getenv(coordinatorDSNEnv)
	if admin == "" {
		t.Skipf("%s is not set", coordinatorDSNEnv)
	}
	database := coordinatortest.Bootstrap(t, admin, "ranking_runtime")
	runtime := database.Runtimes["ranking_runtime"]
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, runtime.DSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var tableUpdate, lockColumnUpdate bool
	if err = pool.QueryRow(ctx, `SELECT has_table_privilege(current_user,'ranking.snapshots','UPDATE'),has_column_privilege(current_user,'ranking.snapshots','completed_at','UPDATE')`).Scan(&tableUpdate, &lockColumnUpdate); err != nil || tableUpdate || !lockColumnUpdate {
		t.Fatalf("snapshot lock privileges: table=%v column=%v: %v", tableUpdate, lockColumnUpdate, err)
	}
	guard, err := queue.NewEpochGuard("recovery", coordinatortest.Epoch, coordinatortest.Region)
	if err != nil {
		t.Fatal(err)
	}
	publication, err := NewPublicationStore(pool, "ranking", guard)
	if err != nil {
		t.Fatal(err)
	}
	runID := "runtime-prune-" + strings.Repeat("a", 8)
	if _, err = pool.Exec(ctx, `INSERT INTO ranking.snapshots(run_id,category,input_digest,output_digest,artifact,public_header) VALUES($1,'trending',repeat('a',64),repeat('b',64),'{}','{"cutoffTime":"1"}')`, runID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO ranking.publication_outbox(run_id,category,artifact,published_at) VALUES($1,'trending','{}',clock_timestamp())`, runID); err != nil {
		t.Fatal(err)
	}
	pruned, err := publication.Prune(ctx, time.Unix(10_000, 0), 1)
	if err != nil || pruned != 1 {
		t.Fatalf("runtime prune = %d, %v", pruned, err)
	}
	var remaining int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM ranking.snapshots WHERE run_id=$1`, runID).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("pruned snapshot count = %d, %v", remaining, err)
	}
	pruned, err = publication.Prune(ctx, time.Unix(10_000, 0), 1)
	if err != nil || pruned != 0 {
		t.Fatalf("second runtime prune = %d, %v", pruned, err)
	}
}
