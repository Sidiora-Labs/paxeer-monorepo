package api_test

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	_ "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types/proposal"
)

func TestCosmosParamsDescriptorParity(t *testing.T) {
	var files []string
	var readProtos func(string)
	readProtos = func(dir string) {
		entries, err := os.ReadDir(filepath.Join("..", "sdk", "proto", dir))
		if err != nil {
			t.Fatalf("%s: read proto sources: %v", dir, err)
		}
		for _, entry := range entries {
			name := filepath.Join(dir, entry.Name())
			if entry.IsDir() {
				readProtos(name)
				continue
			}
			if filepath.Ext(entry.Name()) == ".proto" {
				files = append(files, filepath.ToSlash(name))
			}
		}
	}
	for _, dir := range []string{"cosmos/params/types", "cosmos/params/v1beta1"} {
		before := len(files)
		readProtos(dir)
		if len(files) == before {
			t.Fatalf("%s: no proto sources found", dir)
		}
	}
	for _, name := range files {
		t.Run(name, func(t *testing.T) {
			source := shapeOf(bufFile(t, name))
			generated := shapeOf(registeredFile(t, name))
			for _, difference := range compareShapes(name, source, generated) {
				t.Error(difference)
			}
		})
	}
}
