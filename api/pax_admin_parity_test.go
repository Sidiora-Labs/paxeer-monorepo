package api_test

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/Sidiora-Labs/Paxeer-X-Network/admin/types"
)

func TestPaxAdminDescriptorParity(t *testing.T) {
	moduleRoot := filepath.Join("..", "api")
	sourceRoot := filepath.Join(moduleRoot, "pax", "admin", "v0")
	files := 0
	var visit func(string)
	visit = func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read admin proto directory %s: %v", dir, err)
		}
		for _, entry := range entries {
			path := filepath.Join(dir, entry.Name())
			if entry.IsDir() {
				visit(path)
				continue
			}
			if filepath.Ext(entry.Name()) != ".proto" {
				continue
			}
			relative, err := filepath.Rel(moduleRoot, path)
			if err != nil {
				t.Fatalf("resolve admin proto %s: %v", path, err)
			}
			name := filepath.ToSlash(relative)
			files++
			t.Run(name, func(t *testing.T) {
				generated := registeredFile(t, name)
				source := bufFile(t, name)
				for _, difference := range compareShapes(name, shapeOf(source), shapeOf(generated)) {
					t.Error(difference)
				}
			})
		}
	}
	visit(sourceRoot)
	if files == 0 {
		t.Fatalf("%s: no proto sources found", sourceRoot)
	}
}
