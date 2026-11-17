package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/delivery"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/feeds"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/testdb"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/ws"
)

func TestPriceWindow(t *testing.T) {
	db := testdb.Open(t)
	now := time.Now().UTC().Truncate(time.Millisecond)
	testdb.SeedPrices(t, db, now)
	// BTC has price history but no fills. ETH's fill change remains independent
	// of both the historical index price and the historical mark price.
	fill(t, db, 1, 5, 0, now.Add(-2*time.Hour), true, "100", "1000")
	fill(t, db, 1, 6, 0, now.Add(-time.Hour), false, "125", "1500")
	rollingStats(t, db, 1, "2500", "2500", "2500", 2, "0", "0", "100")
	testdb.Exec(t, db, `UPDATE perps.market_stats_rolling SET last_price = $1, high_24h = $1, low_24h = $2 WHERE market_id = 1`, e18(t, "125"), e18(t, "100"))
	markets, err := NewMarkets(db, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	venue, err := NewVenue(db)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	markets.Register(mux)
	venue.Register(mux)
	p, err := feeds.NewProviders(context.Background(), db, nil)
	if err != nil {
		t.Fatal(err)
	}
	hub, err := ws.NewHub(priceWindowSocketFeeds{p}, 125, "price-window", nil)
	if err != nil {
		t.Fatal(err)
	}
	socketServer, err := ws.NewServer("price-window", p.Served, ws.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	socketServer.OnSubscribe, socketServer.OnUnsubscribe, socketServer.OnClose = hub.Subscribe, hub.Unsubscribe, hub.Leave
	mux.Handle("GET /ws", socketServer)
	server := httptest.NewServer(mux)
	defer server.Close()
	client := server.Client()
	client.Timeout = 10 * time.Second
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	snapshot := func(topic string) []byte {
		t.Helper()
		if err := conn.WriteJSON(ws.ClientFrame{Event: ws.EventSubscribe, Topic: topic}); err != nil {
			t.Fatal(err)
		}
		if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
			t.Fatal(err)
		}
		for {
			var frame struct {
				Event    string          `json:"event"`
				Topic    string          `json:"topic"`
				Snapshot bool            `json:"snapshot"`
				Data     json.RawMessage `json:"data"`
			}
			if err := conn.ReadJSON(&frame); err != nil {
				t.Fatal(err)
			}
			if frame.Event == "error" {
				t.Fatalf("socket error for %s", topic)
			}
			if frame.Topic == topic && frame.Snapshot {
				return frame.Data
			}
		}
	}
	read := func(path, fieldName string) map[string]json.RawMessage {
		t.Helper()
		return field[map[string]json.RawMessage](t, ProofRESTGet(t, client, server.URL, path, http.StatusOK), fieldName)
	}
	checkMarket := func(rec map[string]json.RawMessage, want *float64) {
		t.Helper()
		wantKeys(t, rec, recordFields...)
		if want == nil {
			wantNull(t, rec, "price_24h")
		} else {
			wantNumber(t, rec, "price_24h", *want)
		}
	}
	btc := 83.0
	eth := 85.0
	allBody := ProofRESTGet(t, client, server.URL, "/api/markets", http.StatusOK)
	all := field[[]map[string]json.RawMessage](t, allBody, "markets")
	checkMarket(all[0], &btc)
	checkMarket(all[1], &eth)
	checkMarket(all[2], nil)
	checkMarket(all[3], nil)
	for _, tc := range []struct {
		symbol string
		price  *float64
	}{{"BTC", &btc}, {"ETH", &eth}, {"SOL", nil}, {"AVAX", nil}} {
		rec := read("/api/markets/"+tc.symbol, "market")
		checkMarket(rec, tc.price)
		if tc.symbol == "BTC" {
			wantNull(t, rec, "change_24h")
			wantNumber(t, rec, "price", 120)
		}
		if tc.symbol == "ETH" {
			wantNumber(t, rec, "change_24h", 0.25)
			wantNumber(t, rec, "price", 150)
		}
		raw := snapshot(tc.symbol + "@market")
		var topic map[string]json.RawMessage
		if err := json.Unmarshal(raw, &topic); err != nil {
			t.Fatal(err)
		}
		checkMarket(topic, tc.price)
		if tc.symbol == "BTC" {
			wantNull(t, topic, "change_24h")
		}
	}
	raw := snapshot(feeds.TopicMarketsAll)
	var topics []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &topics); err != nil {
		t.Fatal(err)
	}
	if len(topics) != len(all) {
		t.Fatalf("topics %d, records %d", len(topics), len(all))
	}
	for i := range topics {
		if string(topics[i]["price_24h"]) != string(all[i]["price_24h"]) {
			t.Fatalf("market %d price_24h differs: %s / %s", i, topics[i]["price_24h"], all[i]["price_24h"])
		}
	}
	for _, prefix := range []string{"/api", "/api/markets"} {
		body := ProofRESTGet(t, client, server.URL, prefix+"/tickers", http.StatusOK)
		entries := field[[]map[string]json.RawMessage](t, body, "data")
		if len(entries) != len(all) {
			t.Fatalf("tickers %d, markets %d", len(entries), len(all))
		}
		for i, tc := range []struct {
			symbol string
			price  *float64
		}{{"BTC", &btc}, {"ETH", nil}, {"SOL", nil}, {"AVAX", nil}} {
			ticker := read(prefix+"/ticker/"+tc.symbol, "data")
			entry := entries[i]
			if tc.symbol == "ETH" {
				wantNumber(t, ticker, "price_24h", 90)
				wantNumber(t, entry, "price_24h", 90)
				for name, want := range map[string]float64{"24h_open": 100, "24h_close": 125, "24h_high": 125, "24h_low": 100, "24h_amount": 22, "24h_volume": 2500} {
					wantNumber(t, entry, name, want)
				}
				wantNumber(t, ticker, "index_price", 150)
				wantNumber(t, ticker, "mark_price", 130)
			} else if tc.price != nil {
				wantNumber(t, ticker, "price_24h", *tc.price)
				wantNumber(t, entry, "price_24h", *tc.price)
			} else {
				wantNull(t, ticker, "price_24h")
				wantNull(t, entry, "price_24h")
			}
			if tc.symbol == "AVAX" {
				wantNull(t, ticker, "updated_at")
				wantNull(t, entry, "updated_at")
			} else {
				wantAt := now.Add(-2 * time.Hour).UnixMilli()
				if got := field[int64](t, entry, "updated_at"); got != wantAt {
					t.Fatalf("%s entry timestamp %d, want %d", tc.symbol, got, wantAt)
				}
				if got := field[int64](t, ticker, "updated_at"); got != wantAt {
					t.Fatalf("%s ticker timestamp %d, want %d", tc.symbol, got, wantAt)
				}
			}
		}
	}
}

// priceWindowSocketFeeds converts the feed frame type at the hub boundary;
// every read and timed frame is produced by the database-backed Providers.
type priceWindowSocketFeeds struct{ *feeds.Providers }

func (p priceWindowSocketFeeds) Message(ctx context.Context, subject string, env delivery.Envelope) ([]ws.Frame, []string, error) {
	frames, topics, err := p.Providers.Message(ctx, subject, env)
	out := make([]ws.Frame, len(frames))
	for i, frame := range frames {
		out[i] = ws.Frame(frame)
	}
	return out, topics, err
}

func (p priceWindowSocketFeeds) Run(ctx context.Context, emit func(ws.Frame)) error {
	return p.Providers.Run(ctx, func(frame feeds.Frame) { emit(ws.Frame(frame)) })
}
