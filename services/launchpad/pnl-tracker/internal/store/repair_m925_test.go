package store

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/pnlmath"
)

func repairM925Trade(id string, buy bool, usdl, tokens string, block, ts int64) TradeInput {
	return TradeInput{
		ID: id, UserAddress: "0xUSER", PoolAddress: "0xPOOL", TokenAddress: "0xTOKEN",
		IsBuy: buy, UsdlAmount: usdl, TokenAmount: tokens, Price: "0", Fee: "0",
		BlockNumber: block, BlockTimestamp: ts, TxHash: strings.Split(id, "-")[0],
	}
}

// Exercise the production journal projection FoldTrade consumes with in-memory
// persisted-row fixtures, rather than a replacement fold or database mock.
func repairM925Project(t *testing.T, journal []TradeInput) pnlmath.Position {
	t.Helper()
	got, token, err := projectOrderedPosition(journal)
	if err != nil {
		t.Fatalf("project journal: %v", err)
	}
	if token != "0xtoken" {
		t.Fatalf("resolved token = %q, want 0xtoken", token)
	}
	return got
}

func repairM925Want(t *testing.T, got pnlmath.Position, spent, bought, received, sold, avg, holdings, realized string, first, last int64, count int) {
	t.Helper()
	want := pnlmath.Position{
		TotalUsdlSpent: spent, TotalTokensBought: bought, TotalUsdlReceived: received,
		TotalTokensSold: sold, AvgCostBasis: avg, CurrentHoldings: holdings,
		RealizedPnlUsdl: realized, FirstBuyTs: &first, LastTradeTs: last, TradeCount: count,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("position = %+v, want %+v", got, want)
	}
}

func TestRepairM_925_DelayedSell(t *testing.T) {
	buy := repairM925Trade("0xfirst-0", true, "1000000", "1000000000000000000", 10, 100)
	sell := repairM925Trade("0xsell-0", false, "5000000", "1000000000000000000", 11, 200)
	rebuy := repairM925Trade("0xrebuy-0", true, "9000000", "1000000000000000000", 12, 300)
	journal := []TradeInput{buy, rebuy}
	before := repairM925Project(t, journal)
	repairM925Want(t, before, "10000000", "2000000000000000000", "0", "0", "5000000", "2000000000000000000", "0", 100, 300, 2)
	journal = append(journal, sell)
	got := repairM925Project(t, journal)
	repairM925Want(t, got, "10000000", "2000000000000000000", "5000000", "1000000000000000000", "9000000", "1000000000000000000", "4000000", 100, 300, 3)
	if ordered := repairM925Project(t, []TradeInput{buy, sell, rebuy}); !reflect.DeepEqual(got, ordered) {
		t.Fatalf("delayed sell differs from ordered delivery: %+v vs %+v", got, ordered)
	}
}

func TestRepairM_925_DelayedBuy(t *testing.T) {
	early := repairM925Trade("0xearly-0", true, "1000000", "1000000000000000000", 10, 100)
	later := repairM925Trade("0xlater-0", true, "9000000", "1000000000000000000", 11, 200)
	sell := repairM925Trade("0xsale-0", false, "5000000", "1000000000000000000", 12, 300)
	journal := []TradeInput{later, sell}
	before := repairM925Project(t, journal)
	repairM925Want(t, before, "9000000", "1000000000000000000", "5000000", "1000000000000000000", "0", "0", "-4000000", 200, 300, 2)
	journal = append(journal, early)
	got := repairM925Project(t, journal)
	repairM925Want(t, got, "10000000", "2000000000000000000", "5000000", "1000000000000000000", "5000000", "1000000000000000000", "0", 100, 300, 3)
	if ordered := repairM925Project(t, []TradeInput{early, later, sell}); !reflect.DeepEqual(got, ordered) {
		t.Fatalf("delayed buy differs from ordered delivery: %+v vs %+v", got, ordered)
	}
}

