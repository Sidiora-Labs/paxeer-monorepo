package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
)

func TestExactTransactionRejectsUnavailableDurableRelations(t *testing.T) {
	ctx := context.Background()
	for _, relation := range []string{"chart_traders", "chart_exact", "chart_outbox"} {
		t.Run(relation, func(t *testing.T) {
			pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
			ddl, err := store.ExactSchemaDDL("candles")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, ddl); err != nil {
				t.Fatal(err)
			}
			exact, err := store.NewExact(pool, "candles")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `ALTER TABLE candles.`+relation+` RENAME TO `+relation+`_held`); err != nil {
				t.Fatal(err)
			}
			fill := fillFixture()
			if err := ProcessExactFill(ctx, exact, fill, 1); err == nil || !strings.Contains(err.Error(), relation) {
				t.Fatalf("exact transaction without %s: %v", relation, err)
			}
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			failure := ProcessExactFillTx(ctx, tx, exact, fill, 1)
			if err := tx.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			if failure == nil || !strings.Contains(failure.Error(), relation) {
				t.Fatalf("exact outer transaction without %s: %v", relation, failure)
			}
			var claims int
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM candles.chart_inbox`).Scan(&claims); err != nil || claims != 0 {
				t.Fatalf("uncommitted exact identity: %d, %v", claims, err)
			}
		})
	}
	t.Run("invalid prior candle", func(t *testing.T) {
		pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
		ddl, err := store.ExactSchemaDDL("candles")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, ddl); err != nil {
			t.Fatal(err)
		}
		exact, err := store.NewExact(pool, "candles")
		if err != nil {
			t.Fatal(err)
		}
		fill := fillFixture()
		if _, err := pool.Exec(ctx, `INSERT INTO candles.chart_exact(chain_id,pool,interval,start,snapshot) VALUES($1,$2,'1m',3600,'"invalid"'::jsonb)`, fill.ChainID, fill.PoolAddress); err != nil {
			t.Fatal(err)
		}
		if err := ProcessExactFill(ctx, exact, fill, 1); err == nil || !strings.Contains(err.Error(), "unmarshal") {
			t.Fatalf("invalid persisted exact candle: %v", err)
		}
		var claims int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM candles.chart_inbox`).Scan(&claims); err != nil || claims != 0 {
			t.Fatalf("invalid candle changed durable fill identity: %d, %v", claims, err)
		}
	})
}
