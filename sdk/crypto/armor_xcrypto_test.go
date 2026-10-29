package crypto_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
)

func TestXCryptoBuildMatchesRequire(t *testing.T) {
	// Go 1.25 test binaries omit module dependencies from ReadBuildInfo.
	// Read the runtime metadata of an executable linked against the SDK instead.
	const source = `package main

import (
	"encoding/json"
	"os"
	"runtime/debug"

	_ "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto"
)

func main() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		panic("SDK executable has no Go build information")
	}
	if err := json.NewEncoder(os.Stdout).Encode(info); err != nil {
		panic(err)
	}
}
`
	probe := filepath.Join(t.TempDir(), "main.go")
	if err := os.WriteFile(probe, []byte(source), 0600); err != nil {
		t.Fatalf("write SDK build-info executable: %v", err)
	}
	output, err := exec.Command("go", "run", "-mod=readonly", probe).CombinedOutput()
	if err != nil {
		t.Fatalf("run SDK build-info executable: %v\n%s", err, output)
	}
	var info debug.BuildInfo
	if err := json.Unmarshal(output, &info); err != nil {
		t.Fatalf("decode SDK runtime build information: %v\n%s", err, output)
	}
	for _, dep := range info.Deps {
		if dep.Path != "golang.org/x/crypto" {
			continue
		}
		if dep.Version != "v0.47.0" {
			t.Errorf("golang.org/x/crypto version = %q, want v0.47.0", dep.Version)
		}
		if dep.Replace != nil {
			t.Errorf("golang.org/x/crypto has a replacement: %+v", dep.Replace)
		}
		return
	}
	t.Fatal("golang.org/x/crypto is missing from the SDK executable's dependencies")
}

func TestArmorPrivKeyRoundTripOnRequiredXCrypto(t *testing.T) {
	key := secp256k1.GenPrivKey()
	const passphrase = "required-xcrypto-armor-round-trip"
	armored := crypto.EncryptArmorPrivKey(key.Bytes(), passphrase, key.Type())
	decrypted, algo, err := crypto.UnarmorDecryptPrivKey(armored, passphrase)
	if err != nil {
		t.Fatalf("decrypt armored private key: %v", err)
	}
	if !bytes.Equal(decrypted, key.Bytes()) {
		t.Fatal("decrypted private key differs from the original")
	}
	if algo != key.Type() {
		t.Errorf("decrypted key algorithm = %q, want %q", algo, key.Type())
	}
}
