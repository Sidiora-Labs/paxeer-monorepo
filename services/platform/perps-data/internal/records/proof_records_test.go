// Proves: records.Symbol and MarketSet -> exported records bodies and REST JSON -> feeds.Providers and portal.Pages.
//
// The proof owns PostgreSQL, applies store.Migrate, writes real fact rows, and
// stops and reopens the store before constructing the consumers. No database,
// transport, record reader, REST handler, feed builder or portal decoder is
// replaced. Two reads at each of two producer revisions prove that response
// bodies are read afresh, while unknown symbols and incomplete producer rows
// are refused by the real consumers.
//
// Repairs: internal/feeds/topics.go Providers.records omitted MarketID from
// MarketRecord, unlike REST; internal/records/records.go Decimal accepted NaN
// and infinities which cannot be transported as JSON numbers. The feed schema
// assertion in internal/feeds/feeds_test.go now includes market_id. The portal's
// GraphQL Int boundary deliberately refuses an otherwise valid uint256 market
// identifier above int32, before requesting histories or rendering a page.
package records_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/graph-gophers/graphql-go/relay"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/feeds"
	gatewayql "github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/graphql"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/portal"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/records"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/rest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/store"
)

// proofRecordsStore hosts its producer rather than using an operator's URL.
// PostgreSQL's installed binaries are prerequisites, never a reason to skip.
type proofRecordsStore struct {
	db     *sql.DB
	reopen func()
}

func newProofRecordsStore(t *testing.T) *proofRecordsStore {
	t.Helper()
	bins, err := filepath.Glob("/usr/lib/postgresql/*/bin/initdb")
	if err != nil || len(bins) == 0 {
		t.Fatalf("records producer: missing /usr/lib/postgresql/*/bin/initdb: %v", err)
	}
	bin := filepath.Dir(bins[len(bins)-1])
	dir, err := os.MkdirTemp("/tmp", "proof-records-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Error(err)
		}
	})
	var credential *syscall.Credential
	if os.Geteuid() == 0 {
		owner, err := user.Lookup("postgres")
		if err != nil {
			t.Fatalf("records producer: PostgreSQL process owner: %v", err)
		}
		uid, err := strconv.ParseUint(owner.Uid, 10, 32)
		if err != nil {
			t.Fatal(err)
		}
		gid, err := strconv.ParseUint(owner.Gid, 10, 32)
		if err != nil {
			t.Fatal(err)
		}
		credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
		if err := os.Chown(dir, int(uid), int(gid)); err != nil {
			t.Fatal(err)
		}
	}
	run := func(name string, args ...string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, filepath.Join(bin, name), args...)
		cmd.Dir = dir
		cmd.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C"}
		if credential != nil {
			cmd.SysProcAttr = &syscall.SysProcAttr{Credential: credential}
		}
		if out, err := cmd.CombinedOutput(); err != nil {
			log, _ := os.ReadFile(filepath.Join(dir, "postgres.log"))
			t.Fatalf("records producer %s: %v\n%s\n%s", cmd.Path, err, out, log)
		}
	}
	data := filepath.Join(dir, "data")
	run("initdb", "-D", data, "-U", "proof", "--auth=trust", "--no-locale", "--encoding=UTF8")
	start := func() {
		run("pg_ctl", "-D", data, "-l", filepath.Join(dir, "postgres.log"), "-o",
			"-F -h '' -k "+dir+" -p 5432", "-w", "-t", "30", "start")
	}
	stop := func() { run("pg_ctl", "-D", data, "-m", "fast", "-w", "-t", "30", "stop") }
	start()
	s := &proofRecordsStore{}
	t.Cleanup(func() {
		if s.db != nil {
			_ = s.db.Close()
		}
		stop()
	})
	open := func() {
		var err error
		s.db, err = store.Open("host=" + dir + " port=5432 user=proof dbname=postgres sslmode=disable")
		if err != nil {
			t.Fatalf("records producer store.Open: %v", err)
		}
	}
	open()
	if err := store.Migrate(context.Background(), s.db); err != nil {
		t.Fatal(err)
	}
	s.reopen = func() {
		if err := s.db.Close(); err != nil {
			t.Fatal(err)
		}
		stop()
		start()
		open()
	}
	return s
}

func proofRecordsExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("producer SQL: %v\n%s", err, query)
	}
}

