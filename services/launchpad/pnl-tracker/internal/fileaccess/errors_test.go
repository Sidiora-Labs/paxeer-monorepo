package fileaccess

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestFileAccessRejectsMissingParentsAndDirectories(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		name string
		path string
		want error
	}{
		{"missing parent", filepath.Join(root, "missing", "position.json"), os.ErrNotExist},
		{"directory", root, syscall.EISDIR},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := ReadFile(tc.path)
			if !errors.Is(err, tc.want) || data != nil {
				t.Fatalf("read = %q, error = %v, want %v", data, err, tc.want)
			}
		})
	}
	if file, err := Open(filepath.Join(root, "missing", "position.json")); !errors.Is(err, os.ErrNotExist) || file != nil {
		t.Fatalf("open missing parent = %v, error = %v", file, err)
	}
}
