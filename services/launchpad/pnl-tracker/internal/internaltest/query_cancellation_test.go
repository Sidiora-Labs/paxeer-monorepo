package internaltest

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestQueryCancellationPreservesOtherQueries(t *testing.T) {
	ctx := context.Background()
	source := NewPostgres(t)
	config := source.Config()
	trace := new(QueryCancellation)
	config.ConnConfig.Tracer = trace
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	trace.Start(2)
	var one int
	if err = pool.QueryRow(ctx, "SELECT 1").Scan(&one); err != nil || one != 1 {
		t.Fatal(one, err)
	}
	if _, err = pool.Exec(ctx, "SELECT 2"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	queries := trace.Stop()
	if len(queries) != 2 || queries[0] != "SELECT 1" || queries[1] != "SELECT 2" {
		t.Fatal(queries)
	}
	if err = pool.QueryRow(ctx, "SELECT 3").Scan(&one); err != nil || one != 3 {
		t.Fatal(one, err)
	}
	trace.Start(0)
	if err = pool.QueryRow(ctx, "SELECT 4").Scan(&one); err != nil || one != 4 {
		t.Fatal(one, err)
	}
	if queries = trace.Stop(); len(queries) != 1 || queries[0] != "SELECT 4" {
		t.Fatal(queries)
	}
}
