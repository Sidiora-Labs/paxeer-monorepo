package api_test

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	_ "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxbridge/types"
)

func TestLayerxBridgeDescriptorParity(t *testing.T) {
	const root = "../api/layerxbridge"
	files := 0
	var checkDirectory func(string)
	checkDirectory = func(relative string) {
		entries, err := os.ReadDir(filepath.Join(root, relative))
		if err != nil {
			t.Fatalf("read layerxbridge proto directory %q: %v", relative, err)
		}
		for _, entry := range entries {
			relativeName := filepath.Join(relative, entry.Name())
			if entry.IsDir() {
				checkDirectory(relativeName)
				continue
			}
			if filepath.Ext(entry.Name()) != ".proto" {
				continue
			}
			files++
			name := path.Join("layerxbridge", filepath.ToSlash(relativeName))
			t.Run(name, func(t *testing.T) {
				generated := registeredFile(t, name)
				source := bufFile(t, name)
				for _, difference := range compareShapes(name, shapeOf(source), shapeOf(generated)) {
					t.Error(difference)
				}
			})
		}
	}
	checkDirectory("")
	if files == 0 {
		t.Fatal("layerxbridge: no proto source files found")
	}
}
