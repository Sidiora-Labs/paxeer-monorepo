package indexer

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
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
	"github.com/prometheus/client_golang/prometheus"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/decode"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/delivery"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/publish"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/source"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/telemetry"
)

var repair209Tables = []string{"positions_current", "orders", "spot_orders", "account_margin", "user_stats", "user_pnl_daily", "market_stats_rolling", "leaderboard", "spot_token_seen", "market_snapshots", "protocol_snapshots", "keeper_cycles", "spot_leaderboard", "spot_account_legs"}
var repair209Account = common.HexToAddress("0x2091")
var repair209Other = common.HexToAddress("0x2092")
var repair209Orphan = common.HexToAddress("0x2093")
var repair209Token = common.HexToAddress("0x2094")
var repair209Stable = common.HexToAddress("0x2095")

func repair209Indexer(db *sql.DB, holder string) *Indexer {
	return &Indexer{db: db, holder: holder, start: 1, logger: slog.Default(), cursorHeight: prometheus.NewGauge(prometheus.GaugeOpts{Name: "repair209_cursor"}), lagBlocks: prometheus.NewGauge(prometheus.GaugeOpts{Name: "repair209_lag"})}
}

func repair209DB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := store.Open(requireEnv(t, testDatabaseEnv))
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
	return db
}

func repair209Snapshot(t *testing.T, db *sql.DB) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, table := range repair209Tables {
		var data string
		if err := db.QueryRow(`SELECT COALESCE(jsonb_agg(v ORDER BY v::text), '[]'::jsonb)::text FROM (SELECT to_jsonb(t) v FROM perps.` + table + ` t) rows`).Scan(&data); err != nil {
			t.Fatal(err)
		}
		out[table] = data
	}
	return out
}

func repair209Equal(t *testing.T, got, want map[string]string) {
	t.Helper()
	for _, table := range repair209Tables {
		if got[table] != want[table] {
			t.Errorf("%s after recovery = %s; fresh fold = %s", table, got[table], want[table])
		}
	}
}

