package pnlmath

import (
	"strings"
	"testing"
)

func TestFoldRejectsMalformedStoredAmounts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Position)
	}{
		{"spent", func(p *Position) { p.TotalUsdlSpent = "invalid" }}, {"bought", func(p *Position) { p.TotalTokensBought = "invalid" }},
		{"received", func(p *Position) { p.TotalUsdlReceived = "invalid" }}, {"sold", func(p *Position) { p.TotalTokensSold = "invalid" }},
		{"average", func(p *Position) { p.AvgCostBasis = "invalid" }}, {"realized", func(p *Position) { p.RealizedPnlUsdl = "invalid" }},
		{"holdings", func(p *Position) { p.CurrentHoldings = "invalid" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var prior Position
			tc.change(&prior)
			if _, err := Fold(prior, Trade{IsBuy: true, UsdlAmount: "10", TokenAmount: "1"}); err == nil || !strings.Contains(err.Error(), "noncanonical") {
				t.Fatal(err)
			}
		})
	}
	if _, err := Fold(Position{}, Trade{IsBuy: true, UsdlAmount: "10", TokenAmount: "bad"}); err == nil {
		t.Fatal("invalid quantity accepted")
	}
}

func TestValuationRejectsMalformedStoredInputs(t *testing.T) {
	if _, err := HoldingValue("1", "invalid"); err == nil {
		t.Fatal("invalid holdings valued")
	}
	for _, values := range [][4]string{{"0", "invalid", "1", "1"}, {"0", "1", "invalid", "1"}, {"0", "1", "1", "invalid"}} {
		if _, err := TotalPnl(values[0], values[1], values[2], values[3]); err == nil {
			t.Fatal(values)
		}
	}
}
