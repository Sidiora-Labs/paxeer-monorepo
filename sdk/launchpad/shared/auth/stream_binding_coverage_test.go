package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestStreamBindingAdmitsBoundTopicFamilies(t *testing.T) {
	principal := StreamPrincipal{AccountID: "6f9b1a52-7c0e-4b8e-9d4a-1f2e3d4c5b6a", SessionID: "0d7c2e41-5a9b-4c3d-8e1f-2a3b4c5d6e7f", Wallet: "0x1dae78b53c9f39930a2ff2b6b7aac12ec75fb154", AuthEpoch: 3}
	market := "0x1111111111111111111111111111111111111111"
	stream := "4a392f21-701a-4b5d-8c9e-33d40c953e14"
	for _, topic := range []string{
		"private.notifications." + principal.Wallet,
		"private.orders." + principal.Wallet,
		"market.rankings.trending",
		"market.price." + market,
		"market.trades." + market,
		"market.stats." + market,
		"market.candles." + market + ".1m",
		"streams." + stream + ".status",
		"bounties." + stream + ".updates",
	} {
		t.Run(topic, func(t *testing.T) {
			binding := StreamTicketBinding{SchemaMajor: 2, Topics: []string{topic}, Origin: "https://app.example", Endpoint: "/v2/stream"}
			binding.FilterDigest = StreamFilterDigest(binding.Topics)
			if err := ValidateStreamBinding(principal, binding); err != nil {
				t.Fatalf("bound stream topic %s: %v", topic, err)
			}
		})
	}
	for _, topic := range []string{
		"private.unknown." + principal.Wallet,
		"private.notifications." + market,
		"market.rankings.unknown",
		"market.price." + strings.ToUpper(market),
		"market.candles." + market + ".2h",
		"streams.unknown.status",
		"market.unknown." + market,
	} {
		t.Run("refuse/"+topic, func(t *testing.T) {
			binding := StreamTicketBinding{SchemaMajor: 2, Topics: []string{topic}, Origin: "https://app.example", Endpoint: "/v2/stream"}
			binding.FilterDigest = StreamFilterDigest(binding.Topics)
			if err := ValidateStreamBinding(principal, binding); !errors.Is(err, ErrScopeDenied) {
				t.Fatalf("unbound topic %s: %v", topic, err)
			}
		})
	}
	duplicates := []string{"market.rankings.trending", "market.rankings.trending"}
	if err := ValidateStreamBinding(principal, StreamTicketBinding{SchemaMajor: 2, Topics: duplicates, FilterDigest: StreamFilterDigest(duplicates), Origin: "https://app.example", Endpoint: "/v2/stream"}); !errors.Is(err, ErrScopeDenied) {
		t.Fatalf("duplicate topics admitted: %v", err)
	}
	bound := StreamTicketBinding{SchemaMajor: 2, Topics: []string{"market.rankings.trending"}, Origin: "https://app.example", Endpoint: "/v2/stream"}
	bound.FilterDigest = StreamFilterDigest(bound.Topics)
	for _, tc := range []struct {
		name string
		edit func(*StreamTicketBinding)
	}{
		{"version", func(b *StreamTicketBinding) { b.SchemaMajor = 1 }},
		{"origin", func(b *StreamTicketBinding) { b.Origin = "http://app.example" }},
		{"digest", func(b *StreamTicketBinding) { b.FilterDigest = strings.Repeat("0", 64) }},
		{"endpoint", func(b *StreamTicketBinding) { b.Endpoint = "/other" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := bound
			tc.edit(&changed)
			if err := ValidateStreamBinding(principal, changed); !errors.Is(err, ErrScopeDenied) {
				t.Fatalf("changed stream binding: %v", err)
			}
		})
	}
}
