package engine

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
)

func TestReadSeriesReturnsCommittedAndCarriedCandles(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
	for _, schema := range []func(string) (string, error){store.ExactSchemaDDL, store.SeriesSchemaDDL} {
		ddl, err := schema("candles")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, ddl); err != nil {
			t.Fatal(err)
		}
	}
	exact, err := store.NewExact(pool, "candles")
	if err != nil {
		t.Fatal(err)
	}
	fill := seriesSource(0, 60, "")
	if err := ValidateCheckpoint(SeriesCheckpoint{Height: 100, Timestamp: 180}, fill); err != nil {
		t.Fatalf("valid source checkpoint: %v", err)
	}
	if err := ValidateCheckpoint(SeriesCheckpoint{Height: 100, Timestamp: -1}, fill); !errors.Is(err, ErrInvalidFill) {
		t.Fatalf("negative checkpoint timestamp: %v", err)
	}
	bar, err := FoldQualified(nil, fill, "1m", false)
	if err != nil {
		t.Fatal(err)
	}
	state, err := DescribeSeries(bar, SeriesCheckpoint{Height: 100, Timestamp: 180}, time.Unix(181, 0))
	if err != nil {
		t.Fatal(err)
	}
	wire, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	address := "0x" + strings.Repeat("2", 40)
	for _, stmt := range []string{
		`INSERT INTO candles.chart_pool_fences(chain_id,pool,epoch) VALUES('125',$1,1)`,
		`INSERT INTO candles.chart_series_clock(chain_id,pool,revision,watermark_height,watermark_timestamp,has_gaps) VALUES('125',$1,1,100,180,false)`,
	} {
		if _, err := pool.Exec(ctx, stmt, address); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO candles.chart_exact(chain_id,pool,interval,start,snapshot) VALUES('125',$1,'1m',60,$2)`, address, wire); err != nil {
		t.Fatal(err)
	}
	sparse, err := ReadSeries(ctx, exact, address, "1m", 60, 180, false)
	if err != nil || len(sparse) != 1 || sparse[0].Synthetic || sparse[0].Start != 60 {
		t.Fatalf("committed series: %+v, %v", sparse, err)
	}
	dense, err := ReadSeries(ctx, exact, address, "1m", 60, 180, true)
	if err != nil || len(dense) != 2 || dense[0].Synthetic || !dense[1].Synthetic || dense[1].Start != 120 || dense[1].Close != dense[0].Close {
		t.Fatalf("carried series: %+v, %v", dense, err)
	}
	if rows, err := ReadSeries(ctx, nil, address, "1m", 60, 180, false); rows != nil || !errors.Is(err, ErrInvalidFill) {
		t.Fatalf("missing series store: %+v, %v", rows, err)
	}
	if rows, err := ReadSeries(ctx, exact, address, "1m", 61, 180, true); rows != nil || !errors.Is(err, ErrInvalidFill) {
		t.Fatalf("unaligned dense series: %+v, %v", rows, err)
	}
	if _, err = pool.Exec(ctx, `UPDATE candles.chart_exact SET snapshot='"invalid"' WHERE pool=$1 AND interval='1m'`, address); err != nil {
		t.Fatal(err)
	}
	if rows, err := ReadSeries(ctx, exact, address, "1m", 60, 180, false); rows != nil || err == nil {
		t.Fatalf("malformed committed candle: %+v, %v", rows, err)
	}
	if rows, err := ReadSeries(ctx, exact, address, "1m", 120, 180, true); rows != nil || err == nil {
		t.Fatalf("malformed seed candle: %+v, %v", rows, err)
	}
	if _, err = pool.Exec(ctx, `DROP TABLE candles.chart_series_clock`); err != nil {
		t.Fatal(err)
	}
	if rows, err := ReadSeries(ctx, exact, address, "1m", 60, 180, false); rows != nil || err == nil {
		t.Fatalf("unavailable series clock: %+v, %v", rows, err)
	}
}
