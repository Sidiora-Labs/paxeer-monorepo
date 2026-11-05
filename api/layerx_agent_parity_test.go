package api_test

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
)

func TestLayerxAgentDescriptorParity(t *testing.T) {
	fileCount := 0
	var checkDirectory func(string)
	checkDirectory = func(directory string) {
		entries, err := os.ReadDir(filepath.Join("..", "api", directory))
		if err != nil {
			t.Fatalf("%s: read proto directory: %v", directory, err)
		}
		for _, entry := range entries {
			path := filepath.Join(directory, entry.Name())
			if entry.IsDir() {
				checkDirectory(path)
				continue
			}
			if filepath.Ext(entry.Name()) != ".proto" {
				continue
			}
			fileCount++
			name := filepath.ToSlash(path)
			t.Run(name, func(t *testing.T) {
				source := shapeOf(bufFile(t, name))
				generated := shapeOf(registeredFile(t, name))
				for _, difference := range compareShapes(name, source, generated) {
					t.Error(difference)
				}
			})
		}
	}
	checkDirectory("layerxagent")
	if fileCount == 0 {
		t.Fatal("layerxagent: no proto sources found")
	}
}
