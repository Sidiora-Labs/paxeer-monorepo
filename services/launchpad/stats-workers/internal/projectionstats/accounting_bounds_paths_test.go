package projectionstats

import (
	"errors"
	"testing"
)

func TestAccountingRejectsIncompleteAmountEvidence(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*Trade)
	}{
		{"recipient attribution", func(t *Trade) { t.RecipientAttribution = "exact_receipt" }},
		{"evidence collection", func(t *Trade) { t.RecipientEvidenceRefs = nil }},
		{"empty evidence", func(t *Trade) { t.RecipientEvidenceRefs = []string{""} }},
		{"paid input absent", func(t *Trade) { t.PaidInputRaw = nil }},
		{"paid input malformed", func(t *Trade) { t.PaidInputRaw = ptr("01") }},
		{"fee malformed", func(t *Trade) { t.FeeRaw = "invalid" }},
		{"output malformed", func(t *Trade) { t.TokenAmountRaw = "invalid" }},
		{"side", func(t *Trade) { t.Side = "other" }},
		{"unpriced net", func(t *Trade) { t.PricedInputRaw = nil }},
		{"priced quality", func(t *Trade) { t.InputDerivationQuality = "receipt_incomplete" }},
		{"priced integer", func(t *Trade) { t.PricedInputRaw = ptr("invalid") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			trade := fixture("buy", "1")
			tc.edit(&trade)
			if value, err := Account(trade, PoolAssets{usdl, token, 18}, 30); !errors.Is(err, ErrInvalid) || value.TradedUSDLRaw != "" {
				t.Fatalf("invalid evidence accepted: %+v %v", value, err)
			}
		})
	}
	if _, err := Account(fixture("buy", "1"), PoolAssets{usdl, token, 18}, -1); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	p := fixture("buy", "1").Position
	p.ChainID = "1"
	if err := p.Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	p = fixture("buy", "1").Position
	p.Height = "invalid"
	if err := p.Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := Minute(-1); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if err := ValidateTime(-1, 1, 0); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := NormalizeAddress("invalid"); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}

func TestPositionComparisonRequiresValidatedIntegers(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("invalid comparison did not reject unvalidated position")
		}
	}()
	invalid := fixture("buy", "1").Position
	invalid.Height = "invalid"
	invalid.Compare(fixture("buy", "2").Position)
}
