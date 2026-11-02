package api_test

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	_ "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz"
)

func TestCosmosAuthzDescriptorParity(t *testing.T) {
	const protoPackage = "cosmos/authz/v1beta1"
	sourceDir := filepath.Join("..", "sdk", "proto", filepath.FromSlash(protoPackage))
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		t.Fatalf("read proto sources %s: %v", sourceDir, err)
	}
	protoCount := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".proto" {
			continue
		}
		protoCount++
		name := path.Join(protoPackage, entry.Name())
		t.Run(name, func(t *testing.T) {
			source := shapeOf(bufFile(t, name))
			generated := shapeOf(registeredFile(t, name))
			for _, difference := range compareShapes(name, source, generated) {
				t.Error(difference)
			}
		})
	}
	if protoCount == 0 {
		t.Fatalf("%s: no .proto sources found", sourceDir)
	}
}
