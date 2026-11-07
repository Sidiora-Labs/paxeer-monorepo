package api_test

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
)

func TestLayerxAuthDescriptorParity(t *testing.T) {
	files := 0
	var checkDirectory func(string)
	checkDirectory = func(directory string) {
		entries, err := os.ReadDir(filepath.Join("..", "api", directory))
		if err != nil {
			t.Fatalf("%s: read proto source directory: %v", directory, err)
		}
		for _, entry := range entries {
			name := filepath.Join(directory, entry.Name())
			if entry.IsDir() {
				checkDirectory(name)
				continue
			}
			if filepath.Ext(entry.Name()) != ".proto" {
				continue
			}
			files++
			name = filepath.ToSlash(name)
			t.Run(name, func(t *testing.T) {
				generated := registeredFile(t, name)
				source := bufFile(t, name)
				for _, difference := range compareShapes(name, shapeOf(source), shapeOf(generated)) {
					t.Error(difference)
				}
			})
		}
	}
	checkDirectory("layerxauth")
	if files == 0 {
		t.Fatal("layerxauth: no .proto source files found")
	}
}
