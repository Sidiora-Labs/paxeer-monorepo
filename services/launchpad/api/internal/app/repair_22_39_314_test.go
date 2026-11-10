package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/app"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/development"
	chartmigrations "github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/migrations"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// The API fixture already owns candles.candles. Apply the remaining real writer
// migrations so the real chart producer can commit alongside the read schema.
func repair314ChartWriterSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	for _, name := range []string{"00001_init.sql", "00002_processed_swaps.sql", "00003_development_state.sql"} {
		raw, err := chartmigrations.FS.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		up := strings.SplitN(string(raw), "-- +goose Down", 2)[0]
		for _, block := range strings.Split(up, "-- +goose StatementBegin") {
			statement := strings.SplitN(block, "-- +goose StatementEnd", 2)[0]
			if name == "00001_init.sql" && !strings.Contains(statement, `CREATE TABLE "candles"."candle_cursors"`) {
				continue
			}
			if strings.TrimSpace(statement) == "" || strings.TrimSpace(statement) == "-- +goose Up" {
				continue
			}
			if _, err = pool.Exec(context.Background(), statement); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		}
	}
}

func TestRepair_22_39_314_DevelopmentCandleReachesRouteAndTopic(t *testing.T) {
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
	f := dstFixture{pool: pool, pub: rdb, chain: chain, topics: []string{"market.candles." + dstPool + ".1m"}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.Config{
		DatabaseURL: dsn, RedisURL: redisURL, NodeEnv: "development", Port: 3000,
		RateLimitMax: 1000, RateLimitWindowSec: 60, MaxInFlightRequests: 1000,
		WSMaxConnections: 100, WSMaxPerIP: 100, SSEMaxConnections: 100, SSEMaxPerIP: 100,
		ClientSendBuffer: 256, CoalesceFlushMS: 20, CORSAllowedOrigins: dstOrigin,
		NetworkRPCURL: chain.url, StreamSigningKey: strings.Repeat("k", 32),
	}
	start := func() func() {
		runCtx, cancel := context.WithCancel(ctx)
		a, err := app.New(runCtx, &cfg, logger)
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		srv := httptest.NewServer(a.Router)
		f.srv = srv
		var once sync.Once
		stop := func() { once.Do(func() { srv.Close(); cancel(); a.Close() }) }
		t.Cleanup(stop)
		return stop
	}
	stop := start()
	f.createPool(t)
	receipt := f.swap(t, true, 1000000, 400000000000000000, 10000)
	startSecond := chain.blockTime(t, receipt) / 60 * 60
	query := "/v2/pools/" + dstPool + "/candles?interval=1m&from=" + url.QueryEscape(time.Unix(startSecond, 0).UTC().Format("2006-01-02T15:04:05.000Z")) + "&to=" + url.QueryEscape(time.Unix(startSecond+60, 0).UTC().Format("2006-01-02T15:04:05.000Z"))
	get := func(path string) (int, []byte) {
		t.Helper()
		response, err := http.Get(f.srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		return response.StatusCode, raw
	}
	// A real indexed swap alone is not a candle commit.
	if status, raw := get(query); status != 200 || interfaces.ValidateJSON("CandlesResponse", raw) != nil || !bytes.Contains(raw, []byte(`"items":[]`)) {
		t.Fatalf("uncommitted history = %d %s", status, raw)
	}
	if status, raw := f.snapshot(t, dstOrigin); status != 503 || !bytes.Contains(raw, []byte("DEPENDENCY_UNAVAILABLE")) {
		t.Fatalf("uncommitted topic = %d %s", status, raw)
	}
	client, err := rpc.DialContext(ctx, chain.url)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	producer := development.NewSource(pool, rdb, client, logger)
	if n, err := producer.Project(ctx); err != nil || n != 1 {
		t.Fatalf("real source bootstrap: n=%d err=%v", n, err)
	}
	// Assert the producer committed every timeframe before observing the API.
	var count int
	if err = pool.QueryRow(ctx, `SELECT trade_count FROM candles.candles WHERE pool_address=$1 AND timeframe='1m' AND candle_start=$2`, dstPool, startSecond).Scan(&count); err != nil || count != 1 {
		t.Fatalf("chart commit missing: count=%d err=%v", count, err)
	}
	readHistory := func() (int, []byte) { return get(query) }
	readSnapshot := func() (int, []byte) { return f.snapshot(t, dstOrigin) }
	await := func(path, document string, read func() (int, []byte)) []byte {
		t.Helper()
		deadline := time.Now().Add(15 * time.Second)
		for {
			status, raw := read()
			if status == 200 && !bytes.Contains(raw, []byte(`"items":[]`)) {
				if err := interfaces.ValidateJSON(document, raw); err != nil {
					t.Fatalf("%s: %v %s", document, err, raw)
				}
				return raw
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s did not reach committed projection: %d %s", path, status, raw)
			}
			time.Sleep(25 * time.Millisecond)
		}
	}
	var history interfaces.CandlesResponse
	raw := await(query, "CandlesResponse", readHistory)
	if err = json.Unmarshal(raw, &history); err != nil {
		t.Fatal(err)
	}
	if len(history.Data.Items) != 1 || history.Revision == nil || history.Watermark == nil {
		t.Fatalf("canonical history missing revision/watermark: %s", raw)
	}
	candle := history.Data.Items[0]
	expectedHeight := receipt.BlockNumber.String()
	expectedHash := strings.ToLower(receipt.BlockHash.Hex())
	if candle.Synthetic || candle.Trades != "1" || candle.Revision != "1" || candle.VolumeUsdlRaw != "1000000" || candle.BuyVolumeUsdlRaw != "1000000" || candle.VolumeTokenRaw != "400000000000000000" || candle.Coverage != "partial" || candle.FirstPosition.Height != interfaces.UInt256(expectedHeight) || candle.LastPosition.BlockHash != expectedHash || candle.LastPosition.TransactionHash != strings.ToLower(receipt.TxHash.Hex()) || candle.LastPosition.TransactionIndex != interfaces.UInt256(strconv.FormatUint(uint64(receipt.TransactionIndex), 10)) || candle.LastPosition.LogIndex != interfaces.UInt256(strconv.FormatUint(uint64(receipt.Logs[0].Index), 10)) || history.Watermark.Height != interfaces.UInt256(expectedHeight) || history.Watermark.BlockHash != expectedHash || candle.SeriesModelVersion != history.Data.SeriesModelVersion || history.Data.SnapshotId == "" {
		t.Fatalf("canonical economics/position missing: %s", raw)
	}
	if candle.Open.Numerator != "1" || candle.Open.Denominator != "1000000" || candle.Open.Source.Kind != "post_swap_spot" || candle.Open.Source.AmountBasis != "effective_reserves" || candle.Open.Source.Position == nil || candle.Open.Source.Position.BlockHash != expectedHash {
		t.Fatalf("source price provenance lost: %+v", candle.Open)
	}
	snapshotPath := "/v2/snapshots?topics=" + url.QueryEscape(f.topics[0])
	snapshotRaw := await(snapshotPath, "TopicSnapshotReadResponse", readSnapshot)
	var snapshot struct {
		Data struct {
			Snapshot dstSnapshot `json:"snapshot"`
		} `json:"data"`
	}
	if err = json.Unmarshal(snapshotRaw, &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Data.Snapshot.Topics) != 1 {
		t.Fatalf("candle snapshot missing: %s", snapshotRaw)
	}
	entry := snapshot.Data.Snapshot.Topics[0]
	var streamed interfaces.Candle
	if err = json.Unmarshal(entry.Payload, &streamed); err != nil {
		t.Fatal(err)
	}
	streamCandle, _ := json.Marshal(streamed)
	routeCandle, _ := json.Marshal(candle)
	if entry.Topic != f.topics[0] || entry.Revision != "1" || entry.Sequence != "1" || len(entry.Watermark) == 0 || !bytes.Equal(streamCandle, routeCandle) {
		t.Fatalf("route/topic disagree: %s\n%s", raw, snapshotRaw)
	}
	// Restart the API and chart source. Durable projection and source claims
	// recover without a new swap or duplicate stream publication.
	stop()
	producer = development.NewSource(pool, rdb, client, logger)
	if n, err := producer.Project(ctx); err != nil || n != 0 {
		t.Fatalf("chart restart refolded a source: %d %v", n, err)
	}
	stop = start()
	defer stop()
	recovered := await(query, "CandlesResponse", readHistory)
	var after interfaces.CandlesResponse
	if err = json.Unmarshal(recovered, &after); err != nil {
		t.Fatal(err)
	}
	if len(after.Data.Items) != 1 || after.Data.SnapshotId != history.Data.SnapshotId || after.Revision == nil || *after.Revision != *history.Revision {
		t.Fatalf("history changed on restart: %s", recovered)
	}
	snapshotRaw = await(snapshotPath, "TopicSnapshotReadResponse", readSnapshot)
	if err = json.Unmarshal(snapshotRaw, &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Data.Snapshot.Topics) != 1 || snapshot.Data.Snapshot.Topics[0].Sequence != "1" {
		t.Fatalf("restart duplicated candle frame: %s", snapshotRaw)
	}
	// A legacy row has no persisted source order and cannot enter this scope.
	f.exec(t, `INSERT INTO candles.candles(pool_address,timeframe,candle_start,open,high,low,close,last_trade_ts,sequence_num) VALUES($1,'1m',$2,'1','1','1','1',$2,99)`, dstPool, startSecond+60)
	legacyQuery := strings.Replace(query, url.QueryEscape(time.Unix(startSecond+60, 0).UTC().Format("2006-01-02T15:04:05.000Z")), url.QueryEscape(time.Unix(startSecond+120, 0).UTC().Format("2006-01-02T15:04:05.000Z")), 1)
	if status, raw := get(legacyQuery); status != 503 || !bytes.Contains(raw, []byte("INTEGRITY_QUARANTINED")) {
		t.Fatalf("legacy candle escaped quarantine: %d %s", status, raw)
	}
	// Production's stream schema is never created or written by this path.
	var production bool
	if err = pool.QueryRow(ctx, `SELECT to_regclass('api_stream.stream_projection') IS NOT NULL`).Scan(&production); err != nil || production {
		t.Fatalf("development touched production stream: %t %v", production, err)
	}
}
