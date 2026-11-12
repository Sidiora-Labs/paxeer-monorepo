package engine

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
)

func chartDevelopmentBuilder(t *testing.T) (*Builder, *pgxpool.Pool) {
	t.Helper()
	_, pool := internaltest.NewPostgres(t)
	options, err := goredis.ParseURL(internaltest.NewRedisURL(t))
	if err != nil {
		t.Fatal(err)
	}
	rdb := goredis.NewClient(options)
	t.Cleanup(func() { _ = rdb.Close() })
	return New(pool, rdb, store.New(pool), slog.New(slog.NewTextHandler(io.Discard, nil))), pool
}

func TestDevelopmentFoldPreservesCommittedCandleWhenStoredStateInvalid(t *testing.T) {
	for _, tc := range []struct {
		name, keyKind, want string
		edit                func(*developmentCandle)
		invalidWire         bool
	}{
		{"outbox document", "outbox", "invalid character", nil, true},
		{"candle document", "candle", "invalid character", nil, true},
		{"cursor document", "cursor", "invalid character", nil, true},
		{"missing trader membership", "candle", "invalid development candle state", func(c *developmentCandle) { c.Traders = nil }, false},
		{"overflowed trade count", "candle", "invalid development candle state", func(c *developmentCandle) { c.Row.TradeCount = 2147483647 }, false},
		{"invalid high", "candle", "invalid", func(c *developmentCandle) { c.Row.High = "invalid" }, false},
		{"invalid low", "candle", "invalid", func(c *developmentCandle) { c.Row.Low = "invalid" }, false},
		{"invalid high valuation", "candle", "invalid", func(c *developmentCandle) { c.Row.McapHigh = "invalid" }, false},
		{"invalid low valuation", "candle", "invalid", func(c *developmentCandle) { c.Row.McapLow = "invalid" }, false},
		{"invalid traded volume", "candle", "invalid", func(c *developmentCandle) { c.Row.VolumeUsdl = "invalid" }, false},
		{"invalid token volume", "candle", "invalid", func(c *developmentCandle) { c.Row.VolumeToken = "invalid" }, false},
		{"invalid side volume", "candle", "invalid", func(c *developmentCandle) { c.Row.BuyVolumeUsdl = "invalid" }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			builder, pool := chartDevelopmentBuilder(t)
			first := repair1051Swap(0, 1704067260, "1000000000000000000")
			if err := builder.ProcessSwap(ctx, first); err != nil {
				t.Fatal(err)
			}
			key := developmentKey(tc.keyKind, first.PoolAddress)
			if tc.keyKind != "outbox" {
				key = developmentKey(tc.keyKind, first.PoolAddress, "1m")
			}
			if tc.keyKind == "candle" {
				key = developmentKey("candle", first.PoolAddress, "1m", "1704067260")
			}
			var raw []byte
			if err := pool.QueryRow(ctx, `SELECT payload FROM candles.development_state WHERE key=$1`, key).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			if tc.invalidWire {
				raw = []byte("invalid")
			} else {
				var candle developmentCandle
				if err := json.Unmarshal(raw, &candle); err != nil {
					t.Fatal(err)
				}
				tc.edit(&candle)
				var err error
				raw, err = json.Marshal(candle)
				if err != nil {
					t.Fatal(err)
				}
			}
			if _, err := pool.Exec(ctx, `UPDATE candles.development_state SET payload=$2 WHERE key=$1`, key, raw); err != nil {
				t.Fatal(err)
			}
			second := repair1051Swap(1, 1704067260, "2000000000000000000")
			if err := builder.ProcessSwap(ctx, second); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("invalid stored %s accepted: %v", tc.keyKind, err)
			}
			var claimed int
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM candles.processed_swaps`).Scan(&claimed); err != nil || claimed != 1 {
				t.Fatalf("invalid candle claimed a source swap: count=%d err=%v", claimed, err)
			}
		})
	}
}
