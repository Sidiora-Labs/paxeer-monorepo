package card_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image/png"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/card"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/ogrender"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/pnlmath"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/store"
)

func repair980Snapshot(t *testing.T, spent, bought string) card.Snapshot {
	t.Helper()
	p, err := pnlmath.Fold(pnlmath.Position{}, pnlmath.Trade{
		IsBuy: true, UsdlAmount: spent, TokenAmount: bought, Ts: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	price := "2500000"
	return card.Snapshot{
		Version: 1, OwnerAddress: "0xowner", PoolAddress: "0xpool", TokenAddress: "0xtoken",
		TokenName: "Archive", TokenSymbol: "OLD", CapturedAt: 1234,
		BasisQuality: "complete", PricingQuality: "priced",
		Position: card.Position{
			TotalUsdlSpent: p.TotalUsdlSpent, TotalTokensBought: p.TotalTokensBought,
			TotalUsdlReceived: p.TotalUsdlReceived, TotalTokensSold: p.TotalTokensSold,
			AvgCostBasis: p.AvgCostBasis, CurrentHoldings: p.CurrentHoldings,
			RealizedPnlUsdl: p.RealizedPnlUsdl, FirstBuyTs: p.FirstBuyTs,
			LastTradeTs: p.LastTradeTs, TradeCount: p.TradeCount,
		},
		Market: card.Market{PriceWad: &price},
	}
}

// Drive the archive codec used by Get, the production HTTP JSON encoder over
// loopback TCP, the JSON consumer, and the production render-input and PNG path.
func repair980Render(t *testing.T, snapshot card.Snapshot) (ogrender.Input, []byte) {
	t.Helper()
	payload, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	row := store.CardRow{CardID: "archived980", ShortCode: "old980", Snapshot: payload, CreatedAt: 1234}
	svc := card.New(store.New(nil), "", "https://og.example", nil)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		archived, err := svc.DecodeArchive(row)
		if err != nil {
			t.Errorf("production archive decode: %v", err)
			http.Error(w, "decode failed", http.StatusInternalServerError)
			return
		}
		sharedhttp.WriteJSON(w, http.StatusOK, archived)
	}))
	defer server.Close()
	response, err := server.Client().Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("archive response: %s", response.Status)
	}
	var transported card.Minted
	if err := json.NewDecoder(response.Body).Decode(&transported); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(transported.Snapshot, snapshot) || !bytes.Equal(row.Snapshot, payload) {
		t.Fatalf("archive evidence changed in transport: %+v", transported.Snapshot)
	}
	if transported.ShareURL != "" {
		t.Fatalf("archive issued a publication: %+v", transported)
	}
	input := card.BuildRenderInput(transported.Snapshot, "Archived disclosure")
	if input.Title != "Archive" || input.Footer != "Archived disclosure" {
		t.Fatalf("render context lost: %+v", input)
	}
	imageBytes, err := ogrender.New("").Render(input)
	if err != nil {
		t.Fatalf("render archive: %v", err)
	}
	image, err := png.Decode(bytes.NewReader(imageBytes))
	if err != nil || image.Bounds().Dx() != 1200 || image.Bounds().Dy() != 630 {
		t.Fatalf("invalid rendered PNG: %v", err)
	}
	return input, imageBytes
}

func TestRepairM_980_MissingAndMalformedMarks(t *testing.T) {
	valid := repair980Snapshot(t, "2000000", oneToken)
	_, pricedPNG := repair980Render(t, valid)
	cases := []struct {
		name string
		mark *string
	}{
		{name: "missing"},
	}
	for _, raw := range []string{"", "invalid", "1.25", "-1", "+1", "01", " 1", "1e6", strings.Repeat("9", 158)} {
		mark := raw
		cases = append(cases, struct {
			name string
			mark *string
		}{name: "malformed_" + raw, mark: &mark})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := valid
			snapshot.Market.PriceWad = tc.mark
			input, rendered := repair980Render(t, snapshot)
			if input.Pnl != "Unavailable" || input.Multiple != "Unavailable" || input.PnlPositive {
				t.Fatalf("unavailable mark became a numeric result: %+v", input)
			}
			if input.Holdings != "1.00 OLD held" || bytes.Equal(rendered, pricedPNG) {
				t.Fatalf("unavailable render lost holdings or matched priced PNG: %+v", input)
			}
		})
	}
}

func TestRepairM_980_BasisAndPricingQuality(t *testing.T) {
	for _, field := range []string{"basis", "pricing"} {
		for _, quality := range []string{"unknown", "partial", "unavailable", "stale", "invalid"} {
			t.Run(field+"_"+quality, func(t *testing.T) {
				snapshot := repair980Snapshot(t, "2000000", oneToken)
				if field == "basis" {
					snapshot.BasisQuality = quality
				} else {
					snapshot.PricingQuality = quality
				}
				input, _ := repair980Render(t, snapshot)
				if input.Pnl != "Unavailable" || input.Multiple != "Unavailable" || input.PnlPositive {
					t.Fatalf("quality %s/%s became verified performance: %+v", field, quality, input)
				}
			})
		}
	}
}

