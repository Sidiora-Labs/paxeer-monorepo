package api_test

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
)

func TestTokenfactoryDescriptorParity(t *testing.T) {
	dir := filepath.Join("..", "api", "tokenfactory")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read tokenfactory proto sources: %v", err)
	}
	files := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".proto" {
			continue
		}
		files++
		name := "tokenfactory/" + entry.Name()
		t.Run(name, func(t *testing.T) {
			generated := registeredFile(t, name)
			source := bufFile(t, name)
			for _, difference := range compareShapes(name, shapeOf(source), shapeOf(generated)) {
				t.Error(difference)
			}
		})
	}
	if files == 0 {
		t.Fatal("no tokenfactory proto sources found")
	}
}
