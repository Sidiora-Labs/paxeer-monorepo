package source_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/decode"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/delivery"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/indexer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/publish"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/source"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/stream"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/telemetry"
)

var repair1483Account = common.HexToAddress("0x2091")
var repair1483Other = common.HexToAddress("0x2092")
var repair1483Orphan = common.HexToAddress("0x2093")
var repair1483Token = common.HexToAddress("0x2094")
var repair1483Stable = common.HexToAddress("0x2095")
var repair1483Tables = []string{"positions_current", "orders", "account_margin", "user_stats", "user_pnl_daily", "market_stats_rolling", "leaderboard", "spot_account_legs", "spot_leaderboard"}

func repair1483Write(t *testing.T, db *sql.DB, b store.Block, fork bool) {
	t.Helper()
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO perps.blocks(number,hash,parent_hash,time,proposer,tx_count) VALUES($1,$2,$3,$4,$5,$6)`, b.Number, b.Hash.Bytes(), b.ParentHash.Bytes(), b.Time, b.Proposer, b.TxCount); err != nil {
		t.Fatal(err)
	}
	var decoded []decode.Decoded
	var index int64
	fact := func(kind string, account common.Address) decode.Fact {
		index++
		return decode.Fact{Block: b.Number, BlockHash: b.Hash, BlockTime: b.Time, TxIndex: 0, LogIndex: index, TxHash: crypto.Keccak256Hash([]byte(fmt.Sprintf("tx/%d", b.Number))), ReceiptKind: decode.ReceiptTx, Emitter: decode.PerpsAddress, Kind: kind, Account: account, TxFrom: &account}
	}
	put := func(row decode.Row) {
		decoded = append(decoded, decode.Decoded{Event: decode.Event{EventName: row.Common().Kind}, Rows: []decode.Row{row}})
	}
	open := func(account common.Address, id, market string) {
		put(decode.PositionOpenedRow{Fact: fact("opened", account), User: account, PositionID: id, MarketID: market, IsLong: true, SizeUSD: "100", Leverage: "2", EntryPrice: "10", CollateralToken: decode.USDLAddress, CollateralAmount: "1"})
		put(decode.OrderPlacedRow{Fact: fact("placed", account), OrderID: id, User: account, MarketID: market, OrderType: 0, IsLong: true, TriggerPrice: "10", SizeUSD: "100"})
	}
	swap := func(account common.Address, buy bool, amount string) {
		put(decode.SpotMarketOrderRow{Fact: fact("market_order", account), User: account, Stablecoin: repair1483Stable, Token: repair1483Token, StablecoinAmount: amount, TokenAmount: "10", IsBuy: buy})
	}
	if !fork {
		for i, account := range []common.Address{repair1483Account, repair1483Other} {
			put(decode.CollateralDepositedRow{Fact: fact("deposited", account), AccountArg: account, Amount: "100", NewBalance: "100"})
			open(account, fmt.Sprint(2091+i), "1")
		}
		put(decode.SpotTokenRegisteredRow{Fact: fact("token_registered", repair1483Account), Token: repair1483Stable, Decimals: 18, IsStablecoin: true})
		put(decode.SpotTokenRegisteredRow{Fact: fact("token_registered", repair1483Account), Token: repair1483Token, Decimals: 18})
		swap(repair1483Account, true, "100")
		swap(repair1483Other, true, "200")
		put(decode.MarkPriceUpdatedRow{Fact: fact("mark", repair1483Account), MarketID: "1", MarkPrice: "10", IndexPrice: "10", MidPrice: "10"})
	} else {
		put(decode.PositionClosedRow{Fact: fact("closed", repair1483Account), PositionID: "2091", User: repair1483Account, MarketID: "1", ClosedSizeUSD: "100", ExitPrice: "12", RealizedPnl: "20", IsFullClose: true})
		put(decode.OrderCancelledRow{Fact: fact("cancelled", repair1483Account), OrderID: "2091", User: repair1483Account})
		put(decode.TradeSettledRow{Fact: fact("settled", repair1483Account), PositionID: "2091", User: repair1483Account, MarketID: "1", SizeUSD: "100", ExecutionPrice: "12", GrossPnl: "20", TotalFeesUSD: "0", FundingPaidUSD: "0", NetPayoutTokens: "1"})
		open(repair1483Orphan, "2093", "209")
		swap(repair1483Account, false, "150")
		swap(repair1483Orphan, true, "50")
		put(decode.OpenInterestChangedRow{Fact: fact("oi", repair1483Orphan), MarketID: "209", LongOI: "100", ShortOI: "0", DeltaUSD: "100", IsIncrease: true})
	}
	if err := store.WriteBlock(ctx, tx, b, &decode.Result{Decoded: decoded}); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`INSERT INTO perps.cursors(name,height,hash) VALUES('head',$1,$2) ON CONFLICT(name) DO UPDATE SET height=EXCLUDED.height,hash=EXCLUDED.hash`, b.Number, b.Hash.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func repair1483Snapshot(t *testing.T, db *sql.DB, account *common.Address) map[string]string {
	t.Helper()
	out := make(map[string]string)
	for _, table := range repair1483Tables {
		where := ""
		var args []any
		if account != nil {
			if table == "market_stats_rolling" {
				continue
			}
			where = " WHERE account=$1"
			args = []any{account.Bytes()}
		}
		var data string
		if err := db.QueryRow(`SELECT COALESCE(jsonb_agg(v ORDER BY v::text),'[]'::jsonb)::text FROM (SELECT to_jsonb(t) v FROM perps.`+table+` t`+where+`) rows`, args...).Scan(&data); err != nil {
			t.Fatal(err)
		}
		out[table] = data
	}
	return out
}

func repair1483Node(t *testing.T) (string, func()) {
	t.Helper()
	wire, err := os.ReadFile("testdata/repair_m1483_rpc.json")
	if err != nil {
		t.Fatal(err)
	}
	var exchanges []struct {
		Method string          `json:"method"`
		Params any             `json:"params"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(wire, &exchanges); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	seen := make([]int, len(exchanges))
	node := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Method string          `json:"method"`
			Params any             `json:"params"`
			ID     json.RawMessage `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			http.Error(w, "invalid request", 400)
			return
		}
		for i, exchange := range exchanges {
			if request.Method == exchange.Method && reflect.DeepEqual(request.Params, exchange.Params) {
				mu.Lock()
				seen[i]++
				mu.Unlock()
				_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": exchange.Result})
				return
			}
		}
		t.Errorf("unrecorded request: %s %v", request.Method, request.Params)
		http.Error(w, "unrecorded request", 400)
	}))
	t.Cleanup(node.Close)
	return node.URL, func() {
		mu.Lock()
		defer mu.Unlock()
		for i, count := range seen {
			if count == 0 {
				t.Errorf("unused exchange: %s %v", exchanges[i].Method, exchanges[i].Params)
			}
		}
	}
}

type repair1483RewindLog struct {
	capture repair1483Capture
	once    sync.Once
	rewound func()
}

func (w *repair1483RewindLog) Write(p []byte) (int, error) {
	n, err := w.capture.Write(p)
	if strings.Contains(string(p), "perps indexer rewound") && w.rewound != nil {
		w.once.Do(w.rewound)
	}
	return n, err
}

func repair1483Indexer(t *testing.T, db *sql.DB, url, holder string, logger *slog.Logger) (*indexer.Indexer, *source.RPC, func()) {
	t.Helper()
	ctx := context.Background()
	pool, err := source.NewRPC(ctx, source.RPCOptions{URLs: []string{url}, Logger: logger, HealthInterval: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	comet, err := source.NewComet([]string{url}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	metrics, err := telemetry.New(config.Indexer)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := decode.NewRegistry(metrics, decode.Options{PerpsModule: decode.ModuleAddress("perps"), SpotModule: decode.ModuleAddress("spot"), PricefeedModule: decode.ModuleAddress("pricefeed"), State: pool})
	if err != nil {
		t.Fatal(err)
	}
	outbox, err := delivery.NewStore(db, "perps", store.Policy(125))
	if err != nil {
		t.Fatal(err)
	}
	broker, err := server.NewServer(&server.Options{Host: "127.0.0.1", Port: server.RANDOM_PORT, JetStream: true, StoreDir: t.TempDir(), NoLog: true, NoSigs: true})
	if err != nil {
		t.Fatal(err)
	}
	go broker.Start()
	t.Cleanup(broker.Shutdown)
	if !broker.ReadyForConnections(5 * time.Second) {
		t.Fatal("broker did not start")
	}
	nc, err := nats.Connect(broker.ClientURL())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	streamConfig := stream.Config(125)
	streamConfig.Replicas = 1
	if _, err := js.CreateStream(ctx, streamConfig); err != nil {
		t.Fatal(err)
	}
	publisher, err := publish.New(db, nc, store.Policy(125), publish.Config{Owner: holder, Batch: 100, MaxAttempts: 20, LeaseDuration: indexer.LeaseTTL, PublishTimeout: time.Second, RetryDelay: time.Second, IdleInterval: 10 * time.Millisecond, MaxHeadAge: indexer.MaxOutboxAge, Logger: logger})
	if err != nil {
		t.Fatal(err)
	}
	ix, err := indexer.New(ctx, indexer.Deps{DB: db, RPC: pool, Comet: comet, Outbox: outbox, ChainID: 125, Registry: registry, Publisher: publisher, Metrics: metrics, Holder: holder, StartHeight: 1, Logger: logger})
	if err != nil {
		t.Fatal(err)
	}
	return ix, pool, func() { pool.Close(); nc.Close(); broker.Shutdown(); broker.WaitForShutdown() }
}

// This proof uses the real PostgreSQL store. A service-free store backend is
// outside this task's scope; the task record keeps that prerequisite explicit.
func repair1483FollowerRecovery(t *testing.T) {
	dsn := os.Getenv("PERPS_DATA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("production recovery needs the existing PostgreSQL fixture; no service-free store backend exists")
	}
	db, err := store.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`DROP SCHEMA IF EXISTS perps CASCADE`); err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	url, assertConsumed := repair1483Node(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var log repair1483RewindLog
	log.rewound = func() {
		var left float64
		if err := db.QueryRow(`SELECT EXTRACT(EPOCH FROM expires_at-clock_timestamp()) FROM perps.writer_lease WHERE name='indexer' AND holder='repair1483-first'`).Scan(&left); err != nil {
			t.Error(err)
		} else if left < (indexer.LeaseTTL - 5*time.Second).Seconds() {
			t.Errorf("recovery lease has only %.1fs left", left)
		}
		cancel()
	}
	logger := slog.New(slog.NewTextHandler(&log, nil))
	ix, pool, stop := repair1483Indexer(t, db, url, "repair1483-first", logger)
	header, err := pool.Block(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	one := store.Block{Number: 1, Hash: header.Hash, ParentHash: header.ParentHash, Time: time.Unix(header.Time, 0).UTC(), Proposer: strings.Repeat("AB", 20), TxCount: 1}
	fork := store.Block{Number: 2, Hash: crypto.Keccak256Hash([]byte("fork/2")), ParentHash: one.Hash, Time: one.Time.Add(2 * time.Hour), Proposer: one.Proposer, TxCount: 1}
	repair1483Write(t, db, one, false)
	canonical := repair1483Snapshot(t, db, nil)
	other := repair1483Snapshot(t, db, &repair1483Other)
	repair1483Write(t, db, fork, true)
	forkState := repair1483Snapshot(t, db, nil)
	for _, table := range []string{"positions_current", "orders", "user_pnl_daily", "market_stats_rolling", "spot_account_legs", "spot_leaderboard"} {
		if forkState[table] == "[]" || forkState[table] == canonical[table] {
			t.Fatalf("fork fixture did not populate/change %s", table)
		}
	}
	if err := ix.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("recovery Run: %v; %s", err, log.capture.String())
	}
	repair1483Contains(t, log.capture.String(), "perps indexer rewound")
	if got := repair1483Snapshot(t, db, nil); !reflect.DeepEqual(got, canonical) {
		t.Fatalf("recovery differs from canonical fold:\ngot %v\nwant %v", got, canonical)
	}
	if got := repair1483Snapshot(t, db, &repair1483Other); !reflect.DeepEqual(got, other) {
		t.Fatal("recovery changed unrelated account rows")
	}
	for _, table := range []string{"position_events", "order_events", "trade_settlements", "spot_swaps"} {
		var count int
		if err := db.QueryRow(`SELECT count(*) FROM perps.` + table + ` WHERE block>=2`).Scan(&count); err != nil || count != 0 {
			t.Fatalf("fork facts in %s=%d err=%v", table, count, err)
		}
	}
	if height, ok, err := store.Cursor(context.Background(), db); err != nil || !ok || height != 1 {
		t.Fatalf("recovery cursor=%d present=%t err=%v", height, ok, err)
	}
	var wire []byte
	if err := db.QueryRow(`SELECT envelope FROM perps.delivery_outbox WHERE subject='perps.125.chain.rewind'`).Scan(&wire); err != nil {
		t.Fatal(err)
	}
	var envelope delivery.Envelope
	if err := json.Unmarshal(wire, &envelope); err != nil {
		t.Fatal(err)
	}
	if string(envelope.Payload) != `{"from":2}` || envelope.PayloadHash != delivery.Hash(envelope.Payload) {
		t.Fatalf("rewind notification %+v", envelope)
	}
	stop()
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := store.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if got := repair1483Snapshot(t, reopened, nil); !reflect.DeepEqual(got, canonical) {
		t.Fatal("reopened recovery differs from canonical fold")
	}
	if got := repair1483Snapshot(t, reopened, &repair1483Other); !reflect.DeepEqual(got, other) {
		t.Fatal("reopened recovery changed unrelated account")
	}
	restarted, _, stopRestarted := repair1483Indexer(t, reopened, url, "repair1483-second", slog.New(slog.NewTextHandler(&repair1483Capture{}, nil)))
	defer stopRestarted()
	replayCtx, replayCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer replayCancel()
	result := make(chan error, 1)
	go func() { result <- restarted.Run(replayCtx) }()
	for {
		height, ok, err := store.Cursor(context.Background(), reopened)
		if err != nil {
			replayCancel()
			<-result
			t.Fatal(err)
		}
		if ok && height == 3 {
			break
		}
		select {
		case err := <-result:
			t.Fatalf("restarted indexer stopped before canonical replay: %v", err)
		case <-time.After(5 * time.Millisecond):
		}
	}
	replayCancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("restart shutdown: %v", err)
	}
	var count int
	if err := reopened.QueryRow(`SELECT count(*) FROM perps.usdl_transfers WHERE block=3 AND account=$1 AND value=7 AND kind='in'`, repair1483Other.Bytes()).Scan(&count); err != nil || count != 1 {
		t.Fatalf("canonical producer log consumed %d times err=%v", count, err)
	}
	assertConsumed()
}
