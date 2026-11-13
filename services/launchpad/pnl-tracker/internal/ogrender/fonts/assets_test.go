package fonts_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/ogrender/fonts"
)

func TestMaterializeEmbeddedFonts(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "fonts")
	for repeat := 0; repeat < 2; repeat++ {
		if err := fonts.Materialize(directory); err != nil {
			t.Fatal(err)
		}
	}
	info, err := os.Stat(directory)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 {
		t.Fatalf("font directory = %v, error = %v", info, err)
	}
	for name, want := range map[string]string{
		"Inter-Regular.ttf": fonts.RegularSHA256,
		"Inter-Bold.ttf":    fonts.BoldSHA256,
		"OFL.txt":           fonts.LicenseSHA256,
	} {
		path := filepath.Join(directory, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("asset %s = %v, error = %v", name, info, err)
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != want {
			t.Fatalf("asset %s digest = %s, want %s", name, got, want)
		}
	}
	if err := os.WriteFile(filepath.Join(directory, "OFL.txt"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := fonts.Materialize(directory); err == nil || !strings.Contains(err.Error(), "asset must be an owned regular private file") {
		t.Fatalf("changed asset: %v", err)
	}
}

func TestMaterializeRejectsUnsafeDestination(t *testing.T) {
	root := t.TempDir()
	for _, destination := range []string{"relative", root + "/../" + filepath.Base(root), string(filepath.Separator)} {
		if err := fonts.Materialize(destination); err == nil {
			t.Fatalf("accepted destination %q", destination)
		}
	}
	public := filepath.Join(root, "public")
	if err := os.Mkdir(public, 0755); err != nil {
		t.Fatal(err)
	}
	if err := fonts.Materialize(public); err == nil || !strings.Contains(err.Error(), "private directory") {
		t.Fatalf("public directory: %v", err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(public, link); err != nil {
		t.Fatal(err)
	}
	if err := fonts.Materialize(link); err == nil {
		t.Fatal("accepted symlink destination")
	}
}
