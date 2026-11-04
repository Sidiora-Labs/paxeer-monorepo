package ranker

import (
	"slices"
	"testing"
)

func TestRepairM_932_CoordinatorCategoryCount(t *testing.T) {
	want := []string{"trending", "breakout", "volume", "gainers", "losers", "movers", "unusual"}
	if got := HeavyCategoryNames(); !slices.Equal(got, want) {
		t.Fatalf("HeavyCategoryNames() = %v, want %v", got, want)
	}
	t.Run("TestHeavyRunUsesOneFrozenUniverse", TestHeavyRunUsesOneFrozenUniverse)
	t.Run("TestCompletedEmptyRunAndInputImmutability", TestCompletedEmptyRunAndInputImmutability)
}
