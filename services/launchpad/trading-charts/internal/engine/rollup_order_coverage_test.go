package engine

import (
	"errors"
	"testing"
)

func TestQualifiedRollupPreservesSourceOrderAcrossMinuteBuckets(t *testing.T) {
	firstSource := seriesSource(1, 60, "")
	secondSource := seriesSource(0, 120, "")
	first, err := FoldQualified(nil, firstSource, "1m", false)
	if err != nil {
		t.Fatal(err)
	}
	second, err := FoldQualified(nil, secondSource, "1m", false)
	if err != nil {
		t.Fatal(err)
	}
	rolled, digest, err := RollupQualified([]QualifiedCandle{second, first}, nil, "5m", 0)
	if err != nil || rolled.Trades != 2 || rolled.FirstPosition != second.FirstPosition || rolled.LastPosition != first.LastPosition || rolled.OpenSource.SourceFillID != secondSource.SourceFillID || rolled.CloseSource.SourceFillID != firstSource.SourceFillID || len(digest) != 64 {
		t.Fatalf("source ordered rollup: %+v digest=%s err=%v", rolled, digest, err)
	}
	for _, tc := range []struct {
		name   string
		change func(*QualifiedCandle, *QualifiedCandle)
	}{
		{"unbound source position", func(a, b *QualifiedCandle) { a.OpenSource.Position.Height = "bad" }},
		{"unbound source identity", func(a, b *QualifiedCandle) { a.OpenSource.SourceFillID = "other" }},
		{"invalid high ratio", func(a, b *QualifiedCandle) { a.High.Numerator = "bad" }},
		{"invalid low ratio", func(a, b *QualifiedCandle) { a.Low.Denominator = "0" }},
		{"trade counter overflow", func(a, b *QualifiedCandle) { a.Trades = ^uint64(0) }},
		{"unknown actor counter overflow", func(a, b *QualifiedCandle) { a.UnknownActorTradeCount = ^uint64(0); b.UnknownActorTradeCount = 1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, b := first, second
			tc.change(&a, &b)
			if _, _, err := RollupQualified([]QualifiedCandle{a, b}, nil, "5m", 0); !errors.Is(err, ErrInvalidFill) {
				t.Fatalf("unqualified minute rollup: %v", err)
			}
		})
	}
}
