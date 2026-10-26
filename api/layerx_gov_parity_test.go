package api_test

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
)

func TestLayerxGovDescriptorParity(t *testing.T) {
	root := filepath.Join("..", "api", "layerxgov")
	count := 0
	var visit func(string)
	visit = func(relative string) {
		directory := filepath.Join(root, relative)
		entries, err := os.ReadDir(directory)
		if err != nil {
			t.Fatalf("%s: read proto sources: %v", directory, err)
		}
		for _, entry := range entries {
			file := filepath.Join(relative, entry.Name())
			if entry.IsDir() {
				visit(file)
				continue
			}
			if filepath.Ext(entry.Name()) != ".proto" {
				continue
			}
			count++
			name := filepath.ToSlash(filepath.Join("layerxgov", file))
			t.Run(name, func(t *testing.T) {
				generated := registeredFile(t, name)
				source := bufFile(t, name)
				for _, difference := range compareShapes(name, shapeOf(source), shapeOf(generated)) {
					t.Error(difference)
				}
			})
		}
	}
	visit("")
	if count == 0 {
		t.Fatalf("%s: no .proto sources found", root)
	}
}
