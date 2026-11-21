package db

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"strings"
	"testing"
	"time"
)

func poolSessionOptions(t *testing.T, dsn string, transport Transport) PoolOptions {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	return PoolOptions{
		Transport: transport, PoolID: "session", BudgetIdentity: "session_budget",
		DatabaseAlias: cfg.ConnConfig.Database, DatabaseUser: cfg.ConnConfig.User,
		DatabaseHost: cfg.ConnConfig.Host, DatabasePort: cfg.ConnConfig.Port,
		MaxConns: 2, QueryClass: Interactive, Service: "session", Instance: "one",
	}
}

func TestPoolSessionDirectJITOff(t *testing.T) {
	const dsn = "postgres://u:p@host:5432/mydb?sslmode=disable"
	for _, transport := range []Transport{"", TransportDirect} {
		t.Run(string(transport), func(t *testing.T) {
			options := poolSessionOptions(t, dsn, transport)
			pool, err := NewPool(context.Background(), dsn, options)
			if err != nil {
				t.Fatal(err)
			}
			defer ClosePool(pool)
			params := pool.Config().ConnConfig.RuntimeParams
			if params["jit"] != "off" || params["statement_timeout"] != "350" || params["lock_timeout"] != "100" || params["application_name"] != PoolApplicationName(options) {
				t.Fatalf("direct session settings = %v", params)
			}
		})
	}
}

func TestPoolSessionCallerJITPreserved(t *testing.T) {
	for _, dsn := range []string{
		"postgres://u:p@host:5432/mydb?sslmode=disable&jit=on",
		"postgres://u:p@host:5432/mydb?sslmode=disable&jit=off",
		"postgres://u:p@host:5432/mydb?sslmode=disable&jit=true",
		"host=host port=5432 user=u password=p dbname=mydb sslmode=disable jit=false",
	} {
		for _, transport := range []Transport{"", TransportDirect} {
			options := poolSessionOptions(t, dsn, transport)
			parsed, err := pgxpool.ParseConfig(dsn)
			if err != nil {
				t.Fatal(err)
			}
			pool, err := NewPool(context.Background(), dsn, options)
			if err != nil {
				t.Fatal(err)
			}
			got := pool.Config().ConnConfig.RuntimeParams["jit"]
			ClosePool(pool)
			if want := parsed.ConnConfig.RuntimeParams["jit"]; got != want {
				t.Fatalf("caller jit = %q, want %q", got, want)
			}
		}
	}
}

func TestPoolSessionTransactionPoolerJITAbsent(t *testing.T) {
	const dsn = "postgres://u:p@host:5432/mydb?sslmode=disable"
	options := poolSessionOptions(t, dsn, TransportTransactionPooler)
	pool, err := NewPool(context.Background(), dsn, options)
	if err != nil {
		t.Fatal(err)
	}
	defer ClosePool(pool)
	cfg := pool.Config().ConnConfig
	for _, key := range []string{"jit", "statement_timeout", "lock_timeout"} {
		if value, present := cfg.RuntimeParams[key]; present {
			t.Errorf("transaction pooler startup setting %s = %q", key, value)
		}
	}
	if cfg.DefaultQueryExecMode != pgx.QueryExecModeSimpleProtocol || cfg.StatementCacheCapacity != 0 || cfg.DescriptionCacheCapacity != 0 {
		t.Fatal("transaction pooler requires uncached simple protocol")
	}
	for _, value := range []string{"on", "off"} {
		if other, err := NewPool(context.Background(), dsn+"&jit="+value, options); err == nil || other != nil {
			ClosePool(other)
			t.Fatalf("transaction pooler accepted jit=%s", value)
		} else if !strings.Contains(err.Error(), "unsupported transaction-pooler startup parameter") {
			t.Fatalf("transaction pooler startup refusal = %v", err)
		}
	}
}

