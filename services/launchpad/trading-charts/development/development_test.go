package development

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/redis/go-redis/v9"
)

func TestDevelopmentSourceAndMigrationUseOwnedResources(t *testing.T) {
	ctx := context.Background()
	dsn := internaltest.NewDatabase(t)
	if err := Migrate(ctx, dsn); err != nil {
		t.Fatal(err)
	}
	pool := internaltest.OpenPool(t, dsn)
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass('candles.candles') IS NOT NULL`).Scan(&exists); err != nil || !exists {
		t.Fatalf("development candle schema: %t, %v", exists, err)
	}
	options, err := redis.ParseURL(internaltest.NewRedisURL(t))
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(options)
	t.Cleanup(func() { _ = client.Close() })
	source := NewSource(pool, client, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if source == nil {
		t.Fatal("development chart source was not constructed")
	}
	if count, err := source.Project(ctx); count != 0 || err == nil || !strings.Contains(err.Error(), "development source RPC required") {
		t.Fatalf("development source without chain authority: %d, %v", count, err)
	}
}
