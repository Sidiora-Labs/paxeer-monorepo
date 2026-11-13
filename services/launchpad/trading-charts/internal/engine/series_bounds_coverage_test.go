package engine

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDenseSeriesSeedAndCommittedIdentityBounds(t *testing.T) {
	source := seriesSource(0, 60, "")
	qualified, err := FoldQualified(nil, source, "1m", false)
	if err != nil {
		t.Fatal(err)
	}
	seed, err := DescribeSeries(qualified, SeriesCheckpoint{Height: 100, Timestamp: 180}, time.Unix(181, 0))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := DenseView(nil, 120, 240, "1m", &seed, 3)
	if err != nil || len(rows) != 2 || !rows[0].Synthetic || rows[0].Start != 120 || rows[1].DisplayRevision != 3 || rows[1].Close != seed.Close {
		t.Fatalf("seeded series carry: %+v, %v", rows, err)
	}
	invalid := []struct {
		name     string
		rows     []SeriesCandle
		seed     *SeriesCandle
		from     int64
		to       int64
		interval string
	}{
		{"unaligned start", nil, nil, 61, 180, "1m"},
		{"unbounded range", nil, nil, 0, 600120, "1m"},
		{"unsupported interval", nil, nil, 0, 60, "invalid"},
		{"seed inside range", nil, &seed, 60, 180, "1m"},
		{"duplicate committed candle", []SeriesCandle{seed, seed}, nil, 60, 180, "1m"},
		{"synthetic committed candle", []SeriesCandle{func() SeriesCandle { copy := seed; copy.Synthetic = true; return copy }()}, nil, 60, 180, "1m"},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := DenseView(tc.rows, tc.from, tc.to, tc.interval, tc.seed, 3); !errors.Is(err, ErrInvalidFill) {
				t.Fatalf("series identity accepted: %v", err)
			}
		})
	}
}

func TestRollupRejectsConflictingChildrenAndAmountBounds(t *testing.T) {
	first, err := FoldQualified(nil, seriesSource(0, 60, ""), "1m", false)
	if err != nil {
		t.Fatal(err)
	}
	second, err := FoldQualified(nil, seriesSource(1, 120, ""), "1m", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		children []QualifiedCandle
		actors   []string
		interval string
	}{
		{"minute parent", []QualifiedCandle{first}, nil, "1m"},
		{"duplicate minute", []QualifiedCandle{first, first}, nil, "5m"},
		{"conflicting pool", []QualifiedCandle{first, func() QualifiedCandle { copy := second; copy.PoolAddress = "0x" + strings.Repeat("a", 40); return copy }()}, nil, "5m"},
		{"invalid actor", []QualifiedCandle{first}, []string{"not-an-address"}, "5m"},
		{"too many actors", []QualifiedCandle{first}, []string{"0x" + strings.Repeat("a", 40)}, "5m"},
		{"non-numeric volume", []QualifiedCandle{func() QualifiedCandle { copy := first; copy.VolumeUsdlRaw = "bad"; return copy }()}, nil, "5m"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := RollupQualified(tc.children, tc.actors, tc.interval, 0); !errors.Is(err, ErrInvalidFill) {
				t.Fatalf("invalid parent fold accepted: %v", err)
			}
		})
	}
	if _, err := boundedSum(strings.Repeat("9", 1024), "1"); !errors.Is(err, ErrInvalidFill) {
		t.Fatalf("overlong amount accepted: %v", err)
	}
}
