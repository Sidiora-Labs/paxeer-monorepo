package markets

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io/fs"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/httpx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/telemetry"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/migrations"
)

const (
	testDatabaseEnv = "PERPS_DATA_TEST_DATABASE_URL"
	testRPCEnv      = "PERPS_DATA_TEST_RPC"
	seedFile        = "0005_markets_seed.sql"
)

type row struct {
	id                   uint64
	symbol               string
	name                 string
	oracleSymbol         string
	szDecimals           sql.NullInt16
	pxDecimals           sql.NullInt16
	maxLeverage          string
	maintenanceMarginBps string
	maxOpenInterest      string
	enabled              bool
	source               string
	refreshedBlock       sql.NullInt64
}

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	url := os.Getenv(testDatabaseEnv)
	if url == "" {
		t.Fatal("TestMarkets requires " + testDatabaseEnv)
	}
	db, err := store.Open(url)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return db
}

func testClient(t *testing.T) *ethclient.Client {
	t.Helper()
	url := os.Getenv(testRPCEnv)
	if url == "" {
		t.Fatal("TestMarkets requires " + testRPCEnv)
	}
	client, err := ethclient.Dial(url)
	if err != nil {
		t.Fatalf("dial %s: %v", testRPCEnv, err)
	}
	t.Cleanup(client.Close)
	return client
}

