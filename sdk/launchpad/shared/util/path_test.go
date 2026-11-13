package util

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFilePathsUseDirectoryRoot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "payload.txt")
	file, err := OpenFilePath(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("stored payload"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := ReadFilePath(path)
	if err != nil || string(data) != "stored payload" {
		t.Fatalf("stored payload = %q, %v", data, err)
	}
	if _, err := OpenPath(dir + string(os.PathSeparator)); err == nil {
		t.Fatal("path without a file name was accepted")
	}
	if _, err := OpenPath(filepath.Join(dir, "missing")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing file: %v", err)
	}
	if _, err := OpenPath(dir + string(os.PathSeparator) + ".."); err == nil {
		t.Fatal("parent traversal was accepted")
	}
	if _, err := ReadFilePath(filepath.Join(dir, "missing")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing read: %v", err)
	}
}
