package exportcheck_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/exportcheck"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/testvectors"
)

func TestTestvectorsAnchorLoaderAbsent(t *testing.T) {
	const importPath = "github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/testvectors"
	exports, err := exportcheck.PackageExports(importPath)
	if err != nil {
		t.Fatalf("PackageExports(%q): %v", importPath, err)
	}
	for _, name := range []string{"Load", "LoadPaxeerBind"} {
		if !exports[name] {
			t.Errorf("PackageExports(%q) missing exported identifier %q", importPath, name)
		}
	}
	if exports["LoadAnchor"] {
		t.Errorf("PackageExports(%q) contains retired exported identifier LoadAnchor", importPath)
	}

	root, err := testvectors.RepositoryRoot()
	if err != nil {
		t.Fatalf("RepositoryRoot(): %v", err)
	}
	path := filepath.Join(root, "layerxproof", "testdata", "anchor_vectors.json")
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("os.Stat(%q) = %v, want os.ErrNotExist", path, err)
	}
}
