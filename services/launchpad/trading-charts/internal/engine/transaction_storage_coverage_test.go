package engine

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
)

func TestExactTransactionsRejectUnavailableDurableComponents(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name      string
		qualified bool
		prepare   func(*testing.T, *store.ExactStore)
	}{
		{"legacy snapshot table", false, func(t *testing.T, st *store.ExactStore) {
			if _, err := st.Pool.Exec(ctx, `ALTER TABLE candles.chart_exact RENAME TO chart_exact_unavailable`); err != nil {
				t.Fatal(err)
			}
		}},
		{"legacy stored snapshot", false, func(t *testing.T, st *store.ExactStore) {
			fill := fillFixture()
			start, err := BucketStart(fill.BlockTimestamp, "1m")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = st.Pool.Exec(ctx, `INSERT INTO candles.chart_exact(chain_id,pool,interval,start,snapshot) VALUES($1,$2,'1m',$3,'"invalid"')`, fill.ChainID, fill.PoolAddress, start); err != nil {
				t.Fatal(err)
			}
		}},
		{"legacy trader table", false, func(t *testing.T, st *store.ExactStore) {
			if _, err := st.Pool.Exec(ctx, `ALTER TABLE candles.chart_traders RENAME TO chart_traders_unavailable`); err != nil {
				t.Fatal(err)
			}
		}},
		{"legacy outbox table", false, func(t *testing.T, st *store.ExactStore) {
			if _, err := st.Pool.Exec(ctx, `ALTER TABLE candles.chart_outbox RENAME TO chart_outbox_unavailable`); err != nil {
				t.Fatal(err)
			}
		}},
		{"qualified snapshot table", true, func(t *testing.T, st *store.ExactStore) {
			if _, err := st.Pool.Exec(ctx, `ALTER TABLE candles.chart_exact RENAME TO chart_exact_unavailable`); err != nil {
				t.Fatal(err)
			}
		}},
		{"qualified stored snapshot", true, func(t *testing.T, st *store.ExactStore) {
			fill := seriesSource(0, 1704067260, "")
			start, err := BucketStart(1704067260, "1m")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = st.Pool.Exec(ctx, `INSERT INTO candles.chart_exact(chain_id,pool,interval,start,snapshot) VALUES('125',$1,'1m',$2,'"invalid"')`, fill.PoolAddress, start); err != nil {
				t.Fatal(err)
			}
		}},
		{"qualified outbox table", true, func(t *testing.T, st *store.ExactStore) {
			if _, err := st.Pool.Exec(ctx, `ALTER TABLE candles.chart_outbox RENAME TO chart_outbox_unavailable`); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
			ddl, err := store.ExactSchemaDDL("candles")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = pool.Exec(ctx, ddl); err != nil {
				t.Fatal(err)
			}
			exact, err := store.NewExact(pool, "candles")
			if err != nil {
				t.Fatal(err)
			}
			tc.prepare(t, exact)
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = tx.Rollback(ctx) })
			if tc.qualified {
				err = ApplyQualifiedTx(ctx, tx, exact, seriesSource(0, 1704067260, ""), 1)
			} else {
				err = ProcessExactFillTx(ctx, tx, exact, fillFixture(), 1)
			}
			if err == nil {
				t.Fatal("unavailable durable component accepted")
			}
			if err = tx.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			var count int
			if err = pool.QueryRow(ctx, `SELECT count(*) FROM candles.chart_inbox`).Scan(&count); err != nil || count != 0 {
				t.Fatalf("refused transaction retained a claim: count=%d err=%v", count, err)
			}
		})
	}
}

func TestExactFillEntryAndOuterTransactionRejectInvalidAuthority(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name string
		fill Fill
		want error
	}{
		{"unpriced fill", func() Fill { f := fillFixture(); f.TokenAmountRaw = "invalid"; return f }(), ErrInvalidFill},
		{"missing emitter", func() Fill { f := fillFixture(); f.EmittingContract = ""; return f }(), ErrInvalidFill},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ProcessExactFill(ctx, nil, tc.fill, 1); !errors.Is(err, tc.want) {
				t.Fatalf("outer exact fill admission: %v", err)
			}
			if err := ProcessExactFillTx(ctx, nil, nil, tc.fill, 1); !errors.Is(err, tc.want) {
				t.Fatalf("transactional exact fill admission: %v", err)
			}
		})
	}
	for _, relation := range []string{"chart_exact", "chart_traders", "chart_outbox"} {
		t.Run("outer "+relation, func(t *testing.T) {
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
			if err := ProcessExactFill(ctx, exact, fillFixture(), 1); err == nil || !strings.Contains(err.Error(), relation) {
				t.Fatalf("outer exact fill relation %s: %v", relation, err)
			}
			var claims int
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM candles.chart_inbox`).Scan(&claims); err != nil || claims != 0 {
				t.Fatalf("refused outer fill claimed identity: %d, %v", claims, err)
			}
		})
	}
}
