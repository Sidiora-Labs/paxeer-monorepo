package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/candlewire"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/candlewire/testfixture"
)

func candleIdentityRecords(t *testing.T) []testfixture.Record {
	t.Helper()
	raw, err := os.ReadFile("../../../trading-charts/candlewire/testdata/candle_identity.json")
	if err != nil {
		t.Fatal(err)
	}
	var records []testfixture.Record
	if err = json.Unmarshal(raw, &records); err != nil {
		t.Fatal(err)
	}
	return records
}

func TestCandleIdentityPages(t *testing.T) {
	model, db := productionAccountingFixture(t)
	ctx := context.Background()
	r := store.AccountingPageRequest{Pool: productionAddress, Interval: "1m", From: 1699999800000, To: 1700000400000, Limit: 10}
	empty, err := model.Candles(ctx, r)
	if !errors.Is(err, store.ErrAccountingUntraded) || len(empty.Items) != 0 || empty.SeriesModelVersion != candlewire.SeriesModelVersion {
		t.Fatalf("untraded candle page: %+v %v", empty, err)
	}
	insertProductionFill(t, db, 1, "")
	fill, _, _ := productionFill(t, productionAddress, productionBase, 1, "")
	for _, record := range candleIdentityRecords(t) {
		var bar struct {
			Interval string `json:"interval"`
			Start    int64  `json:"start"`
		}
		if err = json.Unmarshal(record.StoredBar, &bar); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(record.Fills[fill.SourceFillID], fill) {
			t.Fatal("record fill differs from committed fill")
		}
		if _, err = db.Exec(ctx, `INSERT INTO candles.chart_exact VALUES('125',$1,$2,$3,$4)`, productionAddress, bar.Interval, bar.Start, record.StoredBar); err != nil {
			t.Fatal(err)
		}
		r.Interval = bar.Interval
		page, err := model.Candles(ctx, r)
		if err != nil || len(page.Items) != 1 || page.SeriesModelVersion != candlewire.SeriesModelVersion {
			t.Fatalf("candle page: %+v %v", page, err)
		}
		var actual, expected map[string]any
		if err = json.Unmarshal(page.Items[0], &actual); err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(record.PublicCandle, &expected); err != nil {
			t.Fatal(err)
		}
		if actual["snapshotId"] != page.SnapshotID || actual["seriesModelVersion"] != page.SeriesModelVersion {
			t.Fatal(actual)
		}
		expected["snapshotId"] = page.SnapshotID
		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("history and published candle differ: got %s want %v", page.Items[0], expected)
		}
		rangeRequest := r
		rangeRequest.From, rangeRequest.To = 1700001000000, 1700001060000
		empty, err = model.Candles(ctx, rangeRequest)
		if err != nil || len(empty.Items) != 0 || empty.SeriesModelVersion != candlewire.SeriesModelVersion || empty.SnapshotID != page.SnapshotID {
			t.Fatalf("empty range: %+v %v", empty, err)
		}
		for _, change := range []string{
			`jsonb_set(snapshot,'{algorithmVersion}','"another-series"')`,
			`snapshot-'algorithmVersion'`,
			`snapshot-'algorithmVersion'-'sourceWatermarkHeight'-'sourceWatermarkTimestamp'-'intervalEnd'-'completenessStatus'-'synthetic'-'provenance'-'displayRevision'-'correctionReason'-'actorSemantics'`,
			`jsonb_set(snapshot,'{open,numerator}','"2"')`,
			`jsonb_set(snapshot,'{closeSource,sourceFillId}','"unknown-fill"')`,
		} {
			if _, err = db.Exec(ctx, `UPDATE candles.chart_exact SET snapshot=`+change+` WHERE interval=$1`, bar.Interval); err != nil {
				t.Fatal(err)
			}
			if _, err = model.Candles(ctx, r); !errors.Is(err, store.ErrDataIntegrity) {
				t.Fatalf("invalid stored candle accepted: %s %v", change, err)
			}
			if _, err = db.Exec(ctx, `UPDATE candles.chart_exact SET snapshot=$2 WHERE interval=$1`, bar.Interval, record.StoredBar); err != nil {
				t.Fatal(err)
			}
		}
	}
	trades, err := model.Trades(ctx, store.AccountingPageRequest{Pool: productionAddress, Limit: 10})
	if err != nil || trades.SeriesModelVersion != productionProjection {
		t.Fatalf("trade projection: %+v %v", trades, err)
	}
}

func TestCandleIdentityMaximumPageByteBound(t *testing.T) {
	model, db := productionAccountingFixture(t)
	insertProductionFill(t, db, 1, "")
	record := candleIdentityRecords(t)[0]
	if _, err := db.Exec(t.Context(), `INSERT INTO candles.chart_exact SELECT '125',$1,'1m',1699999980+n*60,jsonb_set(jsonb_set($2::jsonb,'{start}',to_jsonb(1699999980+n*60)),'{intervalEnd}',to_jsonb(1700000040+n*60)) FROM generate_series(0,1999) n`, productionAddress, record.StoredBar); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	page, err := model.Candles(t.Context(), store.AccountingPageRequest{Pool: productionAddress, Interval: "1m", From: 1699999980000, To: (1699999980 + 2000*60) * 1000, Limit: store.MaxCanonicalCandles})
	elapsed := time.Since(started)
	if !errors.Is(err, store.ErrDataIntegrity) || len(page.Items) != 0 {
		t.Fatalf("stored page byte bound: %d items %v", len(page.Items), err)
	}
	t.Logf("2000 stored bars exceed the 256 KiB page bound; rejection took %s", elapsed)
}
