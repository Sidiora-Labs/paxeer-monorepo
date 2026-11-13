package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func TestValuationHistoryChecksCandleBoundsAndSupplyAvailability(t *testing.T) {
	frame, err := interfaces.DecodeCandleEvent(repairM1050CandleFrames(t)[0])
	if err != nil {
		t.Fatal(err)
	}
	bar := frame.Payload
	page := CanonicalCandlePage{Items: []interfaces.Candle{bar}, SnapshotID: bar.SnapshotId, Algorithm: bar.SeriesModelVersion, Complete: true}
	for _, tc := range []struct {
		name, mode, want string
		change           func(*CanonicalCandlePage)
	}{
		{"unknown mode", "unknown", "invalid valuation mode", func(*CanonicalCandlePage) {}},
		{"unparseable time", modePrice, "cannot parse", func(p *CanonicalCandlePage) { p.Items[0].Start = "unknown" }},
		{"negative time", modePrice, "invalid valuation candle order", func(p *CanonicalCandlePage) { p.Items[0].Start = "1969-12-31T23:59:00.000Z" }},
		{"invalid price", modePrice, "invalid price numerator", func(p *CanonicalCandlePage) { p.Items[0].Open.Numerator = "invalid" }},
		{"invalid volume", modePrice, "invalid", func(p *CanonicalCandlePage) { p.Items[0].VolumeUsdlRaw = "invalid" }},
		{"price underflow", modePrice, "vendor price underflow", func(p *CanonicalCandlePage) {
			p.Items[0].Open.Numerator, p.Items[0].Open.Denominator = "1", interfaces.PositivePriceComponent("1"+strings.Repeat("0", 400))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := page
			candidate.Items = append([]interfaces.Candle(nil), page.Items...)
			tc.change(&candidate)
			wire, err := valuationHistory(context.Background(), candidate, nil, tc.mode)
			if err == nil || !strings.Contains(err.Error(), tc.want) || len(wire) != 0 {
				t.Fatalf("valuation bound: %s, %v", wire, err)
			}
		})
	}
	second := bar
	second.Start = bar.Start
	duplicate := page
	duplicate.Items = []interfaces.Candle{bar, second}
	if wire, err := valuationHistory(context.Background(), duplicate, nil, modePrice); err == nil || !strings.Contains(err.Error(), "invalid valuation candle order") || len(wire) != 0 {
		t.Fatalf("duplicate candle time: %s, %v", wire, err)
	}
	tooMany := page
	tooMany.Items = make([]interfaces.Candle, MaxHistoryBars+1)
	if wire, err := valuationHistory(context.Background(), tooMany, nil, modePrice); err == nil || !strings.Contains(err.Error(), "unbounded valuation history") || len(wire) != 0 {
		t.Fatalf("bounded canonical history: %s, %v", wire, err)
	}
	deadline, stopDeadline := context.WithDeadline(context.Background(), time.Now().Add(500*time.Millisecond))
	defer stopDeadline()
	if wire, err := valuationHistory(deadline, page, nil, modePrice); err != nil || !strings.Contains(string(wire), `"s":"ok"`) {
		t.Fatalf("bounded price response: %s, %v", wire, err)
	}
	price := bar.Open
	provenance := SupplySnapshot{Token: *price.BaseAsset.Address, Decimals: int(*price.BaseAsset.Decimals), SourceBlock: string(price.Source.Position.Height), SourceHash: price.Source.Position.BlockHash, Alignment: "canonical_block_end", TotalSupplyRaw: "1000000"}
	if value, err := totalSupplyValuation(price, provenance); err != nil || value.Sign() <= 0 {
		t.Fatalf("canonical supply valuation: %v, %v", value, err)
	}
	for _, tc := range []struct {
		name, amount, want string
	}{
		{"unparseable supply", "invalid", "invalid total supply"},
		{"supply exceeds uint256", strings.Repeat("9", 78), "invalid total supply"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := provenance
			candidate.TotalSupplyRaw = tc.amount
			if value, err := totalSupplyValuation(price, candidate); value != nil || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("canonical supply bound: %v, %v", value, err)
			}
		})
	}
	invalidPrice := price
	invalidPrice.Numerator = "invalid"
	if value, err := totalSupplyValuation(invalidPrice, provenance); value != nil || err == nil || !strings.Contains(err.Error(), "invalid price numerator") {
		t.Fatalf("canonical price bound: %v, %v", value, err)
	}
	if value, err := vendorRational(new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Exp(big.NewInt(10), big.NewInt(400), nil))); value != 0 || err == nil || !strings.Contains(err.Error(), "vendor price underflow") {
		t.Fatalf("vendor precision bound: %v, %v", value, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if wire, err := valuationHistory(ctx, page, nil, modePrice); !errors.Is(err, context.Canceled) || len(wire) != 0 {
		t.Fatalf("cancelled valuation: %s, %v", wire, err)
	}
	wire, err := valuationHistory(context.Background(), page, nil, modeTotalSupplyValuation)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Status    string `json:"s"`
		Available bool   `json:"valuationAvailable"`
		Reason    []struct {
			Reason string `json:"reason"`
		} `json:"valuations"`
	}
	if err := json.Unmarshal(wire, &result); err != nil || result.Status != statusUnavailable || result.Available || len(result.Reason) != 1 || result.Reason[0].Reason != "historical_supply_unavailable" {
		t.Fatalf("unavailable historical supply: %+v, %v", result, err)
	}
	empty := page
	empty.Items = nil
	wire, err = valuationHistory(context.Background(), empty, nil, modePrice)
	if err != nil || !strings.Contains(string(wire), `"s":"`+statusNoData+`"`) {
		t.Fatalf("empty history: %s, %v", wire, err)
	}
}
