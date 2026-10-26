package api_test

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
)

func TestLayerxCustodyDescriptorParity(t *testing.T) {
	entries, err := os.ReadDir("../api/layerxcustody")
	if err != nil {
		t.Fatalf("read layerxcustody proto sources: %v", err)
	}
	files := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".proto" {
			continue
		}
		files++
		name := "layerxcustody/" + entry.Name()
		t.Run(name, func(t *testing.T) {
			source := shapeOf(bufFile(t, name))
			generated := shapeOf(registeredFile(t, name))
			for _, difference := range compareShapes(name, source, generated) {
				t.Error(difference)
			}
		})
	}
	if files == 0 {
		t.Fatal("layerxcustody: no proto sources found")
	}
}
