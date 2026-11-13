package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestStoreReadsPropagateDatabaseCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	st := newStore(t)
	calls := []struct {
		name string
		run  func() error
	}{
		{"card", func() error { _, e := st.GetCard(ctx, "card"); return e }},
		{"short card", func() error { _, e := st.GetCardByShortCode(ctx, "code"); return e }},
		{"portfolio", func() error { _, _, e := st.Portfolio(ctx, "user"); return e }},
		{"position", func() error { _, e := st.GetPosition(ctx, "user", "pool"); return e }},
		{"positions", func() error { _, e := st.ListPositions(ctx, "user"); return e }},
		{"trades", func() error { _, e := st.ListTrades(ctx, "user", store.TradeFilter{}); return e }},
		{"cursor", func() error { _, e := st.GetCursor(ctx); return e }},
		{"advance", func() error { return st.AdvanceCursor(ctx, 1, 0, "swap", 1) }},
		{"swaps", func() error { _, e := st.ListUnreconciledSwaps(ctx, store.Cursor{}, 0); return e }},
		{"indexer head", func() error { _, e := st.IndexerHead(ctx); return e }},
		{"consumer head", func() error { _, e := st.ConsumerBlock(ctx); return e }},
		{"code", func() error { _, e := st.ShortCodeExists(ctx, "code"); return e }},
		{"card code", func() error { _, e := st.ShortCodeForCard(ctx, "card"); return e }},
		{"user position", func() error { _, e := st.UserHasAnyPosition(ctx, "user"); return e }},
		{"event", func() error {
			_, e := st.LogReferralEvent(ctx, store.ReferralEvent{EventType: "click", ShortCode: "code", WalletAddress: "visitor"})
			return e
		}},
		{"stats", func() error { _, e := st.GetSharerStats(ctx, "owner", 1); return e }},
		{"market", func() error { _, e := st.GetMarket(ctx, "pool"); return e }},
		{"pool token", func() error { _, e := st.GetPoolToken(ctx, "pool"); return e }},
		{"token", func() error { _, e := st.GetTokenMeta(ctx, "token"); return e }},
		{"insert card", func() error { return st.InsertCard(ctx, store.CardRow{}) }},
		{"fold", func() error {
			_, e := st.FoldTrade(ctx, trade("buy", "user", "pool", "token", true, "100", "10", 1, 1))
			return e
		}},
	}
	for _, tc := range calls {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
		})
	}
}

func TestCardInsertionCancellationLeavesNoPartialReferral(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)
	config := st.Pool().Config()
	trace := new(internaltest.QueryCancellation)
	config.ConnConfig.Tracer = trace
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	traced := store.New(pool)
	card := store.CardRow{CardID: "atomic-card", ShortCode: "atomic-code", OwnerAddress: "owner", PoolAddress: "pool", TokenAddress: "token", Snapshot: []byte(`{}`), CreatedAt: 1}
	trace.Start(0)
	err = traced.InsertCard(ctx, card)
	queries := trace.Stop()
	if err != nil || len(queries) < 4 {
		t.Fatal(queries, err)
	}
	if _, err = st.Pool().Exec(ctx, `DELETE FROM pnl.referrals;DELETE FROM pnl.pnl_cards`); err != nil {
		t.Fatal(err)
	}
	for i, query := range queries {
		if query == "rollback" {
			continue
		}
		t.Run(query, func(t *testing.T) {
			trace.Start(i + 1)
			err := traced.InsertCard(ctx, card)
			trace.Stop()
			if !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			var cards, referrals int
			if err = st.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM pnl.pnl_cards),(SELECT count(*) FROM pnl.referrals)`).Scan(&cards, &referrals); err != nil || cards != 0 || referrals != 0 {
				t.Fatal(cards, referrals, err)
			}
		})
	}
}

func TestStoreEmptyReadsAndTradeUpperBound(t *testing.T) {
	st := newStore(t)
	ctx := context.Background()
	ensurePortfolioSchemas(t, st)
	if got, err := st.GetPosition(ctx, "missing", "pool"); err != nil || got != nil {
		t.Fatal(got, err)
	}
	if got, err := st.GetTokenMeta(ctx, "missing"); err != nil || got != nil {
		t.Fatal(got, err)
	}
	if got, err := st.ShortCodeForCard(ctx, "missing"); err != nil || got != "" {
		t.Fatal(got, err)
	}
	mustFold(t, st, trade("bounded-buy-0", "user", "pool", "token", true, "100", "10", 1, 100))
	upper := int64(99)
	if got, err := st.ListTrades(ctx, "user", store.TradeFilter{To: &upper}); err != nil || len(got) != 0 {
		t.Fatal(got, err)
	}
	upper = 100
	if got, err := st.ListTrades(ctx, "user", store.TradeFilter{To: &upper}); err != nil || len(got) != 1 || got[0].ID != "bounded-buy-0" {
		t.Fatal(got, err)
	}
}
