package search

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const initialGenerationDigest = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

func firstGenerationDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("L20_TEST_DSN")
	if dsn == "" {
		t.Fatal("L20_TEST_DSN required for first search generation qualification")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	database := "first_search_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{database}.Sanitize()); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		admin.Close()
		t.Fatal(err)
	}
	cfg.ConnConfig.Database = database
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		if _, err := admin.Exec(context.Background(), "DROP DATABASE "+pgx.Identifier{database}.Sanitize()+" WITH (FORCE)"); err != nil {
			t.Error(err)
		}
		admin.Close()
	})
	if _, err = pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS pg_trgm; CREATE SCHEMA stats_search"); err != nil {
		t.Fatal(err)
	}
	applyFirstGenerationMigration(t, pool, "200001.sql")
	return pool
}

func applyFirstGenerationMigration(t *testing.T, pool *pgxpool.Pool, name string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "migrations", name))
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(context.Background(), string(raw)); err != nil {
		_ = tx.Rollback(context.Background())
		t.Fatal(err)
	}
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestFirstSearchGenerationMigrationSeedsEmptySearch(t *testing.T) {
	pool := firstGenerationDatabase(t)
	applyFirstGenerationMigration(t, pool, "200002.sql")
	ctx := context.Background()
	var generations, active int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM stats_search.generations_v2").Scan(&generations); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM stats_search.active_v2").Scan(&active); err != nil {
		t.Fatal(err)
	}
	if generations != 1 || active != 1 {
		t.Fatalf("first search generation rows: generations=%d active=%d", generations, active)
	}
	var generation, state, digest string
	var rows, watermark, privacy int64
	if err := pool.QueryRow(ctx, "SELECT generation,state,source_watermark,row_count,digest FROM stats_search.generations_v2").Scan(&generation, &state, &watermark, &rows, &digest); err != nil {
		t.Fatal(err)
	}
	if generation != "initial" || state != "active" || watermark != 1 || rows != 0 || digest != initialGenerationDigest {
		t.Fatalf("initial generation: generation=%s state=%s watermark=%d rows=%d digest=%s", generation, state, watermark, rows, digest)
	}
	if err := pool.QueryRow(ctx, "SELECT generation,privacy_revision FROM stats_search.active_v2 WHERE singleton").Scan(&generation, &privacy); err != nil {
		t.Fatal(err)
	}
	if generation != "initial" || privacy != 1 {
		t.Fatalf("initial active pointer: generation=%s privacy_revision=%d", generation, privacy)
	}
	applyFirstGenerationMigration(t, pool, "200002.sql")
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM stats_search.generations_v2").Scan(&generations); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM stats_search.active_v2").Scan(&active); err != nil {
		t.Fatal(err)
	}
	if generations != 1 || active != 1 {
		t.Fatalf("reapplied first search generation rows: generations=%d active=%d", generations, active)
	}
}

func TestFirstSearchGenerationMigrationPreservesExistingRows(t *testing.T) {
	pool := firstGenerationDatabase(t)
	ctx := context.Background()
	observed := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := pool.Exec(ctx, "INSERT INTO stats_search.generations_v2(generation,state,source_watermark,source_observed_at) VALUES('case_generation','active',7,$1)", observed); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO stats_search.active_v2(singleton,generation,revision,privacy_revision) VALUES(true,'case_generation',nextval('stats_search.revision_v2'),3)"); err != nil {
		t.Fatal(err)
	}
	applyFirstGenerationMigration(t, pool, "200002.sql")
	var generation string
	var generations, active int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM stats_search.generations_v2").Scan(&generations); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM stats_search.active_v2").Scan(&active); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT generation FROM stats_search.active_v2 WHERE singleton").Scan(&generation); err != nil {
		t.Fatal(err)
	}
	if generations != 1 || active != 1 || generation != "case_generation" {
		t.Fatalf("existing search rows changed: generations=%d active=%d generation=%s", generations, active, generation)
	}
}

func TestFirstSearchGenerationRuntimeReadinessAndActivation(t *testing.T) {
	database := coordinatortest.Bootstrap(t, os.Getenv("COORDINATOR_TEST_POSTGRES_DSN"), "stats_runtime")
	runtime := database.Runtimes["stats_runtime"]
	pool, err := pgxpool.New(context.Background(), runtime.DSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	want, err := production.LoadExpectedSchema(runtime.ExpectedSchema, runtime.ExpectedSchemaSHA256)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := production.CheckReadiness(ctx, pool, want); err != nil {
		t.Fatalf("stats runtime readiness: %v", err)
	}
	store := Store{Pool: pool}
	if err := store.Ready(ctx); err != nil {
		t.Fatalf("search readiness: %v", err)
	}
	observed := time.Now().UTC().Truncate(time.Microsecond)
	if err := store.BeginGeneration(ctx, "first_search_runtime", 1, observed); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	rows, digest, err := GenerationDigest(ctx, tx, "first_search_runtime")
	if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
		t.Fatal(rollbackErr)
	}
	if err != nil {
		t.Fatal(err)
	}
	if rows != 0 || digest != initialGenerationDigest {
		t.Fatalf("empty runtime generation digest: rows=%d digest=%s", rows, digest)
	}
	if err := store.ActivateGeneration(ctx, GenerationProof{Generation: "first_search_runtime", SourceWatermark: 1, SourceObservedAt: observed, Rows: rows, Digest: digest}); err != nil {
		t.Fatal(err)
	}
	var state string
	if err := pool.QueryRow(ctx, "SELECT state FROM stats_search.generations_v2 WHERE generation='initial'").Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != "retained" {
		t.Fatalf("initial generation state=%s, want retained", state)
	}
	if err := store.Ready(ctx); err != nil {
		t.Fatalf("search readiness after activation: %v", err)
	}
}