func TestPoolSessionDirectJITDatabase(t *testing.T) {
	dsn := os.Getenv("DELIVERY_TEST_DSN")
	if dsn == "" {
		t.Fatal("DELIVERY_TEST_DSN is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	options := poolSessionOptions(t, dsn, TransportDirect)
	pool, err := NewPool(ctx, dsn, options)
	if err != nil {
		t.Fatal(err)
	}
	defer ClosePool(pool)
	for i := int32(0); i < options.MaxConns; i++ {
		conn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Release()
		var jit string
		if err := conn.QueryRow(ctx, "SHOW jit").Scan(&jit); err != nil {
			t.Fatal(err)
		}
		if jit != "off" {
			t.Fatalf("direct session %d jit = %q, want off", i, jit)
		}
	}
}

func TestPoolTag(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"postgres://u:p@host:5432/mydb?sslmode=disable": "mydb",
		"postgres://u:p@host:5432/":                     "default",
		"postgres://u:p@host:5432":                      "default",
		"://bad":                                        "default",
	}
	for dsn, want := range cases {
		if got := poolTag(dsn); got != want {
			t.Errorf("poolTag(%q) = %q, want %q", dsn, got, want)
		}
	}
}

func TestNewPoolBadDSN(t *testing.T) {
	t.Parallel()
	if _, err := NewPool(context.Background(), "not-a-valid-dsn://", PoolOptions{}); err == nil {
		t.Fatal("want error for bad dsn")
	}
}

func TestNewPoolIntegration(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("L07_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("L07_POSTGRES_DSN is not set: configure the PostgreSQL fixture")
	}

	pool, err := NewPool(ctx, dsn, PoolOptions{MaxConns: 5, MinConns: 1, StatementTimeout: 3 * time.Second})
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()
	defer unregister(poolTag(dsn))

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx, "select 1").Scan(&n); err != nil || n != 1 {
		t.Fatalf("select 1 = %d, err %v", n, err)
	}

	m := PoolMetrics()
	if _, ok := m[poolTag(dsn)]; !ok {
		t.Errorf("PoolMetrics missing supplied database tag, got %v", m)
	}
}

func TestPoolOptionsSafety(t *testing.T) {
	for _, o := range []PoolOptions{{MaxConns: -1}, {MaxConns: 1, MinConns: 2}, {QueryClass: "unbounded"}, {QueryClass: Interactive, StatementTimeout: time.Second}, {QueryClass: Migration, LockTimeout: 3 * time.Second}, {StatementTimeout: time.Nanosecond}, {Service: "dsn:password", Instance: "instance"}, {Service: "service"}} {
		if err := o.Validate(); err == nil {
			t.Errorf("unsafe options accepted: %+v", o)
		}
	}
	for _, o := range []PoolOptions{{}, {QueryClass: Interactive}, {QueryClass: Batch}, {QueryClass: Migration}} {
		if err := o.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	statement, lock := (PoolOptions{QueryClass: Interactive}).Deadlines()
	if statement != 350*time.Millisecond || lock != 100*time.Millisecond {
		t.Fatal(statement, lock)
	}
	statement, lock = (PoolOptions{QueryClass: Migration}).Deadlines()
	if statement != 5*time.Minute || lock != 2*time.Second {
		t.Fatal(statement, lock)
	}
}

func TestPhase2QueryObservationOmitsParameters(t *testing.T) {
	dsn := os.Getenv("L07_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("L07_POSTGRES_DSN is unset: real phase2 PostgreSQL required")
	}
	ctx := context.Background()
	options := PoolOptions{MaxConns: 1, QueryClass: Interactive, StatementTimeout: 20 * time.Millisecond, Service: "phase2_query", Instance: "fixture"}
	pool, err := NewPool(ctx, dsn, options)
	if err != nil {
		t.Fatal(err)
	}
	defer ClosePool(pool)
	fingerprint := StatementFingerprint("SELECT pg_sleep($1)")
	var observed QueryObservation
	err = WithObservedTransaction(ctx, pool, options, pgx.TxOptions{}, fingerprint, func(value QueryObservation) { observed = value }, func(tx pgx.Tx) error { _, err := tx.Exec(ctx, "SELECT pg_sleep($1)", 1); return err })
	var postgres *pgconn.PgError
	if !errors.As(err, &postgres) || postgres.Code != "57014" || observed.Outcome != "statement_timeout" || observed.Fingerprint != fingerprint || observed.Service != "phase2_query" || observed.Elapsed > time.Second {
		t.Fatal("actual timeout observation differs", observed, err)
	}
	if strings.Contains(observed.Fingerprint, "pg_sleep") || len(observed.Fingerprint) != 64 {
		t.Fatal("raw statement escaped observation")
	}
	recovery, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err = pool.Ping(recovery); err != nil {
		t.Fatal("canceled transaction connection did not recover", err)
	}
}
