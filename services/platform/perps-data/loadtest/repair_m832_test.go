package loadtest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/nats-io/nats.go"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/decode"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/delivery"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/delivery/natsdriver"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/markets"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/publish"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/records"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/store"
)

// productionFills observes the actual publisher wire during the socket profile.
// delivery.Decode is the gateway hub's admission codec. The profile separately
// checks the real gateway's resulting trade frames, books and sequence guards.
type productionFills struct {
	mu      sync.Mutex
	ids     map[string]bool
	markets map[string]int
	lastSeq int64
	err     error
}

func observeProductionFills(t *testing.T, nc *nats.Conn) *productionFills {
	t.Helper()
	o := &productionFills{ids: map[string]bool{}, markets: map[string]int{}}
	sub, err := nc.Subscribe("perps.125.*.fill", func(msg *nats.Msg) {
		o.mu.Lock()
		defer o.mu.Unlock()
		if o.err != nil {
			return
		}
		env, err := delivery.Decode(msg.Data, store.Policy(testChain), msg.Subject)
		if err == nil {
			err = o.check(msg, env)
		}
		if err != nil {
			o.err = err
			return
		}
		o.ids[env.EventID] = true
		var payload map[string]any
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			o.err = err
			return
		}
		o.markets[payload["market_id"].(string)]++
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sub.Unsubscribe(); err != nil {
			t.Error(err)
		}
	})
	if err := nc.FlushTimeout(5 * time.Second); err != nil {
		t.Fatal(err)
	}
	return o
}
func (o *productionFills) check(msg *nats.Msg, e delivery.Envelope) error {
	var p map[string]any
	if err := json.Unmarshal(e.Payload, &p); err != nil {
		return err
	}
	market, err := strconv.Atoi(strings.Split(msg.Subject, ".")[2])
	if err != nil || market < 0 || market >= len(markets.Seed) {
		return errors.New("invalid fill subject")
	}
	block, err := strconv.ParseInt(e.SourceOrder.Height, 10, 64)
	if err != nil || block <= headBlock {
		return errors.New("invalid fill height")
	}
	n := int(block-headBlock-1)*len(markets.Seed) + market
	k := n % (2 * levelsPerSide)
	at, err := time.Parse(time.RFC3339Nano, fmt.Sprint(p["block_time"]))
	if err != nil {
		return err
	}
	expected := map[string]any{
		"table": "fills", "block": strconv.FormatInt(block, 10), "block_hash": strings.ToLower(common.BytesToHash(hash32(block, -1, 0)).Hex()),
		"tx_index": float64(0), "log_index": float64(2*market + 1), "sub": float64(0),
		"tx_hash": strings.ToLower(common.BytesToHash(hash32(block, 0, market)).Hex()), "receipt_kind": "tx",
		"block_time": at.UTC().Format(time.RFC3339Nano), "emitter": strings.ToLower(common.BytesToAddress(perpsEmitter).Hex()),
		"kind": "fill", "account": strings.ToLower(common.BytesToAddress(taker).Hex()), "tx_from": strings.ToLower(common.BytesToAddress(taker).Hex()),
		"market_id": strconv.Itoa(market), "maker_order_id": strconv.FormatInt(orderID(market, k), 10), "taker_order_id": strconv.Itoa(1_000_000 + n),
		"maker": strings.ToLower(common.BytesToAddress(maker).Hex()), "taker": strings.ToLower(common.BytesToAddress(taker).Hex()),
		"taker_is_long": k >= levelsPerSide, "price": "1000" + e18, "size_usd": "1" + e18, "maker_fee_usd": "0", "taker_fee_usd": "0",
	}
	if !reflect.DeepEqual(p, expected) {
		return fmt.Errorf("full fill payload differs: got %v want %v", p, expected)
	}
	id, err := natsdriver.MsgID(fmt.Sprintf("%d:%d:0x%x:0:%d:0", testChain, block, hash32(block, -1, 0), 2*market+1))
	if err != nil {
		return err
	}
	seq, err := strconv.ParseInt(msg.Header.Get(publish.HeaderSeq), 10, 64)
	if err != nil || seq <= o.lastSeq {
		return fmt.Errorf("publisher sequence %d after %d: %v", seq, o.lastSeq, err)
	}
	if e.EventID != id || o.ids[id] || e.TraceID != id || e.EventType != "fill" || e.SchemaVersion != 1 || e.ChainID != testChain ||
		e.SourceOrder.TransactionIndex != "0" || e.SourceOrder.LogIndex != strconv.Itoa(2*market+1) || !e.EmittedAt.Equal(at) ||
		msg.Header.Get("Nats-Msg-Id") != id || msg.Header.Get(publish.HeaderBlock) != e.SourceOrder.Height || msg.Header.Get(publish.HeaderKind) != "fill" || msg.Header.Get("Nats-Expected-Stream") != "PERPS_V1" {
		return fmt.Errorf("production envelope or headers differ for %s: %v", id, msg.Header)
	}
	o.lastSeq = seq
	return nil
}
func (o *productionFills) assert(t *testing.T, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		o.mu.Lock()
		count, err := len(o.ids), o.err
		covered := len(o.markets)
		o.mu.Unlock()
		if err != nil {
			t.Fatal(err)
		}
		if count == want && covered == len(markets.Seed) && want > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("consumed %d/%d full production fills across %d/%d markets", count, want, covered, len(markets.Seed))
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func assertProductionTrades(t *testing.T, base string, symbols []string, rounds int64) {
	t.Helper()
	client := &http.Client{Timeout: 5 * time.Second}
	for market, symbol := range symbols {
		response, err := client.Get(base + "/api/" + symbol + "/perp_trades?limit=1")
		if err != nil {
			t.Fatal(err)
		}
		var body records.Trades[records.PerpTrade]
		err = json.NewDecoder(response.Body).Decode(&body)
		response.Body.Close()
		if err != nil || response.StatusCode != http.StatusOK || body.Symbol != symbol || len(body.Trades) != 1 {
			t.Fatalf("gateway %s trade response %+v, status %d: %v", symbol, body, response.StatusCode, err)
		}
		trade := body.Trades[0]
		wantSide := "SELL"
		if (int(rounds-1)*len(markets.Seed)+market)%(2*levelsPerSide) >= levelsPerSide {
			wantSide = "BUY"
		}
		wantID := fmt.Sprintf("%d-%d", headBlock+rounds, 2*market+1)
		if trade.Price != 1000 || trade.Contracts != 0.001 || trade.Notional != 1 || trade.ID != wantID || trade.TradeTime <= 0 || trade.Liquidation || trade.Side != wantSide {
			t.Fatalf("gateway changed production fill: %+v", trade)
		}
	}
}

func TestRepairM_832_ProductionProfile(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	db, memory := m832Database(t)
	seedBooks(t, ctx, db)
	nc, _ := broker(t, ctx)
	// The profile serves only database-backed perps routes. A local HTTP peer
	// refuses unexpected node use and proves no live RPC dependency is hidden.
	rpcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected node request %s", r.URL)
		http.Error(w, "no node calls in perps profile", http.StatusBadRequest)
	}))
	t.Cleanup(rpcServer.Close)
	node, err := ethclient.DialContext(ctx, rpcServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(node.Close)
	runLoadProfile(t, ctx, cancel, db, nc, node)
	memory.mu.Lock()
	defer memory.mu.Unlock()
	if len(memory.outboxPending) != 0 {
		t.Fatalf("published outbox left %d pending index entries", len(memory.outboxPending))
	}
	if len(memory.tables["fills"]) < 18*120 || len(memory.tables["market_stats_rolling"]) != 18 {
		t.Fatalf("profile did not commit production facts/projections: fills=%d stats=%d", len(memory.tables["fills"]), len(memory.tables["market_stats_rolling"]))
	}
	for _, r := range memory.tables["delivery_outbox"] {
		if r["state"] != "published" || m832Int(r["broker_sequence"]) < 1 {
			t.Fatalf("outbox did not record persistence acknowledgement: %v", r)
		}
	}
}

