package safefile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadRegularBoundsAndPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.json")
	if err := os.WriteFile(path, []byte(`{"pool":"one"}`), 0600); err != nil {
		t.Fatal(err)
	}
	raw, err := ReadRegular(path, 64)
	if err != nil || string(raw) != `{"pool":"one"}` {
		t.Fatalf("read regular file: %q, %v", raw, err)
	}
	if _, err := ReadRegular(path, int64(len(raw)-1)); err == nil || !strings.Contains(err.Error(), "bounded regular file required") {
		t.Fatalf("expected size bound, got %v", err)
	}
	if _, err := ReadRegular(filepath.Join(dir, "missing"), 64); err == nil {
		t.Fatal("missing file was accepted")
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadRegular(link, 64); err == nil {
		t.Fatal("symbolic link was accepted")
	}
	if _, err := ReadRegular("/proc/self/stat", 1); err == nil || !strings.Contains(err.Error(), "bounded regular file required") {
		t.Fatalf("dynamic regular file exceeded its read bound: %v", err)
	}
}
