package engine

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
)

func TestReconstructionProjectionKeepsOnlyRequestedCanonicalBuckets(t *testing.T) {
	first := seriesSource(0, 1704067260, "0x"+strings.Repeat("a", 40))
	outside := seriesSource(1, 1704672060, "")
	projection, err := buildReconstructionProjection([]accounting.FillV1{outside, first, first}, 1704067200, 1704067500, SeriesCheckpoint{Height: 2, Timestamp: 1704067500}, time.Unix(1704067500, 0).UTC())
	if err != nil || len(projection.Bars) == 0 || len(projection.Traders) == 0 || len(projection.Digest) != 64 {
		t.Fatalf("bounded reconstruction: bars=%d traders=%d digest=%q err=%v", len(projection.Bars), len(projection.Traders), projection.Digest, err)
	}
	for _, bar := range projection.Bars {
		if bar.Trades != 2 || bar.PoolAddress != first.PoolAddress {
			t.Fatalf("reconstructed source boundary: %+v", bar)
		}
	}
	invalid := first
	invalid.SourceFillID = "changed"
	if result, err := buildReconstructionProjection([]accounting.FillV1{invalid}, 1704067200, 1704067500, SeriesCheckpoint{}, time.Now()); err == nil || len(result.Bars) != 0 {
		t.Fatalf("invalid canonical fill: %+v, %v", result, err)
	}
	if result, err := buildReconstructionProjection(nil, 10, 10, SeriesCheckpoint{}, time.Now()); !errors.Is(err, ErrInvalidFill) || len(result.Bars) != 0 {
		t.Fatalf("invalid reconstruction range: %+v, %v", result, err)
	}
}

func TestRollupRejectsInvalidMinuteEconomics(t *testing.T) {
	source := seriesSource(0, 60, "")
	minute, err := FoldQualified(nil, source, "1m", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		edit func(*QualifiedCandle)
	}{
		{"invalid volume", func(c *QualifiedCandle) { c.VolumeUsdlRaw = "bad" }},
		{"invalid token volume", func(c *QualifiedCandle) { c.VolumeTokenRaw = strings.Repeat("9", 1025) }},
		{"zero trades", func(c *QualifiedCandle) { c.Trades = 0 }},
		{"wrong source identity", func(c *QualifiedCandle) { c.OpenSource.SourceFillID = "changed" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := minute
			tc.edit(&candidate)
			rolled, digest, err := RollupQualified([]QualifiedCandle{candidate}, nil, "5m", 0)
			if err == nil || digest != "" || rolled.Trades != 0 {
				t.Fatalf("invalid minute economics: %+v digest=%q err=%v", rolled, digest, err)
			}
		})
	}
	if rolled, digest, err := RollupQualified([]QualifiedCandle{minute}, nil, "1m", 0); !errors.Is(err, ErrInvalidFill) || digest != "" || rolled.Trades != 0 {
		t.Fatalf("minute parent rejected: %+v digest=%q err=%v", rolled, digest, err)
	}
	if rolled, digest, err := RollupQualified([]QualifiedCandle{minute}, nil, "5m", 60); !errors.Is(err, ErrInvalidFill) || digest != "" || rolled.Trades != 0 {
		t.Fatalf("unaligned parent rejected: %+v digest=%q err=%v", rolled, digest, err)
	}
}

func TestMeasurementReferencePreservesBudgetOnNewBucket(t *testing.T) {
	reference, err := NewMeasurementReference(7)
	if err != nil {
		t.Fatal(err)
	}
	first := seriesSource(0, 1704067260, "")
	if err := reference.Add(first); err != nil {
		t.Fatal(err)
	}
	later := seriesSource(1, 1704067860, "")
	if err := reference.Add(later); err == nil || !strings.Contains(err.Error(), "bar_budget") {
		t.Fatalf("new bucket beyond budget: %v", err)
	}
	if bars := reference.Bars(); len(bars) != 7 {
		t.Fatalf("retained bars after refused addition: %d", len(bars))
	}
}

func TestMeasurementReferenceRejectsConflictingSourceOrderAndTradeCount(t *testing.T) {
	fill := seriesSource(0, 1704067260, "")
	if err := (*MeasurementReference)(nil).Add(fill); !errors.Is(err, ErrInvalidFill) {
		t.Fatalf("missing measurement reference: %v", err)
	}
	if bars := (*MeasurementReference)(nil).Bars(); bars != nil {
		t.Fatalf("missing reference bars: %+v", bars)
	}
	for _, tc := range []struct {
		name string
		edit func(*measurementAccumulator)
	}{
		{"first source order", func(acc *measurementAccumulator) { acc.bar.First.Height = "invalid" }},
		{"last source order", func(acc *measurementAccumulator) { acc.bar.Last.Height = "invalid" }},
		{"trade count", func(acc *measurementAccumulator) { acc.bar.Trades = ^uint64(0) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reference, err := NewMeasurementReference(7)
			if err != nil || reference.Add(fill) != nil {
				t.Fatalf("initial canonical measurement: %v", err)
			}
			for _, acc := range reference.bars {
				tc.edit(acc)
			}
			if err := reference.Add(fill); err == nil {
				t.Fatal("conflicting measurement state was accepted")
			}
		})
	}
	if _, err := measurementBucket(1704067260, "unsupported"); !errors.Is(err, ErrUnsupportedResolution) {
		t.Fatalf("unsupported measurement interval: %v", err)
	}
	bar := MeasurementBar{Pool: fill.PoolAddress, Interval: "1m", Start: 1704067260}
	if digest, err := MeasurementDigest([]MeasurementBar{bar, bar}); digest != "" || !errors.Is(err, ErrInvalidFill) {
		t.Fatalf("duplicate measurement bar: %q, %v", digest, err)
	}
	changed := seriesSource(1, 1704067260, "")
	changed.PoolAddress = "0x" + strings.Repeat("b", 40)
	if projection, err := buildReconstructionProjection([]accounting.FillV1{fill, changed}, 1704067200, 1704067500, SeriesCheckpoint{Height: 2, Timestamp: 1704067500}, time.Unix(1704067500, 0)); err == nil || len(projection.Bars) != 0 {
		t.Fatalf("cross-pool reconstruction: %+v, %v", projection, err)
	}
}
