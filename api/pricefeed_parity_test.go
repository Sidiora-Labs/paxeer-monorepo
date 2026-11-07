package api_test

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
)

func TestPricefeedDescriptorParity(t *testing.T) {
	entries, err := os.ReadDir("../api/pricefeed")
	if err != nil {
		t.Fatalf("read pricefeed proto sources: %v", err)
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".proto" {
			continue
		}
		count++
		name := "pricefeed/" + entry.Name()
		t.Run(name, func(t *testing.T) {
			generated := registeredFile(t, name)
			source := bufFile(t, name)
			for _, difference := range compareShapes(name, shapeOf(source), shapeOf(generated)) {
				t.Error(difference)
			}
		})
	}
	if count == 0 {
		t.Fatal("../api/pricefeed: no proto sources found")
	}
}
