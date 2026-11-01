package ranker

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"slices"
	"testing"
)

func TestCoordinatorAndMeasurementCoverEveryPolicyCategory(t *testing.T) {
	var expectedHeavy []string
	for _, category := range Categories {
		if category != "new" {
			expectedHeavy = append(expectedHeavy, category)
		}
	}
	if !slices.Equal(heavyCategories[:], expectedHeavy) {
		t.Fatalf("heavyCategories = %v, want %v", heavyCategories, expectedHeavy)
	}
	if !slices.Equal(HeavyCategoryNames(), expectedHeavy) {
		t.Fatalf("HeavyCategoryNames() = %v, want %v", HeavyCategoryNames(), expectedHeavy)
	}

	in := fixture(60)
	in.Candidates[0].Pool.PriceChange24h = "-25"
	in.Candidates[1].Pool.PriceChange24h = "40"
	snapshots, err := ComputeRun(in, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != len(expectedHeavy) {
		t.Fatalf("heavy run produced %d snapshots, want %d", len(snapshots), len(expectedHeavy))
	}
	byCategory := make(map[string]Snapshot, len(snapshots))
	for i, snapshot := range snapshots {
		if snapshot.Category() != expectedHeavy[i] {
			t.Fatalf("snapshot %d category = %q, want %q", i, snapshot.Category(), expectedHeavy[i])
		}
		if snapshot.RunID() != in.RunID || snapshot.InputDigest() != snapshots[0].InputDigest() {
			t.Fatalf("%s snapshot does not share the frozen run", snapshot.Category())
		}
		byCategory[snapshot.Category()] = snapshot
	}
	trending := byCategory["trending"]
	for _, category := range expectedHeavy {
		var dependency *Snapshot
		if category == "movers" {
			dependency = &trending
		}
		want, err := BuildSnapshot(in, category, dependency)
		if err != nil {
			t.Fatalf("BuildSnapshot(%q): %v", category, err)
		}
		got := byCategory[category]
		if got.InputDigest() != want.InputDigest() || got.OutputDigest() != want.OutputDigest() {
			t.Errorf("%s coordinator output differs from independently built snapshot", category)
		}
	}
	for _, tc := range []struct {
		category string
		address  string
		score    float64
	}{
		{"gainers", in.Candidates[1].Pool.PoolAddress, 40},
		{"losers", in.Candidates[0].Pool.PoolAddress, 25},
	} {
		entries := byCategory[tc.category].Entries()
		if len(entries) != len(in.Candidates) || entries[0].Address != tc.address || entries[0].Score != tc.score {
			t.Errorf("%s ranking = %v, want %d entries led by %s with score %v", tc.category, entries, len(in.Candidates), tc.address, tc.score)
		}
	}
	newOnly, err := ComputeRun(in, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(newOnly) != 1 || newOnly[0].Category() != "new" || len(newOnly[0].Entries()) != len(in.Candidates) {
		t.Fatal("new-only run must produce only the new category")
	}

	files := make(map[string]string)
	for _, name := range []string{"finite.go", "score.go", "snapshot.go"} {
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(raw)
		files["services/launchpad/ranking-algo/internal/ranker/"+name] = hex.EncodeToString(sum[:])
	}
	manifest, formulaDigest, err := CurrentFormulaManifest(files)
	if err != nil {
		t.Fatal(err)
	}
	if formulaDigest != digest(manifest) || manifest.AlgorithmVersion != AlgorithmVersion || manifest.FeatureSchemaVersion != FeatureSchemaVersion {
		t.Fatal("formula manifest identity differs from the current algorithm")
	}
	if len(manifest.Policies) != len(Categories) {
		t.Fatalf("manifest has %d policies, want %d", len(manifest.Policies), len(Categories))
	}
	for i, category := range Categories {
		want, err := Policy(category)
		if err != nil {
			t.Fatal(err)
		}
		if manifest.Policies[i] != want {
			t.Errorf("manifest policy %d = %+v, want %+v", i, manifest.Policies[i], want)
		}
	}
}
