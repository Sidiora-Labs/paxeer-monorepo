package rest

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func TestCoinTextCodePoints(t *testing.T) {
	for _, tc := range []struct {
		label, name, symbol, wantName, wantSymbol string
	}{
		{"ordinary", "Coin", "COIN", "Coin", "COIN"},
		{"empty", "", "", "", ""},
		{"bounds", strings.Repeat("n", 100), strings.Repeat("s", 16), strings.Repeat("n", 100), strings.Repeat("s", 16)},
		{"long", strings.Repeat("n", 101), strings.Repeat("s", 17), strings.Repeat("n", 100), strings.Repeat("s", 16)},
		{"multibyte", strings.Repeat("界", 99) + "🪙x", strings.Repeat("é", 15) + "🚀x", strings.Repeat("界", 99) + "🪙", strings.Repeat("é", 15) + "🚀"},
		{"combining", strings.Repeat("e\u0301", 51), strings.Repeat("e\u0301", 9), strings.Repeat("e\u0301", 50), strings.Repeat("e\u0301", 8)},
		{"text", " 0x\n", " \t", " 0x\n", " \t"},
	} {
		t.Run(tc.label, func(t *testing.T) {
			name, symbol := coinText(tc.name, tc.symbol)
			if name != tc.wantName || symbol != tc.wantSymbol || !utf8.ValidString(name) || !utf8.ValidString(symbol) {
				t.Fatalf("coin text = %q %q, want %q %q", name, symbol, tc.wantName, tc.wantSymbol)
			}
			row := store.PoolRegistryRow{Token: "0x" + strings.Repeat("1", 40), Metadata: &store.TokenCreationMetadata{Name: tc.name, Symbol: tc.symbol, Decimals: 18, TotalSupply: "1", Position: marketDocumentPosition()}}
			doc := tokenDocument(row)
			if doc["name"] != tc.wantName || doc["symbol"] != tc.wantSymbol || doc["decimalsQuality"] != "verified" || doc["supplyQuality"] != "verified" {
				t.Fatalf("token document: %v", doc)
			}
			raw, err := json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			if err = interfaces.ValidateJSON("Token", raw); err != nil {
				t.Fatalf("token contract: %v", err)
			}
			for _, kind := range []string{"pool", "token"} {
				hit := store.DevelopmentSearchHit{Name: tc.name, Symbol: tc.symbol, Token: row.Token, Pool: row.Token}
				want := tc.wantName
				if want == "" {
					want = tc.wantSymbol
				}
				if want == "" {
					want = row.Token
				}
				if got := devSearchResult(hit, kind)["label"]; got != want {
					t.Fatalf("%s label = %q, want %q", kind, got, want)
				}
			}
		})
	}
}