// The producer is WriteBlock, including its real fact codec, projection folds,
// daily PnL, spot fold, snapshots and transactional outbox. No store or callback
// is replaced. The second block changes pre-existing rows and adds entities
// whose entire history disappears on rewind.
func repair209Write(t *testing.T, db *sql.DB, b store.Block, fork bool) {
	t.Helper()
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO perps.blocks(number, hash, parent_hash, time, proposer, tx_count) VALUES($1,$2,$3,$4,$5,$6)`, b.Number, b.Hash.Bytes(), b.ParentHash.Bytes(), b.Time, b.Proposer, b.TxCount); err != nil {
		t.Fatal(err)
	}
	var decoded []decode.Decoded
	index := int64(0)
	fact := func(kind string, account common.Address) decode.Fact {
		index++
		return decode.Fact{Block: b.Number, BlockHash: b.Hash, BlockTime: b.Time, TxIndex: 0, LogIndex: index, TxHash: crypto.Keccak256Hash([]byte(fmt.Sprintf("tx/%d", b.Number))), ReceiptKind: decode.ReceiptTx, Emitter: decode.PerpsAddress, Kind: kind, Account: account, TxFrom: &account}
	}
	put := func(row decode.Row) {
		decoded = append(decoded, decode.Decoded{Event: decode.Event{EventName: row.Common().Kind}, Rows: []decode.Row{row}})
	}
	deposit := func(a common.Address, amount string) {
		put(decode.CollateralDepositedRow{Fact: fact("deposited", a), AccountArg: a, Amount: amount, NewBalance: amount})
	}
	open := func(a common.Address, id, market string) {
		put(decode.PositionOpenedRow{Fact: fact("opened", a), User: a, PositionID: id, MarketID: market, IsLong: true, SizeUSD: "100", Leverage: "2", EntryPrice: "10", CollateralToken: decode.USDLAddress, CollateralAmount: "1"})
	}
	order := func(a common.Address, id, market string) {
		put(decode.OrderPlacedRow{Fact: fact("placed", a), OrderID: id, User: a, MarketID: market, OrderType: 0, IsLong: true, TriggerPrice: "10", SizeUSD: "100"})
	}
	swap := func(a common.Address, buy bool, amount string) {
		put(decode.SpotMarketOrderRow{Fact: fact("market_order", a), User: a, Stablecoin: repair209Stable, Token: repair209Token, StablecoinAmount: amount, TokenAmount: "10", IsBuy: buy})
	}
	if !fork {
		deposit(repair209Account, "100")
		deposit(repair209Other, "200")
		open(repair209Account, "2091", "1")
		open(repair209Other, "2092", "1")
		order(repair209Account, "2091", "1")
		order(repair209Other, "2092", "1")
		put(decode.SpotTokenRegisteredRow{Fact: fact("token_registered", repair209Account), Token: repair209Stable, Decimals: 18, IsStablecoin: true})
		put(decode.SpotTokenRegisteredRow{Fact: fact("token_registered", repair209Account), Token: repair209Token, Decimals: 18})
		swap(repair209Account, true, "100")
		swap(repair209Other, true, "200")
		put(decode.SpotSimpleSwapRow{Fact: fact("simple_swap", repair209Account), User: repair209Account, TokenIn: repair209Stable, TokenOut: repair209Token, AmountIn: "10", AmountOut: "1", ExecutionPrice: "10"})
		put(decode.SpotLimitCreatedRow{Fact: fact("limit_created", repair209Account), OrderID: "2091", User: repair209Account, OrderType: 0, Amount: "10", TargetPrice: "10", Token: repair209Token, Stablecoin: repair209Stable})
		put(decode.SpotLimitCreatedRow{Fact: fact("limit_created", repair209Other), OrderID: "2092", User: repair209Other, OrderType: 0, Amount: "10", TargetPrice: "10", Token: repair209Token, Stablecoin: repair209Stable})
		put(decode.SpotOrderExecutedRow{Fact: fact("limit_executed", repair209Other), OrderID: "2092", Token: repair209Token, Stablecoin: repair209Stable, ExecutionPrice: "1000000000000000000"})
		put(decode.MarkPriceUpdatedRow{Fact: fact("mark", repair209Account), MarketID: "1", MarkPrice: "10", IndexPrice: "10", MidPrice: "10"})
	} else {
		deposit(repair209Account, "25")
		put(decode.PositionClosedRow{Fact: fact("closed", repair209Account), PositionID: "2091", User: repair209Account, MarketID: "1", ClosedSizeUSD: "100", ExitPrice: "12", RealizedPnl: "20", IsFullClose: true})
		put(decode.OrderCancelledRow{Fact: fact("cancelled", repair209Account), OrderID: "2091", User: repair209Account})
		put(decode.TradeSettledRow{Fact: fact("settled", repair209Account), PositionID: "2091", User: repair209Account, MarketID: "1", SizeUSD: "100", ExecutionPrice: "12", GrossPnl: "20", TotalFeesUSD: "0", FundingPaidUSD: "0", NetPayoutTokens: "1"})
		open(repair209Orphan, "2093", "209")
		order(repair209Orphan, "2093", "209")
		swap(repair209Account, false, "150")
		swap(repair209Orphan, true, "50")
		put(decode.SpotOrderExecutedRow{Fact: fact("limit_executed", repair209Account), OrderID: "2091", Token: repair209Token, Stablecoin: repair209Stable, ExecutionPrice: "1000000000000000000"})
		put(decode.SpotTokenRegisteredRow{Fact: fact("token_registered", repair209Orphan), Token: common.HexToAddress("0x2096"), Decimals: 18})
		put(decode.OpenInterestChangedRow{Fact: fact("oi", repair209Orphan), MarketID: "209", LongOI: "100", ShortOI: "0", DeltaUSD: "100", IsIncrease: true})
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

func repair209Blocks() (store.Block, store.Block) {
	at := time.Date(2026, 3, 2, 10, 15, 0, 0, time.UTC)
	one := store.Block{Number: 1, Hash: crypto.Keccak256Hash([]byte("canonical/1")), Time: at, Proposer: strings.Repeat("AB", 20), TxCount: 1}
	two := store.Block{Number: 2, Hash: crypto.Keccak256Hash([]byte("fork/2")), ParentHash: one.Hash, Time: at.Add(2 * time.Hour), Proposer: one.Proposer, TxCount: 1}
	return one, two
}

// Delete only facts/blocks, as guaranteed by the callback contract. Leaving all
// old projections here makes the test assert the adapter's own orphan cleanup.
func repair209Delete(t *testing.T, tx *sql.Tx, from uint64) {
	t.Helper()
	tables := map[string]bool{"projection_seeds": true, "raw_logs": true, "protocol_events": true, "tx_actions": true, "blocks": true}
	for _, kind := range decode.KindRows() {
		tables[kind.Table] = true
	}
	for table := range tables {
		column := "block"
		if table == "blocks" {
			column = "number"
		}
		if _, err := tx.Exec(`DELETE FROM perps.`+table+` WHERE `+column+` >= $1`, from); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRepair_209_RefoldAndRestart(t *testing.T) {
	db := repair209DB(t)
	ctx := context.Background()
	ix := repair209Indexer(db, "repair209")
	if err := ix.acquire(ctx); err != nil {
		t.Fatal(err)
	}
	one, two := repair209Blocks()
	repair209Write(t, db, one, false)
	want := repair209Snapshot(t, db)
	repair209Write(t, db, two, true)
	before := repair209Snapshot(t, db)
	for _, table := range []string{"user_pnl_daily", "market_stats_rolling", "leaderboard", "protocol_snapshots"} {
		if before[table] == "[]" {
			t.Fatalf("fork did not populate %s", table)
		}
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	repair209Delete(t, tx, 2)
	if err := ix.rebuild(ctx, tx, source.Affected{From: 2, FromTime: two.Time, Accounts: []common.Address{repair209Account, repair209Orphan}, Markets: []string{"1", "209"}}); err != nil {
		t.Fatal(err)
	}
	// Until commit, readers still observe the complete fork, never a half fold.
	repair209Equal(t, repair209Snapshot(t, db), before)
	if _, err := tx.Exec(`UPDATE perps.cursors SET height=$1,hash=$2 WHERE name='head'`, one.Number, one.Hash.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	repair209Equal(t, repair209Snapshot(t, db), want)
	ix.release(ctx)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := store.Open(requireEnv(t, testDatabaseEnv))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	restarted := repair209Indexer(reopened, "repair209-restarted")
	if err := restarted.acquire(ctx); err != nil {
		t.Fatal(err)
	}
	defer restarted.release(ctx)
	if next, err := restarted.resume(ctx); err != nil || next != 2 {
		t.Fatalf("restart next=%d err=%v", next, err)
	}
	repair209Equal(t, repair209Snapshot(t, reopened), want)
}

func TestRepair_209_EmptySurvivorAndRollback(t *testing.T) {
	db := repair209DB(t)
	ctx := context.Background()
	ix := repair209Indexer(db, "repair209-empty")
	if err := ix.acquire(ctx); err != nil {
		t.Fatal(err)
	}
	defer ix.release(ctx)
	one, _ := repair209Blocks()
	empty := repair209Snapshot(t, db)
	repair209Write(t, db, one, false)
	before := repair209Snapshot(t, db)
	a := source.Affected{From: 1, FromTime: one.Time, Accounts: []common.Address{repair209Account, repair209Other}, Markets: []string{"1"}}
	for _, commit := range []bool{false, true} {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		repair209Delete(t, tx, 1)
		if err := ix.rebuild(ctx, tx, a); err != nil {
			tx.Rollback()
			t.Fatal(err)
		}
		if commit {
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
			repair209Equal(t, repair209Snapshot(t, db), empty)
		} else {
			if err := tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			repair209Equal(t, repair209Snapshot(t, db), before)
		}
	}
}

type repair209Exchange struct {
	Method string          `json:"method"`
	Params any             `json:"params"`
	Result json.RawMessage `json:"result"`
}

func repair209RPC(t *testing.T) (string, func()) {
	t.Helper()
	wire, err := os.ReadFile("testdata/repair_209_rpc.json")
	if err != nil {
		t.Fatal(err)
	}
	var transcript []repair209Exchange
	if err := json.Unmarshal(wire, &transcript); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	seen := make([]int, len(transcript))
	node := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string          `json:"method"`
			Params any             `json:"params"`
			ID     json.RawMessage `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("RPC request: %v", err)
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		for i, exchange := range transcript {
			if req.Method != exchange.Method || !reflect.DeepEqual(req.Params, exchange.Params) {
				continue
			}
			mu.Lock()
			seen[i]++
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(struct {
				Version string          `json:"jsonrpc"`
				ID      json.RawMessage `json:"id"`
				Result  json.RawMessage `json:"result"`
			}{"2.0", req.ID, exchange.Result}); err != nil {
				t.Errorf("RPC response: %v", err)
			}
			return
		}
		t.Errorf("unrecorded RPC request: %s %v", req.Method, req.Params)
		http.Error(w, "unrecorded request", http.StatusBadRequest)
	}))
	t.Cleanup(node.Close)
	return node.URL, func() {
		mu.Lock()
		defer mu.Unlock()
		for i, count := range seen {
			if count == 0 {
				t.Errorf("unused RPC exchange: %s %v", transcript[i].Method, transcript[i].Params)
			}
		}
	}
}

