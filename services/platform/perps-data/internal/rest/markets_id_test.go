package rest

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"testing"

	"github.com/lib/pq"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/store"
)

func TestMarketRecordCarriesID(t *testing.T) {
	raw := os.Getenv("PERPS_DATA_TEST_DATABASE_URL")
	if raw == "" {
		t.Fatal("PERPS_DATA_TEST_DATABASE_URL is unset")
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	name := "perps_market_record_" + hex.EncodeToString(suffix[:])
	admin, err := store.Open(raw)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	if _, err := admin.Exec(`CREATE DATABASE ` + pq.QuoteIdentifier(name)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(`DROP DATABASE ` + pq.QuoteIdentifier(name) + ` WITH (FORCE)`); err != nil {
			t.Error(err)
		}
	})
	u.Path = "/" + name
	db, err := store.Open(u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO perps.markets
		(market_id, symbol, name, oracle_symbol, sz_decimals, px_decimals,
		max_leverage, maintenance_margin_bps, max_open_interest, enabled, source, refreshed_block)
		VALUES (18, 'DOGE', 'Dogecoin', 'DOGE', NULL, NULL, 1000, 5, 1000000, true, 'chain', 1)`); err != nil {
		t.Fatal(err)
	}
	var wantID string
	if err := db.QueryRow(`SELECT market_id::text FROM perps.markets WHERE symbol = 'DOGE'`).Scan(&wantID); err != nil {
		t.Fatal(err)
	}
	if wantID != "18" {
		t.Fatalf("DOGE database market_id = %q, want 18", wantID)
	}
	markets, err := NewMarkets(db, nil)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	markets.Register(mux)
	for _, path := range []string{"/api/markets/DOGE", "/api/markets/doge"} {
		body := serveOK(t, mux, path)
		record := field[map[string]json.RawMessage](t, body, "market")
		if got := field[string](t, record, "market_id"); got != wantID {
			t.Fatalf("%s: market_id = %q, want %q", path, got, wantID)
		}
		if got := field[string](t, record, "symbol"); got != "DOGE" {
			t.Fatalf("%s: symbol = %q, want DOGE", path, got)
		}
	}
	body := serveOK(t, mux, "/api/markets")
	listed := field[[]map[string]json.RawMessage](t, body, "markets")
	found := false
	for _, record := range listed {
		symbol := field[string](t, record, "symbol")
		var id string
		if err := db.QueryRow(`SELECT market_id::text FROM perps.markets WHERE symbol = $1`, symbol).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if got := field[string](t, record, "market_id"); got != id {
			t.Fatalf("listed %s market_id = %q, want %q", symbol, got, id)
		}
		found = found || symbol == "DOGE"
	}
	if !found {
		t.Fatal("/api/markets omitted DOGE")
	}
	code, _ := serve(t, mux, "/api/markets/NOPE")
	if code != http.StatusNotFound {
		t.Fatalf("unknown market status = %d, want 404", code)
	}
}
