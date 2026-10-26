package api_test

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	_ "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
)

func TestCosmosGovDescriptorParity(t *testing.T) {
	const protoPackage = "cosmos/gov/v1beta1"
	sourceDir := filepath.Join("..", "sdk", "proto", filepath.FromSlash(protoPackage))
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		t.Fatalf("%s: read proto sources: %v", sourceDir, err)
	}
	fileCount := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".proto" {
			continue
		}
		fileCount++
		name := path.Join(protoPackage, entry.Name())
		t.Run(name, func(t *testing.T) {
			generated := registeredFile(t, name)
			source := bufFile(t, name)
			for _, difference := range compareShapes(name, shapeOf(source), shapeOf(generated)) {
				t.Error(difference)
			}
		})
	}
	if fileCount == 0 {
		t.Fatalf("%s: no proto sources found", sourceDir)
	}
}
