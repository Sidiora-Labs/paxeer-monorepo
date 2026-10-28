package main

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInputPathsResolveCommittedInputs(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("test source path unavailable")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", "..", "..", ".."))
	reviewed, artifact := inputPaths(root)
	if want := filepath.Join(root, "sdk", "launchpad", "protocol", "catalog", "reviewed.json"); reviewed != want {
		t.Fatalf("reviewed path = %q, want %q", reviewed, want)
	}
	if want := filepath.Join(root, "contracts", "launchpad", "artifacts", "contracts", "core", "SidioraERC20.sol", "SidioraERC20.json"); artifact != want {
		t.Fatalf("artifact path = %q, want %q", artifact, want)
	}
	if _, err := os.ReadFile(reviewed); err != nil {
		t.Fatalf("read committed reviewed catalog: %v", err)
	}
	data, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatalf("read committed artifact: %v", err)
	}
	var compiled struct {
		DeployedBytecode string `json:"deployedBytecode"`
	}
	if err := json.Unmarshal(data, &compiled); err != nil {
		t.Fatalf("decode committed artifact: %v", err)
	}
	code, err := hex.DecodeString(strings.TrimPrefix(compiled.DeployedBytecode, "0x"))
	if err != nil {
		t.Fatalf("decode deployed bytecode: %v", err)
	}
	if len(code) == 0 {
		t.Fatal("committed artifact has empty deployed bytecode")
	}
}
