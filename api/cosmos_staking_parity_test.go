package api_test

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	_ "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
)

func TestCosmosStakingDescriptorParity(t *testing.T) {
	const protoDir = "cosmos/staking/v1beta1"
	sourceDir := filepath.Join("..", "sdk", "proto", filepath.FromSlash(protoDir))
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		t.Fatalf("read staking proto sources %s: %v", sourceDir, err)
	}
	fileCount := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".proto" {
			continue
		}
		fileCount++
		name := path.Join(protoDir, entry.Name())
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
