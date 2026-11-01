package api_test

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	_ "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution/types"
)

func TestCosmosDistributionDescriptorParity(t *testing.T) {
	const protoPackage = "cosmos/distribution/v1beta1"
	root := filepath.Join("..", "sdk", "proto", filepath.FromSlash(protoPackage))
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read distribution proto sources %s: %v", root, err)
	}

	files := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".proto" {
			continue
		}
		files++
		name := path.Join(protoPackage, entry.Name())
		t.Run(name, func(t *testing.T) {
			generated := registeredFile(t, name)
			source := bufFile(t, name)
			for _, difference := range compareShapes(name, shapeOf(source), shapeOf(generated)) {
				t.Error(difference)
			}
		})
	}
	if files < 4 {
		t.Errorf("%s: found %d proto sources, want at least the 4 baseline files", root, files)
	}
}
