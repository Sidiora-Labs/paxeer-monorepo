package migrate_test

import (
	"context"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/migrate"
)

func TestUpAppliesEmbeddedSchemaAndIsIdempotent(t *testing.T) {
	dsn, pool := internaltest.NewPostgresWithDSN(t)
	for range 2 {
		if err := migrate.Up(context.Background(), dsn); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM stats.pool_stats").Scan(&count); err != nil || count != 0 {
		t.Fatalf("migrated pool stats: count=%d err=%v", count, err)
	}
}
