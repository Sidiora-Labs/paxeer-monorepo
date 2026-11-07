package discovery

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func TestV2AlertCreateRejectsBodiesOutsideFrozenSchema(t *testing.T) {
	wallet := "0x1234567890abcdef1234567890abcdef12345678"
	chainID, currency, provider := "125", "USD", "market-provider"
	decimals := v2.SafeInteger(18)
	input := v2.AlertCreateBody{
		PoolAddress: wallet, Direction: "crosses_above", CooldownSeconds: 300,
		Threshold: v2.InputPrice{
			Numerator: "100", Denominator: "1",
			BaseAsset:  v2.InputAssetRef{Kind: "erc20", ChainId: &chainID, Address: &wallet, Decimals: &decimals},
			QuoteAsset: v2.InputAssetRef{Kind: "fiat_display", CurrencyCode: &currency},
			Source:     v2.InputPriceSource{Kind: "external_display", AmountBasis: "provider_display", ProviderId: &provider, ObservedAt: "2026-10-26T12:34:56.789Z"},
		},
	}
	valid, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := v2.ValidateJSON("AlertCreateBody", valid); err != nil {
		t.Fatalf("invalid frozen baseline: %v", err)
	}
	handler := (HTTP{
		Store:        Store{},
		Authenticate: func(*http.Request) (string, error) { return "verified-account", nil },
		Wallet:       func(*http.Request) (string, error) { return wallet, nil },
	}).Handler()
	for _, tc := range []struct {
		name  string
		field string
		value any
	}{
		{"unknown field", "unknown", true},
		{"invalid direction", "direction", "sideways"},
		{"cooldown below minimum", "cooldownSeconds", 59},
		{"cooldown above maximum", "cooldownSeconds", 86401},
		{"short pool address", "poolAddress", "0x1234"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body map[string]any
			if err := json.Unmarshal(valid, &body); err != nil {
				t.Fatal(err)
			}
			body[tc.field] = tc.value
			raw, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			if err := v2.ValidateJSON("AlertCreateBody", raw); err == nil {
				t.Fatal("test body unexpectedly satisfies the frozen schema")
			}
			r := httptest.NewRequest(http.MethodPost, "/v2/me/alerts", bytes.NewReader(raw))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("X-Request-Id", discoveryErrorRequestID)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			assertDiscoveryFrozenError(t, w, http.StatusBadRequest, "INVALID_REQUEST", discoveryErrorRequestID)
		})
	}
}
