package holdings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPhase4RequiredSourcesExistAtRepositoryRoot(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
			break
		} else if !os.IsNotExist(err) {
			t.Fatalf("locate repository root: %v", err)
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Fatal("repository root not found")
		}
		root = parent
	}
	if len(phase4RequiredSources) == 0 {
		t.Fatal("required source list is empty")
	}
	for _, source := range phase4RequiredSources {
		t.Run(source, func(t *testing.T) {
			info, err := os.Stat(filepath.Join(root, source))
			if err != nil {
				t.Fatalf("required source %q at repository root: %v", source, err)
			}
			if !info.Mode().IsRegular() {
				t.Fatalf("required source %q is not a regular file", source)
			}
		})
	}
}
