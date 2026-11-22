package search

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCoinTextCanonicalProjection(t *testing.T) {
	for _, tc := range []struct {
		name, symbol, displayName, displaySymbol, key, ticker string
		withoutMetadata                                       bool
	}{
		{"猫", "CAT", "猫", "CAT", "猫", "CAT", false},
		{"0xnot-an-address", "HEX", "0xnot-an-address", "HEX", "0xnot-an-address", "HEX", false},
		{strings.Repeat("🌕", 101), "MOON", strings.Repeat("🌕", 100), "MOON", strings.Repeat("🌕", 100), "MOON", false},
		{"Long symbol", strings.Repeat("🌕", 17), "Long symbol", strings.Repeat("🌕", 16), "long symbol", strings.Repeat("🌕", 16), false},
		{"Whitespace symbol", " \t\u2000 ", "Whitespace symbol", " \t\u2000 ", "whitespace symbol", "", false},
		{" \t\u2000 ", "SPACE", " \t\u2000 ", "SPACE", "", "SPACE", false},
		{"", "", "", "", "", "", false},
		{"", "", "", "", "", "", true},
		{"\u202e\u2066", "BIDI", "\u202e\u2066", "BIDI", "", "BIDI", false},
		{strings.Repeat("ﬃ", 101), "FOLD", strings.Repeat("ﬃ", 100), "FOLD", strings.Repeat("ffi", 100), "FOLD", false},
	} {
		t.Run(tc.symbol+"/"+tc.key, func(t *testing.T) {
			coinTextBrokerProjection(t, tc.name, tc.symbol, tc.displayName, tc.displaySymbol, tc.key, tc.ticker, tc.withoutMetadata, 2)
		})
	}
}

func TestCoinTextStoredNameFold(t *testing.T) {
	for _, tc := range []struct{ name, key string }{
		{"猫", "猫"}, {"0xnot-an-address", "0xnot-an-address"}, {"", ""},
		{" \t\u2000", ""}, {"\u202e\u2066", ""}, {"\u00a0ＣＡＦÉ\u2000", "café"},
		{"Straße", "strasse"}, {strings.Repeat("ﬃ", 100), strings.Repeat("ffi", 100)},
	} {
		if got := storedNameKey(tc.name); got != tc.key {
			t.Fatalf("stored name %q: %q, want %q", tc.name, got, tc.key)
		}
	}
	for _, raw := range []string{"猫", "0xnot-an-address", "", " \t\u2000", "\u202e\u2066", strings.Repeat("ﬃ", 100)} {
		if _, err := Normalize(raw); !errors.Is(err, ErrInvalid) {
			t.Fatalf("query %q: %v", raw, err)
		}
	}
	for _, raw := range []string{"\u00a0ＣＡＦÉ\u2000", "Straße", "Café", "\u202eHello\u2066"} {
		query, err := Normalize(raw)
		if err != nil || query.Key != storedNameKey(raw) {
			t.Fatalf("text fold %q: %+v %v", raw, query, err)
		}
	}
	if got := displayPrefix("🌕ab", 3, 5); got != "🌕a" {
		t.Fatalf("byte-bounded display: %q", got)
	}
}

func TestCoinTextEmptyKeysKeepAddressOrder(t *testing.T) {
	pool := integratedDatabase(t)
	for _, number := range []int{3, 1, 2} {
		document := indexedFixture(number)
		document.Name, document.Ticker = "", ""
		if number == 2 {
			document.Name = "Named"
		}
		if number == 3 {
			document.Ticker = "COIN"
		}
		document.NameKey = storedNameKey(document.Name)
		document.CanonicalTicker = canonicalTicker(document.Ticker)
		applyFixture(t, pool, Change{Version: 2, Kind: valueCreation, Revision: 1, PoolAddress: document.PoolAddress, Document: &document})
	}
	store := Store{Pool: pool}
	request := Request{Query: fixtureAddress(99), Kind: "token", Limit: 1}
	integratedRequest := IntegratedRequest{Request: request, Sort: "address"}
	for number := 1; number <= 3; number++ {
		page, err := store.Search(context.Background(), request)
		if err != nil || len(page.Items) != 1 || page.Items[0].ID != "125:"+fixtureAddress(number) || (page.Next == nil) != (number == 3) {
			t.Fatalf("address page %d: %+v %v", number, page, err)
		}
		request.Cursor = page.Next
		integrated, err := store.SearchIntegrated(context.Background(), integratedRequest)
		if err != nil || len(integrated.Items) != 1 || integrated.Items[0].ID != "125:"+fixtureAddress(number) || (integrated.Next == nil) != (number == 3) {
			t.Fatalf("integrated address page %d: %+v %v", number, integrated, err)
		}
		integratedRequest.Position = integrated.Next
	}
	for _, tc := range []struct {
		query string
		pool  int
	}{{"named", 2}, {"coin", 3}} {
		page, err := store.Search(context.Background(), Request{Query: tc.query, Kind: "pool", Limit: 25})
		if err != nil || len(page.Items) != 1 || page.Items[0].ID != "125:"+fixtureAddress(tc.pool) {
			t.Fatalf("text member %q: %+v %v", tc.query, page, err)
		}
	}
}
