package l17

import (
	"bytes"
	"encoding/json"
	"image/png"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/ogrender"
)

func TestSelectedShareRenderingValidatesEveryDisplayedValue(t *testing.T) {
	fontDir := filepath.Join("..", "..", "ogrender", "fonts")
	renderer, err := ogrender.NewStrict(fontDir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	share := PublicShareV3{Version: 3, Fields: []string{fieldKnownRealizedSubtotalUsdlRaw, fieldRealizedPnlRaw, fieldCompleteValueUsdlRaw, fieldHistoricalReturn}, Values: ShareValuesV3{KnownRealizedSubtotalUsdlRaw: json.RawMessage(`"-1500000"`), RealizedPnlRaw: json.RawMessage(`null`), CompleteValueUsdlRaw: json.RawMessage(`"2000000"`), HistoricalReturn: json.RawMessage(`{"method":"time_weighted_boundary_cash_flows_v1","return":{"numerator":"1","denominator":"5"},"periods":1}`)}, RangeStart: now.Add(-time.Hour), RangeEnd: now, ExpiresAt: now.Add(time.Hour), Watermark: fixture().SourceWatermark, BasisQuality: "partial", PricingQuality: "priced", Methodology: Algorithm}
	data, err := renderSelectedV3(renderer, share, fontDir)
	config, decodeErr := png.DecodeConfig(bytes.NewReader(data))
	if err != nil || decodeErr != nil || config.Width != 1200 || config.Height != 630 {
		t.Fatal(config, err, decodeErr)
	}
	for _, tc := range []struct {
		name   string
		change func(*PublicShareV3)
		fonts  string
	}{
		{name: "amount type", change: func(s *PublicShareV3) { s.Values.KnownRealizedSubtotalUsdlRaw = json.RawMessage(`{}`) }},
		{name: "amount grammar", change: func(s *PublicShareV3) { s.Values.KnownRealizedSubtotalUsdlRaw = json.RawMessage(`"bad"`) }},
		{name: "return grammar", change: func(s *PublicShareV3) { s.Values.HistoricalReturn = json.RawMessage(`{`) }},
		{name: "unknown field", change: func(s *PublicShareV3) { s.Fields = []string{"secret"} }},
		{name: "empty selection", change: func(s *PublicShareV3) { s.Fields = nil }},
		{name: "missing fonts", fonts: t.TempDir()},
		{name: "oversized amount", change: func(s *PublicShareV3) {
			s.Values.KnownRealizedSubtotalUsdlRaw = json.RawMessage(`"` + strings.Repeat("9", 156) + `"`)
		}},
		{name: "oversized basis label", change: func(s *PublicShareV3) { s.BasisQuality = strings.Repeat("W", 1000) }},
		{name: "oversized identity", change: func(s *PublicShareV3) { s.Wallet = ptr(strings.Repeat("W", 1000)) }},
		{name: "oversized footer", change: func(s *PublicShareV3) { s.Methodology = strings.Repeat("W", 1000) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := share
			if tc.change != nil {
				tc.change(&bad)
			}
			directory := fontDir
			if tc.fonts != "" {
				directory = tc.fonts
			}
			if _, err := renderSelectedV3(renderer, bad, directory); err == nil {
				t.Fatal("invalid selected rendering accepted")
			}
		})
	}
	for _, tc := range []struct {
		raw  json.RawMessage
		want string
	}{
		{nil, "Unavailable"}, {json.RawMessage(`null`), "Unavailable"}, {json.RawMessage(`"-1500000"`), "-1.500000 USDL"}, {json.RawMessage(`"0"`), "0.000000 USDL"},
	} {
		actual, err := formatShareAmountV3(tc.raw)
		if err != nil || actual != tc.want {
			t.Fatal(actual, err)
		}
	}
}
