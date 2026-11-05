package api_test

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	_ "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/feegrant"
)

func TestCosmosFeegrantDescriptorParity(t *testing.T) {
	const protoDir = "cosmos/feegrant/v1beta1"
	sourceDir := filepath.Join("..", "sdk", "proto", filepath.FromSlash(protoDir))
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		t.Fatalf("read feegrant proto sources %s: %v", sourceDir, err)
	}

	files := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".proto" {
			continue
		}
		files++
		name := path.Join(protoDir, entry.Name())
		t.Run(name, func(t *testing.T) {
			source := bufFile(t, name)
			generated := registeredFile(t, name)
			for _, difference := range compareShapes(name, shapeOf(source), shapeOf(generated)) {
				t.Error(difference)
			}
		})
	}
	if files < 4 {
		t.Errorf("%s: found %d proto sources, want at least the 4 baseline files plus additions", sourceDir, files)
	}
}