// Facts use the actual migration schema and its constraints, including the
// nullable spot swap fields used below to prove a malformed result is refused.
func proofRecordsFact(t *testing.T, db *sql.DB, table, kind string, at time.Time, log int, columns string, values ...any) {
	t.Helper()
	common := []any{at, log, make([]byte, 32), make([]byte, 20), kind}
	params := make([]string, len(values))
	for i := range values {
		params[i] = fmt.Sprintf("$%d", i+6)
	}
	proofRecordsExec(t, db, `INSERT INTO perps.`+table+`
        (block, tx_index, log_index, sub, block_hash, tx_hash, receipt_kind, block_time, emitter, account, kind, `+columns+`)
        VALUES (10, 0, $2, 0, $3, $3, 'tx', $1, $4, $4, $5, `+strings.Join(params, ", ")+`)`, append(common, values...)...)
}

func proofRecordsSeed(t *testing.T, db *sql.DB, at time.Time) {
	t.Helper()
	// PAX is a real seeded market, and its registry rank is the spot reader's
	// published mapping. Resolve the identifier from producer state.
	var market string
	if err := db.QueryRow(`SELECT market_id::text FROM perps.markets WHERE symbol = 'PAX'`).Scan(&market); err != nil {
		t.Fatal(err)
	}
	token := bytes.Repeat([]byte{0x42}, 20)
	proofRecordsFact(t, db, "spot_token_registry", "token_registered", at, 0,
		"token, decimals, is_stablecoin", token, 6, false)
	proofRecordsFact(t, db, "spot_swaps", "market_order", at, 1,
		`"user", token, stablecoin_amount, token_amount, is_buy`, token, token, "10000000", "2000000", true)
	proofRecordsFact(t, db, "mark_prices", "mark", at, 2,
		"market_id, mark_price, index_price, mid_price", market, "101000000000000000000", "100000000000000000000", "101000000000000000000")
	proofRecordsFact(t, db, "fills", "fill", at, 3,
		"market_id, maker_order_id, taker_order_id, maker, taker, taker_is_long, price, size_usd, maker_fee_usd, taker_fee_usd",
		market, "1", "2", token, token, true, "100000000000000000000", "200000000000000000000", "0", "0")
	proofRecordsFact(t, db, "open_interest", "oi", at, 4,
		"market_id, long_oi, short_oi, delta_usd, is_increase", market, "300000000000000000000", "100000000000000000000", "0", true)
	proofRecordsFact(t, db, "funding_settlements", "settled", at, 5,
		"market_id, funding_rate, long_payment, short_payment", market, "12500000000000", "0", "0")
	proofRecordsExec(t, db, `INSERT INTO perps.cursors (name, height, hash) VALUES ('head', 10, $1)`, make([]byte, 32))
	proofRecordsExec(t, db, `INSERT INTO perps.market_stats_rolling
        (market_id, volume_24h, volume_7d, volume_30d, trades_count, open_interest_long, open_interest_short, last_price, open_24h, updated_block)
        VALUES ($1, 200000000000000000000, 700000000000000000000, 900000000000000000000, 1,
            300000000000000000000, 100000000000000000000, 100000000000000000000, 100000000000000000000, 10)`, market)
}

func proofRecordsJSON[T any](t *testing.T, raw []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("consumer decode: %v\n%s", err, raw)
	}
	return v
}

func proofRecordsGET(t *testing.T, client *http.Client, url string, status int) []byte {
	t.Helper()
	resp, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != status {
		t.Fatalf("%s: status %d, want %d: %s", url, resp.StatusCode, status, raw)
	}
	return raw
}

func proofRecordsFloat(t *testing.T, name string, got *float64, want float64) {
	t.Helper()
	if got == nil || *got != want {
		t.Fatalf("%s: got %v, want %g", name, got, want)
	}
}

