package app_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/app"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/development"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/redis/go-redis/v9"
)

type repair372Fixture struct {
	dstFixture
	start  func() func()
	stop   func()
	source *development.Source
	client *rpc.Client
}

func newRepair372Fixture(t *testing.T) *repair372Fixture {
	t.Helper()
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
		RateLimitMax: 10000, RateLimitWindowSec: 60, MaxInFlightRequests: 1000,
		WSMaxConnections: 100, WSMaxPerIP: 100, SSEMaxConnections: 100, SSEMaxPerIP: 100,
		ClientSendBuffer: 256, CoalesceFlushMS: 20, CORSAllowedOrigins: dstOrigin,
		NetworkRPCURL: chain.url, StreamSigningKey: strings.Repeat("k", 32),
	}
	f := &repair372Fixture{dstFixture: dstFixture{pool: pool, pub: rdb, chain: chain, topics: []string{"market.trades." + dstPool, "market.candles." + dstPool + ".1d"}}}
	f.start = func() func() {
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
	f.stop = f.start()
	f.client, err = rpc.DialContext(ctx, chain.url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(f.client.Close)
	f.source = development.NewSource(pool, rdb, f.client, logger)
	return f
}

func (f *repair372Fixture) get(t *testing.T, path string) (int, []byte) {
	t.Helper()
	resp, err := http.Get(f.srv.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("GET %s -> %d %s", path, resp.StatusCode, raw)
	return resp.StatusCode, raw
}

func (f *repair372Fixture) project(t *testing.T, want int) {
	t.Helper()
	if n, err := f.source.Project(context.Background()); err != nil || n != want {
		t.Fatalf("chart source Project = %d %v, want %d", n, err, want)
	}
}

func (f *repair372Fixture) awaitTrade(t *testing.T, receipt *types.Receipt) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		var committed bool
		err := f.pool.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM api_stream_dev.stream_delivery_outbox WHERE event_id=$1)`, "125:"+strings.ToLower(receipt.TxHash.Hex())+":0").Scan(&committed)
		if err != nil {
			t.Fatal(err)
		}
		if committed {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("stream projector did not commit swap")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func repair372CandlePath(start int64) string {
	format := func(second int64) string {
		return url.QueryEscape(time.Unix(second, 0).UTC().Format("2006-01-02T15:04:05.000Z"))
	}
	return "/v2/pools/" + dstPool + "/candles?interval=1d&from=" + format(start) + "&to=" + format(start+2*86400)
}

func (f *repair372Fixture) candle(t *testing.T, path string, wantTrades string) interfaces.CandlesResponse {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		status, raw := f.get(t, path)
		if status == 409 {
			if interfaces.ValidateJSON("Error", raw) != nil || !strings.Contains(string(raw), `"RESNAPSHOT_REQUIRED"`) {
				t.Fatalf("wrong moving snapshot: %s", raw)
			}
		} else if status == 200 {
			if err := interfaces.ValidateJSON("CandlesResponse", raw); err != nil {
				t.Fatal(err)
			}
			var page interfaces.CandlesResponse
			if err := json.Unmarshal(raw, &page); err != nil {
				t.Fatal(err)
			}
			if len(page.Data.Items) > 0 && page.Data.Items[0].Trades == interfaces.PositiveUInt256(wantTrades) {
				for _, item := range page.Data.Items {
					if item.SnapshotId != page.Data.SnapshotId {
						t.Fatal("candle escaped its page snapshot")
					}
				}
				return page
			}
		} else {
			t.Fatalf("committed candle read = %d %s", status, raw)
		}
		if time.Now().After(deadline) {
			t.Fatalf("candle did not reach %s trades: %d %s", wantTrades, status, raw)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestRepair_22_39_372_candle_read_survives_swaps_and_replay(t *testing.T) {
	f := newRepair372Fixture(t)
	f.createPool(t)
	first := f.swap(t, true, 1000000, 400000000000000000, 10000)
	f.project(t, 1)
	f.awaitTrade(t, first)
	start := f.chain.blockTime(t, first) / 86400 * 86400
	path := repair372CandlePath(start)
	initial := f.candle(t, path, "1")
	deadlineAll := time.Now().Add(15 * time.Second)
	for {
		var lagging bool
		if err := f.pool.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM candles.candles c LEFT JOIN api_stream_dev.candle_projection p USING(pool_address,timeframe,candle_start) WHERE p.source_revision IS NULL OR p.source_revision<>c.trade_count)`).Scan(&lagging); err != nil {
			t.Fatal(err)
		}
		if !lagging {
			break
		}
		if time.Now().After(deadlineAll) {
			t.Fatal("initial candle projection did not finish")
		}
		time.Sleep(20 * time.Millisecond)
	}
	// Read the same web request while real writer and stream commits advance.
	ctx := context.Background()
	projectionLock, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer projectionLock.Rollback(ctx)
	if _, err = projectionLock.Exec(ctx, `LOCK TABLE api_stream_dev.candle_projection IN SHARE MODE`); err != nil {
		t.Fatal(err)
	}
	second := f.swap(t, false, 200000000000000000, 500000, 5000)
	f.project(t, 1)
	if status, raw := f.get(t, path); status != 409 || !strings.Contains(string(raw), `"RESNAPSHOT_REQUIRED"`) {
		t.Fatalf("chart source ahead of stream must resnapshot: %d %s", status, raw)
	}
	if err = projectionLock.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		status, raw := f.get(t, path)
		if status == 200 {
			if interfaces.ValidateJSON("CandlesResponse", raw) != nil {
				t.Fatalf("invalid live page: %s", raw)
			}
		} else if status != 409 || !strings.Contains(string(raw), `"RESNAPSHOT_REQUIRED"`) {
			t.Fatalf("read while committing = %d %s", status, raw)
		}
	}
	f.awaitTrade(t, second)
	after := f.candle(t, path, "2")
	if after.Data.SnapshotId == initial.Data.SnapshotId || after.Data.Items[0].VolumeUsdlRaw != "1500000" || after.Data.Items[0].LastPosition.TransactionHash != strings.ToLower(second.TxHash.Hex()) {
		t.Fatalf("new swap missing: %+v", after)
	}
	// A second day creates a real cursor; a later swap invalidates it.
	if err := f.chain.backend.AdjustTime(24 * time.Hour); err != nil {
		t.Fatal(err)
	}
	third := f.swap(t, true, 1000000, 400000000000000000, 10000)
	f.project(t, 1)
	f.awaitTrade(t, third)
	f.candle(t, path, "2")
	var firstPage interfaces.CandlesResponse
	deadline := time.Now().Add(15 * time.Second)
	for {
		status, raw := f.get(t, path+"&limit=1")
		if status == 200 {
			if err := json.Unmarshal(raw, &firstPage); err != nil {
				t.Fatal(err)
			}
			if firstPage.NextCursor != nil {
				break
			}
		} else if status != 409 {
			t.Fatalf("cursor page = %d %s", status, raw)
		}
		if time.Now().After(deadline) {
			t.Fatal("second day not projected")
		}
		time.Sleep(20 * time.Millisecond)
	}
	fourth := f.swap(t, true, 2000000, 400000000000000000, 20000)
	f.project(t, 1)
	f.awaitTrade(t, fourth)
	// Wait for both daily documents at the new committed revision.
	for {
		status, raw := f.get(t, path)
		if status == 200 {
			var page interfaces.CandlesResponse
			if err := json.Unmarshal(raw, &page); err != nil {
				t.Fatal(err)
			}
			if len(page.Data.Items) == 2 && page.Data.Items[1].Trades == "2" {
				break
			}
		} else if status != 409 {
			t.Fatalf("fourth swap read = %d %s", status, raw)
		}
		if time.Now().After(deadline) {
			t.Fatal("fourth swap not projected")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if status, raw := f.get(t, path+"&limit=1&cursor="+url.QueryEscape(*firstPage.NextCursor)); status != 409 || !strings.Contains(string(raw), `"RESNAPSHOT_REQUIRED"`) {
		t.Fatalf("stale cursor = %d %s", status, raw)
	}
	// Restart both consumers and replay the real source; durable claims prevent
	// duplicate folds and the read snapshot remains stable.
	before := f.candle(t, path, "2")
	f.stop()
	f.source = development.NewSource(f.pool, f.pub, f.client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	f.project(t, 0)
	f.stop = f.start()
	recovered := f.candle(t, path, "2")
	if before.Data.SnapshotId != recovered.Data.SnapshotId || len(recovered.Data.Items) != 2 || recovered.Data.Items[1].Trades != "2" {
		t.Fatalf("replay changed candles: %+v", recovered)
	}
	// A missing dependency remains a typed 503, unlike snapshot movement.
	f.exec(t, `ALTER TABLE candles.candles RENAME TO candles_unavailable`)
	if status, raw := f.get(t, path); status != 503 || !strings.Contains(string(raw), `"DEPENDENCY_UNAVAILABLE"`) {
		t.Fatalf("dependency failure = %d %s", status, raw)
	}
}

func TestRepair_22_39_372_trade_read_serves_committed_swaps(t *testing.T) {
	f := newRepair372Fixture(t)
	f.createPool(t)
	first := f.swap(t, true, 1000000, 400000000000000000, 10000)
	second := f.swap(t, false, 200000000000000000, 500000, 5000)
	f.awaitTrade(t, second)
	path := "/v2/pools/" + dstPool + "/trades"
	read := func(path string, want []*types.Receipt) {
		t.Helper()
		status, raw := f.get(t, path)
		if status != 200 || interfaces.ValidateJSON("TradesResponse", raw) != nil {
			t.Fatalf("trades = %d %s", status, raw)
		}
		var page interfaces.TradesResponse
		if err := json.Unmarshal(raw, &page); err != nil {
			t.Fatal(err)
		}
		if page.Quality != "partial" || page.NextCursor != nil || len(page.Data.Items) != len(want) {
			t.Fatalf("trade page = %s", raw)
		}
		for i, r := range want {
			p := page.Data.Items[i].Position
			if p.TransactionHash != strings.ToLower(r.TxHash.Hex()) || p.BlockHash != strings.ToLower(r.BlockHash.Hex()) || string(p.Height) != r.BlockNumber.String() || p.TransactionIndex != "0" || p.LogIndex != "0" {
				t.Fatalf("trade position = %+v receipt %+v", p, r)
			}
		}
	}
	read(path, []*types.Receipt{second, first})
	read(path+"?limit=1", []*types.Receipt{second})
	read(path+"?side=buy", []*types.Receipt{first})
	read(path+"?side=sell", []*types.Receipt{second})
	f.exec(t, `DELETE FROM api_stream_dev.trade_projection`)
	f.stop()
	f.stop = f.start()
	read(path, []*types.Receipt{second, first})
	f.exec(t, `DELETE FROM api_stream_dev.stream_delivery_outbox`)
	read(path, []*types.Receipt{second, first})
	for q, want := range map[string]int{"?cursor=unexpected": 400, "?limit=101": 422, "?side=other": 422, "?unknown=value": 422} {
		status, raw := f.get(t, path+q)
		if status != want || interfaces.ValidateJSON("Error", raw) != nil {
			t.Fatalf("request checks lost: %d %s", status, raw)
		}
	}
	f.stop()
	f.stop = f.start()
	read(path, []*types.Receipt{second, first})
}

func TestRepair_22_39_372_trade_read_keeps_legacy_rows_quarantined(t *testing.T) {
	f := newRepair372Fixture(t)
	f.createPool(t)
	ctx := context.Background()
	lock, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback(ctx)
	if _, err = lock.Exec(ctx, `LOCK TABLE api_stream_dev.projector_cursor IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	first := f.swap(t, true, 1000000, 400000000000000000, 10000)
	// This indexed row contradicts its real receipt before projection. The
	// actual projector rejects it, while committing the subsequent valid row.
	f.exec(t, `UPDATE indexer.swaps SET amount_out='1' WHERE tx_hash=$1`, strings.ToLower(first.TxHash.Hex()))
	if err = lock.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	second := f.swap(t, false, 200000000000000000, 500000, 5000)
	f.awaitTrade(t, second)
	for _, q := range []string{"", "?limit=1", "?side=buy"} {
		status, raw := f.get(t, "/v2/pools/"+dstPool+"/trades"+q)
		if status != 503 || !strings.Contains(string(raw), `"INTEGRITY_QUARANTINED"`) || interfaces.ValidateJSON("Error", raw) != nil {
			t.Fatalf("unprojected source escaped: %d %s", status, raw)
		}
	}
}

func TestRepair_22_39_372_pool_detail_serves_the_verified_token(t *testing.T) {
	f := newRepair372Fixture(t)
	// Deploy the real immutable SidioraERC20 constructor bytecode, including
	// its storage-backed name, symbol, six decimals and minted total supply.
	raw, err := os.ReadFile("../rest/testdata/repair_22_39_372_token.json")
	if err != nil {
		t.Fatal(err)
	}
	var artifact struct {
		ABI      json.RawMessage `json:"abi"`
		Bytecode string          `json:"bytecode"`
	}
	if err = json.Unmarshal(raw, &artifact); err != nil {
		t.Fatal(err)
	}
	parsed, err := abi.JSON(strings.NewReader(string(artifact.ABI)))
	if err != nil {
		t.Fatal(err)
	}
	supply := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	opts, err := bind.NewKeyedTransactorWithChainID(f.chain.key, big.NewInt(125))
	if err != nil {
		t.Fatal(err)
	}
	opts.GasLimit = 3000000
	address, tx, _, err := bind.DeployContract(opts, parsed, common.FromHex(artifact.Bytecode), f.chain.backend.Client(), "Verified Candle Token", "VCT", supply, common.HexToAddress(dstTrader))
	if err != nil {
		t.Fatal(err)
	}
	f.chain.nonce++
	f.chain.backend.Commit()
	receipt, err := f.chain.backend.Client().TransactionReceipt(context.Background(), tx.Hash())
	if err != nil || receipt == nil || receipt.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("real token deployment: %v %+v", err, receipt)
	}
	f.chain.token = address
	f.createPool(t)
	swap := f.swap(t, true, 1000000, 400000000000000000, 10000)
	f.awaitTrade(t, swap)
	status, body := f.get(t, "/v2/pools/"+dstPool)
	if status != 200 || interfaces.ValidateJSON("PoolDetailResponse", body) != nil {
		t.Fatalf("detail = %d %s", status, body)
	}
	var page interfaces.PoolDetailResponse
	if err = json.Unmarshal(body, &page); err != nil {
		t.Fatal(err)
	}
	token := page.Data.Token
	if token.Name != "Verified Candle Token" || token.Symbol != "VCT" || token.Decimals == nil || *token.Decimals != 6 || token.DecimalsQuality != "verified" || token.TotalSupplyRaw == nil || string(*token.TotalSupplyRaw) != supply.String() || token.SupplyQuality != "verified" || token.SourcePosition == nil || page.Watermark == nil || token.SourcePosition.Height != page.Watermark.Height || token.SourcePosition.BlockHash != page.Watermark.BlockHash || token.SourcePosition.TransactionHash != strings.ToLower(swap.TxHash.Hex()) {
		t.Fatalf("verified token missing provenance: %s", body)
	}
	// An address with no code on this chain holds none of those values.
	f.exec(t, `UPDATE indexer.pools SET token_address=$1 WHERE pool_address=$2`, dstCreator, dstPool)
	status, body = f.get(t, "/v2/pools/"+dstPool)
	if status != 200 || interfaces.ValidateJSON("PoolDetailResponse", body) != nil {
		t.Fatalf("unavailable metadata = %d %s", status, body)
	}
	if err = json.Unmarshal(body, &page); err != nil {
		t.Fatal(err)
	}
	token = page.Data.Token
	if token.Name != "" || token.Symbol != "" || token.Decimals != nil || token.TotalSupplyRaw != nil || token.DecimalsQuality != "unavailable" || token.SupplyQuality != "unavailable" || token.SourcePosition != nil {
		t.Fatalf("metadata invented: %s", body)
	}
}
