package internaltest

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestEmptyUniverseRuntimeDatabase(t *testing.T) {
	database := NewRankingRuntimeDatabase(t)
	ctx := context.Background()
	runtime, err := pgx.Connect(ctx, database.RuntimeDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := runtime.Close(ctx); err != nil {
			t.Error(err)
		}
	}()
	for _, relation := range []string{"indexer.indexer_canonical_creations_v1", "stats.accounting_market_commits_v2", "ranking.category_heads"} {
		var count int64
		if err = runtime.QueryRow(ctx, "SELECT count(*) FROM "+relation).Scan(&count); err != nil {
			t.Fatalf("empty runtime relation %s: %v", relation, err)
		}
		if count != 0 {
			t.Fatalf("initial %s count = %d, want zero", relation, count)
		}
	}
	var canCreate bool
	if err = runtime.QueryRow(ctx, "SELECT has_schema_privilege(current_user,'ranking','CREATE')").Scan(&canCreate); err != nil {
		t.Fatal(err)
	}
	if canCreate {
		t.Fatal("ranking runtime may create schema objects")
	}
}
