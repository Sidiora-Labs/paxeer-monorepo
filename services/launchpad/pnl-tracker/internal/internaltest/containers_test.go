package internaltest

import (
	"context"
	"testing"
)

func TestFixtureSchemasAndCache(t *testing.T) {
	ctx := context.Background()
	pool := NewPostgres(t)
	EnsureIndexerSchema(t, pool)
	EnsureStatsSchema(t, pool)
	EnsureMetadataSchema(t, pool)
	for _, table := range []string{"indexer.pools", "indexer.swaps", "stats.pool_stats", "metadata.token_metadata"} {
		var present bool
		if err := pool.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", table).Scan(&present); err != nil {
			t.Fatal(err)
		}
		if !present {
			t.Fatalf("table %s is absent", table)
		}
	}
	rdb := NewRedis(t)
	if err := rdb.Set(ctx, "pnl:fixture", "ready", 0).Err(); err != nil {
		t.Fatal(err)
	}
	if got, err := rdb.Get(ctx, "pnl:fixture").Result(); err != nil || got != "ready" {
		t.Fatalf("cache value = %q, error = %v", got, err)
	}
}
