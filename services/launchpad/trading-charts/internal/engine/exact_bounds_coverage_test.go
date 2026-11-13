package engine

import (
	"errors"
	"strings"
	"testing"
)

func TestExactPriceRejectsUnqualifiedMonetaryFacts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Fill)
	}{
		{"wrong chain", func(f *Fill) { f.ChainID = "1" }},
		{"untrusted timestamp", func(f *Fill) { f.TimestampAuthenticated = false }},
		{"invalid decimal metadata", func(f *Fill) { f.TokenDecimals = 256 }},
		{"wrong settlement scale", func(f *Fill) { f.UsdlDecimals = 18 }},
		{"unknown side", func(f *Fill) { f.Side = "exchange" }},
		{"zero settlement", func(f *Fill) { f.UsdlAmountRaw = "0" }},
		{"bad paid input", func(f *Fill) { f.PaidInputRaw = "-1" }},
		{"fee exceeds payment", func(f *Fill) { f.FeeRaw = "2000001" }},
		{"payment mismatch", func(f *Fill) { f.PaidInputRaw = "1" }},
		{"wrong fee treatment", func(f *Fill) { f.FeeDisposition = "retained_token_reserve" }},
		{"invalid exact input", func(f *Fill) { raw := "01"; f.PricedInputRaw = &raw }},
		{"unqualified derivation", func(f *Fill) { f.InputDerivationQuality = "inferred" }},
		{"incomplete exact evidence", func(f *Fill) { f.InputDerivationQuality = "exact" }},
		{"inconsistent exact fee", func(f *Fill) {
			priced, amm := "2000000", "1990000"
			f.PricedInputRaw, f.AmmInputAfterFeeRaw = &priced, &amm
			f.InputDerivationQuality = "exact"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fill := fillFixture()
			tc.change(&fill)
			if _, err := fill.Price(); !errors.Is(err, ErrInvalidFill) {
				t.Fatalf("unqualified exact price: %v", err)
			}
		})
	}
	fill := fillFixture()
	priced, amm := "2000000", "1999900"
	fill.PricedInputRaw, fill.AmmInputAfterFeeRaw = &priced, &amm
	fill.InputDerivationQuality = "exact"
	feeExclusive, err := fill.FeeExclusivePrice()
	if err != nil || feeExclusive == nil || feeExclusive.Numerator != "19999" || feeExclusive.Denominator != "40000" {
		t.Fatalf("fee-exclusive canonical price: %+v, %v", feeExclusive, err)
	}
	fill.Side = "sell"
	fill.PaidInputRaw = "4000000"
	fill.FeeDisposition = "retained_token_reserve"
	priced, amm = "4000000", "3999900"
	feeExclusive, err = fill.FeeExclusivePrice()
	if err != nil || feeExclusive == nil || feeExclusive.Numerator == "0" {
		t.Fatalf("sell reserve price: %+v, %v", feeExclusive, err)
	}
}

func TestExactFoldRejectsCorruptStoredAmountsAndCounters(t *testing.T) {
	fill := fillFixture()
	prior, err := FoldExact(nil, fill, "1h", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*ExactCandle)
	}{
		{"empty amount", func(c *ExactCandle) { c.VolumeUsdlRaw = "" }},
		{"leading zero", func(c *ExactCandle) { c.VolumeTokenRaw = "01" }},
		{"non-numeric amount", func(c *ExactCandle) { c.BuyVolumeUsdlRaw = "1x" }},
		{"oversized amount", func(c *ExactCandle) { c.SellVolumeUsdlRaw = strings.Repeat("9", 1025) }},
		{"wrong pool", func(c *ExactCandle) { c.PoolAddress = "other" }},
		{"invalid high price", func(c *ExactCandle) { c.High = Rational{"invalid", "1"} }},
		{"invalid low price", func(c *ExactCandle) { c.Low = Rational{"1", "0"} }},
		{"trade counter overflow", func(c *ExactCandle) { c.Trades = ^uint64(0) }},
		{"revision counter overflow", func(c *ExactCandle) { c.Revision = ^uint64(0) }},
		{"trader counter overflow", func(c *ExactCandle) { c.UniqueTraders = ^uint64(0) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			copy := prior
			tc.change(&copy)
			_, err := FoldExact(&copy, fill, "1h", true)
			if err == nil {
				t.Fatal("invalid stored candle accepted")
			}
		})
	}
}

func TestQualifiedFoldRejectsConflictingStoredProvenance(t *testing.T) {
	source := seriesSource(0, 1704067260, "")
	prior, err := FoldQualified(nil, source, "1m", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		edit func(*QualifiedCandle)
	}{
		{"asset", func(c *QualifiedCandle) { c.Token.Address = "0x" + strings.Repeat("a", 40) }},
		{"opening source order", func(c *QualifiedCandle) { c.OpenSource.Position.Height = "999" }},
		{"high source identity", func(c *QualifiedCandle) { c.HighSource.SourceFillID = "other" }},
		{"high price", func(c *QualifiedCandle) { c.High = Rational{Numerator: "invalid", Denominator: "1"} }},
		{"low price", func(c *QualifiedCandle) { c.Low = Rational{Numerator: "1", Denominator: "0"} }},
		{"unknown actor count", func(c *QualifiedCandle) { c.UnknownActorTradeCount = ^uint64(0) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := prior
			tc.edit(&candidate)
			if _, err := FoldQualified(&candidate, source, "1m", false); !errors.Is(err, ErrInvalidFill) {
				t.Fatalf("conflicting stored provenance: %v", err)
			}
		})
	}
}
