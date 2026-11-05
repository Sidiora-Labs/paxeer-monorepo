package api_test

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/types"
)

func TestSpotDescriptorParity(t *testing.T) {
	const root = "../api/spot"
	count := 0
	var checkDir func(string)
	checkDir = func(relative string) {
		dir := filepath.Join(root, relative)
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("%s: read proto source directory: %v", dir, err)
		}
		for _, entry := range entries {
			path := filepath.Join(relative, entry.Name())
			if entry.IsDir() {
				checkDir(path)
				continue
			}
			if filepath.Ext(entry.Name()) != ".proto" {
				continue
			}
			count++
			name := filepath.ToSlash(filepath.Join("spot", path))
			t.Run(name, func(t *testing.T) {
				generated := registeredFile(t, name)
				source := bufFile(t, name)
				for _, difference := range compareShapes(name, shapeOf(source), shapeOf(generated)) {
					t.Error(difference)
				}
			})
		}
	}
	checkDir("")
	if count == 0 {
		t.Fatalf("%s: no proto sources found", root)
	}
}
