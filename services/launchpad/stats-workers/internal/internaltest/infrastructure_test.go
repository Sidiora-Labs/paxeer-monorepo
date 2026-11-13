package internaltest

import (
	"context"
	"os"
	"testing"
)

func TestInfrastructureProvidesMigratedDatabaseAndLiveCache(t *testing.T) {
	ctx := context.Background()
	dsn, pool := NewPostgresWithDSN(t)
	if dsn == "" {
		t.Fatal("empty database connection string")
	}
	var exists bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('stats.pool_stats') IS NOT NULL").Scan(&exists); err != nil || !exists {
		t.Fatalf("stats migration: exists=%v err=%v", exists, err)
	}
	EnsureMetadataSchema(t, pool)
	if err := pool.QueryRow(ctx, "SELECT to_regclass('metadata.token_metadata') IS NOT NULL").Scan(&exists); err != nil || !exists {
		t.Fatalf("metadata table: exists=%v err=%v", exists, err)
	}
	if got := NewRedisURL(t); got == "" {
		t.Fatal("empty cache URL")
	}
	if err := NewRedis(t).Ping(ctx).Err(); err != nil {
		t.Fatalf("cache ping: %v", err)
	}
	brokerURL := os.Getenv("L13_PHASE3_NATS_URL")
	if brokerURL == "" {
		t.Fatal("L13_PHASE3_NATS_URL is required")
	}
	if conn := ConnectNATS(t, brokerURL); !conn.IsConnected() {
		t.Fatal("broker connection is not active")
	}
}
