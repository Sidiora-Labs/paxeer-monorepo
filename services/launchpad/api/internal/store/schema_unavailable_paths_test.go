package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
)

func TestDevelopmentReadsReportMissingIndexedSchema(t *testing.T) {
	st, db, _ := newStore(t)
	ctx := context.Background()
	if _, err := db.Exec(ctx, `DROP SCHEMA indexer CASCADE`); err != nil {
		t.Fatal(err)
	}
	const pool = "0x1111111111111111111111111111111111111111"
	if holders, err := st.DevelopmentHolders(ctx, pool, nil, "", "", 10); err != nil || len(holders) != 0 {
		t.Fatalf("holder read independent of indexed pool schema: %v, %v", holders, err)
	}
	page := store.AccountingPageRequest{Pool: pool, Limit: 10}
	for _, check := range []struct {
		name string
		read func() error
	}{
		{"indexer cursor", func() error { _, err := st.DevelopmentIndexerCursor(ctx, 125); return err }},
		{"indexer head", func() error { _, err := st.DevelopmentIndexerHead(ctx); return err }},
		{"pool indexed", func() error { _, err := st.DevelopmentPoolIndexed(ctx, pool); return err }},
		{"pools indexed", func() error { _, err := st.DevelopmentPoolsIndexed(ctx, []string{pool}); return err }},
		{"swaps recorded", func() error { _, err := st.DevelopmentSwapsRecorded(ctx, pool, ""); return err }},
		{"development trades", func() error { _, err := st.DevelopmentTrades(ctx, page); return err }},
		{"development pools", func() error { _, err := st.DevelopmentPools(ctx, []string{pool}); return err }},
		{"development search", func() error { _, err := st.DevelopmentSearch(ctx, "pool", "exact_address", pool, "", 10); return err }},
		{"launched chains", func() error { _, err := st.LaunchedChains(ctx, 10, 0); return err }},
		{"launched chain", func() error { _, err := st.LaunchedChain(ctx, 7); return err }},
		{"chain revenue", func() error { _, err := st.ChainRevenue(ctx, 7, pool, 10, 0); return err }},
	} {
		t.Run(check.name, func(t *testing.T) {
			if err := check.read(); err == nil {
				t.Fatal("read accepted a missing indexed schema")
			} else if check.name == "indexer cursor" && !errors.Is(err, store.ErrIndexerCursorUnavailable) {
				t.Fatalf("cursor availability: %v", err)
			}
		})
	}
}
