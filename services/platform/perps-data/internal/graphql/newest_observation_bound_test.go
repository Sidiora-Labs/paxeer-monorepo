package graphql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/store"
	"github.com/lib/pq"
)

const newestObservationRowsPerMarket = 200
const newestObservationHistoryRows = 20000

type newestObservationPlan struct {
	NodeType  string                  `json:"Node Type"`
	Schema    string                  `json:"Schema"`
	Relation  string                  `json:"Relation Name"`
	Rows      float64                 `json:"Actual Rows"`
	Loops     float64                 `json:"Actual Loops"`
	Filtered  float64                 `json:"Rows Removed by Filter"`
	Rechecked float64                 `json:"Rows Removed by Index Recheck"`
	Plans     []newestObservationPlan `json:"Plans"`
}

func (p newestObservationPlan) priceRowsRead() float64 {
	var rows float64
	if p.Schema == "perps" && (p.Relation == "mark_prices" || p.Relation == "index_prices") && strings.Contains(p.NodeType, "Scan") {
		rows = (p.Rows + p.Filtered + p.Rechecked) * p.Loops
	}
	for _, child := range p.Plans {
		rows += child.priceRowsRead()
	}
	return rows
}

func newestObservationExplain(t *testing.T, ctx context.Context, db *sql.DB, query string, args ...any) float64 {
	t.Helper()
	var raw []byte
	if err := db.QueryRowContext(ctx, "EXPLAIN (ANALYZE, VERBOSE, FORMAT JSON, TIMING OFF, SUMMARY OFF) "+query, args...).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var plans []struct{ Plan newestObservationPlan }
	if err := json.Unmarshal(raw, &plans); err != nil {
		t.Fatal(err)
	}
	if len(plans) != 1 || plans[0].Plan.Loops == 0 {
		t.Fatalf("expected one executed plan: %s", raw)
	}
	read := plans[0].Plan.priceRowsRead()
	if read > newestObservationRowsPerMarket {
		t.Logf("executed price observation plan: %s", raw)
	}
	return read
}

type newestObservationResult struct {
	Columns []string
	Types   []string
	Rows    [][]any
}

