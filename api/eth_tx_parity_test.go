package api_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
)

func TestEthTxDescriptorParity(t *testing.T) {
	representatives := map[string]interface{ Descriptor() ([]byte, []int) }{
		"eth/tx.proto": &ethtx.AccessTuple{},
	}
	entries, err := os.ReadDir("../api/eth")
	if err != nil {
		t.Fatalf("read eth proto sources: %v", err)
	}
	seen := make(map[string]bool)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".proto" {
			continue
		}
		name := "eth/" + entry.Name()
		seen[name] = true
		representative, ok := representatives[name]
		if !ok {
			t.Errorf("%s: proto source has no representative generated type", name)
			continue
		}
		t.Run(name, func(t *testing.T) {
			source := bufFile(t, name)
			generated := generatedFile(t, representative)
			if generated.GetName() != name {
				t.Errorf("%s: file name: source %q generated %q", name, source.GetName(), generated.GetName())
			}
			for _, difference := range compareShapes(name, shapeOf(source), shapeOf(generated)) {
				t.Error(difference)
			}
		})
	}
	for name := range representatives {
		if !seen[name] {
			t.Errorf("%s: representative generated type has no proto source", name)
		}
	}
}
