package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestStoreReadsReportMissingSourceSchema(t *testing.T) {
	st, db, _ := newStore(t)
	ctx := context.Background()
	if _, err := db.Exec(ctx, "DROP SCHEMA indexer CASCADE"); err != nil {
		t.Fatal(err)
	}
	const pool = "0x1111111111111111111111111111111111111111"
	for _, check := range []struct {
		name string
		read func() error
	}{
		{"indexer head", func() error { _, err := st.DevelopmentIndexerHead(ctx); return err }},
		{"pool registry", func() error { _, err := st.PoolRegistry(ctx, []string{pool}); return err }},
		{"pool reserves", func() error { _, err := st.PoolReserves(ctx, []string{pool}); return err }},
		{"pool home chain", func() error { _, err := st.PoolHomeChains(ctx, []string{pool}); return err }},
		{"indexed pool", func() error { _, err := st.DevelopmentPoolIndexed(ctx, pool); return err }},
		{"indexed pools", func() error { _, err := st.DevelopmentPoolsIndexed(ctx, []string{pool}); return err }},
		{"recorded swaps", func() error { _, err := st.DevelopmentSwapsRecorded(ctx, pool, ""); return err }},
		{"development pools", func() error { _, err := st.DevelopmentPools(ctx, []string{pool}); return err }},
		{"development trades", func() error {
			_, err := st.DevelopmentTrades(ctx, store.AccountingPageRequest{Pool: pool, Limit: 10})
			return err
		}},
		{"launched chains", func() error { _, err := st.LaunchedChains(ctx, 10, 0); return err }},
		{"launched chain", func() error { _, err := st.LaunchedChain(ctx, 7); return err }},
		{"chain revenue", func() error { _, err := st.ChainRevenue(ctx, 7, "", 10, 0); return err }},
	} {
		t.Run(check.name, func(t *testing.T) {
			err := check.read()
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "42P01" {
				t.Fatalf("missing source error: %v", err)
			}
		})
	}
}
