// Proves: rest.Markets, rest.Symbol, rest.SpotUsers.StreamItems -> gateway.Server.Handler HTTP and ws.SpotUser -> portal.Pages.Markets/Market and REST/socket JSON consumers.
//
// Repairs portal/pages.go accepting an empty market identity, markets.go
// omitting cache refusal for live records, symbol.go ignoring duplicate limit
// values, venue.go differing from Markets.Record's case-insensitive lookup,
// and spotusers.go StreamItems silently losing an event with no stored trade.
// Producer state comes from a private PostgreSQL process and store.WriteFacts /
// store.Project; envelopes come from their outbox, never a hand-built message.
// Each consumer reads twice, then observes changed producer state. A malformed
// record emitted by the real route is refused by the portal before rendering.
// Outside scope: records/records.go MarketRecord and store market ingestion
// admit an empty symbol; the portal must refuse that producer result.
package rest_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	evmruntime "github.com/ethereum/go-ethereum/core/vm/runtime"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/ethclient"
	gethnode "github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/params"
	"github.com/gorilla/websocket"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/artifacts/wpax"
	perpsabi "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/perps/abi"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/decode"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/delivery"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/gateway"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/graphql"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/portal"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/rest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/telemetry"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/ws"
)

// TestVenueLatestIndexCoherence drives ABI logs and finalize-block events
// through the production decoder, writer, SQL readers and registered routes.
func TestVenueLatestIndexCoherence(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db, err := store.Open(proofRESTPostgres(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	metrics, err := telemetry.New(config.Indexer)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := decode.NewRegistry(metrics, decode.Options{
		PerpsModule: decode.ModuleAddress("perps"), SpotModule: decode.ModuleAddress("spot"), PricefeedModule: decode.ModuleAddress("pricefeed"),
	})
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := perpsabi.ABI()
	if err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	units := func(value string) *big.Int {
		t.Helper()
		n, ok := new(big.Rat).SetString(value)
		if !ok {
			t.Fatalf("invalid price %q", value)
		}
		n.Mul(n, new(big.Rat).SetInt(big.NewInt(1_000_000_000_000_000_000)))
		if !n.IsInt() {
			t.Fatalf("inexact producer value %q", value)
		}
		return n.Num()
	}
	logEvent := func(name string, txIndex, logIndex uint, values ...any) decode.Log {
		t.Helper()
		event := leaf.Events[name]
		if len(values) != len(event.Inputs) {
			t.Fatalf("%s: %d values for %d inputs", name, len(values), len(event.Inputs))
		}
		topics := []common.Hash{event.ID}
		var data []any
		for i, field := range event.Inputs {
			if field.Indexed {
				encoded, err := abi.MakeTopics([]any{values[i]})
				if err != nil {
					t.Fatal(err)
				}
				topics = append(topics, encoded[0][0])
			} else {
				data = append(data, values[i])
			}
		}
		packed, err := event.Inputs.NonIndexed().Pack(data...)
		if err != nil {
			t.Fatal(err)
		}
		return decode.Log{Log: types.Log{Address: decode.PerpsAddress, Topics: topics, Data: packed, TxIndex: txIndex, Index: logIndex}, ReceiptKind: decode.ReceiptPerpsEndBlock}
	}
	mark := func(market int64, txIndex, logIndex uint, price, index string) decode.Log {
		return logEvent("MarkPriceUpdated", txIndex, logIndex, big.NewInt(market), units(price), units(index), units(price))
	}
	feed := func(market int64, index string) abci.Event {
		typ := decode.EventAggregate
		attrs := []string{"market_id", strconv.FormatInt(market, 10), "sources", "3", "required", "2"}
		if index == "" {
			typ = decode.EventAggregateSkipped
			attrs[3] = "1"
		} else {
			// This mark is deliberately different: aggregate marks must never
			// replace the independently selected MarkPriceUpdated value.
			attrs = append(attrs, "index_price", units(index).String(), "mark_price", units("999").String(),
				"basis", "0", "timestamp_ms", strconv.FormatInt(at.UnixMilli(), 10), "height", "1")
		}
		event := abci.Event{Type: typ}
		for i := 0; i < len(attrs); i += 2 {
			event.Attributes = append(event.Attributes, abci.EventAttribute{Key: []byte(attrs[i]), Value: []byte(attrs[i+1])})
		}
		return event
	}
	write := func(height uint64, events []abci.Event, logs ...decode.Log) {
		t.Helper()
		hash := common.BigToHash(new(big.Int).SetUint64(height))
		when := at.Add(time.Duration(height) * time.Second)
		for i := range logs {
			logs[i].BlockNumber, logs[i].BlockHash, logs[i].BlockTime = height, hash, when
			logs[i].TxHash = common.BigToHash(new(big.Int).SetUint64(height*1000 + uint64(logs[i].TxIndex)))
		}
		result, err := registry.DecodeBlock(ctx, db, decode.BlockInput{Number: height, Hash: hash, Time: when, Events: events, Logs: logs})
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Raw) != 0 || len(result.Decoded) != len(events)+len(logs) {
			t.Fatalf("producer decode: %+v", result)
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		if err := store.WriteBlock(ctx, tx, store.Block{Number: height, Hash: hash, Time: when}, result); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	venue, err := rest.NewVenue(db)
	if err != nil {
		t.Fatal(err)
	}
	markets, err := rest.NewMarkets(db, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	schema, err := graphql.NewSchema(db)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	venue.Register(mux)
	markets.Register(mux)
	mux.Handle("/graphql", &relay.Handler{Schema: schema})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	client := ts.Client()
	client.Timeout = 10 * time.Second
	request := func(method, path, payload string) map[string]json.RawMessage {
		t.Helper()
		req, err := http.NewRequestWithContext(ctx, method, ts.URL+path, strings.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var body map[string]json.RawMessage
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: status %d: %s", path, resp.StatusCode, body)
		}
		return body
	}
	fields := func(body map[string]json.RawMessage, names ...string) {
		t.Helper()
		got := make([]string, 0, len(body))
		for name := range body {
			got = append(got, name)
		}
		slices.Sort(got)
		slices.Sort(names)
		if !slices.Equal(got, names) {
			t.Fatalf("response fields %v, want %v", got, names)
		}
	}
	number := func(body map[string]json.RawMessage, name string, want *float64) {
		t.Helper()
		raw, exists := body[name]
		var got *float64
		if !exists {
			t.Fatalf("missing %s", name)
		}
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s = %s, want %v", name, raw, want)
		}
	}
	ptr := func(v float64) *float64 { return &v }
	type prices struct{ mark, index *float64 }
	want := map[string]prices{}
	observe := func(stage string) {
		t.Helper()
		t.Log(stage)
		body := request(http.MethodPost, "/graphql", `{"query":"{ orderlyTickers { paxeerSymbol mark_price index_price } }"}`)
		if raw, ok := body["errors"]; ok {
			t.Fatalf("GraphQL errors: %s", raw)
		}
		var graph struct{ OrderlyTickers []map[string]json.RawMessage }
		if err := json.Unmarshal(body["data"], &graph); err != nil {
			t.Fatal(err)
		}
		var records struct{ Markets []rest.MarketRecord }
		if err := json.Unmarshal(request(http.MethodGet, "/api/markets", "")["markets"], &records.Markets); err != nil {
			t.Fatal(err)
		}
		if len(records.Markets) != 18 || len(graph.OrderlyTickers) != len(records.Markets) {
			t.Fatalf("inventory: %d records, %d GraphQL tickers", len(records.Markets), len(graph.OrderlyTickers))
		}
		for _, prefix := range []string{"/api", "/api/markets"} {
			listBody := request(http.MethodGet, prefix+"/tickers", "")
			fields(listBody, "success", "data")
			if string(listBody["success"]) != "true" {
				t.Fatalf("unsuccessful ticker list: %s", listBody)
			}
			var entries []map[string]json.RawMessage
			if err := json.Unmarshal(listBody["data"], &entries); err != nil {
				t.Fatal(err)
			}
			if len(entries) != len(records.Markets) {
				t.Fatalf("%s: %d entries", prefix, len(entries))
			}
			for i, rec := range records.Markets {
				expected := want[rec.Symbol]
				entry := entries[i]
				fields(entry, "symbol", "paxeerSymbol", "mark_price", "index_price", "open_interest", "last_funding_rate_hourly", "last_funding_rate",
					"24h_open", "24h_close", "24h_high", "24h_low", "24h_amount", "24h_volume")
				if string(entry["paxeerSymbol"]) != strconv.Quote(rec.Symbol) || string(entry["symbol"]) != strconv.Quote("PERP_"+rec.Symbol+"_USDC") ||
					string(graph.OrderlyTickers[i]["paxeerSymbol"]) != strconv.Quote(rec.Symbol) {
					t.Fatalf("ticker identity/order: %s / %s / %s", entry, graph.OrderlyTickers[i], rec.Symbol)
				}
				number(entry, "mark_price", expected.mark)
				number(entry, "index_price", expected.index)
				number(graph.OrderlyTickers[i], "mark_price", expected.mark)
				number(graph.OrderlyTickers[i], "index_price", expected.index)
				if !reflect.DeepEqual(rec.PerpMarkPrice, expected.mark) || !reflect.DeepEqual(rec.PerpIndexPrice, expected.index) {
					t.Fatalf("%s market-record prices: %+v", rec.Symbol, rec)
				}
				var basis *float64
				if expected.mark != nil && expected.index != nil && *expected.index != 0 {
					basis = ptr(math.Round((*expected.mark-*expected.index)*10000 / *expected.index * 1000) / 1000)
				}
				if !reflect.DeepEqual(rec.PerpBasisBps, basis) {
					t.Fatalf("%s market-record basis: %v, want %v", rec.Symbol, rec.PerpBasisBps, basis)
				}
				oneBody := request(http.MethodGet, prefix+"/ticker/"+strings.ToLower(rec.Symbol), "")
				fields(oneBody, "success", "data")
				if string(oneBody["success"]) != "true" {
					t.Fatalf("unsuccessful ticker: %s", oneBody)
				}
				var one map[string]json.RawMessage
				if err := json.Unmarshal(oneBody["data"], &one); err != nil {
					t.Fatal(err)
				}
				fields(one, "symbol", "index_price", "mark_price", "last_funding_rate_hourly", "last_funding_rate", "open_interest", "volume_24h")
				if string(one["symbol"]) != strconv.Quote(rec.Symbol) {
					t.Fatalf("single ticker symbol: %s", one)
				}
				for _, key := range []string{"index_price", "mark_price", "last_funding_rate_hourly", "last_funding_rate", "open_interest"} {
					if !bytes.Equal(one[key], entry[key]) {
						t.Fatalf("%s %s single/list mismatch: %s / %s", prefix, key, one[key], entry[key])
					}
				}
				for _, key := range []string{"24h_open", "24h_close", "24h_high", "24h_low"} {
					number(entry, key, nil)
				}
				for _, key := range []string{"24h_amount", "24h_volume"} {
					number(entry, key, ptr(0))
				}
				number(one, "volume_24h", ptr(0))
				if rec.Symbol == "BTC" {
					number(one, "open_interest", ptr(2000.75))
					number(one, "last_funding_rate", ptr(-0.00005))
					if string(one["last_funding_rate_hourly"]) != `"-0.00005"` {
						t.Fatalf("exact hourly funding: %s", one)
					}
				} else {
					for _, key := range []string{"open_interest", "last_funding_rate", "last_funding_rate_hourly"} {
						if string(one[key]) != "null" {
							t.Fatalf("%s %s: %s", rec.Symbol, key, one[key])
						}
					}
				}
			}
		}
	}

	write(10, nil, mark(0, 0, 90, "101.25", "100.5"), mark(2, 0, 91, "51.25", "50.5"),
		logEvent("FundingSettled", 0, 92, big.NewInt(0), units("-0.00005"), units("-1"), units("1")),
		logEvent("OpenInterestChanged", 0, 93, big.NewInt(0), units("1250.5"), units("750.25"), units("2000.75"), true))
	want["BTC"], want["SOL"] = prices{ptr(101.25), ptr(100.5)}, prices{ptr(51.25), ptr(50.5)}
	observe("mark_only_and_empty")
	write(11, []abci.Event{feed(0, "102.5"), feed(1, "200.25"), feed(3, "")})
	want["BTC"], want["ETH"] = prices{ptr(101.25), ptr(102.5)}, prices{nil, ptr(200.25)}
	observe("newer_aggregate_and_aggregate_only")
	write(12, []abci.Event{feed(0, ""), feed(1, ""), feed(3, "")})
	observe("skipped_aggregates_preserve_index")
	write(13, []abci.Event{feed(0, "103.5"), feed(0, "104.5")})
	want["BTC"] = prices{ptr(101.25), ptr(104.5)}
	observe("aggregate_log_order")
	// A higher transaction wins despite the older transaction's higher log
	// index; the mark also beats finalize-block events (tx_index = -1).
	write(14, []abci.Event{feed(0, "105.5")}, mark(0, 0, 99, "106.25", "106.5"), mark(0, 1, 0, "108.25", "107.5"))
	want["BTC"] = prices{ptr(108.25), ptr(107.5)}
	observe("transaction_order_and_mark_after_aggregate")
	write(15, nil, mark(0, 1, 1, "110.25", "109.5"), mark(0, 1, 0, "109.25", "108.5"))
	want["BTC"] = prices{ptr(110.25), ptr(109.5)}
	observe("mark_log_order")
	// Lower transaction/log components in a later block still win. An
	// aggregate's timestamp_ms is deliberately unchanged throughout.
	write(16, []abci.Event{feed(0, "112.5")})
	want["BTC"] = prices{ptr(110.25), ptr(112.5)}
	observe("block_order_and_independent_mark")
	write(17, []abci.Event{feed(0, "0")})
	want["BTC"] = prices{ptr(110.25), ptr(0)}
	observe("zero_index_and_null_basis")
	// Current price decoders emit sub=0. Vary only persisted source metadata
	// of genuine decoded/written events to exercise the reader's sub ordering;
	// the price payloads still come exclusively from those producer events.
	write(18, []abci.Event{feed(0, "115.5"), feed(0, "114.5")})
	proofRESTExec(t, db, `UPDATE perps.index_prices SET log_index=0, sub=1 WHERE block=18 AND log_index=1`)
	want["BTC"] = prices{ptr(110.25), ptr(114.5)}
	observe("aggregate_sub_order")
	write(19, nil, mark(0, 0, 0, "117.25", "116.5"), mark(0, 0, 1, "116.25", "115.5"))
	proofRESTExec(t, db, `UPDATE perps.mark_prices SET log_index=0, sub=1 WHERE block=19 AND log_index=1`)
	want["BTC"] = prices{ptr(116.25), ptr(115.5)}
	observe("mark_sub_order")
}

func TestProofRestProducerTransportConsumer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	databaseURL := proofRESTPostgres(t)
	db, err := store.Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(ctx, db); err != nil {
		db.Close()
		t.Fatal(err)
	}
	at := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	rest.ProofRESTSeedMarkets(t, db, at)
	env := rest.ProofRESTProduceSpot(t, db, at, 50)
	// Reopen the connection pool to demonstrate that only PostgreSQL carries
	// producer state; no constructor or test-side response cache supplies it.
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = store.Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	ns, err := server.NewServer(&server.Options{Host: "127.0.0.1", Port: server.RANDOM_PORT, JetStream: true, StoreDir: t.TempDir(), NoLog: true, NoSigs: true})
	if err != nil {
		t.Fatal(err)
	}
	go ns.Start()
	t.Cleanup(func() { ns.Shutdown(); ns.WaitForShutdown() })
	if !ns.ReadyForConnections(10 * time.Second) {
		t.Fatal("gateway NATS producer failed to start")
	}
	nc, err := nats.Connect(ns.ClientURL())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	node := proofRESTNode(t)
	metrics, err := telemetry.New(config.Gateway)
	if err != nil {
		t.Fatal(err)
	}
	gw, err := gateway.NewServer(ctx, gateway.Options{DB: db, NATS: nc, Chain: 125, AuthSecret: "proof-rest-local-secret-at-least-32-bytes", Node: node, Metrics: metrics})
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(gw.Handler())
	t.Cleanup(httpServer.Close)
	client := httpServer.Client()
	client.Timeout = 10 * time.Second
	pages, err := portal.NewPages(httpServer.URL, httpServer.URL+"/tx", client)
	if err != nil {
		t.Fatal(err)
	}
	spot, err := rest.NewSpotUsers(db, node)
	if err != nil {
		t.Fatal(err)
	}

	for round := 1; round <= 2; round++ {
		mark, index := float64(70000), float64(69999)
		if round == 2 {
			proofRESTExec(t, db, `UPDATE perps.mark_prices SET mark_price=71000*1e18::numeric, index_price=70999*1e18::numeric WHERE market_id=0`)
			env = rest.ProofRESTProduceSpot(t, db, at.Add(time.Second), 53)
			mark, index = 71000, 70999
		}
		for repeat := 0; repeat < 2; repeat++ {
			rest.ProofRESTObserveMarkets(t, client, httpServer.URL, mark)
			rest.ProofRESTObserveSymbol(t, client, httpServer.URL, index)
			rest.ProofRESTObserveSpot(t, client, httpServer.URL, round)
			rest.ProofRESTObserveVenue(t, client, httpServer.URL, mark)
			var home, market bytes.Buffer
			if err := pages.Markets(ctx, &home); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(home.String(), "Bitcoin") || !strings.Contains(home.String(), strconv.Itoa(int(mark))) {
				t.Fatalf("portal did not observe market: %s", home.String())
			}
			if err := pages.Market(ctx, &market, "btc"); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(market.String(), strconv.Itoa(int(index))) || !strings.Contains(market.String(), "140000") {
				t.Fatalf("portal market did not observe producer: %s", market.String())
			}
			proofRESTStream(t, ctx, spot, env, client, httpServer.URL)
		}
	}
	var missing bytes.Buffer
	if err := pages.Market(ctx, &missing, "unknown"); !errors.Is(err, portal.ErrNotFound) || missing.Len() != 0 {
		t.Fatalf("unknown market: %v, wrote %d", err, missing.Len())
	}

	// Both venue lookups must agree with Markets.Record for stored mixed case.
	proofRESTExec(t, db, `UPDATE perps.markets SET symbol='bTc' WHERE market_id=0`)
	rest.ProofRESTGet(t, client, httpServer.URL, "/api/ticker/BTC", http.StatusOK)
	rest.ProofRESTGet(t, client, httpServer.URL, "/api/orderbook/BTC", http.StatusOK)
	proofRESTExec(t, db, `UPDATE perps.markets SET symbol='' WHERE market_id=0`)
	// The route really emits this malformed identity; no HTTP layer is replaced.
	body := rest.ProofRESTGet(t, client, httpServer.URL, "/api/markets", http.StatusOK)
	var records []struct {
		Symbol string `json:"symbol"`
	}
	if err := json.Unmarshal(body["markets"], &records); err != nil || len(records) == 0 || records[0].Symbol != "" {
		t.Fatalf("malformed producer record: %s, %v", body["markets"], err)
	}
	var rejected bytes.Buffer
	if err := pages.Markets(ctx, &rejected); err == nil || !strings.Contains(err.Error(), "invalid market identity") || rejected.Len() != 0 {
		t.Fatalf("malformed record was not refused: %v, wrote %d", err, rejected.Len())
	}

	// The valid outbox event now names a missing producer fact. StreamItems
	// must fail so the gateway can close sockets and require REST resync.
	proofRESTExec(t, db, `DELETE FROM perps.spot_swaps WHERE block=55`)
	if items, err := spot.StreamItems(ctx, env); err == nil || !strings.Contains(err.Error(), "not found") || len(items) != 0 {
		t.Fatalf("missing producer trade accepted: %+v, %v", items, err)
	}
}

func proofRESTExec(t *testing.T, db *sql.DB, query string) {
	t.Helper()
	if _, err := db.Exec(query); err != nil {
		t.Fatal(err)
	}
}

func proofRESTStream(t *testing.T, ctx context.Context, spot *rest.SpotUsers, env delivery.Envelope, client *http.Client, base string) {
	t.Helper()
	items, err := spot.StreamItems(ctx, env)
	if err != nil {
		t.Fatal(err)
	}
	consumer := &ws.SpotUser{}
	ts := httptest.NewServer(consumer)
	defer ts.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(ts.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var control struct {
		Type string `json:"type"`
	}
	if err := conn.ReadJSON(&control); err != nil || control.Type != "hello" {
		t.Fatalf("socket hello: %+v %v", control, err)
	}
	address := "0x00000000000000000000000000000000000000a1"
	if err := conn.WriteJSON(map[string]string{"op": "subscribe", "stream": "trade", "address": address}); err != nil {
		t.Fatal(err)
	}
	if err := conn.ReadJSON(&control); err != nil || control.Type != "subscribed" {
		t.Fatalf("socket subscribe: %+v %v", control, err)
	}
	streams := map[string]bool{}
	var trade json.RawMessage
	for _, item := range items {
		streams[item.Stream] = true
		if item.Address != address {
			t.Fatalf("stream account %s", item.Address)
		}
		if err := consumer.Deliver(item.Stream, item.Address, item.Data); err != nil {
			t.Fatal(err)
		}
		if item.Stream == "trade" {
			trade = item.Data
		}
	}
	for _, name := range []string{"trade", "pnl", "closed-leg"} {
		if !streams[name] {
			t.Fatalf("StreamItems missing %s: %+v", name, items)
		}
	}
	var frame struct {
		Stream  string
		Address string
		Data    json.RawMessage
	}
	if err := conn.ReadJSON(&frame); err != nil {
		t.Fatal(err)
	}
	if frame.Stream != "trade" || frame.Address != address || !bytes.Equal(frame.Data, trade) {
		t.Fatalf("socket consumer: %+v, want %s", frame, trade)
	}
	body := rest.ProofRESTGet(t, client, base, "/v1/users/"+address+"/trades?limit=1", http.StatusOK)
	var trades []json.RawMessage
	if err := json.Unmarshal(body["items"], &trades); err != nil || len(trades) != 1 {
		t.Fatalf("REST trade consumer: %s %v", body["items"], err)
	}
	if !bytes.Equal(trades[0], trade) {
		t.Fatalf("REST/socket trade mismatch: %s / %s", trades[0], trade)
	}
}

// proofRESTPostgres owns a temporary database process. It reads no environment
// URL, path or configuration name and reports a missing executable as failure.
func proofRESTPostgres(t *testing.T) string {
	t.Helper()
	bins, err := filepath.Glob("/usr/lib/postgresql/*/bin/initdb")
	if err != nil || len(bins) == 0 {
		t.Fatalf("producer PostgreSQL missing: /usr/lib/postgresql/*/bin/initdb: %v", err)
	}
	bin := filepath.Dir(bins[len(bins)-1])
	dir, err := os.MkdirTemp("", "proof-rest-postgres-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	var credential *syscall.Credential
	if os.Geteuid() == 0 {
		account, err := user.Lookup("postgres")
		if err != nil {
			t.Fatalf("PostgreSQL producer service account: %v", err)
		}
		uid, err := strconv.ParseUint(account.Uid, 10, 32)
		if err != nil {
			t.Fatal(err)
		}
		gid, err := strconv.ParseUint(account.Gid, 10, 32)
		if err != nil {
			t.Fatal(err)
		}
		credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
		if err := os.Chown(dir, int(uid), int(gid)); err != nil {
			t.Fatal(err)
		}
	}
	run := func(name string, args ...string) error {
		cmd := exec.Command(filepath.Join(bin, name), args...)
		cmd.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "HOME=" + dir}
		if credential != nil {
			cmd.SysProcAttr = &syscall.SysProcAttr{Credential: credential}
		}
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s: %w: %s", cmd.Path, err, out)
		}
		return nil
	}
	data := filepath.Join(dir, "data")
	if err := run("initdb", "-D", data, "-A", "trust", "-U", "postgres", "--no-locale"); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	options := fmt.Sprintf("-h 127.0.0.1 -p %d -k %s -F -c max_connections=30", port, dir)
	if err := run("pg_ctl", "-D", data, "-l", filepath.Join(dir, "postgres.log"), "-o", options, "-w", "-t", "15", "start"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := run("pg_ctl", "-D", data, "-m", "immediate", "-w", "stop"); err != nil {
			t.Error(err)
		}
	})
	return fmt.Sprintf("postgres://postgres@127.0.0.1:%d/postgres?sslmode=disable&connect_timeout=5", port)
}

// proofRESTNode hosts the repository's Ethereum service and real WPAX EVM
// bytecode, whose balanceOf reads the empty holdings after the fully sold leg.
func proofRESTNode(t *testing.T) *ethclient.Client {
	t.Helper()
	code, _, _, err := evmruntime.Create(wpax.GetBin(), &evmruntime.Config{GasLimit: 30000000})
	if err != nil || len(code) == 0 {
		t.Fatalf("modules/evm/artifacts/wpax producer deployment: %v", err)
	}
	stack, err := gethnode.New(&gethnode.Config{P2P: p2p.Config{NoDiscovery: true, MaxPeers: 0, ListenAddr: "127.0.0.1:0"}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := stack.Close(); err != nil {
			t.Error(err)
		}
	})
	genesis := &core.Genesis{Config: params.AllDevChainProtocolChanges, GasLimit: 30000000, BaseFee: big.NewInt(params.InitialBaseFee), Alloc: types.GenesisAlloc{
		common.HexToAddress("0x000000000000000000000000000000000000a202"): {Code: code, Balance: new(big.Int)},
	}}
	if _, err := eth.New(stack, &ethconfig.Config{Genesis: genesis, RPCGasCap: 30000000}); err != nil {
		t.Fatal(err)
	}
	if err := stack.Start(); err != nil {
		t.Fatal(err)
	}
	client := ethclient.NewClient(stack.Attach())
	t.Cleanup(client.Close)
	return client
}
