package store

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/ranker"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type databaseClockVisibilityReader struct {
	pool       *pgxpool.Pool
	calls      int
	waitSecond bool
	age        time.Duration
	observed   time.Time
}

func (r *databaseClockVisibilityReader) ReadRankingVisibility(ctx context.Context) (RankingVisibility, error) {
	r.calls++
	bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var now time.Time
	if err := r.pool.QueryRow(bounded, "SELECT clock_timestamp()").Scan(&now); err != nil {
		return RankingVisibility{}, err
	}
	if r.waitSecond {
		first := now.Unix()
		ticker := time.NewTicker(3 * time.Millisecond)
		defer ticker.Stop()
		for now.Unix() <= first {
			select {
			case <-bounded.Done():
				return RankingVisibility{}, bounded.Err()
			case <-ticker.C:
				if err := r.pool.QueryRow(bounded, "SELECT clock_timestamp()").Scan(&now); err != nil {
					return RankingVisibility{}, err
				}
			}
		}
	}
	r.observed = now.Add(-r.age)
	return RankingVisibility{Revision: strings.Repeat("a", 64), ObservedAt: r.observed, Complete: true, Pools: map[string]bool{}}, nil
}

func visibilityOrderPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	admin := os.Getenv(internaltest.PostgresDSNEnv)
	if admin == "" {
		t.Fatalf("%s is not set: point it at a running PostgreSQL whose role may CREATE DATABASE", internaltest.PostgresDSNEnv)
	}
	pool, err := pgxpool.New(context.Background(), internaltest.NewExternalDatabase(t, admin))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func readVisibilityOrder(t *testing.T, ctx context.Context, visibility *databaseClockVisibilityReader, cutoffs ...time.Time) (ranker.InputSnapshot, error) {
	t.Helper()
	reader, err := NewCanonicalInputReader("indexer", "stats", universeStream, universeProjection, visibility)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := visibility.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	return reader.ReadRankingUniverseTxDetailed(ctx, tx, "run", cutoffs...)
}

func assertVisibilityReachesTables(t *testing.T, err error) {
	t.Helper()
	step, class, refusal := ranker.RefreshRefusalDetails(err)
	if err == nil || refusal {
		t.Fatalf("canonical read error = %v, refusal = %q/%q/%t", err, step, class, refusal)
	}
	var databaseError *pgconn.PgError
	if !errors.As(err, &databaseError) || databaseError.Code != "42P01" || !strings.Contains(databaseError.Message, "indexer_ingest_watermarks_v1") {
		t.Fatalf("canonical read error = %v, want missing watermark table", err)
	}
}

func TestDatabaseClockCutoffFollowsTheVisibilityRead(t *testing.T) {
	pool := visibilityOrderPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	visibility := &databaseClockVisibilityReader{pool: pool, waitSecond: true}
	in, err := readVisibilityOrder(t, ctx, visibility)
	if visibility.calls != 1 {
		t.Fatalf("visibility calls = %d, want 1", visibility.calls)
	}
	assertVisibilityReachesTables(t, err)
	if delta := in.CutoffTimeUTC - visibility.observed.Unix(); delta < 0 || delta > 5 {
		t.Fatalf("cutoff = %d, observation = %d, want cutoff within 0 to 5 seconds after observation", in.CutoffTimeUTC, visibility.observed.Unix())
	}
}

func TestDatabaseClockCutoffRefusesAnOldObservation(t *testing.T) {
	pool := visibilityOrderPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	visibility := &databaseClockVisibilityReader{pool: pool, age: 61 * time.Second}
	_, err := readVisibilityOrder(t, ctx, visibility)
	step, class, refusal := ranker.RefreshRefusalDetails(err)
	if !refusal || step != "visibility" || class != "cutoff" {
		t.Fatalf("old observation refusal = %q/%q/%t, err = %v", step, class, refusal, err)
	}
	if visibility.calls != 1 {
		t.Fatalf("visibility calls = %d, want 1", visibility.calls)
	}
	visibility.age = 30 * time.Second
	_, err = readVisibilityOrder(t, ctx, visibility)
	assertVisibilityReachesTables(t, err)
	if visibility.calls != 2 {
		t.Fatalf("visibility calls = %d, want 2", visibility.calls)
	}
}

func TestGivenCutoffRefusesALaterObservation(t *testing.T) {
	pool := visibilityOrderPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var now time.Time
	if err := pool.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&now); err != nil {
		t.Fatal(err)
	}
	visibility := &databaseClockVisibilityReader{pool: pool}
	cutoff := now.UTC().Truncate(time.Second).Add(-time.Second)
	_, err := readVisibilityOrder(t, ctx, visibility, cutoff)
	step, class, refusal := ranker.RefreshRefusalDetails(err)
	if !refusal || step != "visibility" || class != "cutoff" {
		t.Fatalf("later observation refusal = %q/%q/%t, err = %v", step, class, refusal, err)
	}
	if visibility.calls != 1 {
		t.Fatalf("visibility calls = %d, want 1", visibility.calls)
	}
	_, err = readVisibilityOrder(t, ctx, visibility, time.Unix(0, 0))
	step, class, refusal = ranker.RefreshRefusalDetails(err)
	if !refusal || step != "cutoff" || class != "validation" {
		t.Fatalf("invalid cutoff refusal = %q/%q/%t, err = %v", step, class, refusal, err)
	}
	if visibility.calls != 1 {
		t.Fatalf("visibility calls after invalid cutoff = %d, want 1", visibility.calls)
	}
}
