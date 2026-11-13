package db

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAdmittedPoolBindsTransactionTransportAndRollback(t *testing.T) {
	dsn := os.Getenv("DELIVERY_TEST_DSN")
	if dsn == "" {
		t.Fatal("private PostgreSQL fixture is required")
	}
	endpoint, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	query := endpoint.Query()
	query.Set("client_encoding", "UTF8")
	endpoint.RawQuery = query.Encode()
	dsn = endpoint.String()
	parsed, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	options := PoolOptions{Transport: TransportTransactionPooler, PoolID: "admitted_fixture", BudgetIdentity: "fixture_budget", DatabaseAlias: parsed.ConnConfig.Database, DatabaseUser: parsed.ConnConfig.User, DatabaseHost: parsed.ConnConfig.Host, DatabasePort: parsed.ConnConfig.Port, MaxConns: 2, QueryClass: Interactive, Service: "shared_fixture", Instance: "one", AcquireTimeout: time.Second}
	if err := options.Validate(); err != nil {
		t.Fatalf("bounded transaction pool = %v", err)
	}
	ctx := context.Background()
	pool, err := NewPool(ctx, dsn, options)
	if err != nil {
		t.Fatal(err)
	}
	defer ClosePool(pool)
	if err := pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	if err := CheckPoolAdmission(pool, options); err != nil {
		t.Fatalf("admitted pool authority = %v", err)
	}
	if metric, ok := PoolMetrics()["shared_fixture/one/interactive/admitted_fixture"]; !ok || metric.MaxConns != 2 {
		t.Fatalf("admitted pool metric = %+v, %t", metric, ok)
	}
	changed := options
	changed.BudgetIdentity = "other_budget"
	if err := CheckPoolAdmission(pool, changed); err == nil {
		t.Fatal("unrelated pool budget accepted")
	}
	if _, err := AcquireBounded(ctx, pool, changed); err == nil {
		t.Fatal("unrelated pool access accepted")
	}
	if second, err := NewPool(ctx, dsn, options); err == nil {
		ClosePool(second)
		t.Fatal("duplicate pool authority accepted")
	}
	if err := WithTransaction(ctx, pool, options, pgx.TxOptions{}, func(tx pgx.Tx) error {
		var value int
		if err := tx.QueryRow(ctx, "SELECT 125").Scan(&value); err != nil || value != 125 {
			t.Fatalf("bounded SQL result = %d, %v", value, err)
		}
		return nil
	}); err != nil {
		t.Fatalf("committed transaction = %v", err)
	}
	want := errors.New("rollback requested")
	if err := WithTransaction(ctx, pool, options, pgx.TxOptions{}, func(tx pgx.Tx) error {
		var value int
		if err := tx.QueryRow(ctx, "SELECT 1").Scan(&value); err != nil || value != 1 {
			t.Fatalf("rollback transaction read = %d, %v", value, err)
		}
		return want
	}); !errors.Is(err, want) {
		t.Fatalf("transaction error = %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("pool after rollback = %v", err)
	}
	wrongRoute := options
	wrongRoute.DatabaseAlias = "different"
	if other, err := NewPool(ctx, dsn, wrongRoute); err == nil {
		ClosePool(other)
		t.Fatal("changed database route accepted")
	}
	query = endpoint.Query()
	query.Set("search_path", "public")
	endpoint.RawQuery = query.Encode()
	unsupported := endpoint.String()
	if other, err := NewPool(ctx, unsupported, options); err == nil {
		ClosePool(other)
		t.Fatal("transaction pooler startup parameter accepted")
	}
	for _, mode := range []string{"database", "cancelled", "deadline", "closed connection", "invalid isolation", "unadmitted"} {
		t.Run(mode, func(t *testing.T) {
			bounded := ctx
			candidate := options
			transactionOptions := pgx.TxOptions{}
			if mode == "cancelled" {
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				bounded = cancelled
			}
			if mode == "deadline" {
				expired, cancel := context.WithDeadline(ctx, time.Now().Add(-time.Second))
				defer cancel()
				bounded = expired
			}
			if mode == "invalid isolation" {
				transactionOptions.IsoLevel = "invalid"
			}
			if mode == "unadmitted" {
				candidate.PoolID = "unadmitted"
			}
			var observations []QueryObservation
			err := WithObservedTransaction(bounded, pool, candidate, transactionOptions, strings.Repeat("a", 64), func(o QueryObservation) { observations = append(observations, o) }, func(tx pgx.Tx) error {
				if mode == "closed connection" {
					return tx.Conn().Close(ctx)
				}
				_, err := tx.Exec(ctx, "SELECT * FROM unavailable_observed_table")
				return err
			})
			want := "database_error"
			if mode == "cancelled" {
				want = "canceled"
			}
			if mode == "deadline" {
				want = "deadline"
			}
			if err == nil || len(observations) != 1 || observations[0].Outcome != want {
				t.Fatalf("observed transaction %s: %+v %v", mode, observations, err)
			}
		})
	}
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("discarded connection recovery: %v", err)
	}
	if err := WithObservedTransaction(ctx, pool, options, pgx.TxOptions{}, "invalid", func(QueryObservation) { t.Fatal("invalid fingerprint observed") }, func(pgx.Tx) error { return nil }); err == nil {
		t.Fatal("unreviewed query admitted")
	}
	ClosePool(nil)
	pool.Close()
	if err := WithTransaction(ctx, pool, options, pgx.TxOptions{}, func(pgx.Tx) error { t.Fatal("closed pool transaction invoked"); return nil }); err == nil {
		t.Fatal("closed pool admitted transaction")
	}

}
