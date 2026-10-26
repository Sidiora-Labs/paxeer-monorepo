package api_test

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/types"
)

func TestOracleDescriptorParity(t *testing.T) {
	entries, err := os.ReadDir("../api/oracle")
	if err != nil {
		t.Fatalf("read oracle proto sources: %v", err)
	}
	files := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".proto" {
			continue
		}
		files++
		name := "oracle/" + entry.Name()
		t.Run(name, func(t *testing.T) {
			generated := registeredFile(t, name)
			source := bufFile(t, name)
			for _, difference := range compareShapes(name, shapeOf(source), shapeOf(generated)) {
				t.Error(difference)
			}
		})
	}
	if files == 0 {
		t.Fatal("../api/oracle: no .proto sources found")
	}
}
