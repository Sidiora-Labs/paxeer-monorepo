package api_test

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
)

func TestLaunchpadDescriptorParity(t *testing.T) {
	files := 0
	var checkDirectory func(string)
	checkDirectory = func(directory string) {
		entries, err := os.ReadDir(filepath.Join("..", "api", filepath.FromSlash(directory)))
		if err != nil {
			t.Fatalf("%s: read proto directory: %v", directory, err)
		}
		for _, entry := range entries {
			name := path.Join(directory, entry.Name())
			if entry.IsDir() {
				checkDirectory(name)
				continue
			}
			if !strings.HasSuffix(entry.Name(), ".proto") {
				continue
			}
			files++
			t.Run(name, func(t *testing.T) {
				source := shapeOf(bufFile(t, name))
				generated := shapeOf(registeredFile(t, name))
				for _, difference := range compareShapes(name, source, generated) {
					t.Error(difference)
				}
			})
		}
	}
	checkDirectory("launchpad")
	if files == 0 {
		t.Fatal("launchpad: no .proto files found")
	}
}
