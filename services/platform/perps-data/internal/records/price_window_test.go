package records

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/testdb"
)

func TestPriceWindow(t *testing.T) {
	db := testdb.Open(t)
	now := time.Now().UTC().Truncate(time.Millisecond)
	testdb.SeedPrices(t, db, now)
	check := func(id string, got *float64, want float64) {
		t.Helper()
		if got == nil || *got != want {
			t.Fatalf("market %s price %v, want %g", id, got, want)
		}
	}
	prices, err := PriceWindow(context.Background(), db, now)
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM perps.markets`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if len(prices) != count {
		t.Fatalf("prices for %d markets, want %d", len(prices), count)
	}
	check("0", prices["0"].Price, 83)
	check("0", prices["0"].MarkPrice, 83)
	check("1", prices["1"].Price, 85) // The mark event follows the aggregate in its block.
	check("1", prices["1"].MarkPrice, 90)
	for _, id := range []string{"2", "3"} {
		p, ok := prices[id]
		if !ok || p.Price != nil || p.MarkPrice != nil {
			t.Fatalf("market %s: %+v, present %v", id, p, ok)
		}
	}
	// The later aggregate within the boundary must beat the mark candidate.
	testdb.Exec(t, db, `UPDATE perps.index_prices SET block_time = $1 WHERE block = 6`, now.Add(-24*time.Hour))
	prices, err = PriceWindow(context.Background(), db, now)
	if err != nil {
		t.Fatal(err)
	}
	check("1", prices["1"].Price, 150)
	check("1", prices["1"].MarkPrice, 90)
	if prices["2"].MarkPrice != nil {
		t.Fatal("index-only history supplied a mark")
	}
	check("2", prices["2"].Price, 160)
}

func TestPriceWindowPlan(t *testing.T) {
	db := testdb.Open(t)
	now := time.Now().UTC().Truncate(time.Millisecond)
	for _, table := range []string{"mark_prices", "index_prices"} {
		columns, values := "mark_price, index_price, mid_price", "100e18, 99e18, 100e18"
		kind := "mark"
		if table == "index_prices" {
			columns, values, kind = "index_price, sources, required", "99e18, 3, 3", "aggregate"
		}
		testdb.Exec(t, db, fmt.Sprintf(`INSERT INTO perps.%s
   (block, block_hash, tx_index, log_index, sub, tx_hash, receipt_kind, block_time, emitter, kind, account, market_id, %s)
   SELECT n, decode(md5(n::text),'hex') || decode(md5(n::text),'hex'), 0, 0, 0,
    decode(md5(n::text),'hex') || decode(md5(n::text),'hex'), 'tx',
    $1::timestamptz - interval '48 hours' + n * interval '1 second',
    decode(repeat('00',20),'hex'), '%s', decode(repeat('00',20),'hex'), n %% 18, %s
   FROM generate_series(1,200000) n`, table, columns, kind, values), now)
		testdb.Exec(t, db, `ANALYZE perps.`+table)
		var count int
		if err := db.QueryRow(`SELECT count(*) FROM perps.` + table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 200000 {
			t.Fatalf("%s rows = %d", table, count)
		}
	}
	var raw []byte
	if err := db.QueryRow(`EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) `+priceWindowSQL, now.Add(-Window24h), USDScale).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	t.Logf("price window plan: %s", raw)
	var plans []struct{ Plan map[string]any }
	if err := json.Unmarshal(raw, &plans); err != nil {
		t.Fatal(err)
	}
	timeSeek, marketSeek := map[string]bool{}, map[string]bool{}
	marketEquality := regexp.MustCompile(`\bmarket_id\)*\s*(?:::\w+\s*)?=`)
	var walk func(map[string]any)
	walk = func(node map[string]any) {
		table, _ := node["Relation Name"].(string)
		typ, _ := node["Node Type"].(string)
		if table == "mark_prices" || table == "index_prices" {
			if strings.Contains(typ, "Seq Scan") {
				t.Fatalf("sequential read of %s: %s", table, raw)
			}
			cond, _ := node["Index Cond"].(string)
			if strings.Contains(cond, "block_time <=") {
				timeSeek[table] = true
			}
			if marketEquality.MatchString(cond) && strings.Contains(cond, "block <=") {
				marketSeek[table] = true
			}
		}
		children, _ := node["Plans"].([]any)
		for _, child := range children {
			walk(child.(map[string]any))
		}
	}
	walk(plans[0].Plan)
	for _, table := range []string{"mark_prices", "index_prices"} {
		if !timeSeek[table] || !marketSeek[table] {
			t.Fatalf("%s missing bounded seeks: %s", table, raw)
		}
	}
}