func TestRepairM_832_AtomicBlock(t *testing.T) {
	ctx := context.Background()
	db, memory := m832Database(t)
	seedBooks(t, ctx, db)
	tr := &trader{db: db}
	// A missing late maker order fails the real projection after facts and the
	// outbox were written. The entire block must roll back, including earlier
	// maker updates and taker orders; no publisher can observe those envelopes.
	key := strconv.FormatInt(orderID(17, 17), 10)
	memory.mu.Lock()
	removed := memory.tables["orders"][key]
	delete(memory.tables["orders"], key)
	memory.mu.Unlock()
	if err := tr.round(ctx); !errors.Is(err, store.ErrNotProjected) {
		t.Fatalf("missing maker should refuse projection, got %v", err)
	}
	memory.mu.Lock()
	if len(memory.outboxPending) != 0 {
		t.Fatal("rolled back block left pending delivery index entries")
	}
	for _, rows := range memory.fillMarkets {
		if len(rows) != 0 {
			t.Fatal("rolled back block left indexed fills")
		}
	}
	if len(memory.tables["fills"]) != 0 || len(memory.tables["order_events"]) != 0 || len(memory.tables["delivery_outbox"]) != 0 || len(memory.tables["blocks"]) != 0 || len(memory.tables["market_stats_rolling"]) != 0 || len(memory.tables["orders"]) != 18*2*levelsPerSide-1 || m832Int(memory.tables["cursors"]["head"]["height"]) != headBlock {
		t.Fatal("failed production block left facts, projections, outbox or cursor effects")
	}
	for _, r := range memory.tables["orders"] {
		if m832String(r["remaining_size_usd"]) != m832String(r["size_usd"]) {
			t.Fatal("failed block changed resting size")
		}
	}
	memory.tables["orders"][key] = removed
	memory.mu.Unlock()
	if tr.block != 0 || tr.fills != 0 {
		t.Fatal("failed block advanced workload")
	}
	if err := tr.round(ctx); err != nil {
		t.Fatal(err)
	}
	memory.mu.Lock()
	defer memory.mu.Unlock()
	if len(memory.tables["fills"]) != 18 || len(memory.tables["order_events"]) != 18 || len(memory.tables["protocol_events"]) != 36 || len(memory.tables["tx_actions"]) != 36 || len(memory.tables["delivery_outbox"]) != 73 || m832Int(memory.tables["cursors"]["head"]["height"]) != 11 {
		t.Fatal("committed block lost production facts, actions, envelopes or cursor")
	}
	for m := range len(markets.Seed) {
		indexed := memory.fillMarkets[strconv.Itoa(m)]
		if len(indexed) != 1 {
			t.Fatalf("market %d committed %d indexed fills, want 1", m, len(indexed))
		}
		for key, row := range indexed {
			if !reflect.DeepEqual(row, memory.tables["fills"][key]) {
				t.Fatalf("market %d indexed fill differs from committed fact", m)
			}
		}
		makerRow := memory.tables["orders"][strconv.FormatInt(orderID(m, m), 10)]
		want := strconv.Itoa(999+m) + e18
		if m832String(makerRow["remaining_size_usd"]) != want {
			t.Fatalf("market %d maker size %v want %s", m, makerRow["remaining_size_usd"], want)
		}
		takerRow := memory.tables["orders"][strconv.Itoa(1_000_000+m)]
		if takerRow["status"] != "filled" || takerRow["remaining_size_usd"] != "0" {
			t.Fatalf("taker projection not filled: %v", takerRow)
		}
		stats := memory.tables["market_stats_rolling"][strconv.Itoa(m)]
		if stats["volume_24h"] != "1"+e18 || m832Int(stats["trades_count"]) != 1 {
			t.Fatalf("production rolling projection lost fill: %v", stats)
		}
	}
	if !reflect.DeepEqual(memory.outboxPending, memory.tables["delivery_outbox"]) {
		t.Fatal("pending index differs from committed outbox")
	}
	// Check the producer's exact typed row also survives admission at the wire.
	for _, r := range memory.tables["delivery_outbox"] {
		env, err := delivery.Decode(r["envelope"].([]byte), store.Policy(testChain), r["subject"].(string))
		if err != nil {
			t.Fatal(err)
		}
		if env.EventType == "fill" {
			var payload map[string]any
			if err := json.Unmarshal(env.Payload, &payload); err != nil {
				t.Fatal(err)
			}
			if payload["table"] != (decode.FillRow{}).Table() || payload["maker_fee_usd"] != "0" || payload["size_usd"] != "1"+e18 {
				t.Fatal("production serialization lost typed fill fields")
			}
		}
	}
}
