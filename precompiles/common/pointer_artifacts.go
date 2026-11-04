package common

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

// The committed runtimes retain their original compiler output and layout,
// with only PUSH4 getSeiAddr(address) operands linked to getPaxAddr(address).
// manifest.json records source hashes, compiler versions and relocation offsets.
//
//go:embed pointer_artifacts/*.bin pointer_artifacts/manifest.json
var pointerArtifacts embed.FS

type pointerArtifactManifest struct {
	OriginalRuntimeSHA256  string `json:"original_runtime_sha256"`
	CorrectedRuntimeSHA256 string `json:"corrected_runtime_sha256"`
}

func CWPointerRuntime(kind string) ([]byte, error) {
	var manifest map[string]pointerArtifactManifest
	data, err := pointerArtifacts.ReadFile("pointer_artifacts/manifest.json")
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	entry, ok := manifest[kind]
	if !ok {
		return nil, fmt.Errorf("unknown CW pointer artifact %q", kind)
	}
	data, err = pointerArtifacts.ReadFile("pointer_artifacts/" + kind + ".bin")
	if err != nil {
		return nil, err
	}
	code, err := hex.DecodeString(strings.TrimSpace(string(data)))
	if err != nil {
		return nil, err
	}
	if fmt.Sprintf("%x", sha256.Sum256(code)) != entry.CorrectedRuntimeSHA256 {
		return nil, fmt.Errorf("CW pointer artifact checksum mismatch: %s", kind)
	}
	return code, nil
}

func InstallCWPointerRuntime(evm *vm.EVM, address common.Address, kind string) error {
	code, err := CWPointerRuntime(kind)
	if err != nil {
		return err
	}
	current := evm.StateDB.GetCode(address)
	if bytes.Equal(current, code) {
		return nil
	}
	data, err := pointerArtifacts.ReadFile("pointer_artifacts/manifest.json")
	if err != nil {
		return err
	}
	var manifest map[string]pointerArtifactManifest
	if err = json.Unmarshal(data, &manifest); err != nil {
		return err
	}
	if fmt.Sprintf("%x", sha256.Sum256(current)) != manifest[kind].OriginalRuntimeSHA256 {
		return fmt.Errorf("unexpected deployed CW pointer artifact: %s", kind)
	}
	evm.StateDB.SetCode(address, code)
	return nil
}
