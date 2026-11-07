package api_test

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	_ "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/vesting/types"
)

func TestCosmosAuthVestingDescriptorParity(t *testing.T) {
	for _, directory := range []struct {
		name     string
		baseline int
	}{
		{name: "cosmos/auth/v1beta1", baseline: 3},
		{name: "cosmos/vesting/v1beta1", baseline: 2},
	} {
		t.Run(directory.name, func(t *testing.T) {
			root := filepath.Join("..", "sdk", "proto", filepath.FromSlash(directory.name))
			entries, err := os.ReadDir(root)
			if err != nil {
				t.Fatalf("%s: read proto source directory: %v", root, err)
			}
			count := 0
			for _, entry := range entries {
				if entry.IsDir() || filepath.Ext(entry.Name()) != ".proto" {
					continue
				}
				count++
				name := directory.name + "/" + entry.Name()
				t.Run(entry.Name(), func(t *testing.T) {
					source := bufFile(t, name)
					generated := registeredFile(t, name)
					for _, difference := range compareShapes(name, shapeOf(source), shapeOf(generated)) {
						t.Error(difference)
					}
				})
			}
			if count < directory.baseline {
				t.Errorf("%s: found %d proto sources, want at least the %d baseline files plus additions", root, count, directory.baseline)
			}
		})
	}
}