func TestRepairM_925_MixedRealtimeBackfill(t *testing.T) {
	trades := []TradeInput{
		repairM925Trade("0xzz-2", true, "2000000", "2000000000000000000", 100, 100),
		repairM925Trade("0xaa-10", false, "5000000", "1000000000000000000", 100, 100),
		repairM925Trade("0xbb-1", true, "9000000", "1000000000000000000", 101, 200),
		repairM925Trade("0xcc-3", false, "7000000", "1000000000000000000", 102, 300),
	}
	want := repairM925Project(t, trades)
	repairM925Want(t, want, "11000000", "3000000000000000000", "12000000", "2000000000000000000", "5000000", "1000000000000000000", "6000000", 100, 300, 4)
	// The journal is the in-memory persisted-row fixture; as in user_trades,
	// its primary key admits one row for overlapping realtime/backfill IDs.
	for _, delivery := range [][]int{
		{0, 2, 1, 0, 1, 2, 3, 3},
		{0, 2, 3, 0, 1, 2, 3},
		{0, 1, 3, 2, 1, 0},
	} {
		journal := map[string]TradeInput{}
		var got pnlmath.Position
		for _, index := range delivery {
			trade := trades[index]
			journal[trade.ID] = trade
			rows := make([]TradeInput, 0, len(journal))
			for _, row := range journal {
				rows = append(rows, row)
			}
			got = repairM925Project(t, rows)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("delivery %v: %+v, want %+v", delivery, got, want)
		}
	}
}

func TestRepairM_925_CanonicalLogOrder(t *testing.T) {
	// Same block/timestamp, separate transactions: neither lexical ID order,
	// timestamp order nor per-transaction grouping gives the canonical result.
	trades := []TradeInput{
		repairM925Trade("0xz-2", true, "1000000", "1000000000000000000", 10, 100),
		repairM925Trade("0xa-10", false, "5000000", "1000000000000000000", 10, 100),
		repairM925Trade("0xm-11", true, "9000000", "1000000000000000000", 10, 100),
	}
	for _, order := range [][]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}} {
		journal := []TradeInput{trades[order[0]], trades[order[1]], trades[order[2]]}
		original := append([]TradeInput(nil), journal...)
		got := repairM925Project(t, journal)
		repairM925Want(t, got, "10000000", "2000000000000000000", "5000000", "1000000000000000000", "9000000", "1000000000000000000", "4000000", 100, 100, 3)
		if !reflect.DeepEqual(journal, original) {
			t.Fatalf("projection mutated persisted journal: %+v", journal)
		}
	}
}

func TestRepairM_925_PreservesTokenAndRejectsCorruptJournal(t *testing.T) {
	buy := repairM925Trade("0xbuy-2", true, "1000000", "1000000000000000000", 10, 100)
	sell := repairM925Trade("0xsell-10", false, "5000000", "1000000000000000000", 10, 100)
	sell.TokenAddress = ""
	got := repairM925Project(t, []TradeInput{sell, buy})
	repairM925Want(t, got, "1000000", "1000000000000000000", "5000000", "1000000000000000000", "0", "0", "4000000", 100, 100, 2)

	for _, id := range []string{"missing-index", "0xbuy-", "-2", "0xbuy-01", "0xbuy-+2", "0xbuy-18446744073709551616"} {
		invalid := buy
		invalid.ID = id
		if _, _, err := projectOrderedPosition([]TradeInput{invalid}); err == nil {
			t.Fatalf("accepted invalid canonical id %q", id)
		}
	}
	negativeBlock := buy
	negativeBlock.BlockNumber = -1
	collision := buy
	collision.ID = "0xother-2"
	badAmount := buy
	badAmount.UsdlAmount = "not-an-integer"
	for _, journal := range [][]TradeInput{{negativeBlock}, {buy, collision}, {sell}, {badAmount}} {
		p, token, err := projectOrderedPosition(journal)
		if err == nil || token != "" || !reflect.DeepEqual(p, pnlmath.Position{}) {
			t.Fatalf("corrupt journal published partial state: p=%+v token=%q err=%v", p, token, err)
		}
	}
}

func TestRepairM_925_BlockOrderPrecedesTimestamp(t *testing.T) {
	buy := repairM925Trade("0xbuy-20", true, "1000000", "1000000000000000000", 10, 300)
	sell := repairM925Trade("0xsell-1", false, "5000000", "1000000000000000000", 11, 100)
	rebuy := repairM925Trade("0xrebuy-0", true, "9000000", "1000000000000000000", 12, 200)
	got := repairM925Project(t, []TradeInput{rebuy, sell, buy})
	repairM925Want(t, got, "10000000", "2000000000000000000", "5000000", "1000000000000000000", "9000000", "1000000000000000000", "4000000", 300, 300, 3)
}
