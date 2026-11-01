package api_test

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	_ "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx"
	_ "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx/signing"
)

func TestCosmosTxDescriptorParity(t *testing.T) {
	for _, directory := range []string{
		"cosmos/tx/signing/v1beta1",
		"cosmos/tx/v1beta1",
	} {
		sourceDirectory := filepath.Join("..", "sdk", "proto", filepath.FromSlash(directory))
		entries, err := os.ReadDir(sourceDirectory)
		if err != nil {
			t.Errorf("%s: read proto source directory: %v", sourceDirectory, err)
			continue
		}
		files := 0
		for _, entry := range entries {
			if entry.IsDir() || path.Ext(entry.Name()) != ".proto" {
				continue
			}
			files++
			name := path.Join(directory, entry.Name())
			t.Run(name, func(t *testing.T) {
				generated := registeredFile(t, name)
				source := bufFile(t, name)
				for _, difference := range compareShapes(name, shapeOf(source), shapeOf(generated)) {
					t.Error(difference)
				}
			})
		}
		if files == 0 {
			t.Errorf("%s: no proto source files found", sourceDirectory)
		}
	}
}
