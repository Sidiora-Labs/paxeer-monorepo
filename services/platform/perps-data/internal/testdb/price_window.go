// Package testdb supplies isolated PostgreSQL databases for market read cases.
package testdb

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/store"
	"github.com/lib/pq"
)

func Open(t *testing.T) *sql.DB {
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
	name := "perps_window_" + hex.EncodeToString(suffix[:])
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

// SeedPrices supplies equal index and mark prices for BTC, separate price
// producers for ETH, a recent-only SOL market and an empty AVAX market.
func SeedPrices(t *testing.T, db *sql.DB, now time.Time) {
	t.Helper()
	Exec(t, db, `INSERT INTO perps.mark_prices
  (block, block_hash, tx_index, log_index, sub, tx_hash, receipt_kind, block_time, emitter, kind, account,
   market_id, mark_price, index_price, mid_price)
 SELECT block, decode(md5(block::text), 'hex') || decode(md5(block::text), 'hex'), tx, log, sub,
  decode(md5(block::text || ':' || tx::text), 'hex') || decode(md5(block::text || ':' || tx::text), 'hex'), 'tx', $1::timestamptz + age * interval '1 hour',
  decode(repeat('00',20),'hex'), 'mark', decode(repeat('00',20),'hex'), market,
  mark * 1e18::numeric, idx * 1e18::numeric, mark * 1e18::numeric
 FROM (VALUES
  (1,0,0,0,0,-30,70,70), (2,0,0,0,0,-25,80,80),
  (2,1,0,0,0,-25,81,81), (2,1,1,0,0,-25,82,82), (2,1,1,1,0,-25,83,83),
  (3,0,0,0,1,-24,90,85),
  (5,0,0,0,0,-2,120,120), (5,0,1,0,1,-2,130,125), (5,0,2,0,2,-2,140,140)
 ) v(block,tx,log,sub,market,age,mark,idx)`, now)
	Exec(t, db, `INSERT INTO perps.index_prices
  (block, block_hash, tx_index, log_index, sub, tx_hash, receipt_kind, block_time, emitter, kind, account,
   market_id, index_price, sources, required)
 SELECT block, decode(md5(block::text), 'hex') || decode(md5(block::text), 'hex'), -1, log, 0,
  decode(md5(block::text), 'hex') || decode(md5(block::text), 'hex'), 'block_event', $1::timestamptz + age * interval '1 hour',
  decode(repeat('00',20),'hex'), kind, decode(repeat('00',20),'hex'), market, idx * 1e18::numeric, 3, 3
 FROM (VALUES
  (2,0,1,-25,'aggregate',84),
  (3,0,1,-24,'aggregate',86), (3,1,1,-24,'aggregate',87),
  (4,0,1,-24,'skipped',NULL), (6,0,1,-1,'aggregate',150),
  (6,1,2,-1,'aggregate',160)
 ) v(block,log,market,age,kind,idx)`, now)
}

func Exec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("price facts: %v", err)
	}
}
