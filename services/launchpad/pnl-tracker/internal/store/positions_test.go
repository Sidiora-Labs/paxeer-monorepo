package store_test

import (
	"context"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/store"
)

const (
	oneToken  = "1000000000000000000" // 1e18 token raw
	usdl1     = "1000000"             // 1 USDL (6-dec)
	usdl3     = "3000000"             // 3 USDL
	usdl5     = "5000000"             // 5 USDL
	usdl10    = "10000000"            // 10 USDL
	someUser  = "0xUSER"
	somePool  = "0xPool"
	someToken = "0xToken"
)

func TestFoldTradeAccumulatesAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)

	// Buy 1 @ 1 USDL, buy 1 @ 3 USDL (avg 2), sell 1 @ 5 USDL -> realised 3 USDL.
	steps := []store.TradeInput{
		trade("0xb1-0", someUser, somePool, someToken, true, usdl1, oneToken, 10, 100),
		trade("0xb2-0", someUser, somePool, someToken, true, usdl3, oneToken, 11, 200),
		trade("0xs1-0", someUser, somePool, someToken, false, usdl5, oneToken, 12, 300),
	}
	for _, s := range steps {
		inserted, err := st.FoldTrade(ctx, s)
		if err != nil {
			t.Fatalf("fold %s: %v", s.ID, err)
		}
		if !inserted {
			t.Fatalf("fold %s: expected inserted=true", s.ID)
		}
	}

	pos, err := st.GetPosition(ctx, someUser, somePool)
	if err != nil || pos == nil {
		t.Fatalf("get position: pos=%v err=%v", pos, err)
	}
	if pos.UserAddress != "0xuser" || pos.PoolAddress != "0xpool" || pos.TokenAddress != "0xtoken" {
		t.Fatalf("position keys lowercased? %+v", pos)
	}
	if pos.TotalUsdlSpent != "4000000" || pos.AvgCostBasis != "2000000" {
		t.Errorf("spent=%s avgCost=%s, want 4000000 / 2000000", pos.TotalUsdlSpent, pos.AvgCostBasis)
	}
	if pos.RealizedPnlUsdl != "3000000" {
		t.Errorf("realizedPnlUsdl = %s, want 3000000", pos.RealizedPnlUsdl)
	}
	if pos.CurrentHoldings != oneToken {
		t.Errorf("currentHoldings = %s, want %s", pos.CurrentHoldings, oneToken)
	}
	if pos.TradeCount != 3 {
		t.Errorf("tradeCount = %d, want 3", pos.TradeCount)
	}
	if pos.FirstBuyTs == nil || *pos.FirstBuyTs != 100 || pos.LastTradeTs != 300 {
		t.Errorf("firstBuyTs=%v lastTradeTs=%d", pos.FirstBuyTs, pos.LastTradeTs)
	}

	// Redelivering the first buy must NOT double-count (inserted=false, state unchanged).
	inserted, err := st.FoldTrade(ctx, steps[0])
	if err != nil {
		t.Fatalf("redeliver: %v", err)
	}
	if inserted {
		t.Fatal("redeliver: expected inserted=false (already folded)")
	}
	pos2, _ := st.GetPosition(ctx, someUser, somePool)
	if pos2.TotalUsdlSpent != "4000000" || pos2.TradeCount != 3 || pos2.RealizedPnlUsdl != "3000000" {
		t.Fatalf("redeliver mutated position: %+v", pos2)
	}
}

func TestFoldTradeRealisesLoss(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)
	// Buy 1 @ 10 USDL, sell it for 4 USDL -> realised -6 USDL (signed).
	if _, err := st.FoldTrade(ctx, trade("0xb-0", "0xa", "0xp", "0xt", true, usdl10, oneToken, 1, 10)); err != nil {
		t.Fatalf("buy: %v", err)
	}
	if _, err := st.FoldTrade(ctx, trade("0xs-0", "0xa", "0xp", "0xt", false, "4000000", oneToken, 2, 20)); err != nil {
		t.Fatalf("sell: %v", err)
	}
	pos, _ := st.GetPosition(ctx, "0xa", "0xp")
	if pos.RealizedPnlUsdl != "-6000000" {
		t.Fatalf("realizedPnlUsdl = %s, want -6000000", pos.RealizedPnlUsdl)
	}
	if pos.CurrentHoldings != "0" {
		t.Fatalf("currentHoldings = %s, want 0", pos.CurrentHoldings)
	}
}

