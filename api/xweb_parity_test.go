package api_test

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	_ "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
)

func TestXwebDescriptorParity(t *testing.T) {
	files := 0
	var checkDirectory func(string)
	checkDirectory = func(relative string) {
		directory := filepath.Join("..", "api", filepath.FromSlash(relative))
		entries, err := os.ReadDir(directory)
		if err != nil {
			t.Fatalf("%s: read proto source directory: %v", directory, err)
		}
		for _, entry := range entries {
			name := path.Join(relative, entry.Name())
			if entry.IsDir() {
				checkDirectory(name)
				continue
			}
			if filepath.Ext(entry.Name()) != ".proto" {
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
	checkDirectory("xweb")
	if files == 0 {
		t.Fatal("xweb: no .proto sources found")
	}
}
