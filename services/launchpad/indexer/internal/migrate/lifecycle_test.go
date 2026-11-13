package migrate_test

import (
	"context"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/migrate"
	"testing"
)

func TestMigrationsCreateSchemaAndRemainIdempotent(t *testing.T) {
	dsn, pool := internaltest.NewPostgres(t)
	if err := migrate.Up(context.Background(), dsn); err != nil {
		t.Fatal(err)
	}
	var tables int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM pg_tables WHERE schemaname='indexer'`).Scan(&tables); err != nil || tables < 5 {
		t.Fatalf("migrated product tables %d %v", tables, err)
	}
	if err := migrate.Up(context.Background(), "invalid"); err == nil {
		t.Fatal("invalid database address accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := migrate.Up(ctx, dsn); err == nil {
		t.Fatal("cancelled migration accepted")
	}
}