func TestProofRecordsProducerTransportConsumer(t *testing.T) {
	ctx := context.Background()
	producer := newProofRecordsStore(t)
	at := time.Now().UTC().Truncate(time.Millisecond)
	proofRecordsSeed(t, producer.db, at)
	producer.reopen() // Prove persistence before constructing any consumer.
	db := producer.db
	set := records.NewMarketSet(db, 0)
	market, ok, err := set.ID(ctx, "PAX")
	if err != nil || !ok || market == "" {
		t.Fatalf("MarketSet.ID(PAX): %q %v %v", market, ok, err)
	}
	if id, ok, err := set.ID(ctx, "NOPE"); err != nil || ok || id != "" {
		t.Fatalf("MarketSet unknown: %q %v %v", id, ok, err)
	}
	symbol, err := records.NewSymbol(db)
	if err != nil {
		t.Fatal(err)
	}
	routes, err := rest.NewSymbol(db)
	if err != nil {
		t.Fatal(err)
	}
	markets, err := rest.NewMarkets(db, nil)
	if err != nil {
		t.Fatal(err)
	}
	p, err := feeds.NewProviders(ctx, db, nil)
	if err != nil {
		t.Fatal(err)
	}
	schema, err := gatewayql.NewSchema(db)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	routes.Register(mux)
	markets.Register(mux)
	mux.Handle("POST /graphql", &relay.Handler{Schema: schema})
	server := httptest.NewServer(mux)
	defer server.Close()
	client := server.Client()
	client.Timeout = 10 * time.Second
	pages, err := portal.NewPages(server.URL, server.URL+"/tx/", client)
	if err != nil {
		t.Fatal(err)
	}

	for round := 1; round <= 2; round++ {
		observedAt, backfill := at, "complete"
		if round == 2 {
			observedAt, backfill = at.Add(time.Millisecond), "pending"
			// Mutate real producer state without reconstructing any reader.
			proofRecordsExec(t, db, `UPDATE perps.mark_prices SET index_price = 120000000000000000000`)
			proofRecordsExec(t, db, `UPDATE perps.fills SET size_usd = 400000000000000000000`)
			proofRecordsExec(t, db, `UPDATE perps.open_interest SET long_oi = 500000000000000000000`)
			proofRecordsExec(t, db, `UPDATE perps.spot_swaps SET stablecoin_amount = 14000000`)
			proofRecordsExec(t, db, `UPDATE perps.market_stats_rolling SET volume_24h = 400000000000000000000,
                open_interest_long = 500000000000000000000`)
			proofRecordsExec(t, db, `UPDATE perps.mark_prices SET block_time = $1`, observedAt)
			proofRecordsExec(t, db, `UPDATE perps.fills SET block_time = $1`, observedAt)
			proofRecordsExec(t, db, `UPDATE perps.spot_swaps SET block_time = $1`, observedAt)
			proofRecordsExec(t, db, `DELETE FROM perps.cursors WHERE name = 'head'`)
		}
		index, volume, oi, spotPrice := 100.0, 200.0, 400.0, 5.0
		if round == 2 {
			index, volume, oi, spotPrice = 120, 400, 600, 7
		}
		for read := 0; read < 2; read++ {
			t.Run(fmt.Sprintf("revision_%d_read_%d", round, read), func(t *testing.T) {
				if id, ok, err := set.ID(ctx, "PAX"); err != nil || !ok || id != market {
					t.Fatalf("MarketSet reread: %q %v %v", id, ok, err)
				}
				price, err := symbol.Price(ctx, "PAX")
				if err != nil {
					t.Fatal(err)
				}
				proofRecordsFloat(t, "Price", price.Price, index)
				proofRecordsFloat(t, "Open", price.Open, 100)
				if price.Timestamp == nil || *price.Timestamp != observedAt.Format(records.ISOMillis) {
					t.Fatalf("Price timestamp: %+v", price)
				}
				stats, err := symbol.Stats(ctx, "PAX")
				if err != nil {
					t.Fatal(err)
				}
				if stats.Volume24h != volume || stats.OpenInterest != oi || stats.LastFundingRateHourly == nil || *stats.LastFundingRateHourly != "0.0000125" {
					t.Fatalf("Stats: %+v", stats)
				}
				proofRecordsFloat(t, "funding rate", stats.LastFundingRate, 0.0000125)
				perp, err := symbol.PerpStats(ctx, "PAX")
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(perp.Stats, stats) {
					t.Fatalf("PerpStats base: %+v", perp)
				}
				proofRecordsFloat(t, "mark", perp.MarkPrice, 101)
				proofRecordsFloat(t, "index", perp.IndexPrice, index)
				proofRecordsFloat(t, "long/short", perp.LongShortRatio, oi/100-1)
				trades, err := symbol.Trades(ctx, "PAX", 10)
				if err != nil {
					t.Fatal(err)
				}
				if trades.Count != 1 || len(trades.Trades) != 1 || trades.Trades[0] != (records.SpotTrade{ID: "10-1", Side: "BUY", Price: spotPrice, Size: 2, TradeTime: observedAt.UnixMilli()}) {
					t.Fatalf("Trades: %+v", trades)
				}
				perps, err := symbol.PerpTrades(ctx, "PAX", 10)
				if err != nil {
					t.Fatal(err)
				}
				if perps.Count != 1 || len(perps.Trades) != 1 || perps.Trades[0] != (records.PerpTrade{ID: "10-3", Side: "BUY", Price: 100, Contracts: volume / 100, Notional: volume, TradeTime: observedAt.UnixMilli()}) {
					t.Fatalf("PerpTrades: %+v", perps)
				}
				health, err := symbol.Health(ctx, "PAX")
				if err != nil {
					t.Fatal(err)
				}
				if health.Status != "ok" || health.Backfill != backfill || health.LastFill == nil || *health.LastFill != observedAt.Format(records.ISOMillis) {
					t.Fatalf("Health: %+v", health)
				}
				for route, want := range map[string]any{"price": price, "stats": stats, "perp_stats": perp, "trades": trades, "perp_trades": perps, "health": health} {
					got := proofRecordsJSON[any](t, proofRecordsGET(t, client, server.URL+"/api/pax/"+route, http.StatusOK))
					raw, err := json.Marshal(want)
					if err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(got, proofRecordsJSON[any](t, raw)) {
						t.Fatalf("REST %s: %v, want %s", route, got, raw)
					}
				}
				for topic, want := range map[string]any{"PAX@stats": stats, "PAX@perp_stats": perp} {
					raw, err := p.Snapshot(ctx, topic)
					if err != nil {
						t.Fatal(err)
					}
					expected, err := json.Marshal(want)
					if err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(proofRecordsJSON[any](t, raw), proofRecordsJSON[any](t, expected)) {
						t.Fatalf("feed %s: %s, want %s", topic, raw, expected)
					}
				}
				for topic, want := range map[string]any{"PAX@trade": trades.Trades, "PAX@perp_trade": perps.Trades} {
					raw, err := p.Snapshot(ctx, topic)
					if err != nil {
						t.Fatal(err)
					}
					tape := proofRecordsJSON[struct {
						Symbol string           `json:"symbol"`
						Trades []map[string]any `json:"trades"`
					}](t, raw)
					got := tape.Trades
					if tape.Symbol != "PAX" || len(got) != 1 || got[0]["symbol"] != "PAX" {
						t.Fatalf("feed %s: %s", topic, raw)
					}
					delete(got[0], "symbol")
					expected, err := json.Marshal(want)
					if err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(got, proofRecordsJSON[[]map[string]any](t, expected)) {
						t.Fatalf("feed %s trade: %v, want %s", topic, got, expected)
					}
				}
				restRecord := proofRecordsJSON[struct {
					Market records.MarketRecord `json:"market"`
				}](t,
					proofRecordsGET(t, client, server.URL+"/api/markets/pax", http.StatusOK)).Market
				raw, err := p.Snapshot(ctx, "PAX@market")
				if err != nil {
					t.Fatal(err)
				}
				feedRecord := proofRecordsJSON[records.MarketRecord](t, raw)
				for source, rec := range map[string]records.MarketRecord{"REST": restRecord, "feed": feedRecord} {
					if rec.MarketID != market || rec.Symbol != "PAX" || rec.Volume24h != volume || rec.OpenInterest != oi || rec.Funding8h == nil || *rec.Funding8h != "0.0001" {
						t.Fatalf("%s MarketRecord: %+v", source, rec)
					}
					proofRecordsFloat(t, source+" record price", rec.Price, index)
					proofRecordsFloat(t, source+" record mark", rec.PerpMarkPrice, 101)
					if rec.PerpVolume24hUSD != volume || rec.PerpVolume24hContracts != volume/100 {
						t.Fatalf("%s record volumes: %+v", source, rec)
					}
				}
				var html bytes.Buffer
				if err := pages.Markets(ctx, &html); err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(html.String(), `href="/markets/pax">PAX</a>`) || !strings.Contains(html.String(), fmt.Sprintf("<td>%g</td><td>101</td>", index)) {
					t.Fatalf("portal market table: %s", html.String())
				}
				html.Reset()
				if err := pages.Market(ctx, &html, "pax"); err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(html.String(), "10-3") || !strings.Contains(html.String(), fmt.Sprintf("<td>%g</td>", volume)) {
					t.Fatalf("portal market and fills: %s", html.String())
				}
			})
		}
	}

	// Each exported symbol producer must refuse an unknown identifier, and
	// the REST/portal consumers must preserve that refusal.
	for name, call := range map[string]func() error{
		"price":       func() error { _, err := symbol.Price(ctx, "NOPE"); return err },
		"stats":       func() error { _, err := symbol.Stats(ctx, "NOPE"); return err },
		"perp_stats":  func() error { _, err := symbol.PerpStats(ctx, "NOPE"); return err },
		"trades":      func() error { _, err := symbol.Trades(ctx, "NOPE", 10); return err },
		"perp_trades": func() error { _, err := symbol.PerpTrades(ctx, "NOPE", 10); return err },
		"health":      func() error { _, err := symbol.Health(ctx, "NOPE"); return err },
	} {
		if err := call(); !errors.Is(err, records.ErrUnknownSymbol) {
			t.Fatalf("%s unknown: %v", name, err)
		}
		body := proofRecordsJSON[map[string]string](t, proofRecordsGET(t, client, server.URL+"/api/nope/"+name, http.StatusNotFound))
		if body["error"] != "Unknown symbol: NOPE" {
			t.Fatalf("%s unknown body: %v", name, body)
		}
	}
	if _, err := p.Snapshot(ctx, "NOPE@market"); err == nil {
		t.Fatal("feed accepted unknown market")
	}
	var html bytes.Buffer
	if err := pages.Market(ctx, &html, "NOPE"); !errors.Is(err, portal.ErrNotFound) || html.Len() != 0 {
		t.Fatalf("portal unknown: %v, %s", err, html.String())
	}
	proofRecordsGET(t, client, server.URL+"/api/pax/trades?limit=bad", http.StatusBadRequest)

	// A schema-permitted incomplete swap is a malformed producer result.
	// Real records.Trades must fail scanning it; REST and feeds must not
	// turn the failure into an empty or cached successful tape.
	token := bytes.Repeat([]byte{0x42}, 20)
	proofRecordsFact(t, db, "spot_swaps", "simple_swap", at, 6, `"user", token_out`, token, token)
	if _, err := symbol.Trades(ctx, "PAX", 10); err == nil {
		t.Fatal("records.Trades decoded a NULL price/size")
	}
	proofRecordsGET(t, client, server.URL+"/api/pax/trades", http.StatusInternalServerError)
	if raw, err := p.Snapshot(ctx, "PAX@trade"); err == nil {
		t.Fatalf("feed accepted malformed producer trade: %s", raw)
	}

	// Real market producer JSON can carry a uint256 outside the portal's
	// GraphQL Int range. Refuse it at identity validation with no partial HTML.
	proofRecordsExec(t, db, `UPDATE perps.markets SET market_id = 2147483648 WHERE symbol = 'PAX'`)
	if id, ok, err := set.ID(ctx, "PAX"); err != nil || !ok || id != "2147483648" {
		t.Fatalf("MarketSet hid changed producer identity: %q %v %v", id, ok, err)
	}
	html.Reset()
	if err := pages.Market(ctx, &html, "PAX"); err == nil || err.Error() != "portal: invalid market identity" || html.Len() != 0 {
		t.Fatalf("portal accepted unsupported producer ID: %v, %s", err, html.String())
	}
}

func TestProofRecordsDecimalRefusesMalformedProducerNumbers(t *testing.T) {
	for _, raw := range []string{"NaN", "+Inf", "-Inf", "not-a-number", "1e999"} {
		if value, err := records.Decimal(sql.NullString{String: raw, Valid: true}); err == nil || value != nil {
			t.Fatalf("Decimal(%q): %v, %v", raw, value, err)
		}
		if value, err := records.DecimalOrZero(sql.NullString{String: raw, Valid: true}); err == nil || value != 0 {
			t.Fatalf("DecimalOrZero(%q): %v, %v", raw, value, err)
		}
	}
	if value, err := records.Decimal(sql.NullString{}); err != nil || value != nil {
		t.Fatalf("nullable decimal: %v, %v", value, err)
	}
	value, err := records.Decimal(sql.NullString{String: "-0.0000125", Valid: true})
	if err != nil {
		t.Fatal(err)
	}
	proofRecordsFloat(t, "finite decimal", value, -0.0000125)
}
