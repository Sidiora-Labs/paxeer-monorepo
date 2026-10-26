package api_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
)

func TestLayerxLedgerDescriptorParity(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join("..", "api", "layerxledger"))
	if err != nil {
		t.Fatalf("read api/layerxledger proto sources: %v", err)
	}
	files := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".proto") {
			continue
		}
		files++
		name := "layerxledger/" + entry.Name()
		t.Run(name, func(t *testing.T) {
			generated := registeredFile(t, name)
			source := bufFile(t, name)
			for _, difference := range compareShapes(name, shapeOf(source), shapeOf(generated)) {
				t.Error(difference)
			}
		})
	}
	if files == 0 {
		t.Fatal("api/layerxledger: no .proto sources found")
	}
}
