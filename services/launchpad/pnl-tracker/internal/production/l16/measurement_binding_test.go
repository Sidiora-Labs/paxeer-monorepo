package l16

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInventoryMeasurementSourcesExistAtRepositoryRoot(t *testing.T) {
	root, err := filepath.Abs("../../../../../..")
	if err != nil {
		t.Fatal(err)
	}
	if inventoryFrozenAuthority != "sdk/launchpad/packages/sdk/src/operations.gen.ts" {
		t.Fatalf("unexpected frozen inventory authority: %q", inventoryFrozenAuthority)
	}
	sources := requiredInventorySources()
	if len(sources) != 6 {
		t.Fatalf("expected five l16 sources and the frozen authority, got %d sources", len(sources))
	}
	seen := make(map[string]bool, len(sources))
	for _, source := range sources {
		if seen[source] {
			t.Fatalf("duplicate inventory measurement source: %q", source)
		}
		seen[source] = true
		info, err := os.Lstat(filepath.Join(root, source))
		if err != nil {
			t.Fatalf("inventory measurement source %q at repository root %q: %v", source, root, err)
		}
		if !info.Mode().IsRegular() {
			t.Fatalf("inventory measurement source %q is not a regular file", source)
		}
	}
	if !seen[inventoryFrozenAuthority] {
		t.Fatal("inventory measurement sources do not bind the frozen authority")
	}
}