func repair209NewFollowerIndexer(t *testing.T, db *sql.DB, url, holder string) (*Indexer, func()) {
	t.Helper()
	ctx := context.Background()
	pool, err := source.NewRPC(ctx, source.RPCOptions{URLs: []string{url}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	comet, err := source.NewComet([]string{url}, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	metrics, err := telemetry.New(config.Indexer)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := decode.NewRegistry(metrics, decode.Options{
		PerpsModule: decode.ModuleAddress("perps"), SpotModule: decode.ModuleAddress("spot"),
		PricefeedModule: decode.ModuleAddress("pricefeed"), State: pool,
	})
	if err != nil {
		t.Fatal(err)
	}
	outbox, err := delivery.NewStore(db, "perps", store.Policy(125))
	if err != nil {
		t.Fatal(err)
	}
	broker, err := server.NewServer(&server.Options{Host: "127.0.0.1", Port: server.RANDOM_PORT, NoLog: true, NoSigs: true})
	if err != nil {
		t.Fatal(err)
	}
	go broker.Start()
	t.Cleanup(broker.Shutdown)
	if !broker.ReadyForConnections(10 * time.Second) {
		t.Fatal("in-process broker did not start")
	}
	nc, err := nats.Connect(broker.ClientURL())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	publisher, err := publish.New(db, nc, store.Policy(125), publish.Config{
		Owner: holder, Batch: 100, MaxAttempts: 20, LeaseDuration: LeaseTTL,
		PublishTimeout: 5 * time.Second, RetryDelay: time.Second, IdleInterval: IdleInterval, MaxHeadAge: MaxOutboxAge,
	})
	if err != nil {
		t.Fatal(err)
	}
	ix, err := New(ctx, Deps{DB: db, RPC: pool, Comet: comet, Outbox: outbox, ChainID: 125,
		Registry: registry, Publisher: publisher, Metrics: metrics, Holder: holder, StartHeight: 1})
	if err != nil {
		t.Fatal(err)
	}
	return ix, func() { pool.Close(); nc.Close(); broker.Shutdown() }
}

// Exercise both sides of FollowerOptions.Rebuild over loopback RPC and Comet
// transcripts, including a reopened indexer and continued log consumption.
func TestRepair_209_FollowerRecovery(t *testing.T) {
	t.Setenv(testRPCEnv, "")
	t.Setenv(testCometEnv, "")
	db := repair209DB(t)
	url, assertConsumed := repair209RPC(t)
	ctx := context.Background()
	ix, stop := repair209NewFollowerIndexer(t, db, url, "repair209-follower")
	if err := ix.acquire(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if ix.held.Load() {
			ix.release(ctx)
		}
	}()
	h, err := ix.rpc.Block(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	one, two := repair209Blocks()
	one.Hash, one.ParentHash = h.Hash, h.ParentHash
	repair209Write(t, db, one, false)
	want := repair209Snapshot(t, db)
	two.ParentHash = one.Hash
	repair209Write(t, db, two, true)
	if next, err := ix.step(ctx, 3); err != nil || next != 2 {
		t.Fatalf("rewind next=%d err=%v", next, err)
	}
	repair209Equal(t, repair209Snapshot(t, db), want)
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM perps.delivery_outbox WHERE subject=$1`, "perps.125.chain.rewind").Scan(&count); err != nil || count != 1 {
		t.Fatalf("rewind notifications=%d err=%v", count, err)
	}
	var envelope delivery.Envelope
	var wire []byte
	if err := db.QueryRow(`SELECT envelope FROM perps.delivery_outbox WHERE subject=$1`, "perps.125.chain.rewind").Scan(&wire); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(wire, &envelope); err != nil {
		t.Fatal(err)
	}
	if string(envelope.Payload) != `{"from":2}` || envelope.PayloadHash != delivery.Hash(envelope.Payload) {
		t.Fatalf("rewind envelope: %+v", envelope)
	}
	ix.release(ctx)
	stop()
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := store.Open(requireEnv(t, testDatabaseEnv))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	restarted, stopRestarted := repair209NewFollowerIndexer(t, reopened, url, "repair209-restarted-follower")
	defer stopRestarted()
	if err := restarted.acquire(ctx); err != nil {
		t.Fatal(err)
	}
	defer restarted.release(ctx)
	if next, err := restarted.resume(ctx); err != nil || next != 2 {
		t.Fatalf("restart next=%d err=%v", next, err)
	}
	repair209Equal(t, repair209Snapshot(t, reopened), want)
	if next, err := restarted.step(ctx, 2); err != nil || next != 4 {
		t.Fatalf("canonical replay next=%d err=%v", next, err)
	}
	if err := reopened.QueryRow(`SELECT count(*) FROM perps.usdl_transfers WHERE block=3 AND account=$1 AND value=7 AND kind='in'`, repair209Other.Bytes()).Scan(&count); err != nil || count != 1 {
		t.Fatalf("recorded log consumed %d times, err=%v", count, err)
	}
	if err := reopened.QueryRow(`SELECT count(*) FROM perps.delivery_outbox WHERE subject=$1`, "perps.125.chain.rewind").Scan(&count); err != nil || count != 1 {
		t.Fatalf("rewind notifications after restart=%d err=%v", count, err)
	}
	assertConsumed()
}

// A rewind deletes the entire hour containing the fork, including a snapshot
// written by a surviving block earlier in that hour. It must be regenerated.
func TestRepair_209_SnapshotBoundary(t *testing.T) {
	db := repair209DB(t)
	ctx := context.Background()
	ix := repair209Indexer(db, "repair209-snapshots")
	if err := ix.acquire(ctx); err != nil {
		t.Fatal(err)
	}
	defer ix.release(ctx)
	one, fork := repair209Blocks()
	repair209Write(t, db, one, false)
	two := one
	two.Number, two.Hash, two.ParentHash, two.Time = 2, crypto.Keccak256Hash([]byte("canonical/2")), one.Hash, one.Time.Add(time.Hour)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO perps.blocks(number,hash,parent_hash,time,proposer,tx_count) VALUES($1,$2,$3,$4,$5,0)`, two.Number, two.Hash.Bytes(), two.ParentHash.Bytes(), two.Time, two.Proposer); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteBlock(ctx, tx, two, &decode.Result{}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	want := repair209Snapshot(t, db)
	if want["market_snapshots"] == "[]" || want["protocol_snapshots"] == "[]" {
		t.Fatal("canonical history did not produce snapshots")
	}
	fork.Number, fork.Hash, fork.ParentHash, fork.Time = 3, crypto.Keccak256Hash([]byte("fork/3")), two.Hash, two.Time.Add(30*time.Minute)
	repair209Write(t, db, fork, true)
	tx, err = db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	repair209Delete(t, tx, 3)
	for _, table := range []string{"market_snapshots", "protocol_snapshots"} {
		if _, err := tx.Exec(`DELETE FROM perps.`+table+` WHERE hour >= date_trunc('hour',$1::timestamptz)`, fork.Time); err != nil {
			t.Fatal(err)
		}
	}
	if err := ix.rebuild(ctx, tx, source.Affected{From: 3, FromTime: fork.Time, Accounts: []common.Address{repair209Account, repair209Orphan}, Markets: []string{"1", "209"}}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	repair209Equal(t, repair209Snapshot(t, db), want)
}

func TestRepair_209_CapturedScope(t *testing.T) {
	_, fork := repair209Blocks()
	a := source.Affected{From: 2, FromTime: fork.Time,
		Accounts: []common.Address{repair209Account, repair209Orphan}, Markets: []string{"1", "209"}}
	from, got, err := rewindRecovery(a)
	if err != nil {
		t.Fatal(err)
	}
	want := store.Recovery{Accounts: a.Accounts, Markets: a.Markets, FromTime: a.FromTime}
	if from != 2 || !reflect.DeepEqual(got, want) {
		t.Fatalf("captured scope: height=%d recovery=%+v; want height=2 recovery=%+v", from, got, want)
	}
	// The store must refuse this real adapter output outside a transaction.
	if err := store.Rebuild(context.Background(), nil, from, from, got); err == nil || err.Error() != "store: Rebuild needs a transaction" {
		t.Fatalf("store accepted recovery without a transaction: %v", err)
	}
}

func TestRepair_209_InvalidRecovery(t *testing.T) {
	for _, a := range []source.Affected{
		{From: uint64(math.MaxInt64) + 1},
		{From: 2, Accounts: []common.Address{repair209Account}},
		{From: 2, Markets: []string{"209"}},
	} {
		if _, _, err := rewindRecovery(a); err == nil {
			t.Fatalf("accepted invalid recovery %+v", a)
		}
	}
	if from, _, err := rewindRecovery(source.Affected{From: math.MaxInt64}); err != nil || from != math.MaxInt64 {
		t.Fatalf("maximum database height: from=%d err=%v", from, err)
	}
	if err := rebuildAffected(context.Background(), nil, source.Affected{From: 1}); err == nil || err.Error() != "indexer: rewind rebuild needs a transaction" {
		t.Fatalf("rebuild accepted missing transaction: %v", err)
	}
}