// reseed restores perps.markets to the content of the embedded seed migration
// and empties perps.fee_config.
func reseed(t *testing.T, db *sql.DB) {
	t.Helper()
	body, err := fs.ReadFile(migrations.FS, seedFile)
	if err != nil {
		t.Fatalf("read %s: %v", seedFile, err)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	for _, stmt := range []string{`DELETE FROM perps.market_tiers`, `DELETE FROM perps.markets`, `DELETE FROM perps.fee_config`, string(body)} {
		if _, err := tx.Exec(stmt); err != nil {
			_ = tx.Rollback()
			t.Fatalf("reseed: %v", err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("reseed commit: %v", err)
	}
}

func readRows(t *testing.T, db *sql.DB) []row {
	t.Helper()
	rows, err := db.Query(`SELECT market_id::bigint, symbol, name, oracle_symbol, sz_decimals, px_decimals, max_leverage::text, maintenance_margin_bps::text, max_open_interest::text, enabled, source, refreshed_block FROM perps.markets ORDER BY market_id`)
	if err != nil {
		t.Fatalf("read perps.markets: %v", err)
	}
	defer rows.Close()
	var out []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.symbol, &r.name, &r.oracleSymbol, &r.szDecimals, &r.pxDecimals, &r.maxLeverage, &r.maintenanceMarginBps, &r.maxOpenInterest, &r.enabled, &r.source, &r.refreshedBlock); err != nil {
			t.Fatalf("scan perps.markets: %v", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read perps.markets: %v", err)
	}
	return out
}

func seedRow(m Market) row {
	return row{
		id:                   m.ID,
		symbol:               m.Symbol,
		name:                 m.Name,
		oracleSymbol:         m.OracleSymbol,
		szDecimals:           sql.NullInt16{Int16: int16(m.SzDecimals), Valid: true},
		pxDecimals:           sql.NullInt16{Int16: int16(m.PxDecimals), Valid: true},
		maxLeverage:          big.NewInt(int64(m.MaxLeverage)).String(),
		maintenanceMarginBps: big.NewInt(int64(m.MaintenanceMarginBps)).String(),
		maxOpenInterest:      m.MaxOpenInterest,
		enabled:              m.Enabled,
		source:               "seed",
	}
}

func requireSeeded(t *testing.T, db *sql.DB) {
	t.Helper()
	got := readRows(t, db)
	if len(got) != len(Seed) {
		t.Fatalf("perps.markets holds %d rows, want %d", len(got), len(Seed))
	}
	for i, m := range Seed {
		if want := seedRow(m); got[i] != want {
			t.Fatalf("market %d = %+v, want %+v", i, got[i], want)
		}
	}
}

func height(t *testing.T, client *ethclient.Client) int64 {
	t.Helper()
	h, err := client.BlockNumber(context.Background())
	if err != nil {
		t.Fatalf("BlockNumber: %v", err)
	}
	return int64(h)
}

func TestRefreshMarginTiers(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	client := testClient(t)
	reseed(t, db)
	insertedID := Seed[len(Seed)-1].ID
	res, err := db.ExecContext(ctx, `DELETE FROM perps.markets WHERE market_id = $1`, insertedID)
	if err != nil {
		t.Fatalf("remove market for refresh insertion: %v", err)
	}
	if n, err := res.RowsAffected(); err != nil || n != 1 {
		t.Fatalf("removed %d markets, want 1: %v", n, err)
	}

	for pass := range 2 {
		before := height(t, client)
		if err := Refresh(ctx, db, client); err != nil {
			t.Fatalf("Refresh pass %d: %v", pass, err)
		}
		after := height(t, client)
		markets := readRows(t, db)
		if len(markets) == 0 {
			t.Fatal("Refresh stored no markets")
		}
		block := markets[0].refreshedBlock
		if !block.Valid || block.Int64 < before || block.Int64 > after {
			t.Fatalf("refreshed_block %v, want latest height in [%d, %d]", block, before, after)
		}
		c, _, err := latest(ctx, client)
		if err != nil {
			t.Fatalf("latest: %v", err)
		}
		c.at = big.NewInt(block.Int64)
		out, err := c.call(ctx, "totalMarkets")
		if err != nil {
			t.Fatalf("totalMarkets: %v", err)
		}
		count, err := field[*big.Int](out, 0, "totalMarkets")
		if err != nil {
			t.Fatal(err)
		}
		if count.Cmp(big.NewInt(int64(len(markets)))) != 0 {
			t.Fatalf("stored %d markets, chain has %s", len(markets), count)
		}
		total := 0
		foundInserted := false
		for i, m := range markets {
			if m.id != uint64(i) || m.refreshedBlock != block {
				t.Fatalf("market %d has id %d and height %v, want id %d and height %v", i, m.id, m.refreshedBlock, i, block)
			}
			foundInserted = foundInserted || m.id == insertedID
			out, err := c.call(ctx, "getMarginTiers", new(big.Int).SetUint64(m.id))
			if err != nil {
				t.Fatalf("getMarginTiers(%d) at %d: %v", m.id, block.Int64, err)
			}
			var decoded struct{ Tiers []marginTier }
			if err := c.abi.Methods["getMarginTiers"].Outputs.Copy(&decoded, out); err != nil {
				t.Fatalf("decode getMarginTiers(%d): %v", m.id, err)
			}
			rows, err := db.QueryContext(ctx, `SELECT tier, lower_bound_usd::text, max_leverage::text, maintenance_bps::text, deduction_usd::text, refreshed_block FROM perps.market_tiers WHERE market_id = $1 ORDER BY tier`, m.id)
			if err != nil {
				t.Fatalf("read market %d tiers: %v", m.id, err)
			}
			n := 0
			for rows.Next() {
				var tier int
				var lower, leverage, maintenance, deduction string
				var refreshed int64
				if err := rows.Scan(&tier, &lower, &leverage, &maintenance, &deduction, &refreshed); err != nil {
					rows.Close()
					t.Fatalf("scan market %d tier: %v", m.id, err)
				}
				if n >= len(decoded.Tiers) {
					rows.Close()
					t.Fatalf("market %d has extra tier %d on pass %d", m.id, tier, pass)
				}
				want := decoded.Tiers[n]
				if tier != n || lower != want.LowerBoundUsd.String() || leverage != want.MaxLeverage.String() || maintenance != want.MaintenanceBps.String() || deduction != want.DeductionUsd.String() || refreshed != block.Int64 {
					rows.Close()
					t.Fatalf("market %d tier %d = (%s, %s, %s, %s) at %d, want tier %d %+v at %d", m.id, tier, lower, leverage, maintenance, deduction, refreshed, n, want, block.Int64)
				}
				n++
			}
			if err := rows.Err(); err != nil {
				rows.Close()
				t.Fatalf("read market %d tiers: %v", m.id, err)
			}
			if err := rows.Close(); err != nil {
				t.Fatalf("close market %d tiers: %v", m.id, err)
			}
			if n != len(decoded.Tiers) {
				t.Fatalf("market %d has %d tiers, want %d on pass %d", m.id, n, len(decoded.Tiers), pass)
			}
			total += n
			if pass == 0 {
				if _, err := db.ExecContext(ctx, `UPDATE perps.market_tiers SET lower_bound_usd = 1, max_leverage = 1, maintenance_bps = 1, deduction_usd = 1, refreshed_block = 0 WHERE market_id = $1`, m.id); err != nil {
					t.Fatalf("overwrite market %d tiers: %v", m.id, err)
				}
				if _, err := db.ExecContext(ctx, `INSERT INTO perps.market_tiers (market_id, tier, lower_bound_usd, max_leverage, maintenance_bps, deduction_usd, refreshed_block) VALUES ($1, $2, 1, 1, 1, 1, 0)`, m.id, n); err != nil {
					t.Fatalf("insert stale tier for market %d: %v", m.id, err)
				}
			}
		}
		if !foundInserted || total == 0 {
			t.Fatalf("Refresh inserted market %d: %t, stored %d chain tiers", insertedID, foundInserted, total)
		}
	}
}

func TestSeedMatchesChainConfig(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	client := testClient(t)
	reseed(t, db)
	requireSeeded(t, db)

	c, block, err := latest(ctx, client)
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	for _, seed := range Seed {
		t.Run(seed.Symbol, func(t *testing.T) {
			config, err := c.getMarketConfig(ctx, seed.ID)
			if err != nil {
				t.Fatalf("getMarketConfig(%d) at %d: %v", seed.ID, block, err)
			}
			if seed.SzDecimals != config.SzDecimals {
				t.Errorf("market %d seed sz_decimals = %d, chain = %d at %d", seed.ID, seed.SzDecimals, config.SzDecimals, block)
			}
			if seed.PxDecimals != config.PxDecimals {
				t.Errorf("market %d seed px_decimals = %d, chain = %d at %d", seed.ID, seed.PxDecimals, config.PxDecimals, block)
			}
			if seed.OracleSymbol != config.OracleSymbol {
				t.Errorf("market %d seed oracle_symbol = %q, chain = %q at %d", seed.ID, seed.OracleSymbol, config.OracleSymbol, block)
			}
		})
	}
}

func TestRefreshMarketConfig(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	client := testClient(t)
	reseed(t, db)
	insertedID := Seed[len(Seed)-1].ID
	res, err := db.ExecContext(ctx, `DELETE FROM perps.markets WHERE market_id = $1`, insertedID)
	if err != nil {
		t.Fatalf("remove market for refresh insertion: %v", err)
	}
	if n, err := res.RowsAffected(); err != nil || n != 1 {
		t.Fatalf("removed %d markets, want 1: %v", n, err)
	}

	for _, phase := range []string{"insert_and_update", "update"} {
		t.Run(phase, func(t *testing.T) {
			if _, err := db.ExecContext(ctx, `UPDATE perps.markets SET oracle_symbol = 'stale', sz_decimals = NULL, px_decimals = NULL, tick_size = NULL, lot_size = NULL, tier_class = NULL, impact_notional_usd = NULL, oi_cap_usd = NULL, funding_cap_ppm_per_hour = NULL, listed_at = to_timestamp(1)`); err != nil {
				t.Fatalf("clear stored config: %v", err)
			}
			before := height(t, client)
			if err := Refresh(ctx, db, client); err != nil {
				t.Fatalf("Refresh: %v", err)
			}
			after := height(t, client)
			got := readRows(t, db)
			if len(got) == 0 {
				t.Fatal("Refresh stored no markets")
			}
			block := got[0].refreshedBlock
			if !block.Valid || block.Int64 < before || block.Int64 > after {
				t.Fatalf("refreshed_block %v, want latest height in [%d, %d]", block, before, after)
			}
			c, _, err := latest(ctx, client)
			if err != nil {
				t.Fatalf("latest: %v", err)
			}
			c.at = big.NewInt(block.Int64)
			out, err := c.call(ctx, "totalMarkets")
			if err != nil {
				t.Fatalf("totalMarkets: %v", err)
			}
			count, err := field[*big.Int](out, 0, "totalMarkets")
			if err != nil {
				t.Fatal(err)
			}
			if count.Cmp(big.NewInt(int64(len(got)))) != 0 {
				t.Fatalf("stored %d markets, chain has %s", len(got), count)
			}
			foundInserted := false
			for _, r := range got {
				out, err := c.call(ctx, "getMarketConfig", new(big.Int).SetUint64(r.id))
				if err != nil {
					t.Fatalf("getMarketConfig(%d) at %d: %v", r.id, block.Int64, err)
				}
				var decoded struct{ Config marketConfig }
				if err := c.abi.Methods["getMarketConfig"].Outputs.Copy(&decoded, out); err != nil {
					t.Fatalf("decode getMarketConfig(%d): %v", r.id, err)
				}
				want := decoded.Config
				if want.MarketId.Cmp(new(big.Int).SetUint64(r.id)) != 0 || r.symbol != want.Symbol || r.name != want.Name || r.oracleSymbol != want.OracleSymbol ||
					!r.szDecimals.Valid || r.szDecimals.Int16 != int16(want.SzDecimals) || !r.pxDecimals.Valid || r.pxDecimals.Int16 != int16(want.PxDecimals) ||
					r.maxLeverage != want.MaxLeverage.String() || r.enabled != want.Enabled || r.source != "chain" || r.refreshedBlock != block {
					t.Fatalf("market %d = %+v, want chain config %+v at %d", r.id, r, want, block.Int64)
				}
				var tick, lot, impact, oiCap, fundingCap string
				var tierClass int16
				var listedAt sql.NullInt64
				if err := db.QueryRowContext(ctx, `SELECT tick_size::text, lot_size::text, tier_class, impact_notional_usd::text, oi_cap_usd::text, funding_cap_ppm_per_hour::text, EXTRACT(EPOCH FROM listed_at)::bigint FROM perps.markets WHERE market_id = $1`, r.id).
					Scan(&tick, &lot, &tierClass, &impact, &oiCap, &fundingCap, &listedAt); err != nil {
					t.Fatalf("read config for market %d: %v", r.id, err)
				}
				if tick != want.TickSize.String() || lot != want.LotSize.String() || tierClass != int16(want.TierClass) || impact != want.ImpactNotionalUsd.String() || oiCap != want.OiCapUsd.String() || fundingCap != want.FundingCapPpmPerHour.String() {
					t.Fatalf("market %d config = (%s, %s, %d, %s, %s, %s), want %+v", r.id, tick, lot, tierClass, impact, oiCap, fundingCap, want)
				}
				if listedAt.Valid != (want.ListedAt != 0) || (listedAt.Valid && listedAt.Int64 != int64(want.ListedAt)) {
					t.Fatalf("market %d listed_at = %v, want epoch %d (null for zero)", r.id, listedAt, want.ListedAt)
				}
				if r.id == insertedID {
					foundInserted = true
					if !r.szDecimals.Valid || !r.pxDecimals.Valid {
						t.Fatalf("inserted market %d has null decimals", r.id)
					}
				}
			}
			if !foundInserted {
				t.Fatalf("Refresh did not insert market %d", insertedID)
			}
		})
	}
}

func TestMarketsCheck(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	client := testClient(t)
	reseed(t, db)
	requireSeeded(t, db)
	check := Check(db, 2*time.Minute)
	if err := check(ctx); err == nil {
		t.Fatal("Check accepted seed rows before Refresh")
	}

	mux := http.NewServeMux()
	httpx.RegisterHealth(mux, httpx.HealthDeps{Postgres: db.PingContext, Custom: map[string]httpx.Check{"markets": check}})
	ready := func(want int, status string) {
		t.Helper()
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz/ready", nil))
		if rec.Code != want {
			t.Fatalf("readiness status %d, want %d: %s", rec.Code, want, rec.Body)
		}
		var body struct {
			Checks map[string]string `json:"checks"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("readiness response: %v", err)
		}
		if body.Checks["markets"] != status {
			t.Fatalf("markets readiness %q, want %q", body.Checks["markets"], status)
		}
	}
	ready(http.StatusServiceUnavailable, "failed")

	if err := Refresh(ctx, db, client); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	var newest int64
	if err := db.QueryRowContext(ctx, `SELECT MAX(refreshed_block) FROM perps.markets WHERE source = 'chain'`).Scan(&newest); err != nil {
		t.Fatalf("read refreshed height: %v", err)
	}
	block, err := client.BlockByNumber(ctx, big.NewInt(newest))
	if err != nil {
		t.Fatalf("read refresh block: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO perps.blocks (number, hash, parent_hash, time, proposer, tx_count)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (number) DO UPDATE SET hash = EXCLUDED.hash, parent_hash = EXCLUDED.parent_hash, time = EXCLUDED.time, proposer = EXCLUDED.proposer, tx_count = EXCLUDED.tx_count`,
		newest, block.Hash().Bytes(), block.ParentHash().Bytes(), time.Unix(int64(block.Time()), 0), strings.ToUpper(block.Coinbase().Hex()[2:]), len(block.Transactions())); err != nil {
		t.Fatalf("index real refresh block: %v", err)
	}
	if err := check(ctx); err != nil {
		t.Fatalf("Check after Refresh: %v", err)
	}
	ready(http.StatusOK, "ok")
	if err := Check(db, time.Nanosecond)(ctx); err == nil {
		t.Fatal("Check accepted a refresh older than maxAge")
	}
	if err := Check(db, 0)(ctx); err == nil {
		t.Fatal("Check accepted a nonpositive maxAge")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := check(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("Check with cancelled context = %v, want context.Canceled", err)
	}

	if _, err := db.ExecContext(ctx, `UPDATE perps.markets SET source = 'seed'`); err != nil {
		t.Fatalf("restore seed sources: %v", err)
	}
	if err := check(ctx); err == nil {
		t.Fatal("Check accepted seed rows carrying a recent refreshed_block")
	}
	ready(http.StatusServiceUnavailable, "failed")
	reseed(t, db)
}

func TestMarkets(t *testing.T) {
	t.Run("seed_eighteen", func(t *testing.T) {
		db := testDB(t)
		reseed(t, db)
		want := []string{"BTC", "ETH", "SOL", "AVAX", "LINK", "TSLA", "NVDA", "NAS100", "XAU", "SPX500", "GOOGL", "PAX", "SID", "HYPE", "XRP", "ASTER", "TRUMP", "BNB"}
		got := readRows(t, db)
		if len(got) != 18 {
			t.Fatalf("perps.markets holds %d rows, want 18", len(got))
		}
		symbols := make([]string, len(got))
		for i, r := range got {
			if r.id != uint64(i) {
				t.Fatalf("row %d has market_id %d", i, r.id)
			}
			if r.maxLeverage != "1000" || r.source != "seed" || r.refreshedBlock.Valid {
				t.Fatalf("market %d max_leverage %s source %s refreshed_block %v, want 1000 seed null", i, r.maxLeverage, r.source, r.refreshedBlock)
			}
			symbols[i] = r.symbol
		}
		if !slices.Equal(symbols, want) {
			t.Fatalf("symbols %v, want %v", symbols, want)
		}
		requireSeeded(t, db)
		if got[0].name != "Bitcoin" || got[0].szDecimals.Int16 != 5 || got[0].pxDecimals.Int16 != 1 || got[0].maxOpenInterest != "500000000000000000000000000" {
			t.Fatalf("BTC row %+v", got[0])
		}
		if got[11].name != "Paxeer Network" || got[11].oracleSymbol != "PAX" || got[11].szDecimals.Int16 != 0 || got[11].pxDecimals.Int16 != 5 || got[11].maxOpenInterest != "2000000000000000000000000" {
			t.Fatalf("PAX row %+v", got[11])
		}
	})

	t.Run("refresh_from_chain", func(t *testing.T) {
		db := testDB(t)
		client := testClient(t)
		reseed(t, db)
		before := height(t, client)
		if err := Refresh(context.Background(), db, client); err != nil {
			t.Fatalf("Refresh: %v", err)
		}
		after := height(t, client)
		c, _, err := latest(context.Background(), client)
		if err != nil {
			t.Fatalf("latest: %v", err)
		}
		out, err := c.call(context.Background(), "totalMarkets")
		if err != nil {
			t.Fatalf("totalMarkets: %v", err)
		}
		count, err := field[*big.Int](out, 0, "totalMarkets")
		if err != nil {
			t.Fatal(err)
		}
		if count.Cmp(big.NewInt(18)) != 0 {
			t.Fatalf("totalMarkets %s, want 18", count)
		}
		got := readRows(t, db)
		if len(got) != 18 {
			t.Fatalf("perps.markets holds %d rows, want 18", len(got))
		}
		block := got[0].refreshedBlock
		if !block.Valid || block.Int64 < before || block.Int64 > after {
			t.Fatalf("refreshed_block %v, want the latest height in [%d, %d]", block, before, after)
		}
		c.at = big.NewInt(block.Int64)
		for i, r := range got {
			m, err := c.market(context.Background(), uint64(i))
			if err != nil {
				t.Fatalf("getMarket(%d): %v", i, err)
			}
			seed := seedRow(Seed[i])
			want := seed
			want.name, want.maxLeverage, want.maintenanceMarginBps, want.maxOpenInterest, want.enabled = m.name, m.maxLeverage, m.maintenanceMarginBps, m.maxOpenInterest, m.enabled
			want.source, want.refreshedBlock = "chain", block
			if r != want {
				t.Fatalf("market %d = %+v, want %+v", i, r, want)
			}
			if r.szDecimals != seed.szDecimals || r.pxDecimals != seed.pxDecimals {
				t.Fatalf("market %d decimals changed to %v/%v", i, r.szDecimals, r.pxDecimals)
			}
		}
		if btc := got[0]; btc.name != "Bitcoin" || btc.symbol != "BTC" || btc.maxLeverage != "1000" || btc.maintenanceMarginBps != "5" || btc.maxOpenInterest != "500000000000000000000000000" || !btc.enabled {
			t.Fatalf("BTC after refresh %+v, want (Bitcoin, BTC, 1000, 5, 500000000e18, true)", btc)
		}
	})

	t.Run("fee_config", func(t *testing.T) {
		db := testDB(t)
		client := testClient(t)
		reseed(t, db)
		readFees := func() (vals [4]string, block int64) {
			t.Helper()
			err := db.QueryRow(`SELECT taker_fee_ppm::text, maker_fee_ppm::text, liquidation_fee_ppm::text, insurance_share_ppm::text, refreshed_block FROM perps.fee_config WHERE id = 1`).
				Scan(&vals[0], &vals[1], &vals[2], &vals[3], &block)
			if err != nil {
				t.Fatalf("read perps.fee_config: %v", err)
			}
			return vals, block
		}
		want := [4]string{"450", "150", "0", "0"}

		before := height(t, client)
		if err := FeeConfig(context.Background(), db, client); err != nil {
			t.Fatalf("FeeConfig: %v", err)
		}
		after := height(t, client)
		vals, block := readFees()
		if vals != want || block < before || block > after {
			t.Fatalf("fee_config %v at %d, want %v in [%d, %d]", vals, block, want, before, after)
		}

		if _, err := db.Exec(`UPDATE perps.fee_config SET taker_fee_ppm = 1, maker_fee_ppm = 1, liquidation_fee_ppm = 1, insurance_share_ppm = 1, refreshed_block = 0 WHERE id = 1`); err != nil {
			t.Fatalf("overwrite fee_config: %v", err)
		}
		before = height(t, client)
		if err := Refresh(context.Background(), db, client); err != nil {
			t.Fatalf("Refresh: %v", err)
		}
		after = height(t, client)
		vals, block = readFees()
		if vals != want || block < before || block > after {
			t.Fatalf("fee_config after Refresh %v at %d, want %v in [%d, %d]", vals, block, want, before, after)
		}
		var rows int
		if err := db.QueryRow(`SELECT count(*) FROM perps.fee_config`).Scan(&rows); err != nil {
			t.Fatalf("count fee_config: %v", err)
		}
		if rows != 1 {
			t.Fatalf("perps.fee_config holds %d rows, want 1", rows)
		}
	})

	t.Run("refresh_retry", func(t *testing.T) {
		db := testDB(t)
		reseed(t, db)
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen: %v", err)
		}
		addr := ln.Addr().String()
		if err := ln.Close(); err != nil {
			t.Fatalf("close listener: %v", err)
		}
		client, err := ethclient.Dial("http://" + addr)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		t.Cleanup(client.Close)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		metrics, err := telemetry.New(config.Indexer)
		if err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- Run(ctx, db, client, 60*time.Second, metrics) }()
		deadline := time.NewTimer(30 * time.Second)
		defer deadline.Stop()
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
	awaitRetry:
		for {
			select {
			case err := <-done:
				t.Fatalf("Run returned before retrying: %v", err)
			case <-deadline.C:
				t.Fatal("Run reported no failure")
			case <-ticker.C:
				var body bytes.Buffer
				if err := metrics.WritePrometheus(&body); err != nil {
					t.Fatalf("metrics: %v", err)
				}
				if strings.Contains(body.String(), perps_markets_refresh_failures_total+" 1\n") {
					break awaitRetry
				}
			}
		}
		if RetryInterval != 60*time.Second {
			t.Fatalf("retry interval %s, want 60s", RetryInterval)
		}
		requireSeeded(t, db)

		mux := http.NewServeMux()
		httpx.RegisterHealth(mux, httpx.HealthDeps{Postgres: db.PingContext})
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz/ready", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("ready %d while the refresh is failing: %s", rec.Code, rec.Body)
		}
		var body struct {
			Checks map[string]string `json:"checks"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("ready body: %v", err)
		}
		names := make([]string, 0, len(body.Checks))
		for name := range body.Checks {
			names = append(names, name)
		}
		slices.Sort(names)
		if !slices.Equal(names, []string{"memory_alloc_mb", "postgres"}) {
			t.Fatalf("readiness checks %v name more than postgres", names)
		}

		select {
		case err := <-done:
			t.Fatalf("Run returned %v before the 60s wait elapsed", err)
		default:
		}
		cancel()
		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("Run after cancel %v, want context.Canceled", err)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("Run did not stop on cancel")
		}
		requireSeeded(t, db)
	})
}

func TestRunRefreshesAgain(t *testing.T) {
	db := testDB(t)
	client := testClient(t)
	metric := func(t *testing.T, metrics *telemetry.Metrics, name string) float64 {
		t.Helper()
		var body bytes.Buffer
		if err := metrics.WritePrometheus(&body); err != nil {
			t.Fatalf("metrics: %v", err)
		}
		for _, line := range strings.Split(body.String(), "\n") {
			fields := strings.Fields(line)
			if len(fields) == 2 && fields[0] == name {
				value, err := strconv.ParseFloat(fields[1], 64)
				if err != nil {
					t.Fatalf("metric %s: %v", name, err)
				}
				return value
			}
		}
		t.Fatalf("metric %s missing from %s", name, body.String())
		return 0
	}

	t.Run("refreshes_again", func(t *testing.T) {
		reseed(t, db)
		metrics, err := telemetry.New(config.Indexer)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		t.Cleanup(func() {
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Errorf("Run returned %v, want context.Canceled", err)
				}
			case <-time.After(time.Second):
				t.Error("Run did not stop on cancellation")
			}
		})
		go func() { done <- Run(ctx, db, client, 50*time.Millisecond, metrics) }()
		waitRefresh := func() sql.NullInt64 {
			t.Helper()
			pollCtx, stopPoll := context.WithTimeout(context.Background(), 30*time.Second)
			defer stopPoll()
			ticker := time.NewTicker(time.Millisecond)
			defer ticker.Stop()
			for {
				var block sql.NullInt64
				if err := db.QueryRowContext(pollCtx, `SELECT refreshed_block FROM perps.markets WHERE market_id = 0`).Scan(&block); err != nil {
					t.Fatalf("read refreshed_block within 30s: %v", err)
				}
				if block.Valid {
					return block
				}
				select {
				case err := <-done:
					done <- err
					t.Fatalf("Run stopped before refreshing: %v", err)
				case <-pollCtx.Done():
					t.Fatal("refreshed_block remained NULL for 30s")
				case <-ticker.C:
				}
			}
		}
		waitRefresh()
		reseed(t, db)
		last := waitRefresh()
		cancel()
		select {
		case err := <-done:
			done <- err
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("Run returned %v, want context.Canceled", err)
			}
		case <-time.After(time.Second):
			t.Fatal("Run did not stop on cancellation")
		}
		rows := readRows(t, db)
		if len(rows) == 0 {
			t.Fatal("Run stored no markets")
		}
		for _, row := range rows {
			if !row.refreshedBlock.Valid || row.refreshedBlock.Int64 < last.Int64 || row.refreshedBlock != rows[0].refreshedBlock || row.source != "chain" {
				t.Fatalf("inconsistent market after Run: %+v, last observed %v", row, last)
			}
		}
		if got := metric(t, metrics, perps_markets_refresh_height); got != float64(rows[0].refreshedBlock.Int64) {
			t.Fatalf("refresh height %v, want last refreshed_block %d", got, rows[0].refreshedBlock.Int64)
		}
		if got := metric(t, metrics, perps_markets_refresh_failures_total); got != 0 {
			t.Fatalf("failed refreshes %v, want 0 after successful refreshes", got)
		}
	})

	t.Run("failed_refresh", func(t *testing.T) {
		reseed(t, db)
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen: %v", err)
		}
		addr := listener.Addr().String()
		if err := listener.Close(); err != nil {
			t.Fatalf("close listener: %v", err)
		}
		unavailableClient, err := ethclient.Dial("http://" + addr)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		t.Cleanup(unavailableClient.Close)
		metrics, err := telemetry.New(config.Indexer)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- Run(ctx, db, unavailableClient, 50*time.Millisecond, metrics) }()
		t.Cleanup(func() {
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Errorf("Run returned %v, want context.Canceled", err)
				}
			case <-time.After(time.Second):
				t.Error("Run did not cancel its failure retry wait")
			}
		})
		deadline := time.NewTimer(time.Second)
		defer deadline.Stop()
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
	awaitFailure:
		for {
			select {
			case err := <-done:
				done <- err
				t.Fatalf("Run stopped after failure: %v", err)
			case <-deadline.C:
				t.Fatal("Run reported no refresh failure")
			case <-ticker.C:
				var body bytes.Buffer
				if err := metrics.WritePrometheus(&body); err != nil {
					t.Fatal(err)
				}
				if strings.Contains(body.String(), perps_markets_refresh_failures_total+" 1\n") {
					break awaitFailure
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
		if got := metric(t, metrics, perps_markets_refresh_failures_total); got != 1 {
			t.Fatalf("failed refreshes %v, want 1 during RetryInterval", got)
		}
		if got := metric(t, metrics, perps_markets_refresh_height); got != 0 {
			t.Fatalf("height after failed refresh %v, want 0", got)
		}
		requireSeeded(t, db)
	})
}
