package api_test

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxexchange/types"
)

func TestLayerxExchangeDescriptorParity(t *testing.T) {
	entries, err := os.ReadDir("../api/layerxexchange")
	if err != nil {
		t.Fatalf("read layerxexchange proto sources: %v", err)
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".proto" {
			continue
		}
		count++
		name := "layerxexchange/" + entry.Name()
		t.Run(name, func(t *testing.T) {
			generated := registeredFile(t, name)
			source := bufFile(t, name)
			for _, difference := range compareShapes(name, shapeOf(source), shapeOf(generated)) {
				t.Error(difference)
			}
		})
	}
	if count == 0 {
		t.Fatal("layerxexchange: no proto sources found")
	}
}
