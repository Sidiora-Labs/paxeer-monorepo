package internaltest

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/jackc/pgx/v5"
)

func TestRankingRuntimeDatabaseProvidesReadRole(t *testing.T) {
	db := NewRankingRuntimeDatabase(t)
	if db.Database == "" || db.RuntimeRole == "" || db.Epoch != 1 || db.Region != RuntimeRegion {
		t.Fatalf("runtime identity incomplete: database=%q role=%q epoch=%d region=%q", db.Database, db.RuntimeRole, db.Epoch, db.Region)
	}
	var expected production.ExpectedSchema
	if err := json.Unmarshal(db.ExpectedSchema, &expected); err != nil {
		t.Fatal(err)
	}
	if expected.Service != rankingService || expected.RuntimeRole != db.RuntimeRole || len(expected.Migrations) == 0 {
		t.Fatalf("runtime schema identity = %q, %q, %d migrations", expected.Service, expected.RuntimeRole, len(expected.Migrations))
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, db.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := admin.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	var epoch int64
	var region string
	if err := admin.QueryRow(ctx, "SELECT epoch, region FROM recovery.recovery_control WHERE singleton").Scan(&epoch, &region); err != nil {
		t.Fatal(err)
	}
	if epoch != db.Epoch || region != db.Region {
		t.Fatalf("runtime recovery state = %d, %q", epoch, region)
	}
	conn, err := pgx.Connect(ctx, db.RuntimeDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := conn.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	if err := conn.QueryRow(ctx, "SELECT epoch FROM recovery.recovery_control WHERE singleton").Scan(&epoch); err == nil {
		t.Fatal("read role accessed recovery state")
	}
	if _, err := conn.Exec(ctx, "UPDATE recovery.recovery_control SET epoch=epoch+1 WHERE singleton"); err == nil {
		t.Fatal("read role changed recovery state")
	}
}

func TestReadSchemaAndRedisFixtures(t *testing.T) {
	root := RepositoryRoot(t)
	if _, err := os.Stat(filepath.Join(root, "migrations/registry.json")); err != nil {
		t.Fatal(err)
	}
	if raw := readBound(t, root, "migrations/registry.json", ""); len(raw) == 0 {
		t.Fatal("migration registry is empty")
	}
	if raw := readBound(t, root, "core/ranking-algo/go.mod", ""); !strings.Contains(string(raw), "module github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo") {
		t.Fatal("ranking module identity missing")
	}
	dsn, pool := NewPostgresWithDSN(t)
	if !strings.Contains(dsn, "ranking_test_") {
		t.Fatal("read schema is outside its private database")
	}
	var tables int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM information_schema.tables WHERE table_schema IN ('stats','indexer') AND table_name IN ('pool_stats','pools')").Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if tables != 2 {
		t.Fatalf("read schema has %d ranking tables, want 2", tables)
	}
	second := NewPostgres(t)
	if err := second.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	rdb := NewRedis(t)
	ctx := context.Background()
	if err := rdb.Set(ctx, "ranking:fixture", "ready", 0).Err(); err != nil {
		t.Fatal(err)
	}
	if value, err := rdb.Get(ctx, "ranking:fixture").Result(); err != nil || value != "ready" {
		t.Fatalf("ranking cache value = %q, %v", value, err)
	}
}
