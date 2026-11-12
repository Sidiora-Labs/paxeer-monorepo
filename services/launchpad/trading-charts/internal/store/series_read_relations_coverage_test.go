package store

import (
	"context"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
)

func TestSeriesReadRequiresCommittedRelations(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
	for _, ddl := range []func(string) (string, error){ExactSchemaDDL, SeriesSchemaDDL} {
		statement, err := ddl("candles")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	exact, err := NewExact(pool, "candles")
	if err != nil {
		t.Fatal(err)
	}
	address := "0x" + strings.Repeat("a", 40)
	for _, statement := range []string{
		`INSERT INTO candles.chart_pool_fences(chain_id,pool,epoch) VALUES('125',$1,1)`,
		`INSERT INTO candles.chart_series_clock(chain_id,pool,revision,watermark_height,watermark_timestamp,has_gaps) VALUES('125',$1,1,100,180,false)`,
		`INSERT INTO candles.chart_exact(chain_id,pool,interval,start,snapshot) VALUES('125',$1,'1m',60,'{"intervalEnd":120}')`,
	} {
		if _, err := pool.Exec(ctx, statement, address); err != nil {
			t.Fatal(err)
		}
	}
	rows, _, revision, err := exact.SeriesSnapshots(ctx, address, "1m", 60, 180, 10)
	if err != nil || len(rows) != 1 || revision != 1 {
		t.Fatalf("committed series read: rows=%d revision=%d err=%v", len(rows), revision, err)
	}
	for _, relation := range []string{"chart_series_clock", "chart_pool_fences", "chart_delivery_attempts", "chart_exact", "chart_rollup_pending"} {
		t.Run(relation, func(t *testing.T) {
			if _, err := pool.Exec(ctx, `ALTER TABLE candles.`+relation+` RENAME TO `+relation+`_held`); err != nil {
				t.Fatal(err)
			}
			_, _, _, observed := exact.SeriesSnapshots(ctx, address, "1m", 60, 180, 10)
			if _, err := pool.Exec(ctx, `ALTER TABLE candles.`+relation+`_held RENAME TO `+relation); err != nil {
				t.Fatal(err)
			}
			if observed == nil || !strings.Contains(observed.Error(), relation) {
				t.Fatalf("series read without %s: %v", relation, observed)
			}
		})
	}
	if _, err := pool.Exec(ctx, `UPDATE candles.chart_exact SET snapshot='"invalid"'::jsonb WHERE pool=$1 AND start=60`, address); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := exact.SeriesSnapshots(ctx, address, "1m", 60, 180, 10); err == nil || !strings.Contains(err.Error(), "unmarshal") {
		t.Fatalf("unreadable committed candle: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE candles.chart_exact SET snapshot='{"intervalEnd":120}'::jsonb WHERE pool=$1 AND start=60`, address); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO candles.chart_exact(chain_id,pool,interval,start,snapshot) VALUES('125',$1,'1m',120,'{"intervalEnd":180}')`, address); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := exact.SeriesSnapshots(ctx, address, "1m", 60, 180, 1); err == nil || !strings.Contains(err.Error(), "series_range_exceeds_bound") {
		t.Fatalf("bounded committed range: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO candles.chart_exact(chain_id,pool,interval,start,snapshot) VALUES('125',$1,'1m',0,'"invalid"')`, address); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := exact.SeriesSnapshots(ctx, address, "1m", 60, 180, 10); err == nil || !strings.Contains(err.Error(), "unmarshal") {
		t.Fatalf("unreadable seed candle: %v", err)
	}
}
