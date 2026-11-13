package store

import (
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
)

func TestCanonicalPricePreservesCommittedFillAmounts(t *testing.T) {
	pool := "0x" + strings.Repeat("1", 40)
	fill := accounting.FillV1{
		SourceFillID: "source-1", ActualEmitter: pool,
		Position:       accounting.SourcePosition{ChainID: "125", Height: "1", TransactionIndex: "0", LogIndex: "0"},
		Token:          accounting.Asset{Address: "0x" + strings.Repeat("2", 40), Decimals: 18},
		Settlement:     accounting.Asset{Address: "0x" + strings.Repeat("3", 40), Decimals: 6},
		TokenAmountRaw: "1000000000000000000", USDLAmountRaw: "1000000", BlockTimestampSeconds: "1700000000",
	}
	source := accountingPriceSource{SourceFillID: fill.SourceFillID, Position: fill.Position, ActualEmitter: fill.ActualEmitter, Provenance: fill.Provenance}
	price := accountingRational{Numerator: "1", Denominator: "1"}
	doc, err := canonicalPrice(price, source, fill)
	if err != nil || doc["numerator"] != "1" || doc["denominator"] != "1" {
		t.Fatalf("committed price: %v, %v", doc, err)
	}
	for _, check := range []struct {
		name   string
		change func(*accountingRational, *accountingPriceSource, *accounting.FillV1)
	}{
		{"changed source", func(_ *accountingRational, s *accountingPriceSource, _ *accounting.FillV1) { s.SourceFillID = "other" }},
		{"oversized numerator", func(p *accountingRational, _ *accountingPriceSource, _ *accounting.FillV1) {
			p.Numerator = strings.Repeat("1", 1025)
		}},
		{"invalid numerator", func(p *accountingRational, _ *accountingPriceSource, _ *accounting.FillV1) { p.Numerator = "1.5" }},
		{"negative numerator", func(p *accountingRational, _ *accountingPriceSource, _ *accounting.FillV1) { p.Numerator = "-1" }},
		{"zero denominator", func(p *accountingRational, _ *accountingPriceSource, _ *accounting.FillV1) { p.Denominator = "0" }},
		{"zero token", func(_ *accountingRational, _ *accountingPriceSource, f *accounting.FillV1) { f.TokenAmountRaw = "0" }},
		{"changed ratio", func(p *accountingRational, _ *accountingPriceSource, _ *accounting.FillV1) { p.Numerator = "2" }},
		{"invalid timestamp", func(_ *accountingRational, _ *accountingPriceSource, f *accounting.FillV1) {
			f.BlockTimestampSeconds = "later"
		}},
		{"future timestamp", func(_ *accountingRational, _ *accountingPriceSource, f *accounting.FillV1) {
			f.BlockTimestampSeconds = "253402300800"
		}},
	} {
		t.Run(check.name, func(t *testing.T) {
			p, s, f := price, source, fill
			check.change(&p, &s, &f)
			if doc, err := canonicalPrice(p, s, f); doc != nil || !errors.Is(err, ErrDataIntegrity) {
				t.Fatalf("changed price: %v, %v", doc, err)
			}
		})
	}
}

func TestAccountingPageRequestKeepsCanonicalBounds(t *testing.T) {
	valid := AccountingPageRequest{Pool: "0x" + strings.Repeat("1", 40), Limit: 10, Interval: "1m", From: 1000, To: 61000}
	for _, check := range []struct {
		name    string
		candles bool
		change  func(*AccountingPageRequest)
	}{
		{"noncanonical pool", false, func(r *AccountingPageRequest) { r.Pool = strings.ToUpper(r.Pool) }},
		{"reversed candle range", true, func(r *AccountingPageRequest) { r.To = r.From }},
		{"unsupported candle interval", true, func(r *AccountingPageRequest) { r.Interval = "2m" }},
		{"snapshot without identity", false, func(r *AccountingPageRequest) { n := int64(1); r.SnapshotSequence = &n }},
		{"negative snapshot", false, func(r *AccountingPageRequest) {
			n := int64(-1)
			r.SnapshotSequence = &n
			r.ExpectedSnapshot = "snapshot"
		}},
		{"unsupported trade side", false, func(r *AccountingPageRequest) { r.Side = "other" }},
		{"unbounded trade page", false, func(r *AccountingPageRequest) { r.Limit = MaxCanonicalTrades + 1 }},
	} {
		t.Run(check.name, func(t *testing.T) {
			r := valid
			check.change(&r)
			if err := validateAccountingRequest(r, check.candles); !errors.Is(err, ErrDataIntegrity) {
				t.Fatalf("invalid accounting request: %+v, %v", r, err)
			}
		})
	}
	if err := validateAccountingRequest(valid, true); err != nil {
		t.Fatalf("bounded candle request: %v", err)
	}
}
