package api_test

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/evidence/types"
)

func TestCosmosEvidenceDescriptorParity(t *testing.T) {
	const directory = "cosmos/evidence/v1beta1"
	entries, err := os.ReadDir(filepath.Join("..", "sdk", "proto", filepath.FromSlash(directory)))
	if err != nil {
		t.Fatalf("%s: read proto sources: %v", directory, err)
	}
	files := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".proto") {
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
		t.Fatalf("%s: no .proto sources found", directory)
	}
}
