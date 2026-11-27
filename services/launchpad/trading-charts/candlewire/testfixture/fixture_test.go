package testfixture_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/candlewire"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/candlewire/testfixture"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/engine"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
)

func TestRecordsPreserveCanonicalCandles(t *testing.T) {
	records, err := testfixture.Records()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("records = %d, want 2", len(records))
	}
	for i, record := range records {
		candle, err := candlewire.FromStoredBar(record.StoredBar, record.Fills)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(candle)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(raw, record.PublicCandle) {
			t.Fatal("record differs from canonical mapping")
		}
		if candle.Interval != []string{"1m", "5m"}[i] || candle.CompletenessStatus != []string{"complete_through_watermark", "live_interval"}[i] || string(candle.Revision) != "1" || candle.DisplayRevision != "1" {
			t.Fatalf("record candle: %+v", candle)
		}
		for _, fill := range record.Fills {
			if err := accounting.ValidateFillV1(fill); err != nil {
				t.Fatal(err)
			}
			committed, err := testfixture.CommittedBar(fill, candle.Interval)
			if err != nil {
				t.Fatal(err)
			}
			mapped, err := candlewire.FromStoredBar(committed, record.Fills)
			if err != nil {
				t.Fatal(err)
			}
			if mapped.CompletenessStatus != "complete_through_watermark" || mapped.Coverage != "complete" || string(mapped.VolumeUsdlRaw) != fill.USDLAmountRaw || string(mapped.VolumeTokenRaw) != fill.TokenAmountRaw {
				t.Fatalf("committed candle: %+v", mapped)
			}
			gapped, err := testfixture.StoredBar(fill, candle.Interval, engine.SeriesCheckpoint{Height: 110, Timestamp: 1700000600, HasGaps: true}, time.Unix(1700000601, 0))
			if err != nil {
				t.Fatal(err)
			}
			var bar engine.SeriesCandle
			if err := json.Unmarshal(gapped, &bar); err != nil {
				t.Fatal(err)
			}
			if bar.CompletenessStatus != "backfill_incomplete" || bar.SourceWatermarkHeight != 110 {
				t.Fatal(bar)
			}
			if _, err := testfixture.CommittedBar(fill, "unsupported"); !errors.Is(err, engine.ErrUnsupportedResolution) {
				t.Fatalf("unsupported interval: %v", err)
			}
		}
	}
}

func TestStoredBarRejectsInvalidFill(t *testing.T) {
	if _, err := testfixture.StoredBar(accounting.FillV1{}, "1m", engine.SeriesCheckpoint{}, time.Unix(1700000000, 0)); !errors.Is(err, accounting.ErrInvalid) || err.Error() != "invalid accounting value: version or position" {
		t.Fatalf("invalid fill: %v", err)
	}
}
