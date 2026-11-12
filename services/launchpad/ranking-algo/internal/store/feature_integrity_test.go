package store

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/ranker"
)

func TestCanonicalFeatureRowsRejectIncoherentAmounts(t *testing.T) {
	base := universeRow{pool: poolA, creationHeight: 1, day: "0", hour: "0", five: "0", roundTrip: "0", boughtTokens: "0", soldTokens: "0", reserveIn: "0", reserveOut: "0"}
	if _, err := base.candidate(100000); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*universeRow)
	}{
		{"day", func(row *universeRow) { row.day = "bad" }},
		{"hour", func(row *universeRow) { row.hour = "bad" }},
		{"five", func(row *universeRow) { row.five = "bad" }},
		{"round trip", func(row *universeRow) { row.roundTrip = "-1" }},
		{"bought", func(row *universeRow) { row.boughtTokens = "bad" }},
		{"sold", func(row *universeRow) { row.soldTokens = "bad" }},
		{"reserve input", func(row *universeRow) { row.reserveIn = "bad" }},
		{"reserve output", func(row *universeRow) { row.reserveOut = "bad" }},
		{"negative balance", func(row *universeRow) { row.reserveOut = "1" }},
		{"negative supply", func(row *universeRow) { row.soldTokens = "1" }},
		{"negative holders", func(row *universeRow) { row.holders = -1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := base
			tc.change(&row)
			if _, err := row.candidate(100000); err == nil {
				t.Fatal("incoherent feature row accepted")
			}
		})
	}
	if _, err := rawInteger(strings.Repeat("9", 157)); !errors.Is(err, errFeatureIntegrity) {
		t.Fatalf("oversized raw integer: %v", err)
	}
	negative := "-1"
	positive := "1"
	decimals := 6
	if _, err := executionPrice(&negative, &positive, &decimals); !errors.Is(err, errFeatureIntegrity) {
		t.Fatalf("negative price amount: %v", err)
	}
	decimals = 78
	if _, err := executionPrice(&positive, &positive, &decimals); !errors.Is(err, errFeatureIntegrity) {
		t.Fatalf("out-of-range token decimals: %v", err)
	}
}

func TestRankingStoreConstructorsRequireBoundDependencies(t *testing.T) {
	if _, err := NewSnapshotStore(nil, "ranking"); err == nil {
		t.Fatal("snapshot store without a database accepted")
	}
	if _, err := NewFrozenInputs(nil, nil); !errors.Is(err, ranker.ErrUnavailable) {
		t.Fatalf("frozen inputs without a database accepted: %v", err)
	}
	if _, err := NewCanonicalInputReader("invalid-schema", "stats", "stream", "version", nil); !errors.Is(err, ranker.ErrUnavailable) {
		t.Fatalf("unbound ranking schema accepted: %v", err)
	}
	if _, err := NewHTTPRankingVisibility("file:///visibility", http.DefaultClient); !errors.Is(err, ranker.ErrUnavailable) {
		t.Fatalf("non-HTTP visibility accepted: %v", err)
	}
}
