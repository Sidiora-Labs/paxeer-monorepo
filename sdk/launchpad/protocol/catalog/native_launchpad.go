package catalog

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

//go:generate python3 generate_native.py

//go:embed native_launchpad.json
var nativeLaunchpadJSON []byte

// NativeLaunchpadABI supplies the chain's launchpad ABI for origin-scoped decoding.
func NativeLaunchpadABI() (abi.ABI, error) {
	return abi.JSON(bytes.NewReader(nativeLaunchpadJSON))
}

// NativeLaunchpadVersion identifies the exact ABI carried by the catalog.
func NativeLaunchpadVersion() string {
	digest := sha256.Sum256(nativeLaunchpadJSON)
	return hex.EncodeToString(digest[:])
}

func nativeLaunchpadBundle() Bundle {
	version := NativeLaunchpadVersion()
	return Bundle{
		Name:         "NativeLaunchpad",
		SourceName:   "precompiles/launchpad/abi.json",
		ArtifactPath: "precompiles/launchpad/abi.json",
		ArtifactHash: version,
		ABIHash:      version,
		SourceHash:   version,
		Compiler:     "native",
		ABI:          nativeLaunchpadJSON,
	}
}
