package graphql

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestMarketMarginTiers(t *testing.T) {
	databaseURL := os.Getenv("PERPS_DATA_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("PERPS_DATA_TEST_DATABASE_URL is unset")
	}
	db := openPerpsDB(t, databaseURL)
	if _, err := db.Exec(`DELETE FROM perps.market_tiers;
DELETE FROM perps.markets;
INSERT INTO perps.markets (market_id, name, symbol, oracle_symbol, max_leverage,
    maintenance_margin_bps, max_open_interest, enabled, source) VALUES
    (7, 'Bitcoin', 'BTC', 'BTC', 100, 5, 1000, true, 'chain'),
    (13, 'Ether', 'ETH', 'ETH', 50, 10, 2000, true, 'chain'),
    (29, 'No tiers', 'NONE', 'NONE', 30, 5, 1000, false, 'seed');
INSERT INTO perps.market_tiers
    (market_id, tier, lower_bound_usd, max_leverage, maintenance_bps, deduction_usd, refreshed_block) VALUES
    (7, 10, 115792089237316195423570985008687907853269984665640564039457584007913129639935,
        1000000000000000000, 500, 9007199254740993, 42),
    (13, 0, 0, 50000000000000000000, 100, 0, 42),
    (7, 2, 9007199254740993, 20000000000000000000, 100, 1000000000000000000, 42),
    (7, 0, 0, 100000000000000000000, 50, 0, 42)`); err != nil {
		t.Fatal(err)
	}
	schema, err := NewSchema(db)
	if err != nil {
		t.Fatal(err)
	}
	assertSchemaField(t, schema, "Market", "marginTiers", "[MarginTier!]!", nil)
	assertSchemaFields(t, schema, "MarginTier", map[string]string{
		"tier": "Int!", "lowerBoundUsd": "BigInt!", "maxLeverage": "BigInt!",
		"maintenanceBps": "BigInt!", "deductionUsd": "BigInt!",
	})
	check(t, schema, `{ markets { marketId marginTiers { tier maxLeverage maintenanceBps } } }`,
		`{"markets":[
{"marketId":7,"marginTiers":[{"tier":0,"maxLeverage":"100000000000000000000","maintenanceBps":"50"},{"tier":2,"maxLeverage":"20000000000000000000","maintenanceBps":"100"},{"tier":10,"maxLeverage":"1000000000000000000","maintenanceBps":"500"}]},
{"marketId":13,"marginTiers":[{"tier":0,"maxLeverage":"50000000000000000000","maintenanceBps":"100"}]},
{"marketId":29,"marginTiers":[]}]}`)
	const btcTiers = `[{"tier":0,"lowerBoundUsd":"0","maxLeverage":"100000000000000000000","maintenanceBps":"50","deductionUsd":"0"},
{"tier":2,"lowerBoundUsd":"9007199254740993","maxLeverage":"20000000000000000000","maintenanceBps":"100","deductionUsd":"1000000000000000000"},
{"tier":10,"lowerBoundUsd":"115792089237316195423570985008687907853269984665640564039457584007913129639935","maxLeverage":"1000000000000000000","maintenanceBps":"500","deductionUsd":"9007199254740993"}]`
	check(t, schema, `{ market(marketId: 7) { marginTiers { tier lowerBoundUsd maxLeverage maintenanceBps deductionUsd } } }`,
		`{"market":{"marginTiers":`+btcTiers+`}}`)
	check(t, schema, `{ market(marketId: 7) { first: marginTiers { tier lowerBoundUsd maxLeverage maintenanceBps deductionUsd } second: marginTiers { tier lowerBoundUsd maxLeverage maintenanceBps deductionUsd } } }`,
		`{"market":{"first":`+btcTiers+`,"second":`+btcTiers+`}}`)

	ctx := context.Background()
	resolver := &Resolver{DB: db}
	markets, err := resolver.Markets(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(markets) != 3 {
		t.Fatalf("markets = %d, want 3", len(markets))
	}
	if _, err := markets[0].MarginTiers(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE perps.market_tiers SET maintenance_bps = 125 WHERE market_id = 13`); err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	cachedCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	for _, market := range markets {
		tiers, err := market.MarginTiers(cachedCtx)
		if err != nil {
			t.Fatalf("cached tiers for market %d attempted another read: %v", market.MarketID, err)
		}
		switch market.MarketID {
		case 7:
			if len(tiers) != 3 || tiers[0].Tier != 0 || tiers[1].Tier != 2 || tiers[2].Tier != 10 {
				t.Fatalf("BTC tiers = %+v, want tiers 0, 2, 10", tiers)
			}
		case 13:
			want := []*MarginTier{{Tier: 0, LowerBoundUsd: "0", MaxLeverage: "50000000000000000000", MaintenanceBps: "100", DeductionUsd: "0"}}
			if !reflect.DeepEqual(tiers, want) {
				t.Fatalf("ETH tiers = %+v, want the batch loaded before the update", tiers)
			}
		case 29:
			if tiers == nil || len(tiers) != 0 {
				t.Fatalf("missing tiers = %v, want a non-null empty list", tiers)
			}
		}
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	check(t, schema, `{ markets { marketId marginTiers { tier maintenanceBps } } }`,
		`{"markets":[{"marketId":7,"marginTiers":[{"tier":0,"maintenanceBps":"50"},{"tier":2,"maintenanceBps":"100"},{"tier":10,"maintenanceBps":"500"}]},{"marketId":13,"marginTiers":[{"tier":0,"maintenanceBps":"125"}]},{"marketId":29,"marginTiers":[]}]}`)
}

func TestMarketConfigFields(t *testing.T) {
	databaseURL := os.Getenv("PERPS_DATA_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("PERPS_DATA_TEST_DATABASE_URL is unset")
	}
	db := openPerpsDB(t, databaseURL)
	if _, err := db.Exec(`DELETE FROM perps.markets;
INSERT INTO perps.markets (market_id, name, symbol, oracle_symbol, sz_decimals, px_decimals,
    tick_size, lot_size, tier_class, oi_cap_usd, max_leverage, maintenance_margin_bps,
    max_open_interest, enabled, source, refreshed_block, listed_at) VALUES
    (29, 'Unknown config', 'UNKNOWN', 'ORACLE_UNKNOWN', NULL, NULL, NULL, NULL, NULL, NULL,
        30, 5, 1000, false, 'seed', NULL, NULL),
    (7, 'Configured market', 'CONFIG', 'ORACLE_CONFIG', 5, 2, 9007199254740993, 1000000000000000000, 3,
        115792089237316195423570985008687907853269984665640564039457584007913129639935,
        100, 5, 1000, true, 'chain', 42, '2025-01-02 03:04:05.123456+02'),
    (13, 'Zero config', 'ZERO', 'ORACLE_ZERO', 0, 0, 0, 0, 0, 0,
        50, 5, 1000, true, 'chain', 43, '2025-01-03 00:00:00+00')`); err != nil {
		t.Fatal(err)
	}
	schema, err := NewSchema(db)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("schema", func(t *testing.T) {
		assertSchemaFields(t, schema, "Market", map[string]string{
			"oracleSymbol": "String!", "szDecimals": "Int", "pxDecimals": "Int",
			"tickSize": "BigInt", "lotSize": "BigInt", "tierClass": "Int",
			"oiCapUsd": "BigInt", "createdAt": "DateTime",
		})
	})
	t.Run("markets", func(t *testing.T) {
		check(t, schema, `{ markets { marketId szDecimals pxDecimals tickSize lotSize tierClass createdAt } }`,
			`{"markets":[
{"marketId":7,"szDecimals":5,"pxDecimals":2,"tickSize":"9007199254740993","lotSize":"1000000000000000000","tierClass":3,"createdAt":"2025-01-02T01:04:05.123456Z"},
{"marketId":13,"szDecimals":0,"pxDecimals":0,"tickSize":"0","lotSize":"0","tierClass":0,"createdAt":"2025-01-03T00:00:00Z"},
{"marketId":29,"szDecimals":null,"pxDecimals":null,"tickSize":null,"lotSize":null,"tierClass":null,"createdAt":null}]}`)
	})
	const fields = `marketId name symbol oracleSymbol szDecimals pxDecimals tickSize lotSize tierClass oiCapUsd maxLeverage enabled createdAt`
	const configured = `{"marketId":7,"name":"Configured market","symbol":"CONFIG","oracleSymbol":"ORACLE_CONFIG","szDecimals":5,"pxDecimals":2,"tickSize":"9007199254740993","lotSize":"1000000000000000000","tierClass":3,"oiCapUsd":"115792089237316195423570985008687907853269984665640564039457584007913129639935","maxLeverage":"100","enabled":true,"createdAt":"2025-01-02T01:04:05.123456Z"}`
	const zero = `{"marketId":13,"name":"Zero config","symbol":"ZERO","oracleSymbol":"ORACLE_ZERO","szDecimals":0,"pxDecimals":0,"tickSize":"0","lotSize":"0","tierClass":0,"oiCapUsd":"0","maxLeverage":"50","enabled":true,"createdAt":"2025-01-03T00:00:00Z"}`
	const unknown = `{"marketId":29,"name":"Unknown config","symbol":"UNKNOWN","oracleSymbol":"ORACLE_UNKNOWN","szDecimals":null,"pxDecimals":null,"tickSize":null,"lotSize":null,"tierClass":null,"oiCapUsd":null,"maxLeverage":"30","enabled":false,"createdAt":null}`
	t.Run("all_fields", func(t *testing.T) {
		check(t, schema, `{ markets { `+fields+` } }`, `{"markets":[`+configured+`,`+zero+`,`+unknown+`]}`)
	})
	t.Run("market", func(t *testing.T) {
		check(t, schema, `{ market(marketId: 7) { `+fields+` } }`, `{"market":`+configured+`}`)
		check(t, schema, `{ market(marketId: 29) { `+fields+` } }`, `{"market":`+unknown+`}`)
		check(t, schema, `{ market(marketId: 999) { marketId } }`, `{"market":null}`)
	})
	t.Run("subscription_market", func(t *testing.T) {
		position := &marketSubscriptionPosition{subPosition: subPosition{MarketID: 7, db: db}}
		market, err := position.Market(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if market == nil || market.OracleSymbol != "ORACLE_CONFIG" || market.SzDecimals == nil || *market.SzDecimals != 5 ||
			market.TickSize == nil || *market.TickSize != "9007199254740993" || market.CreatedAt == nil ||
			market.CreatedAt.UTC().Format(time.RFC3339Nano) != "2025-01-02T01:04:05.123456Z" {
			t.Fatalf("subscription market config = %+v", market)
		}
	})
}
