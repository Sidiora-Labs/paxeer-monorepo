package graphql

import (
	"context"
	"os"
	"testing"
	"time"
)

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
