package api_test

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/types"
)

func TestLayerxAnchorDescriptorParity(t *testing.T) {
	files := 0
	var checkDir func(string)
	checkDir = func(dir string) {
		entries, err := os.ReadDir(filepath.Join("..", "api", filepath.FromSlash(dir)))
		if err != nil {
			t.Fatalf("%s: read proto source directory: %v", dir, err)
		}
		for _, entry := range entries {
			name := path.Join(dir, entry.Name())
			if entry.IsDir() {
				checkDir(name)
				continue
			}
			if !strings.HasSuffix(entry.Name(), ".proto") {
				continue
			}
			files++
			t.Run(name, func(t *testing.T) {
				generated := registeredFile(t, name)
				source := bufFile(t, name)
				for _, difference := range compareShapes(name, shapeOf(source), shapeOf(generated)) {
					t.Error(difference)
				}
			})
		}
	}
	checkDir("layerxanchor")
	if files == 0 {
		t.Fatal("layerxanchor: no proto source files found")
	}
}
