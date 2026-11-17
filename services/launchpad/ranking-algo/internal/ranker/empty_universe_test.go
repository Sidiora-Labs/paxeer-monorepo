package ranker

import "testing"

func TestEmptyUniverseBothCategoryRuns(t *testing.T) {
	in := fixture(0)
	seen := map[string]bool{}
	for _, newOnly := range []bool{false, true} {
		snapshots, err := ComputeRun(in, newOnly)
		if err != nil {
			t.Fatalf("empty run newOnly=%v: %v", newOnly, err)
		}
		expected := len(HeavyCategoryNames())
		if newOnly {
			expected = 1
		}
		if len(snapshots) != expected {
			t.Fatalf("empty run newOnly=%v has %d categories, want %d", newOnly, len(snapshots), expected)
		}
		for _, snapshot := range snapshots {
			if seen[snapshot.Category()] {
				t.Fatalf("duplicate empty category %s", snapshot.Category())
			}
			seen[snapshot.Category()] = true
			if snapshot.Entries() == nil || len(snapshot.Entries()) != 0 || len(snapshot.Exclusions()) != 0 || snapshot.RunID() != in.RunID || len(snapshot.InputDigest()) != 64 || len(snapshot.OutputDigest()) != 64 {
				t.Fatalf("empty category %s has incomplete snapshot", snapshot.Category())
			}
		}
	}
	for _, category := range Categories {
		if !seen[category] {
			t.Fatalf("empty category %s missing", category)
		}
	}
	in.Complete = false
	for _, newOnly := range []bool{false, true} {
		if _, err := ComputeRun(in, newOnly); err == nil {
			t.Fatalf("incomplete empty run accepted: newOnly=%v", newOnly)
		}
	}
}
