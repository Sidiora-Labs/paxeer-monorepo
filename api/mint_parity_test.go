package api_test

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint/types"
)

func TestMintDescriptorParity(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join("..", "api", "mint", "v1beta1"))
	if err != nil {
		t.Fatalf("list mint proto sources: %v", err)
	}
	files := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".proto" {
			continue
		}
		files++
		name := "mint/v1beta1/" + entry.Name()
		t.Run(name, func(t *testing.T) {
			generated := registeredFile(t, name)
			source := bufFile(t, name)
			for _, difference := range compareShapes(name, shapeOf(source), shapeOf(generated)) {
				t.Error(difference)
			}
		})
	}
	if files == 0 {
		t.Fatal("mint/v1beta1: no proto sources found")
	}
}
