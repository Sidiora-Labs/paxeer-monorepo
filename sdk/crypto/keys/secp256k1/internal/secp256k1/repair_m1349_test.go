//go:build !gofuzz && cgo
// +build !gofuzz,cgo

package secp256k1

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"go/build"
	"io/fs"
	"path/filepath"
	"runtime"
	"testing"
)

func repairM1349PackageDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate package sources")
	}
	return filepath.Dir(file)
}

func TestRepairM_1349_BuildSelection(t *testing.T) {
	dir := repairM1349PackageDir(t)
	for _, tc := range []struct {
		name string
		cgo  bool
		fuzz bool
	}{
		{"cgo", true, false},
		{"pure_go", false, false},
		{"cgo_gofuzz", true, true},
		{"pure_go_gofuzz", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := build.Default
			ctx.CgoEnabled = tc.cgo
			ctx.BuildTags = nil
			if tc.fuzz {
				ctx.BuildTags = []string{"gofuzz"}
			}
			for _, name := range []string{"secp256.go", "scalar_mult_cgo.go", "panic_cb.go", "secp256_test.go", "repair_m1349_test.go"} {
				selected, err := ctx.MatchFile(dir, name)
				if err != nil {
					t.Fatal(err)
				}
				if want := tc.cgo && !tc.fuzz; selected != want {
					t.Errorf("%s selected = %t, want %t", name, selected, want)
				}
			}
			selected, err := ctx.MatchFile(dir, "scalar_mult_nocgo.go")
			if err != nil {
				t.Fatal(err)
			}
			if want := !tc.cgo || tc.fuzz; selected != want {
				t.Errorf("pure-Go scalar implementation selected = %t, want %t", selected, want)
			}
		})
	}
}

func TestRepairM_1349_PureGoCryptoSelection(t *testing.T) {
	dir := repairM1349PackageDir(t)
	cryptoDir := filepath.Clean(filepath.Join(dir, "../../../.."))
	ctx := build.Default
	ctx.CgoEnabled = false
	ctx.BuildTags = nil
	packages := make(map[string]*build.Package)
	err := filepath.WalkDir(cryptoDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			return nil
		}
		if path != cryptoDir && (entry.Name() == "testdata" || entry.Name() == "vendor" || entry.Name()[0] == '.' || entry.Name()[0] == '_') {
			return filepath.SkipDir
		}
		pkg, err := ctx.ImportDir(path, 0)
		var noGo *build.NoGoError
		if errors.As(err, &noGo) {
			return nil
		}
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(cryptoDir, path)
		if err != nil {
			return err
		}
		packages[filepath.ToSlash(rel)] = pkg
		if len(pkg.CgoFiles) != 0 {
			t.Errorf("%s selected cgo sources: %v", rel, pkg.CgoFiles)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".", "codec", "hd", "keyring", "keys/secp256k1", "keys/secp256k1/internal/secp256k1", "types"} {
		if packages[name] == nil {
			t.Fatalf("crypto selection omitted %s", name)
		}
	}
	assertSelected := func(pkg *build.Package, name string, want bool) {
		t.Helper()
		selected := false
		for _, files := range [][]string{pkg.GoFiles, pkg.CgoFiles, pkg.TestGoFiles, pkg.XTestGoFiles} {
			for _, file := range files {
				selected = selected || file == name
			}
		}
		if selected != want {
			t.Errorf("%s/%s selected = %t, want %t", pkg.Dir, name, selected, want)
		}
	}
	wrapper := packages["keys/secp256k1/internal/secp256k1"]
	for _, name := range []string{"secp256.go", "scalar_mult_cgo.go", "panic_cb.go", "secp256_test.go", "repair_m1349_test.go"} {
		assertSelected(wrapper, name, false)
	}
	assertSelected(wrapper, "curve.go", true)
	assertSelected(wrapper, "scalar_mult_nocgo.go", true)
	assertSelected(packages["keys/secp256k1"], "secp256k1_nocgo.go", true)
	assertSelected(packages["keys/secp256k1"], "secp256k1_cgo.go", false)
	t.Logf("selected %d pure-Go crypto packages, including tests", len(packages))
}

func TestRepairM_1349_CgoSignRecoverVerify(t *testing.T) {
	key := make([]byte, 32)
	key[31] = 1
	msg := sha256.Sum256([]byte("repair 22.33.1349 cgo-test-selection"))
	x, y := S256().ScalarBaseMult(key)
	pubkey := S256().Marshal(x, y)
	want, err := hex.DecodeString("0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798")
	if err != nil {
		t.Fatal(err)
	}
	compressed := CompressPubkey(x, y)
	if !bytes.Equal(compressed, want) {
		t.Fatalf("private scalar 1 produced %x, want generator %x", compressed, want)
	}
	dx, dy := DecompressPubkey(compressed)
	if dx == nil || dy == nil || dx.Cmp(x) != 0 || dy.Cmp(y) != 0 {
		t.Fatal("compressed public key did not round-trip")
	}
	sig, err := Sign(msg[:], key)
	if err != nil {
		t.Fatal(err)
	}
	if len(sig) != 65 || sig[64] > 1 {
		t.Fatalf("invalid recoverable signature: %x", sig)
	}
	repeated, err := Sign(msg[:], key)
	if err != nil || !bytes.Equal(sig, repeated) {
		t.Fatalf("deterministic signing changed: %x, %v", repeated, err)
	}
	recovered, err := RecoverPubkey(msg[:], sig)
	if err != nil || !bytes.Equal(recovered, pubkey) {
		t.Fatalf("recovered %x, want %x: %v", recovered, pubkey, err)
	}
	for _, public := range [][]byte{pubkey, compressed, recovered} {
		if !VerifySignature(public, msg[:], sig[:64]) {
			t.Fatalf("signature rejected for public key %x", public)
		}
	}
	msg[0] ^= 1
	if VerifySignature(compressed, msg[:], sig[:64]) {
		t.Fatal("signature accepted for a changed message")
	}
	msg[0] ^= 1
	sig[0] ^= 1
	if VerifySignature(compressed, msg[:], sig[:64]) {
		t.Fatal("changed signature accepted")
	}
}
