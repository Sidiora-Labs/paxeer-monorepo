package loadtest

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/decode"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/feeds"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/gateway"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/markets"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/publish"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/rest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/stream"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/telemetry"
)

const (
	testDatabaseEnv = "PERPS_DATA_TEST_DATABASE_URL"
	testRPCEnv      = "PERPS_DATA_TEST_RPC"
	testChain       = 125
	testSecret      = "loadtest-secret-of-at-least-32-bytes!"
	testSockets     = 2000
	// headBlock is the head cursor of the seeded books; the fills follow it.
	headBlock = 10
	// levelsPerSide resting orders per side and market, more than the 20
	// levels of a live book frame.
	levelsPerSide = 25
	fillEvery     = 500 * time.Millisecond
	e18           = "000000000000000000"
)

var (
	perpsEmitter = address(0x10, 0x15)
	maker        = address(0xb2, 0x05)
	taker        = address(0xb1, 0x04)
)

func address(hi, lo byte) []byte {
	b := make([]byte, 20)
	b[18], b[19] = hi, lo
	return b
}

func hash32(block int64, txIndex, logIndex int) []byte {
	return []byte(fmt.Sprintf("%016d%08d%08d", block, txIndex+1, logIndex))
}

// seedBooks writes the head cursor and, for each of the eighteen markets,
// levelsPerSide active bids below 1000 and asks above it, each order alone
// at its price with a distinct size.
func seedBooks(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `INSERT INTO perps.cursors (name, height, hash) VALUES ('head', $1, $2)`, headBlock, hash32(headBlock, -1, 0)); err != nil {
		t.Fatal(err)
	}
	for m := range len(markets.Seed) {
		for k := range 2 * levelsPerSide {
			bid := k < levelsPerSide
			price := 1001 + k - levelsPerSide
			if bid {
				price = 999 - k
			}
			size := strconv.Itoa(1000+k) + e18
			if _, err := db.ExecContext(ctx, `INSERT INTO perps.orders (order_id, account, market_id, order_type, is_long, trigger_price,
				size_usd, limit_price, remaining_size_usd, status, placed_block) VALUES ($1, $2, $3, 1, $4, 0, $5, $6, $5, 'active', 1)`,
				orderID(m, k), maker, m, bid, size, strconv.Itoa(price)+e18); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func orderID(market, k int) int64 { return int64(market*1000 + k + 1) }

// broker starts an in-process JetStream server holding PERPS_V1 of the test
// chain on one replica.
func broker(t *testing.T, ctx context.Context) (*nats.Conn, jetstream.JetStream) {
	t.Helper()
	ns, err := server.NewServer(&server.Options{Host: "127.0.0.1", Port: server.RANDOM_PORT, JetStream: true, StoreDir: t.TempDir(), NoLog: true, NoSigs: true})
	if err != nil {
		t.Fatal(err)
	}
	go ns.Start()
	t.Cleanup(func() { ns.Shutdown(); ns.WaitForShutdown() })
	if !ns.ReadyForConnections(10 * time.Second) {
		t.Fatal("in-process broker not ready")
	}
	nc, err := nats.Connect(ns.ClientURL())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	cfg := stream.Config(testChain)
	cfg.Replicas = 1
	if _, err := js.CreateStream(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	return nc, js
}

// trader writes typed order and fill facts through the indexer block writer.
// Facts, projections, cursor and delivery outbox commit together; publish.Publisher
// drains that outbox independently, as it does in the indexer.
type trader struct {
	db    *sql.DB
	block int64
	fills int
}

func (tr *trader) run(ctx context.Context) error {
	t := time.NewTicker(fillEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
		if err := tr.round(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
	}
}

// roundRows uses the decoder's production row types. A taker order is placed
// before its fill so both sides of the normal order projection exist.
func (tr *trader) roundRows(block int64, now time.Time) *decode.Result {
	result := &decode.Result{}
	for m := range len(markets.Seed) {
		n := tr.fills + m
		k := n % (2 * levelsPerSide)
		from := common.BytesToAddress(taker)
		fact := decode.Fact{Block: uint64(block), BlockHash: common.BytesToHash(hash32(block, -1, 0)),
			TxHash: common.BytesToHash(hash32(block, 0, m)), TxIndex: 0, LogIndex: int64(2 * m),
			ReceiptKind: decode.ReceiptTx, BlockTime: now, Emitter: common.BytesToAddress(perpsEmitter), Account: from, TxFrom: &from, Kind: "placed"}
		takerID := strconv.Itoa(1_000_000 + n)
		order := decode.OrderPlacedRow{Fact: fact, OrderID: takerID, User: from, MarketID: strconv.Itoa(m),
			OrderType: 0, IsLong: k >= levelsPerSide, TriggerPrice: "0", SizeUSD: "1" + e18}
		result.Decoded = append(result.Decoded, decode.Decoded{Event: decode.Event{EventName: "OrderPlaced"}, Rows: []decode.Row{order}})
		fact.Kind, fact.LogIndex = "fill", int64(2*m+1)
		fill := decode.FillRow{Fact: fact, MarketID: strconv.Itoa(m), MakerOrderID: strconv.FormatInt(orderID(m, k), 10),
			TakerOrderID: takerID, Maker: common.BytesToAddress(maker), Taker: from, TakerIsLong: order.IsLong,
			Price: "1000" + e18, SizeUSD: "1" + e18, MakerFeeUSD: "0", TakerFeeUSD: "0"}
		result.Decoded = append(result.Decoded, decode.Decoded{Event: decode.Event{EventName: "Fill"}, Rows: []decode.Row{fill}})
	}
	return result
}

func (tr *trader) round(ctx context.Context) error {
	block := int64(headBlock) + tr.block + 1
	now := time.Now().UTC().Truncate(time.Microsecond)
	b := store.Block{Number: uint64(block), Hash: common.BytesToHash(hash32(block, -1, 0)),
		ParentHash: common.BytesToHash(hash32(block-1, -1, 0)), Time: now, TxCount: 1}
	tx, err := tr.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO perps.blocks (number, hash, parent_hash, time, proposer, tx_count) VALUES ($1,$2,$3,$4,$5,$6)`,
		block, b.Hash.Bytes(), b.ParentHash.Bytes(), now, "loadtest", b.TxCount); err != nil {
		return err
	}
	if err := store.WriteBlock(ctx, tx, b, tr.roundRows(block, now)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE perps.cursors SET height = $1, hash = $2 WHERE name = 'head'`, block, b.Hash.Bytes()); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	tr.block++
	tr.fills += len(markets.Seed)
	return nil
}

func startLoadPublisher(t *testing.T, ctx context.Context, db *sql.DB, nc *nats.Conn) {
	t.Helper()
	p, err := publish.New(db, nc, store.Policy(testChain), publish.Config{Owner: "loadtest", Batch: 256, MaxAttempts: 3,
		LeaseDuration: 10 * time.Second, PublishTimeout: 2 * time.Second, RetryDelay: time.Second,
		IdleInterval: time.Millisecond, MaxHeadAge: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- p.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Errorf("publisher: %v", err)
		}
	})
}

func TestLoadProfileServedSymbols(t *testing.T) {
	dbURL := os.Getenv(testDatabaseEnv)
	if dbURL == "" {
		t.Fatal(testDatabaseEnv + " is unset")
	}
	ctx := context.Background()
	db, err := store.Open(dbURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.ExecContext(ctx, `DROP SCHEMA IF EXISTS perps CASCADE`); err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	routes, err := rest.NewMarkets(db, nil)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	routes.Register(mux)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	initial, err := FeedSymbols(ctx, ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(initial) != 18 || !slices.Contains(initial, "TSLA") {
		t.Fatalf("seeded gateway symbols %v, want 18 including TSLA", initial)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM perps.markets WHERE symbol = 'TSLA'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE perps.markets SET symbol = 'CUSTOM' WHERE symbol = 'BNB'`); err != nil {
		t.Fatal(err)
	}
	want := make([]string, 0, len(markets.Seed)-1)
	for _, market := range markets.Seed {
		if market.Symbol == "TSLA" {
			continue
		}
		symbol := market.Symbol
		if symbol == "BNB" {
			symbol = "CUSTOM"
		}
		want = append(want, symbol)
	}
	providers, err := feeds.NewProviders(ctx, db, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, gatewayURL := range []string{ts.URL, ts.URL + "/", "ws" + strings.TrimPrefix(ts.URL, "http")} {
		symbols, err := FeedSymbols(ctx, gatewayURL)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(symbols, want) {
			t.Fatalf("gateway symbols %v, want %v", symbols, want)
		}
		profile := FullProfile(symbols)
		if err := profile.Validate(); err != nil {
			t.Fatal(err)
		}
		symbols[0] = "TSLA"
		if !slices.Equal(profile.Symbols, want) {
			t.Fatalf("profile symbols %v, want independent copy of %v", profile.Symbols, want)
		}
		for i := range profile.Sockets {
			symbol := want[i%len(want)]
			wantTopics := []string{symbol + bookKind, symbol + tradeKind, feeds.TopicMarketsAll}
			topics := profile.Topics(i)
			if !slices.Equal(topics, wantTopics) {
				t.Fatalf("socket %d topics %v, want %v", i, topics, wantTopics)
			}
			for _, topic := range topics {
				if strings.HasPrefix(topic, "TSLA@") || !providers.Served(topic) {
					t.Fatalf("socket %d subscribed to unserved topic %q", i, topic)
				}
			}
		}
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM perps.markets`); err != nil {
		t.Fatal(err)
	}
	if symbols, err := FeedSymbols(ctx, ts.URL); !errors.Is(err, ErrNoSymbols) || len(symbols) != 0 {
		t.Fatalf("empty gateway symbols %v, error %v, want ErrNoSymbols", symbols, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := FeedSymbols(cancelled, ts.URL); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled discovery error %v, want context.Canceled", err)
	}
	if _, err := FeedSymbols(ctx, ts.URL+"/missing"); err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("missing endpoint error %v, want HTTP 404", err)
	}
}

func TestLoadProfile(t *testing.T) {
	dbURL := os.Getenv(testDatabaseEnv)
	if dbURL == "" {
		t.Fatal(testDatabaseEnv + " is unset")
	}
	rpcURL := os.Getenv(testRPCEnv)
	if rpcURL == "" {
		t.Fatal(testRPCEnv + " is unset")
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	db, err := store.Open(dbURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.ExecContext(ctx, `DROP SCHEMA IF EXISTS perps CASCADE`); err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	seedBooks(t, ctx, db)
	nc, _ := broker(t, ctx)
	node, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(node.Close)
	runLoadProfile(t, ctx, cancel, db, nc, node)
}

func runLoadProfile(t *testing.T, ctx context.Context, cancel context.CancelFunc, db *sql.DB, nc *nats.Conn, node *ethclient.Client) {
	t.Helper()
	metrics, err := telemetry.New(config.Gateway)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := gateway.NewServer(ctx, gateway.Options{DB: db, NATS: nc, Chain: testChain, AuthSecret: testSecret, Node: node, Metrics: metrics})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	t.Cleanup(cancel)
	runErr := make(chan error, 1)
	go func() { runErr <- srv.Run(ctx) }()
	deadline := time.Now().Add(30 * time.Second)
	for !srv.Ready() {
		select {
		case err := <-runErr:
			t.Fatalf("gateway run stopped before ready: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("gateway not ready within 30 seconds")
		}
		time.Sleep(50 * time.Millisecond)
	}

	tradeCtx, stopTrades := context.WithCancel(ctx)
	t.Cleanup(stopTrades)
	tradeErr := make(chan error, 1)
	startLoadPublisher(t, ctx, db, nc)
	observe := observeProductionFills(t, nc)
	tr := &trader{db: db}
	go func() { tradeErr <- tr.run(tradeCtx) }()

	socketURL, err := SocketURL(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	symbols, err := FeedSymbols(ctx, ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	profile := FullProfile(symbols).Scaled(testSockets)
	report, err := Run(ctx, socketURL, profile)
	stopTrades()
	if err != nil {
		t.Fatal(err)
	}
	if err := <-tradeErr; err != nil {
		t.Fatalf("trader: %v", err)
	}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("report %s", raw)
	observe.assert(t, tr.fills)
	deadline = time.Now().Add(5 * time.Second)
	for {
		var pending int64
		if err := db.QueryRowContext(ctx, `SELECT count(*) FROM perps.delivery_outbox WHERE state <> 'published'`).Scan(&pending); err != nil {
			t.Fatal(err)
		}
		if pending == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d production outbox rows did not receive persistence acknowledgements", pending)
		}
		time.Sleep(5 * time.Millisecond)
	}
	assertProductionTrades(t, ts.URL, symbols, tr.block)
	if report.TradeFrames < int64(testSockets) || report.InvalidTrades != 0 {
		t.Fatalf("production trade frames %d, invalid %d", report.TradeFrames, report.InvalidTrades)
	}

	t.Run("sockets", func(t *testing.T) {
		if profile.Sockets != testSockets || FullProfile(symbols).Sockets != 140000 || len(profile.Symbols) != 18 {
			t.Fatalf("profile %d sockets over %d symbols, full %d", profile.Sockets, len(profile.Symbols), FullProfile(symbols).Sockets)
		}
		if got := profile.Topics(19); len(got) != 3 || got[0] != "ETH@perp_orderbook" || got[1] != "ETH@perp_trade" || got[2] != "markets@all" {
			t.Fatalf("topics of socket 19 %v", got)
		}
		if report.Welcomed != testSockets || report.Guarded != testSockets {
			t.Fatalf("welcomed %d, guarded %d, want %d each", report.Welcomed, report.Guarded, testSockets)
		}
		if report.Snapshots < 3*testSockets {
			t.Fatalf("%d snapshots, want at least %d", report.Snapshots, 3*testSockets)
		}
		if report.ErrorFrames != 0 || report.DialFailures != 0 {
			t.Fatalf("%d error frames, %d dial failures", report.ErrorFrames, report.DialFailures)
		}
	})

	t.Run("reconnect_storm", func(t *testing.T) {
		if report.Storm != 400 || report.Recovered != 400 {
			t.Fatalf("storm %d, recovered %d, want 400 each", report.Storm, report.Recovered)
		}
		if report.MaxRecovery <= 0 || report.MaxRecovery > 60*time.Second {
			t.Fatalf("slowest recovery %s, want within 60s", report.MaxRecovery)
		}
		if report.Live != testSockets {
			t.Fatalf("%d sockets live at the end, want %d", report.Live, testSockets)
		}
	})

	t.Run("sequence_and_latency", func(t *testing.T) {
		if report.Finished.Sub(report.Started) < 60*time.Second {
			t.Fatalf("ran %s, want at least 60s", report.Finished.Sub(report.Started))
		}
		if report.Frames <= report.Snapshots {
			t.Fatalf("%d frames, %d snapshots: no live frame", report.Frames, report.Snapshots)
		}
		if report.UnresolvedGaps != 0 || report.Resynced > report.Gaps {
			t.Fatalf("gaps %d, resynced %d, unresolved %d", report.Gaps, report.Resynced, report.UnresolvedGaps)
		}
		if report.P99Delay > 250*time.Millisecond {
			t.Fatalf("p99 frame delay %s above 250ms", report.P99Delay)
		}
		if report.Closes1013NoRetry != 0 {
			t.Fatalf("%d 1013 closes without retry-after", report.Closes1013NoRetry)
		}
	})

	t.Run("book_checksums", func(t *testing.T) {
		if report.BookFrames == 0 || report.Checksums < int64(testSockets) {
			t.Fatalf("%d book frames, %d checksums, want at least one checksum per socket", report.BookFrames, report.Checksums)
		}
		if report.ChecksumMismatches != 0 || report.ChecksumLate != 0 {
			t.Fatalf("%d checksum mismatches, %d late", report.ChecksumMismatches, report.ChecksumLate)
		}
		if report.BookRateViolations != 0 || report.BookLevelViolations != 0 {
			t.Fatalf("%d frames beyond 10 per second, %d beyond 20 levels", report.BookRateViolations, report.BookLevelViolations)
		}
	})

	if err := report.Err(); err != nil {
		t.Fatalf("report: %v", err)
	}
	cancel()
	select {
	case err := <-runErr:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("gateway run: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("gateway run did not stop after cancel")
	}
}
