package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
)

func TestExactFillRollsBackWhenDurableWriteIsRefused(t *testing.T) {
	ctx := context.Background()
	for _, relation := range []string{"chart_traders", "chart_exact", "chart_outbox"} {
		for _, mode := range []string{"legacy outer", "legacy transaction", "qualified transaction"} {
			t.Run(mode+" "+relation, func(t *testing.T) {
				pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
				ddl, err := store.ExactSchemaDDL("candles")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := pool.Exec(ctx, ddl); err != nil {
					t.Fatal(err)
				}
				if _, err := pool.Exec(ctx, `CREATE FUNCTION candles.refuse_exact_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'durable exact write refused'; END $$`); err != nil {
					t.Fatal(err)
				}
				if _, err := pool.Exec(ctx, `CREATE TRIGGER refuse_exact_write BEFORE INSERT ON candles.`+relation+` FOR EACH ROW EXECUTE FUNCTION candles.refuse_exact_write()`); err != nil {
					t.Fatal(err)
				}
				exact, err := store.NewExact(pool, "candles")
				if err != nil {
					t.Fatal(err)
				}
				var failure error
				if mode == "legacy outer" {
					failure = ProcessExactFill(ctx, exact, fillFixture(), 1)
				} else {
					tx, err := pool.Begin(ctx)
					if err != nil {
						t.Fatal(err)
					}
					if mode == "legacy transaction" {
						failure = ProcessExactFillTx(ctx, tx, exact, fillFixture(), 1)
					} else {
						failure = ApplyQualifiedTx(ctx, tx, exact, seriesSource(0, 1704067260, "0x"+strings.Repeat("a", 40)), 1)
					}
					if err := tx.Rollback(ctx); err != nil {
						t.Fatal(err)
					}
				}
				if failure == nil || !strings.Contains(failure.Error(), "durable exact write refused") {
					t.Fatalf("%s on %s: %v", mode, relation, failure)
				}
				var claims int
				if err := pool.QueryRow(ctx, `SELECT count(*) FROM candles.chart_inbox`).Scan(&claims); err != nil || claims != 0 {
					t.Fatalf("refused exact write retained claim: %d, %v", claims, err)
				}
			})
		}
	}
}
