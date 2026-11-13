package store

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestExactTransactionRetriesDatabaseSerializationErrors(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
	ddl, err := ExactSchemaDDL("candles")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `CREATE FUNCTION candles.retry_conflict() RETURNS void LANGUAGE plpgsql AS $$ BEGIN RAISE SQLSTATE '40001' USING MESSAGE = 'serialization conflict'; END $$`); err != nil {
		t.Fatal(err)
	}
	exact, err := NewExact(pool, "candles")
	if err != nil {
		t.Fatal(err)
	}
	address := "0x" + strings.Repeat("a", 40)
	fill := []byte(`{"price":"1"}`)
	digest := sha256.Sum256(fill)
	claim := Claim{ChainID: "125", Pool: address, Emitter: address, CodeEpoch: "epoch-1", TxHash: "0xretry", LogIndex: "0", Epoch: 1, Digest: digest[:], Fill: fill}
	for _, tc := range []struct {
		name   string
		cancel bool
		want   int
	}{
		{name: "bounded retries", want: 3},
		{name: "cancelled retry", cancel: true, want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			attempts := 0
			callCtx, stop := context.WithCancel(ctx)
			defer stop()
			err := exact.Transaction(callCtx, claim, func(tx pgx.Tx, schema string) error {
				attempts++
				if schema != "candles" {
					t.Fatalf("transaction schema: %s", schema)
				}
				_, queryErr := tx.Exec(ctx, `SELECT candles.retry_conflict()`)
				if tc.cancel {
					stop()
				}
				return queryErr
			})
			if attempts != tc.want {
				t.Fatalf("retry count: %d, %v", attempts, err)
			}
			if tc.cancel {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("cancelled transaction: %v", err)
				}
			} else {
				var postgres *pgconn.PgError
				if !errors.As(err, &postgres) || postgres.Code != "40001" {
					t.Fatalf("serialization result: %v", err)
				}
			}
		})
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM candles.chart_inbox`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("rolled back retry claims: %d, %v", count, err)
	}
}