func TestListPositionsAndTrades(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)

	// Two pools for one user, plus an unrelated user.
	mustFold(t, st, trade("0xa-0", "0xu", "0xp1", "0xt1", true, usdl1, oneToken, 1, 100))
	mustFold(t, st, trade("0xb-0", "0xu", "0xp2", "0xt2", true, usdl3, oneToken, 2, 200))
	mustFold(t, st, trade("0xc-0", "0xother", "0xp1", "0xt1", true, usdl1, oneToken, 3, 50))

	positions, err := st.ListPositions(ctx, "0xu")
	if err != nil {
		t.Fatalf("list positions: %v", err)
	}
	if len(positions) != 2 {
		t.Fatalf("positions = %d, want 2", len(positions))
	}
	// Ordered by last_trade_ts DESC -> pool2 (200) before pool1 (100).
	if positions[0].PoolAddress != "0xp2" || positions[1].PoolAddress != "0xp1" {
		t.Errorf("ordering = %s,%s", positions[0].PoolAddress, positions[1].PoolAddress)
	}

	t.Run("trade history filters by pool and paginates", func(t *testing.T) {
		all, err := st.ListTrades(ctx, "0xu", store.TradeFilter{Limit: 10})
		if err != nil {
			t.Fatalf("list trades: %v", err)
		}
		if len(all) != 2 {
			t.Fatalf("all trades = %d, want 2", len(all))
		}
		onlyP1, _ := st.ListTrades(ctx, "0xu", store.TradeFilter{Pool: "0xp1", Limit: 10})
		if len(onlyP1) != 1 || onlyP1[0].PoolAddress != "0xp1" {
			t.Fatalf("pool filter = %+v", onlyP1)
		}
		// Time window excludes the ts=100 trade.
		from := int64(150)
		windowed, _ := st.ListTrades(ctx, "0xu", store.TradeFilter{From: &from, Limit: 10})
		if len(windowed) != 1 || windowed[0].PoolAddress != "0xp2" {
			t.Fatalf("from filter = %+v", windowed)
		}
	})
}

// mustFold folds a trade and fails on error or a redelivery.
func mustFold(t *testing.T, st *store.Store, in store.TradeInput) {
	t.Helper()
	inserted, err := st.FoldTrade(context.Background(), in)
	if err != nil {
		t.Fatalf("fold %s: %v", in.ID, err)
	}
	if !inserted {
		t.Fatalf("fold %s: expected inserted=true", in.ID)
	}
}

func TestFoldTradeRebuildsDelayedTrades(t *testing.T) {
	for _, tc := range []struct {
		name       string
		order      []int
		realized   string
		average    string
		delayedBuy bool
	}{
		{name: "delayed sell", order: []int{0, 2, 1}, realized: "4000000", average: "9000000"},
		{name: "delayed buy", order: []int{1, 2, 0}, realized: "0", average: "5000000", delayedBuy: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := newStore(t)
			ctx := context.Background()
			trades := []store.TradeInput{
				trade("0xz-2", someUser, somePool, someToken, true, usdl1, oneToken, 10, 100),
				trade("0xa-10", someUser, somePool, someToken, false, usdl5, oneToken, 10, 100),
				trade("0xm-11", someUser, somePool, someToken, true, "9000000", oneToken, 10, 100),
			}
			if tc.delayedBuy {
				trades[1].IsBuy, trades[1].UsdlAmount = true, "9000000"
				trades[2].IsBuy, trades[2].UsdlAmount = false, usdl5
			}
			for _, index := range tc.order {
				mustFold(t, st, trades[index])
			}
			pos, err := st.GetPosition(ctx, someUser, somePool)
			if err != nil || pos == nil {
				t.Fatalf("position=%+v err=%v", pos, err)
			}
			if pos.RealizedPnlUsdl != tc.realized || pos.AvgCostBasis != tc.average ||
				pos.CurrentHoldings != oneToken || pos.TotalUsdlSpent != "10000000" ||
				pos.TotalUsdlReceived != usdl5 || pos.TotalTokensBought != "2000000000000000000" ||
				pos.TotalTokensSold != oneToken || pos.TradeCount != 3 ||
				pos.FirstBuyTs == nil || *pos.FirstBuyTs != 100 || pos.LastTradeTs != 100 {
				t.Fatalf("incorrect canonical projection: %+v", pos)
			}
			if inserted, err := st.FoldTrade(ctx, trades[0]); err != nil || inserted {
				t.Fatalf("redelivery inserted=%v err=%v", inserted, err)
			}
			// A failed projection must roll back its journal insert as well as
			// its aggregates; a corrected redelivery with that ID must succeed.
			bad := trade("0xnext-12", someUser, somePool, someToken, false, usdl5, "2000000000000000000", 10, 100)
			if inserted, err := st.FoldTrade(ctx, bad); err == nil || inserted {
				t.Fatalf("oversell inserted=%v err=%v", inserted, err)
			}
			after, err := st.GetPosition(ctx, someUser, somePool)
			if err != nil || after == nil || after.RealizedPnlUsdl != tc.realized || after.TradeCount != 3 {
				t.Fatalf("failed projection changed position: %+v err=%v", after, err)
			}
			bad.TokenAmount = oneToken
			mustFold(t, st, bad)
		})
	}
}
