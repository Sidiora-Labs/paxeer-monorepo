package store

import (
	"errors"
	"slices"
	"testing"

	"github.com/lib/pq"
)

func TestMarketTiersTable(t *testing.T) {
	db := testDB(t)
	migrate(t, db)

	if got := primaryKey(t, db, "market_tiers"); got != "market_id,tier" {
		t.Fatalf("market_tiers primary key = %q, want market_id,tier", got)
	}
	wantColumns := []string{"market_id", "tier", "lower_bound_usd", "max_leverage", "maintenance_bps", "deduction_usd", "refreshed_block"}
	if got := columnNames(columns(t, db, "market_tiers")); !slices.Equal(got, wantColumns) {
		t.Fatalf("market_tiers columns = %v, want %v", got, wantColumns)
	}

	const insert = `INSERT INTO perps.market_tiers
    (market_id, tier, lower_bound_usd, max_leverage, maintenance_bps, deduction_usd, refreshed_block)
VALUES ($1, 0, 0, 1000, 5, 0, 0)`
	if _, err := db.Exec(insert, 0); err != nil {
		t.Fatalf("insert tier for seeded market 0: %v", err)
	}
	var lowerBound, maxLeverage, maintenanceBps, deduction string
	if err := db.QueryRow(`SELECT lower_bound_usd::text, max_leverage::text, maintenance_bps::text, deduction_usd::text
FROM perps.market_tiers WHERE market_id = 0 AND tier = 0`).Scan(&lowerBound, &maxLeverage, &maintenanceBps, &deduction); err != nil {
		t.Fatalf("read tier for seeded market 0: %v", err)
	}
	if got, want := []string{lowerBound, maxLeverage, maintenanceBps, deduction}, []string{"0", "1000", "5", "0"}; !slices.Equal(got, want) {
		t.Fatalf("stored margin tier = %v, want %v", got, want)
	}

	_, err := db.Exec(insert, 4096)
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) || pqErr.Code != "23503" {
		t.Fatalf("insert tier for unknown market 4096 = %v, want SQLSTATE 23503", err)
	}
}
