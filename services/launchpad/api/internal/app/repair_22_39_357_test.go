package app_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/app"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/development"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/redis/go-redis/v9"
)

func TestRepair_22_39_357_candle_page_items_carry_the_page_snapshot(t *testing.T) {
	ctx := context.Background()
	dsn, pool := internaltest.NewPostgresWithDSN(t)
	redisURL := internaltest.NewRedisURL(t)
	repair314ChartWriterSchema(t, pool)
	dstApplyIndexerMigration(t, pool, "cursors", "swaps")
	chain := newDstChain(t)
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatal(err)
	}
	rdb := redis.NewClient(opt)
	t.Cleanup(func() { _ = rdb.Close() })
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.Config{
		DatabaseURL: dsn, RedisURL: redisURL, NodeEnv: "development", Port: 3000,
		RateLimitMax: 1000, RateLimitWindowSec: 60, MaxInFlightRequests: 1000,
		WSMaxConnections: 100, WSMaxPerIP: 100, SSEMaxConnections: 100, SSEMaxPerIP: 100,
		ClientSendBuffer: 256, CoalesceFlushMS: 20, CORSAllowedOrigins: dstOrigin,
		NetworkRPCURL: chain.url, StreamSigningKey: strings.Repeat("k", 32),
	}
	runCtx, cancel := context.WithCancel(ctx)
	a, err := app.New(runCtx, &cfg, logger)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	srv := httptest.NewServer(a.Router)
	t.Cleanup(func() { srv.Close(); cancel(); a.Close() })
	f := dstFixture{srv: srv, pool: pool, pub: rdb, chain: chain, topics: []string{"market.candles." + dstPool + ".1m"}}
	f.createPool(t)
	firstReceipt := f.swap(t, true, 1000000, 400000000000000000, 10000)
	firstStart := chain.blockTime(t, firstReceipt) / 60 * 60
	if err = chain.backend.AdjustTime(time.Minute); err != nil {
		t.Fatal(err)
	}
	secondReceipt := f.swap(t, false, 200000000000000000, 500000, 5000)
	secondStart := chain.blockTime(t, secondReceipt) / 60 * 60
	if secondStart <= firstStart {
		t.Fatalf("swaps did not enter distinct candles: %d %d", firstStart, secondStart)
	}
	client, err := rpc.DialContext(ctx, chain.url)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	producer := development.NewSource(pool, rdb, client, logger)
	if n, err := producer.Project(ctx); err != nil || n != 2 {
		t.Fatalf("real source bootstrap: n=%d err=%v", n, err)
	}
	query := "/v2/pools/" + dstPool + "/candles?interval=1m&from=" + url.QueryEscape(time.Unix(firstStart, 0).UTC().Format("2006-01-02T15:04:05.000Z")) + "&to=" + url.QueryEscape(time.Unix(secondStart+60, 0).UTC().Format("2006-01-02T15:04:05.000Z"))
	readPage := func(path string, want int) interfaces.CandlesResponse {
		t.Helper()
		deadline := time.Now().Add(15 * time.Second)
		for {
			response, err := http.Get(srv.URL + path)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := io.ReadAll(response.Body)
			response.Body.Close()
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode == http.StatusOK {
				if err = interfaces.ValidateJSON("CandlesResponse", raw); err != nil {
					t.Fatalf("invalid candle page: %v %s", err, raw)
				}
				var page interfaces.CandlesResponse
				if err = json.Unmarshal(raw, &page); err != nil {
					t.Fatal(err)
				}
				if len(page.Data.Items) == want {
					if page.Data.SnapshotId == "" {
						t.Fatalf("page snapshot missing: %s", raw)
					}
					for _, item := range page.Data.Items {
						if item.SnapshotId != page.Data.SnapshotId {
							t.Fatalf("candle %s snapshot %q differs from page %q", item.Start, item.SnapshotId, page.Data.SnapshotId)
						}
					}
					return page
				}
			}
			if time.Now().After(deadline) {
				t.Fatalf("candle page did not reach %d items: %d %s", want, response.StatusCode, raw)
			}
			time.Sleep(25 * time.Millisecond)
		}
	}
	full := readPage(query+"&limit=2", 2)
	if full.NextCursor != nil || full.Data.Items[0].Start != time.Unix(firstStart, 0).UTC().Format("2006-01-02T15:04:05.000Z") || full.Data.Items[1].Start != time.Unix(secondStart, 0).UTC().Format("2006-01-02T15:04:05.000Z") {
		t.Fatalf("wrong candle window: %+v", full)
	}
	first := readPage(query+"&limit=1", 1)
	if first.NextCursor == nil || *first.NextCursor == "" || first.Data.SnapshotId != full.Data.SnapshotId || !reflect.DeepEqual(first.Data.Items[0], full.Data.Items[0]) {
		t.Fatalf("first cursor page disagrees with full window: %+v", first)
	}
	second := readPage(query+"&limit=1&cursor="+url.QueryEscape(*first.NextCursor), 1)
	if second.NextCursor != nil || second.Data.SnapshotId != full.Data.SnapshotId || !reflect.DeepEqual(second.Data.Items[0], full.Data.Items[1]) {
		t.Fatalf("cursor changed snapshot or candle: %+v", second)
	}
}
