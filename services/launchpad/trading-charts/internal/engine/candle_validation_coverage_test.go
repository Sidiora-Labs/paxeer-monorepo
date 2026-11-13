package engine

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
)

func TestCanonicalCandleRejectsInconsistentSourceAndBounds(t *testing.T) {
	source := seriesSource(0, 1704067260, "")
	folded, err := FoldQualified(nil, source, "1m", false)
	if err != nil {
		t.Fatal(err)
	}
	bar, err := DescribeSeries(folded, SeriesCheckpoint{Height: 200, Timestamp: 1704067380}, time.Unix(1704067380, 0))
	if err != nil {
		t.Fatal(err)
	}
	bar.DisplayRevision = 1
	sources := map[string]accounting.FillV1{source.SourceFillID: source}
	if _, err := CanonicalCandle(bar, sources); err != nil {
		t.Fatalf("canonical candle rejected: %v", err)
	}
	for _, tc := range []struct {
		name  string
		edit  func(*SeriesCandle)
		fills map[string]accounting.FillV1
	}{
		{"missing source", func(*SeriesCandle) {}, nil},
		{"unsupported interval", func(c *SeriesCandle) { c.Interval = "2m" }, sources},
		{"wrong interval end", func(c *SeriesCandle) { c.IntervalEnd++ }, sources},
		{"invalid status", func(c *SeriesCandle) { c.CompletenessStatus = "unknown" }, sources},
		{"invalid start", func(c *SeriesCandle) { c.Start = -60; c.IntervalEnd = 0 }, sources},
		{"invalid end", func(c *SeriesCandle) { c.Start = 253402300800; c.IntervalEnd = c.Start + 60 }, sources},
		{"wrong source price", func(c *SeriesCandle) { c.Open.Numerator = "999" }, sources},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := bar
			tc.edit(&changed)
			if _, err := CanonicalCandle(changed, tc.fills); !errors.Is(err, ErrInvalidFill) {
				t.Fatalf("inconsistent candle accepted: %v", err)
			}
		})
	}
}

func TestQualifiedFoldRejectsChangedSourceAuthority(t *testing.T) {
	source := seriesSource(0, 1704067260, "")
	base, err := FoldQualified(nil, source, "1m", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		edit func(*QualifiedCandle)
	}{
		{"asset", func(c *QualifiedCandle) { c.Token.Address = "0x" + strings.Repeat("a", 40) }},
		{"position", func(c *QualifiedCandle) { c.OpenSource.Position.Height = "999" }},
		{"identity", func(c *QualifiedCandle) { c.HighSource.SourceFillID = "different" }},
		{"high price", func(c *QualifiedCandle) { c.High.Denominator = "0" }},
		{"low price", func(c *QualifiedCandle) { c.Low.Denominator = "0" }},
		{"actor count", func(c *QualifiedCandle) { c.UnknownActorTradeCount = ^uint64(0) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := base
			tc.edit(&changed)
			if _, err := FoldQualified(&changed, source, "1m", false); !errors.Is(err, ErrInvalidFill) {
				t.Fatalf("changed source authority accepted: %v", err)
			}
		})
	}
	if _, err := FoldQualified(&base, source, "2m", false); !errors.Is(err, ErrUnsupportedResolution) {
		t.Fatalf("unsupported interval: %v", err)
	}
	if _, err := buildReconstructionProjection(nil, -1, 1, SeriesCheckpoint{}, time.Now()); !errors.Is(err, ErrInvalidFill) {
		t.Fatalf("negative archive boundary: %v", err)
	}
	if _, err := buildReconstructionProjection([]accounting.FillV1{{}}, 0, 1, SeriesCheckpoint{}, time.Now()); err == nil {
		t.Fatal("invalid archive source accepted")
	}
	if err := verifyReconstructionParity("first", "second"); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("changed archive economics: %v", err)
	}
}
