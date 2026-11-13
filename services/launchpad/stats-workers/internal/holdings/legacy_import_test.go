package holdings

import (
	"context"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
)

func TestLegacyHolderRowsRetainTheirAuthorityClassification(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.NewPostgres(t)
	ddl, err := SchemaSQL("holdings_legacy")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	repository, err := NewRepository(pool, "holdings_legacy")
	if err != nil {
		t.Fatal(err)
	}
	market := addr(5)
	valid := addr(6)
	badAmount := addr(7)
	for _, row := range []struct{ address, balance string }{
		{valid, "100"}, {badAmount, "-5"}, {"invalid", "12"},
	} {
		if _, err := pool.Exec(ctx, "INSERT INTO stats.pool_holders(pool_address,holder_address,balance,pct_of_supply,last_updated) VALUES($1,$2,$3,'0',1)", market, row.address, row.balance); err != nil {
			t.Fatal(err)
		}
	}
	importRows := func() {
		t.Helper()
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		if err := repository.ImportLegacyRowsTx(ctx, tx, market); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
	importRows()
	want := map[string]struct{ reason, normalized, numeric string }{
		valid:     {"legacy_swap_projection_not_authoritative", valid, "100"},
		badAmount: {"malformed_or_negative_balance", badAmount, ""},
		"invalid": {"malformed_address", "", ""},
	}
	rows, err := pool.Query(ctx, "SELECT address,reason,COALESCE(normalized_address,''),COALESCE(numeric_balance::text,'') FROM holdings_legacy.legacy_rows WHERE pool=$1", market)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var address, reason, normalized, numeric string
		if err := rows.Scan(&address, &reason, &normalized, &numeric); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		expected, ok := want[address]
		if !ok || reason != expected.reason || normalized != expected.normalized || numeric != expected.numeric {
			rows.Close()
			t.Fatalf("legacy row %s: reason=%s normalized=%s numeric=%s", address, reason, normalized, numeric)
		}
		delete(want, address)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	if len(want) != 0 {
		t.Fatalf("missing legacy rows: %v", want)
	}
	if _, err := pool.Exec(ctx, "UPDATE stats.pool_holders SET balance='25' WHERE pool_address=$1 AND holder_address=$2", market, valid); err != nil {
		t.Fatal(err)
	}
	importRows()
	var updated string
	if err := pool.QueryRow(ctx, "SELECT numeric_balance::text FROM holdings_legacy.legacy_rows WHERE pool=$1 AND address=$2", market, valid).Scan(&updated); err != nil || updated != "25" {
		t.Fatalf("updated legacy balance=%s err=%v", updated, err)
	}
}
