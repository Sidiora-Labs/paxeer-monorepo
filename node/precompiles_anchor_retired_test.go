package app

import "testing"

func TestPrecompileKeepersAnchorKReturnsNil(t *testing.T) {
	a := Setup(t, false, false, false)
	t.Cleanup(func() {
		if err := a.Close(); err != nil {
			t.Fatal(err)
		}
	})

	keepers := NewPrecompileKeepers(a)
	if got := keepers.AnchorK(); got != nil {
		t.Fatalf("AnchorK() = %v, want nil after anchor retirement", got)
	}
}