func TestRepairM_980_MissingAndMalformedPositionEvidence(t *testing.T) {
	for _, field := range []string{"basis", "realized", "holdings", "spent", "received"} {
		values := []string{"", "malformed", "+1", "01", strings.Repeat("9", 158)}
		if field != "realized" {
			values = append(values, "-1")
		}
		for _, raw := range values {
			t.Run(field+"_"+raw, func(t *testing.T) {
				snapshot := repair980Snapshot(t, "2000000", oneToken)
				switch field {
				case "basis":
					snapshot.Position.AvgCostBasis = raw
				case "realized":
					snapshot.Position.RealizedPnlUsdl = raw
				case "holdings":
					snapshot.Position.CurrentHoldings = raw
				case "spent":
					snapshot.Position.TotalUsdlSpent = raw
				case "received":
					snapshot.Position.TotalUsdlReceived = raw
				}
				input, _ := repair980Render(t, snapshot)
				if field == "basis" || field == "realized" || field == "holdings" {
					if input.Pnl != "Unavailable" || input.PnlPositive {
						t.Fatalf("invalid PnL evidence became numeric: %+v", input)
					}
				}
				if field == "spent" || field == "received" || field == "holdings" {
					if input.Multiple != "Unavailable" {
						t.Fatalf("invalid multiple evidence became numeric: %+v", input)
					}
				}
				if field == "holdings" && input.Holdings != "Holdings unavailable" {
					t.Fatalf("invalid holdings became a numeric balance: %+v", input)
				}
			})
		}
	}
	zeroSpent := repair980Snapshot(t, "0", oneToken)
	input, _ := repair980Render(t, zeroSpent)
	if input.Multiple != "Unavailable" {
		t.Fatalf("undefined return ratio became zero: %+v", input)
	}
}

func TestRepairM_980_ExactValuesAndClosedPosition(t *testing.T) {
	for _, tc := range []struct {
		name, spent, mark, pnl, multiple string
		positive                         bool
	}{
		{"gain", "2000000", "2500000", "+$0.50", "1.25x", true},
		{"loss", "2000000", "500000", "-$1.50", "0.25x", false},
		{"explicit_zero_mark", "2000000", "0", "-$2.00", "0.00x", false},
		{"exact_zero_pnl", "2000000", "2000000", "+$0.00", "1.00x", true},
		{"large_exact", "900719925474099312345678", "900719925474099312355678", "+$0.01", "1.00x", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := repair980Snapshot(t, tc.spent, oneToken)
			snapshot.Market.PriceWad = &tc.mark
			// Legacy snapshots without quality extensions still require all numeric evidence.
			snapshot.BasisQuality, snapshot.PricingQuality = "", ""
			input, _ := repair980Render(t, snapshot)
			if input.Pnl != tc.pnl || input.Multiple != tc.multiple || input.PnlPositive != tc.positive {
				t.Fatalf("exact valuation = %+v; want %s, %s, positive=%v", input, tc.pnl, tc.multiple, tc.positive)
			}
		})
	}
	p, err := pnlmath.Fold(pnlmath.Position{}, pnlmath.Trade{IsBuy: true, UsdlAmount: "10000000", TokenAmount: oneToken, Ts: 100})
	if err != nil {
		t.Fatal(err)
	}
	p, err = pnlmath.Fold(p, pnlmath.Trade{IsBuy: false, UsdlAmount: "4000000", TokenAmount: oneToken, Ts: 200})
	if err != nil {
		t.Fatal(err)
	}
	snapshot := repair980Snapshot(t, "10000000", oneToken)
	snapshot.Position.CurrentHoldings = p.CurrentHoldings
	snapshot.Position.AvgCostBasis = p.AvgCostBasis
	snapshot.Position.RealizedPnlUsdl = p.RealizedPnlUsdl
	snapshot.Position.TotalUsdlReceived = p.TotalUsdlReceived
	snapshot.Position.TotalTokensSold = p.TotalTokensSold
	snapshot.Position.TradeCount = p.TradeCount
	snapshot.Position.LastTradeTs = p.LastTradeTs
	snapshot.Market.PriceWad = nil
	input, _ := repair980Render(t, snapshot)
	if input.Pnl != "-$6.00" || input.PnlPositive || input.Multiple != "0.40x" || input.Holdings != "0.00 OLD held" {
		t.Fatalf("closed position required an irrelevant mark or lost realized loss: %+v", input)
	}
}

func TestRepairM_980_MintRemainsRetired(t *testing.T) {
	svc := card.New(store.New(nil), "", "", nil)
	minted, err := svc.Mint(context.Background(), "0xowner", "0xpool")
	if minted != nil || !errors.Is(err, card.ErrMintRetired) {
		t.Fatalf("mint created a disclosure without evidence: %+v, %v", minted, err)
	}
}
