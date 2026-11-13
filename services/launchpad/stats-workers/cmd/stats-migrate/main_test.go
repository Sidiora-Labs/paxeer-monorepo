package main

import (
	"context"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
)

func TestMainAppliesDatabaseMigrations(t *testing.T) {
	dsn, pool := internaltest.NewPostgresWithDSN(t)
	t.Setenv("DATABASE_URL", dsn)
	main()
	var exists bool
	if err := pool.QueryRow(context.Background(), "SELECT to_regclass('stats.pool_stats') IS NOT NULL").Scan(&exists); err != nil || !exists {
		t.Fatalf("stats table: exists=%v err=%v", exists, err)
	}
}
