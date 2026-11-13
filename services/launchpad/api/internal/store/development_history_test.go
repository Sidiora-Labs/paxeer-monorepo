package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
)

func TestDevelopmentHistoryTracksRecordedSources(t *testing.T) {
	st, db, _ := newStore(t)
	ctx := context.Background()
	const pool = "0x1111111111111111111111111111111111111111"
	if _, err := db.Exec(ctx, `CREATE TABLE indexer.cursors(chain_id integer PRIMARY KEY,last_processed_block bigint);
		CREATE TABLE indexer.swaps(pool_address text,is_buy boolean)`); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		height int64
		valid  bool
	}{{42, true}, {-1, false}, {9007199254740992, false}} {
		if _, err := db.Exec(ctx, `INSERT INTO indexer.cursors VALUES(125,$1) ON CONFLICT(chain_id) DO UPDATE SET last_processed_block=$1`, tc.height); err != nil {
			t.Fatal(err)
		}
		height, err := st.DevelopmentIndexerCursor(ctx, 125)
		if tc.valid && (err != nil || height != uint64(tc.height)) || !tc.valid && !errors.Is(err, store.ErrIndexerCursorUnavailable) {
			t.Fatalf("cursor height %d: %d, %v", tc.height, height, err)
		}
	}
	if _, err := st.DevelopmentIndexerCursor(ctx, 126); !errors.Is(err, store.ErrIndexerCursorUnavailable) {
		t.Fatalf("unknown chain cursor: %v", err)
	}
	for _, statement := range []string{
		`INSERT INTO indexer.swaps VALUES('0x1111111111111111111111111111111111111111',true)`,
		`INSERT INTO candles.candles(pool_address,timeframe,candle_start,open,high,low,close,last_trade_ts,sequence_num) VALUES('0x1111111111111111111111111111111111111111','1m',120,'1','1','1','1',120,1)`,
	} {
		if _, err := db.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		side string
		want bool
	}{{"", true}, {"buy", true}, {"sell", false}} {
		found, err := st.DevelopmentSwapsRecorded(ctx, pool, tc.side)
		if err != nil || found != tc.want {
			t.Fatalf("swaps side %q: %t, %v", tc.side, found, err)
		}
	}
	for _, tc := range []struct {
		from, to int64
		want     bool
	}{{60, 120, false}, {120, 180, true}, {180, 240, false}} {
		found, err := st.DevelopmentCandlesRecorded(ctx, pool, "1m", tc.from, tc.to)
		if err != nil || found != tc.want {
			t.Fatalf("candles range [%d,%d): %t, %v", tc.from, tc.to, found, err)
		}
	}
	items, err := st.DevelopmentTrades(ctx, store.AccountingPageRequest{Pool: pool, Side: "sell", Limit: 10})
	if err != nil || len(items) != 0 {
		t.Fatalf("empty sell page: %v, %v", items, err)
	}
	if _, err := st.DevelopmentTrades(ctx, store.AccountingPageRequest{Pool: pool, Side: "buy", Limit: 10}); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("unprojected trade: %v", err)
	}
	for _, tc := range []struct {
		pool, side string
		limit      int
	}{{"invalid", "", 10}, {pool, "other", 10}, {pool, "", 0}} {
		if _, err := st.DevelopmentTrades(ctx, store.AccountingPageRequest{Pool: tc.pool, Side: tc.side, Limit: tc.limit}); !errors.Is(err, store.ErrDevelopmentScope) {
			t.Fatalf("invalid trade scope %+v: %v", tc, err)
		}
	}
	if _, err := st.DevelopmentSwapsRecorded(ctx, pool, "other"); !errors.Is(err, store.ErrDevelopmentScope) {
		t.Fatalf("invalid swap side: %v", err)
	}
	if _, err := st.DevelopmentCandlesRecorded(ctx, pool, "1m", 180, 120); !errors.Is(err, store.ErrDevelopmentScope) {
		t.Fatalf("reversed candle range: %v", err)
	}
}
