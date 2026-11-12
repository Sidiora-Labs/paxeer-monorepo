package store

import (
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"
)

func TestPublicStatsArithmeticAndSnapshotValidation(t *testing.T) {
	const pool = "0x1111111111111111111111111111111111111111"
	const hash = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	watermark := ReadWatermark{ChainID: "125", Height: "42", BlockHash: hash, ProjectionRevision: "2", ObservedAt: time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC), Integrity: "healthy", AppliedEvent: "2"}
	base := map[string]any{
		"poolAddress": pool, "realUsdlBalanceRaw": "1000000", "buys24h": "2", "sells24h": "1", "holderCount": "4",
		"txCount24h": "3", "uniqueTraders24h": "2", "totalSupplyRaw": "100", "circulatingSupplyRaw": "90",
		"windowStart": "2026-10-08T00:00:00.000Z", "windowEnd": "2026-10-09T00:00:00.000Z",
		"sourceUpperPosition": map[string]any{"chainId": "125", "height": "42", "blockHash": hash}, "watermark": watermark,
	}
	build := func(values map[string]any) PublicProjection {
		t.Helper()
		payload, err := json.Marshal(values)
		if err != nil {
			t.Fatal(err)
		}
		return PublicProjection{Topic: "market.stats." + pool, Revision: "2", Watermark: json.RawMessage(`null`), Payload: payload}
	}
	if err := validatePublicStats(build(base)); err != nil {
		t.Fatalf("valid stats: %v", err)
	}
	for _, tc := range []struct {
		name, key string
		value     any
	}{
		{"negative balance", "realUsdlBalanceRaw", "-1"},
		{"invalid buy count", "buys24h", "01"},
		{"invalid sell count", "sells24h", "-1"},
		{"inconsistent fills", "txCount24h", "4"},
		{"invalid fill count", "txCount24h", "invalid"},
		{"more traders than fills", "uniqueTraders24h", "4"},
		{"invalid trader count", "uniqueTraders24h", "invalid"},
		{"circulation exceeds supply", "circulatingSupplyRaw", "101"},
		{"invalid supply", "totalSupplyRaw", "invalid"},
		{"invalid circulation", "circulatingSupplyRaw", "invalid"},
		{"invalid holders", "holderCount", "01"},
		{"invalid start", "windowStart", "invalid"},
		{"invalid end", "windowEnd", "invalid"},
		{"short window", "windowEnd", "2026-10-08T23:59:59.000Z"},
		{"invalid chain", "sourceUpperPosition", map[string]any{"chainId": "1", "height": "42", "blockHash": hash}},
		{"invalid upper height", "sourceUpperPosition", map[string]any{"chainId": "125", "height": "41", "blockHash": hash}},
		{"invalid upper hash", "sourceUpperPosition", map[string]any{"chainId": "125", "height": "42", "blockHash": "0x" + strings.Repeat("b", 64)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			values := make(map[string]any, len(base))
			for k, v := range base {
				values[k] = v
			}
			values[tc.key] = tc.value
			if err := validatePublicStats(build(values)); !errors.Is(err, ErrDataIntegrity) {
				t.Fatalf("invalid stats accepted: %v", err)
			}
		})
	}
	for _, tc := range []struct {
		topic string
		want  error
	}{
		{"invalid", ErrDataIntegrity},
		{"market.unknown." + pool, ErrDataIntegrity},
	} {
		p := build(base)
		p.Topic = tc.topic
		wire, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		if err = validateSnapshotFrame(p, wire); !errors.Is(err, tc.want) {
			t.Fatalf("topic %s accepted: %v", tc.topic, err)
		}
	}
}

func TestHolderRewardPoolOrderingAndPublicDocument(t *testing.T) {
	watermark := ChainWatermark{ChainID: "125", Height: "42", BlockHash: "0x" + strings.Repeat("a", 64), ProjectionRevision: "42", Integrity: "healthy"}
	rows := []HolderRewardPoolRow{
		{Pool: "0x" + strings.Repeat("1", 40), Token: "0x" + strings.Repeat("4", 40), DistributedRaw: "9", HoldersPaid: 2, CreatedAt: 100, MarketCap: big.NewRat(10, 1)},
		{Pool: "0x" + strings.Repeat("2", 40), Token: "0x" + strings.Repeat("5", 40), DistributedRaw: "10", HoldersPaid: 4, CreatedAt: 200, MarketCap: big.NewRat(20, 1)},
		{Pool: "0x" + strings.Repeat("3", 40), Token: "0x" + strings.Repeat("6", 40), DistributedRaw: "2", HoldersPaid: 8, CreatedAt: 300},
	}
	for _, tc := range []struct{ sort, first string }{
		{"rewards", rows[1].Pool}, {"holders", rows[2].Pool}, {"age", rows[2].Pool}, {"mcap", rows[1].Pool},
	} {
		copyRows := append([]HolderRewardPoolRow(nil), rows...)
		ordered := orderHolderRewardPools(tc.sort, copyRows, watermark, "complete")
		if len(ordered.Items) != 3 || ordered.Items[0].Pool != tc.first || ordered.SnapshotID == "" || ordered.Sort != tc.sort {
			t.Fatalf("%s order: %+v", tc.sort, ordered)
		}
		doc := ordered.Items[0].Public()
		if doc["poolAddress"] != tc.first || doc["distributedRaw"] != string(ordered.Items[0].DistributedRaw) {
			t.Fatalf("public reward row: %v", doc)
		}
	}
	if uint256Compare("9", "10") >= 0 || uint256Compare("10", "9") <= 0 || uint256Compare("10", "10") != 0 {
		t.Fatal("uint256 reward ordering")
	}
}

func TestSwapFeeRateIsUniqueForQualifyingInput(t *testing.T) {
	for _, tc := range []struct {
		input, fee string
		want       int
		valid      bool
	}{
		{"100000", "300", 30, true},
		{"10001", "3", 3, true},
		{"10000", "0", 0, true},
		{"9999", "3", 0, false},
		{"10000", "-1", 0, false},
		{"10000", "10001", 0, false},
		{"invalid", "3", 0, false},
	} {
		got, ok := FeeBpsFromSwap(tc.input, tc.fee)
		if ok != tc.valid || got != tc.want {
			t.Fatalf("input %s fee %s: rate %d valid %t", tc.input, tc.fee, got, ok)
		}
	}
}
