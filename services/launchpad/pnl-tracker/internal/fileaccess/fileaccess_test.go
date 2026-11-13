package fileaccess

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestFileAccessRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshot.json")
	file, err := CreateExclusive(path)
	if err != nil {
		t.Fatal(err)
	}
	const payload = `{"position":"open"}`
	if _, err := io.WriteString(file, payload); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateExclusive(path); !errors.Is(err, os.ErrExist) {
		t.Fatalf("second create: %v, want file exists", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("mode = %o, want 600", info.Mode().Perm())
	}
	opened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(opened)
	if err != nil {
		t.Fatal(err)
	}
	if err := opened.Close(); err != nil {
		t.Fatal(err)
	}
	if string(got) != payload {
		t.Fatalf("opened data = %q", got)
	}
	got, err = ReadFile(path)
	if err != nil || string(got) != payload {
		t.Fatalf("read data = %q, error = %v", got, err)
	}
	missing := filepath.Join(filepath.Dir(path), "missing.json")
	if _, err := Open(missing); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("open missing file: %v", err)
	}
	if _, err := ReadFile(missing); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read missing file: %v", err)
	}
	if _, err := CreateExclusive(filepath.Join(missing, "file")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("create in missing directory: %v", err)
	}
}
