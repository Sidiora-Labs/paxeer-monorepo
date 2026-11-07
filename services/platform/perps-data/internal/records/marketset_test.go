package records

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/store"
	"github.com/lib/pq"
)

func marketSetDB(t *testing.T) *sql.DB {
	t.Helper()
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
	name := "perps_records_" + hex.EncodeToString(suffix[:])
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
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db
}

func insertChainMarket(t *testing.T, db *sql.DB, symbol, id string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO perps.markets
		(market_id, symbol, name, oracle_symbol, sz_decimals, px_decimals,
		max_leverage, maintenance_margin_bps, max_open_interest, enabled, source, refreshed_block)
		VALUES ($1, $2, $2, $2, NULL, NULL, 1000, 5, 1000000, true, 'chain', 1)`, id, symbol); err != nil {
		t.Fatal(err)
	}
}

func TestMarketSetLoadsChainMarkets(t *testing.T) {
	db := marketSetDB(t)
	ctx := context.Background()
	var initialCount, dogeCount int
	if err := db.QueryRow(`SELECT count(*), count(*) FILTER (WHERE symbol = 'DOGE') FROM perps.markets`).Scan(&initialCount, &dogeCount); err != nil {
		t.Fatal(err)
	}
	if dogeCount != 0 {
		t.Fatal("DOGE must be absent before its chain record is inserted")
	}
	insertChainMarket(t, db, "DOGE", "18")
	insertChainMarket(t, db, "LARGE", "18446744073709551616")
	set := NewMarketSet(db, time.Hour)
	rows, err := db.Query(`SELECT symbol, market_id::text FROM perps.markets`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var symbol, want string
		if err := rows.Scan(&symbol, &want); err != nil {
			t.Fatal(err)
		}
		got, ok, err := set.ID(ctx, symbol)
		if err != nil || !ok || got != want {
			t.Fatalf("ID(%q) = %q, %v, %v; want %q, true, nil", symbol, got, ok, err, want)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if count != initialCount+2 {
		t.Fatalf("checked %d markets; want %d", count, initialCount+2)
	}
	insertChainMarket(t, db, "MISS", "20")
	if id, ok, err := set.ID(ctx, "MISS"); err != nil || !ok || id != "20" {
		t.Fatalf("first miss ID(MISS) = %q, %v, %v; want 20, true, nil", id, ok, err)
	}
	if id, ok, err := set.ID(ctx, "NOPE"); err != nil || ok || id != "" {
		t.Fatalf("ID(NOPE) = %q, %v, %v", id, ok, err)
	}
	firstMiss := set.lastAttempt
	insertChainMarket(t, db, "LATER", "19")
	if _, ok, err := set.ID(ctx, "LATER"); err != nil || ok {
		t.Fatalf("second miss within TTL = %v, %v; want false, nil", ok, err)
	}
	if !set.lastAttempt.Equal(firstMiss) {
		t.Fatal("repeated miss refreshed within the TTL")
	}
	set.loadedAt = time.Now().Add(-2 * set.ttl)
	if id, ok, err := set.ID(ctx, "BTC"); err != nil || !ok || id != "0" {
		t.Fatalf("expired cached hit = %q, %v, %v", id, ok, err)
	}
	if id, ok, err := set.ID(ctx, "LATER"); err != nil || !ok || id != "19" {
		t.Fatalf("refreshed ID(LATER) = %q, %v, %v", id, ok, err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	set.loadedAt = time.Now().Add(-2 * set.ttl)
	if _, _, err := set.ID(ctx, "BTC"); err == nil {
		t.Fatal("expired cache hid the database error")
	}
}

func TestSymbolResolvesInsertedMarket(t *testing.T) {
	db := marketSetDB(t)
	ctx := context.Background()
	symbol, err := NewSymbol(db)
	if err != nil {
		t.Fatal(err)
	}
	if symbol.markets.ttl != 30*time.Second {
		t.Fatalf("NewSymbol TTL = %s; want 30s", symbol.markets.ttl)
	}
	symbol.markets.ttl = 10 * time.Millisecond
	if _, err := symbol.Price(ctx, "DOGE"); !errors.Is(err, ErrUnknownSymbol) {
		t.Fatalf("unlisted DOGE error = %v; want ErrUnknownSymbol", err)
	}
	insertChainMarket(t, db, "DOGE", "18")
	at := time.Now().UTC().Truncate(time.Millisecond)
	if _, err := db.Exec(`INSERT INTO perps.mark_prices
		(block, block_hash, tx_index, log_index, sub, tx_hash, receipt_kind,
		block_time, emitter, kind, account, market_id, mark_price, index_price, mid_price)
		VALUES (1, $1, 0, 0, 0, $1, 'tx', $2, $3, 'mark', $3, 18,
		'1250000000000000000', '1200000000000000000', '1250000000000000000')`,
		make([]byte, 32), at, make([]byte, 20)); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	price, err := symbol.Price(ctx, "DOGE")
	if err != nil {
		t.Fatalf("Price(DOGE) after TTL: %v", err)
	}
	if price.Symbol != "DOGE" || price.Price == nil || *price.Price != 1.2 ||
		price.Timestamp == nil || *price.Timestamp != at.Format(ISOMillis) {
		t.Fatalf("Price(DOGE) = %+v; want DOGE, 1.2, %s", price, at.Format(ISOMillis))
	}
	for name, call := range map[string]func(string) error{
		"Price":      func(s string) error { _, err := symbol.Price(ctx, s); return err },
		"Stats":      func(s string) error { _, err := symbol.Stats(ctx, s); return err },
		"PerpStats":  func(s string) error { _, err := symbol.PerpStats(ctx, s); return err },
		"Trades":     func(s string) error { _, err := symbol.Trades(ctx, s, 10); return err },
		"PerpTrades": func(s string) error { _, err := symbol.PerpTrades(ctx, s, 10); return err },
		"Health":     func(s string) error { _, err := symbol.Health(ctx, s); return err },
	} {
		t.Run(name, func(t *testing.T) {
			if err := call("DOGE"); err != nil {
				t.Fatalf("chain market: %v", err)
			}
			if err := call("NOPE"); !errors.Is(err, ErrUnknownSymbol) {
				t.Fatalf("unlisted market: %v; want ErrUnknownSymbol", err)
			}
		})
	}
}