func newestObservationRead(t *testing.T, ctx context.Context, db *sql.DB, query string, args ...any) newestObservationResult {
	t.Helper()
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var result newestObservationResult
	result.Columns, err = rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	types, err := rows.ColumnTypes()
	if err != nil {
		t.Fatal(err)
	}
	for _, typ := range types {
		result.Types = append(result.Types, typ.DatabaseTypeName())
	}
	for rows.Next() {
		values := make([]any, len(result.Columns))
		dest := make([]any, len(values))
		for i := range values {
			dest[i] = &values[i]
		}
		if err := rows.Scan(dest...); err != nil {
			t.Fatal(err)
		}
		for i, value := range values {
			switch value := value.(type) {
			case []byte:
				values[i] = string(value)
			case time.Time:
				values[i] = value.UTC()
			}
		}
		result.Rows = append(result.Rows, values)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

func newestObservationSeed(t *testing.T, ctx context.Context, db *sql.DB, anchor time.Time) {
	t.Helper()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec("TRUNCATE perps.market_tiers, perps.markets CASCADE")
	exec(`INSERT INTO perps.markets
        (market_id, symbol, name, oracle_symbol, sz_decimals, px_decimals, max_leverage,
         maintenance_margin_bps, max_open_interest, enabled, source)
        SELECT i, 'M' || i, 'Market ' || i, 'M' || i, 6, 2, 100, 50, 1000000, true, 'chain'
        FROM generate_series(1, 6) i`)

	// Dense history reaches the anchor throughout the price change windows.
	// Position components distinguish observations within the same block.
	for _, source := range []string{"mark_prices", "index_prices"} {
		columns, values := "mark_price, mid_price", "100, 100"
		kind := "mark"
		if source == "index_prices" {
			columns, values = "sources, required", "3, 2"
			kind = "aggregate"
		}
		exec(`INSERT INTO perps.`+source+`
            (block, block_hash, tx_index, log_index, sub, tx_hash, receipt_kind, block_time,
             emitter, kind, account, market_id, index_price, `+columns+`)
            SELECT i + 8, decode(repeat('00',32),'hex'), m, 0, 0, decode(lpad(to_hex((i + 8) * 1000 + m),64,'0'),'hex'),
                'tx', $1::timestamptz - interval '5 minutes' + i * (interval '5 minutes' / $2::integer),
                decode(repeat('00',20),'hex'), '`+kind+`', decode(repeat('00',20),'hex'), m, i + m, `+values+`
            FROM generate_series(1, $2::integer) i CROSS JOIN generate_series(1, 3) m`, anchor, newestObservationHistoryRows)
	}

	// Markets 4 and 5 have a single source, 6 is empty, and 7 has no catalog row.
	// Every window has an observation exactly at its inclusive boundary.
	ages := []time.Duration{30 * 24 * time.Hour, 7 * 24 * time.Hour, 3 * 24 * time.Hour, 24 * time.Hour, 4 * time.Hour, time.Hour, 30 * time.Minute, 5 * time.Minute, 0}
	for market := 1; market <= 7; market++ {
		if market == 6 {
			continue
		}
		for i, age := range ages {
			block := i + 1
			if age == 0 {
				block += newestObservationHistoryRows
			}
			at := anchor.Add(-age)
			for _, source := range []string{"mark_prices", "index_prices"} {
				if market == 4 && source == "index_prices" || market == 5 && source == "mark_prices" {
					continue
				}
				columns, extra, kind := "mark_price, mid_price", "100, 100", "mark"
				if source == "index_prices" {
					columns, extra, kind = "sources, required", "3, 2", "aggregate"
				}
				positions := [][3]int{{market * 10, 0, 0}}
				if age == 0 {
					positions = [][3]int{{market * 10, 2, 0}, {market*10 + 1, 0, 0}, {market*10 + 1, 1, 0}, {market*10 + 1, 1, 1}}
				}
				for j, pos := range positions {
					// Alternate which source wins at equal block heights.
					if market%2 == 0 && source == "index_prices" || market%2 == 1 && source == "mark_prices" {
						pos[0] += 2
					}
					price := market*100000 + i*100 + j
					if source == "index_prices" {
						price += 50
					}
					exec(`INSERT INTO perps.`+source+`
                        (block, block_hash, tx_index, log_index, sub, tx_hash, receipt_kind, block_time,
                         emitter, kind, account, market_id, index_price, `+columns+`)
                        VALUES ($1::bigint, decode(repeat('00',32),'hex'), $2::integer, $3::integer, $4::integer,
                            decode(lpad(to_hex($1::bigint * 1000 + $2::integer),64,'0'),'hex'), 'tx', $5,
                            decode(repeat('00',20),'hex'), '`+kind+`', decode(repeat('00',20),'hex'), $6, $7, `+extra+`)`,
						block, pos[0], pos[1], pos[2], at, market, price)
				}
			}
		}
	}
	// Neither a skipped aggregate nor an aggregate without a price is an observation.
	exec(`INSERT INTO perps.index_prices
        (block, block_hash, tx_index, log_index, sub, tx_hash, receipt_kind, block_time,
         emitter, kind, account, market_id, index_price, sources, required)
        SELECT $1::bigint + i, decode(repeat('00',32),'hex'), m, 0, 0,
            decode(lpad(to_hex(($1::bigint + i) * 1000 + m),64,'0'),'hex'), 'tx', $2::timestamptz + i * interval '1 second',
            decode(repeat('00',20),'hex'), CASE WHEN i=1 THEN 'skipped' ELSE 'aggregate' END,
            decode(repeat('00',20),'hex'), m, NULL, 1, 2
        FROM generate_series(1,2) i CROSS JOIN (VALUES (1), (2), (3), (5)) markets(m)`, newestObservationHistoryRows+9, anchor)
	exec(`INSERT INTO perps.funding_rates
        (block, block_hash, tx_index, log_index, sub, tx_hash, receipt_kind, block_time,
         emitter, kind, account, market_id, new_rate_per_second, funding_rate24h)
        SELECT 1, decode(repeat('00',32),'hex'), i, 0, 0, decode(lpad(to_hex(i),64,'0'),'hex'), 'tx', $1,
            decode(repeat('00',20),'hex'), 'rate', decode(repeat('00',20),'hex'), i, i, i * 24
        FROM generate_series(1,6) i`, anchor)
	exec("ANALYZE perps.mark_prices")
	exec("ANALYZE perps.index_prices")
	exec("ANALYZE perps.markets")
}

func TestNewestObservationReadsStayBounded(t *testing.T) {
	raw := os.Getenv("PERPS_DATA_TEST_DATABASE_URL")
	if raw == "" {
		t.Fatal("PERPS_DATA_TEST_DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	db := retentionDatabase(t, ctx, raw)
	if err := store.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	anchor := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	newestObservationSeed(t, ctx, db, anchor)

	for _, source := range []string{"mark_prices", "index_prices"} {
		for market := 1; market <= 3; market++ {
			var count int
			if err := db.QueryRowContext(ctx, "SELECT count(*) FROM perps."+source+" WHERE market_id=$1", market).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count < newestObservationHistoryRows {
				t.Fatalf("%s market %d has %d observations, want at least %d", source, market, count, newestObservationHistoryRows)
			}
		}
	}

	var chronological bool
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(bool_and(previous <= block_time), true) FROM (
        SELECT block_time, lag(block_time) OVER (ORDER BY block) AS previous FROM (
            SELECT block, block_time FROM perps.mark_prices
            UNION SELECT block, block_time FROM perps.index_prices
        ) observations
    ) ordered`).Scan(&chronological); err != nil {
		t.Fatal(err)
	}
	if !chronological {
		t.Fatal("price observation block times must rise with blocks")
	}
	for _, source := range []string{"mark_prices", "index_prices"} {
		var recent int
		if err := db.QueryRowContext(ctx, "SELECT count(*) FROM perps."+source+" WHERE market_id = 1 AND block_time > $1::timestamptz - interval '5 minutes' AND block_time <= $1", anchor).Scan(&recent); err != nil {
			t.Fatal(err)
		}
		if recent < newestObservationHistoryRows {
			t.Fatalf("%s must contain dense observations newer than the shortest window", source)
		}
	}

	cases := []struct {
		name, query, previous string
		markets               int
		args                  []any
	}{
		{"marketState", marketStateSQL, previousMarketStateSQL, 6, []any{pq.Array([]int64{1, 2, 3, 4, 5, 6})}},
		{"latestIndex", latestIndexSQL(indexPriceMarketsSQL) + "ORDER BY m.market_id", previousLatestIndexSQL, 6, nil},
		{"tickers", orderlyTickersSQL, previousTickersSQL, 6, []any{anchor.Add(-24 * time.Hour)}},
		{"priceChanges", orderlyPriceChangesSQL(), previousPriceChangesSQL, 6, []any{anchor}},
		{"latestPrices", latestPricesSQL, previousLatestPricesSQL, 6, nil},
	}
	for market := 1; market <= 8; market++ {
		cases = append(cases, struct {
			name, query, previous string
			markets               int
			args                  []any
		}{fmt.Sprintf("poolState/%d", market), latestIndexSQL("SELECT $1::numeric AS market_id"), previousPoolStateSQL, 1, []any{market}})
		cases = append(cases, struct {
			name, query, previous string
			markets               int
			args                  []any
		}{fmt.Sprintf("subMarket/%d", market), subMarketSQL, previousSubMarketSQL, 1, []any{market}})
		cases = append(cases, struct {
			name, query, previous string
			markets               int
			args                  []any
		}{fmt.Sprintf("marketState/%d", market), marketStateSQL, previousMarketStateSQL, 1, []any{pq.Array([]int64{int64(market)})}})
	}
	for market := 1; market <= 8; market++ {
		cases = append(cases, struct {
			name, query, previous string
			markets               int
			args                  []any
		}{fmt.Sprintf("priceChanges/%d", market), "SELECT * FROM (" + orderlyPriceChangesSQL() + ") observations WHERE symbol = $2", "SELECT * FROM (" + previousPriceChangesSQL + ") observations WHERE symbol = $2", 1, []any{anchor, fmt.Sprintf("M%d", market)}})
		cases = append(cases, struct {
			name, query, previous string
			markets               int
			args                  []any
		}{fmt.Sprintf("tickers/%d", market), "SELECT * FROM (" + orderlyTickersSQL + ") observations WHERE symbol = $2", "SELECT * FROM (" + previousTickersSQL + ") observations WHERE symbol = $2", 1, []any{anchor.Add(-24 * time.Hour), fmt.Sprintf("M%d", market)}})
	}
	for market := 1; market <= 8; market++ {
		for _, read := range []struct{ name, query, previous string }{
			{"latestIndex", latestIndexSQL(indexPriceMarketsSQL) + "ORDER BY m.market_id", previousLatestIndexSQL},
			{"latestPrices", latestPricesSQL, previousLatestPricesSQL},
		} {
			cases = append(cases, struct {
				name, query, previous string
				markets               int
				args                  []any
			}{fmt.Sprintf("%s/%d", read.name, market), "SELECT * FROM (" + read.query + ") observations WHERE market_id = $1",
				"SELECT * FROM (" + read.previous + ") observations WHERE market_id = $1", 1, []any{market}})
		}
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := newestObservationRead(t, ctx, db, tc.previous, tc.args...)
			got := newestObservationRead(t, ctx, db, tc.query, tc.args...)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("observation rows or column types differ\ngot: %#v\nwant: %#v", got, want)
			}
			read := newestObservationExplain(t, ctx, db, tc.query, tc.args...)
			bound := newestObservationRowsPerMarket * tc.markets
			t.Logf("price source rows read: %.0f, bound: %d (%d per market)", read, bound, newestObservationRowsPerMarket)
			if read > float64(bound) {
				t.Fatalf("read %.0f price source rows for %d markets, bound %d", read, tc.markets, bound)
			}
		})
	}
	read := newestObservationExplain(t, ctx, db, previousMarketStateSQL, pq.Array([]int64{1}))
	if read <= newestObservationRowsPerMarket {
		t.Fatalf("full-union selection read %.0f price source rows, want more than %d", read, newestObservationRowsPerMarket)
	}
	t.Logf("full-union price source rows read for one market: %.0f", read)

	resolver := &Resolver{DB: db}
	latest, err := resolver.LatestPrices(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(latest) != 6 {
		t.Fatalf("priced markets: got %d, want 6 including the market outside the catalog", len(latest))
	}
	for i, market := range []int32{1, 2, 3, 4, 5, 7} {
		price := int(market)*100000 + 803
		if market == 2 || market == 5 {
			price += 50
		}
		if latest[i].MarketID != market || string(latest[i].Price) != fmt.Sprint(price) || latest[i].BlockNumber != newestObservationHistoryRows+9 {
			t.Fatalf("market %d latest observation: %+v", market, latest[i])
		}
	}
}

const previousMarketStateSQL = `SELECT m.id, mp.index_price::text, floor(extract(epoch FROM mp.block_time))::bigint, mp.block, mp.block_time,
    fr.row_id, fr.new_rate_per_second::text, fr.funding_rate24h::text, fr.block, '0x' || encode(fr.tx_hash, 'hex'), fr.block_time
FROM unnest($1::bigint[]) AS m(id)
LEFT JOIN LATERAL (SELECT index_price, block, block_time FROM (SELECT market_id, index_price, block, block_time, tx_index, log_index, sub
FROM perps.mark_prices
UNION ALL
SELECT market_id, index_price, block, block_time, tx_index, log_index, sub
FROM perps.index_prices WHERE kind = 'aggregate' AND index_price IS NOT NULL) prices
    WHERE market_id = m.id::numeric ORDER BY block DESC, tx_index DESC, log_index DESC, sub DESC LIMIT 1) mp ON true
LEFT JOIN LATERAL (SELECT row_id, new_rate_per_second, funding_rate24h, block, tx_hash, block_time FROM perps.funding_rates
    WHERE market_id = m.id::numeric ORDER BY block DESC, tx_index DESC, log_index DESC, sub DESC LIMIT 1) fr ON true`

const previousLatestIndexSQL = `SELECT DISTINCT ON (market_id) market_id, index_price::text, floor(extract(epoch FROM block_time))::bigint,
    block, block_time FROM (SELECT market_id, index_price, block, block_time, tx_index, log_index, sub
FROM perps.mark_prices
UNION ALL
SELECT market_id, index_price, block, block_time, tx_index, log_index, sub
FROM perps.index_prices WHERE kind = 'aggregate' AND index_price IS NOT NULL) prices ORDER BY market_id, block DESC, tx_index DESC, log_index DESC, sub DESC`

const previousPoolStateSQL = `SELECT DISTINCT ON (market_id) market_id, index_price::text, floor(extract(epoch FROM block_time))::bigint,
    block, block_time FROM (SELECT market_id, index_price, block, block_time, tx_index, log_index, sub
FROM perps.mark_prices
UNION ALL
SELECT market_id, index_price, block, block_time, tx_index, log_index, sub
FROM perps.index_prices WHERE kind = 'aggregate' AND index_price IS NOT NULL) prices WHERE market_id = $1::numeric ORDER BY market_id, block DESC, tx_index DESC, log_index DESC, sub DESC`

const previousTickersSQL = `SELECT 'PERP_' || upper(m.symbol) || '_USDC', m.symbol, mp.mark_price / 1e18, ip.index_price / 1e18, NULL::float8,
    trim_scale((SELECT funding_rate FROM perps.funding_settlements x WHERE x.market_id = m.market_id ORDER BY x.block DESC, x.tx_index DESC, x.log_index DESC, x.sub DESC LIMIT 1) * 0.000000000000000001)::text, (SELECT funding_rate FROM perps.funding_settlements x WHERE x.market_id = m.market_id ORDER BY x.block DESC, x.tx_index DESC, x.log_index DESC, x.sub DESC LIMIT 1) / 1e18,
    NULL::float8, (s.open_interest_long + s.open_interest_short) / 1e18, h.open / 1e18, h.close / 1e18, h.high / 1e18,
    h.low / 1e18, h.amount / 1e18, h.volume
FROM perps.markets m
LEFT JOIN perps.market_stats_rolling s ON s.market_id = m.market_id
LEFT JOIN LATERAL (SELECT x.mark_price FROM perps.mark_prices x WHERE x.market_id = m.market_id
    ORDER BY x.block DESC, x.tx_index DESC, x.log_index DESC, x.sub DESC LIMIT 1) mp ON true
LEFT JOIN LATERAL (SELECT x.index_price FROM (SELECT market_id, index_price, block, block_time, tx_index, log_index, sub
FROM perps.mark_prices
UNION ALL
SELECT market_id, index_price, block, block_time, tx_index, log_index, sub
FROM perps.index_prices WHERE kind = 'aggregate' AND index_price IS NOT NULL) x WHERE x.market_id = m.market_id
    ORDER BY x.block DESC, x.tx_index DESC, x.log_index DESC, x.sub DESC LIMIT 1) ip ON true
LEFT JOIN LATERAL (SELECT (array_agg(price ORDER BY block, tx_index, log_index, sub))[1] AS open,
    (array_agg(price ORDER BY block DESC, tx_index DESC, log_index DESC, sub DESC))[1] AS close, max(price) AS high, min(price) AS low, sum(size_usd) AS amount,
    sum(size_usd / NULLIF(price, 0)) AS volume
    FROM perps.fills WHERE market_id = m.market_id AND block_time >= $1) h ON true
ORDER BY m.market_id`

const previousPriceChangesSQL = `SELECT 'PERP_' || upper(m.symbol) || '_USDC', m.symbol, l.index_price / 1e18, (SELECT (l.index_price - x.index_price) / NULLIF(x.index_price, 0) FROM (SELECT market_id, index_price, block, block_time, tx_index, log_index, sub
FROM perps.mark_prices
UNION ALL
SELECT market_id, index_price, block, block_time, tx_index, log_index, sub
FROM perps.index_prices WHERE kind = 'aggregate' AND index_price IS NOT NULL) x
        WHERE x.market_id = m.market_id AND x.block_time <= $1::timestamptz - interval '5 minutes'
        ORDER BY x.block DESC, x.tx_index DESC, x.log_index DESC, x.sub DESC LIMIT 1), (SELECT (l.index_price - x.index_price) / NULLIF(x.index_price, 0) FROM (SELECT market_id, index_price, block, block_time, tx_index, log_index, sub
FROM perps.mark_prices
UNION ALL
SELECT market_id, index_price, block, block_time, tx_index, log_index, sub
FROM perps.index_prices WHERE kind = 'aggregate' AND index_price IS NOT NULL) x
        WHERE x.market_id = m.market_id AND x.block_time <= $1::timestamptz - interval '30 minutes'
        ORDER BY x.block DESC, x.tx_index DESC, x.log_index DESC, x.sub DESC LIMIT 1), (SELECT (l.index_price - x.index_price) / NULLIF(x.index_price, 0) FROM (SELECT market_id, index_price, block, block_time, tx_index, log_index, sub
FROM perps.mark_prices
UNION ALL
SELECT market_id, index_price, block, block_time, tx_index, log_index, sub
FROM perps.index_prices WHERE kind = 'aggregate' AND index_price IS NOT NULL) x
        WHERE x.market_id = m.market_id AND x.block_time <= $1::timestamptz - interval '1 hour'
        ORDER BY x.block DESC, x.tx_index DESC, x.log_index DESC, x.sub DESC LIMIT 1), (SELECT (l.index_price - x.index_price) / NULLIF(x.index_price, 0) FROM (SELECT market_id, index_price, block, block_time, tx_index, log_index, sub
FROM perps.mark_prices
UNION ALL
SELECT market_id, index_price, block, block_time, tx_index, log_index, sub
FROM perps.index_prices WHERE kind = 'aggregate' AND index_price IS NOT NULL) x
        WHERE x.market_id = m.market_id AND x.block_time <= $1::timestamptz - interval '4 hours'
        ORDER BY x.block DESC, x.tx_index DESC, x.log_index DESC, x.sub DESC LIMIT 1), (SELECT (l.index_price - x.index_price) / NULLIF(x.index_price, 0) FROM (SELECT market_id, index_price, block, block_time, tx_index, log_index, sub
FROM perps.mark_prices
UNION ALL
SELECT market_id, index_price, block, block_time, tx_index, log_index, sub
FROM perps.index_prices WHERE kind = 'aggregate' AND index_price IS NOT NULL) x
        WHERE x.market_id = m.market_id AND x.block_time <= $1::timestamptz - interval '24 hours'
        ORDER BY x.block DESC, x.tx_index DESC, x.log_index DESC, x.sub DESC LIMIT 1), (SELECT (l.index_price - x.index_price) / NULLIF(x.index_price, 0) FROM (SELECT market_id, index_price, block, block_time, tx_index, log_index, sub
FROM perps.mark_prices
UNION ALL
SELECT market_id, index_price, block, block_time, tx_index, log_index, sub
FROM perps.index_prices WHERE kind = 'aggregate' AND index_price IS NOT NULL) x
        WHERE x.market_id = m.market_id AND x.block_time <= $1::timestamptz - interval '3 days'
        ORDER BY x.block DESC, x.tx_index DESC, x.log_index DESC, x.sub DESC LIMIT 1), (SELECT (l.index_price - x.index_price) / NULLIF(x.index_price, 0) FROM (SELECT market_id, index_price, block, block_time, tx_index, log_index, sub
FROM perps.mark_prices
UNION ALL
SELECT market_id, index_price, block, block_time, tx_index, log_index, sub
FROM perps.index_prices WHERE kind = 'aggregate' AND index_price IS NOT NULL) x
        WHERE x.market_id = m.market_id AND x.block_time <= $1::timestamptz - interval '7 days'
        ORDER BY x.block DESC, x.tx_index DESC, x.log_index DESC, x.sub DESC LIMIT 1), (SELECT (l.index_price - x.index_price) / NULLIF(x.index_price, 0) FROM (SELECT market_id, index_price, block, block_time, tx_index, log_index, sub
FROM perps.mark_prices
UNION ALL
SELECT market_id, index_price, block, block_time, tx_index, log_index, sub
FROM perps.index_prices WHERE kind = 'aggregate' AND index_price IS NOT NULL) x
        WHERE x.market_id = m.market_id AND x.block_time <= $1::timestamptz - interval '30 days'
        ORDER BY x.block DESC, x.tx_index DESC, x.log_index DESC, x.sub DESC LIMIT 1)
FROM perps.markets m
JOIN LATERAL (SELECT x.index_price FROM (SELECT market_id, index_price, block, block_time, tx_index, log_index, sub
FROM perps.mark_prices
UNION ALL
SELECT market_id, index_price, block, block_time, tx_index, log_index, sub
FROM perps.index_prices WHERE kind = 'aggregate' AND index_price IS NOT NULL) x WHERE x.market_id = m.market_id
    ORDER BY x.block DESC, x.tx_index DESC, x.log_index DESC, x.sub DESC LIMIT 1) l ON true
ORDER BY m.market_id`

const previousLatestPricesSQL = `SELECT m.market_id::int4, mp.index_price::text, floor(extract(epoch FROM mp.block_time))::int4, mp.block::int4, mp.block_time
FROM perps.markets m
CROSS JOIN LATERAL (SELECT p.index_price, p.block, p.block_time FROM (SELECT market_id, index_price, block, block_time, tx_index, log_index, sub
FROM perps.mark_prices
UNION ALL
SELECT market_id, index_price, block, block_time, tx_index, log_index, sub
FROM perps.index_prices WHERE kind = 'aggregate' AND index_price IS NOT NULL) p WHERE p.market_id = m.market_id
    ORDER BY p.block DESC, p.tx_index DESC, p.log_index DESC, p.sub DESC LIMIT 1) mp
ORDER BY m.market_id`

const previousSubMarketSQL = `SELECT m.market_id::int4, m.name, m.symbol, m.max_leverage::text, m.enabled,
    mp.index_price::text, floor(extract(epoch FROM mp.block_time))::int4, mp.block::int4, mp.block_time,
    fr.row_id::int4, fr.new_rate_per_second::text, fr.funding_rate24h::text, fr.block::int4, '0x' || encode(fr.tx_hash, 'hex'), fr.block_time
FROM perps.markets m
LEFT JOIN LATERAL (SELECT p.index_price, p.block, p.block_time FROM (SELECT market_id, index_price, block, block_time, tx_index, log_index, sub
FROM perps.mark_prices
UNION ALL
SELECT market_id, index_price, block, block_time, tx_index, log_index, sub
FROM perps.index_prices WHERE kind = 'aggregate' AND index_price IS NOT NULL) p WHERE p.market_id = m.market_id
    ORDER BY p.block DESC, p.tx_index DESC, p.log_index DESC, p.sub DESC LIMIT 1) mp ON true
LEFT JOIN LATERAL (SELECT r.row_id, r.new_rate_per_second, r.funding_rate24h, r.block, r.tx_hash, r.block_time FROM perps.funding_rates r
    WHERE r.market_id = m.market_id ORDER BY r.block DESC, r.tx_index DESC, r.log_index DESC, r.sub DESC LIMIT 1) fr ON true
WHERE m.market_id = $1`
