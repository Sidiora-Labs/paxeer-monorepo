package engine

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
)

func TestSeriesIdentityRejectsConflictingPoolsAndInvalidSnapshots(t *testing.T) {
	firstSource := seriesSource(0, 60, "")
	first, err := FoldQualified(nil, firstSource, "1m", false)
	if err != nil {
		t.Fatal(err)
	}
	secondSource := seriesSource(1, 120, "")
	secondSource.PoolAddress = "0x" + strings.Repeat("a", 40)
	second, err := FoldQualified(nil, secondSource, "1m", false)
	if err != nil {
		t.Fatal(err)
	}
	a, err := DescribeSeries(first, SeriesCheckpoint{Height: 101, Timestamp: 180}, time.Unix(181, 0))
	if err != nil {
		t.Fatal(err)
	}
	b, err := DescribeSeries(second, SeriesCheckpoint{Height: 101, Timestamp: 180}, time.Unix(181, 0))
	if err != nil {
		t.Fatal(err)
	}
	if bars, err := DenseView([]SeriesCandle{a, b}, 60, 180, "1m", nil, 1); bars != nil || !errors.Is(err, ErrInvalidFill) {
		t.Fatalf("cross-pool dense candle: %+v, %v", bars, err)
	}
	invalid := first
	invalid.Interval = "unknown"
	if observed, err := DescribeSeries(invalid, SeriesCheckpoint{}, time.Now()); !errors.Is(err, ErrInvalidFill) || observed.AlgorithmVersion != "" {
		t.Fatalf("unsupported series interval: %+v, %v", observed, err)
	}
	for _, raw := range []json.RawMessage{[]byte(`{`), []byte(`{"algorithmVersion":"other","trades":1}`)} {
		if bars, err := decodeSeriesSnapshots([]json.RawMessage{raw}); bars != nil || err == nil {
			t.Fatalf("unqualified persisted candle: %+v, %v", bars, err)
		}
	}
	if bars, err := ReadSeries(context.Background(), nil, firstSource.PoolAddress, "1m", 60, 180, false); bars != nil || !errors.Is(err, ErrInvalidFill) {
		t.Fatalf("series without exact store: %+v, %v", bars, err)
	}
	if _, err := measurementBucket(-1, "1m"); !errors.Is(err, ErrInvalidFill) {
		t.Fatalf("negative measurement timestamp: %v", err)
	}
	bad := firstSource
	bad.Position = accounting.SourcePosition{}
	if err := ValidateCheckpoint(SeriesCheckpoint{Height: 1}, bad); !errors.Is(err, ErrInvalidFill) {
		t.Fatalf("checkpoint without canonical fill: %v", err)
	}
}
