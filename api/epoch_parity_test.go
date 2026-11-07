package api_test

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/Sidiora-Labs/Paxeer-X-Network/modules/epoch/types"
)

func TestEpochDescriptorParity(t *testing.T) {
	entries, err := os.ReadDir("../api/epoch")
	if err != nil {
		t.Fatalf("read epoch proto sources: %v", err)
	}
	protoCount := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".proto" {
			continue
		}
		protoCount++
		name := "epoch/" + entry.Name()
		t.Run(name, func(t *testing.T) {
			generated := shapeOf(registeredFile(t, name))
			source := shapeOf(bufFile(t, name))
			for _, difference := range compareShapes(name, source, generated) {
				t.Error(difference)
			}
		})
	}
	if protoCount == 0 {
		t.Fatal("epoch: no proto sources found")
	}
}
