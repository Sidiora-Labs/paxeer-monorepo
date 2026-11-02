package exportcheck_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/exportcheck"
)

func TestPackageExportsReadsCompiledCodec(t *testing.T) {
	const importPath = "github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	exports, err := exportcheck.PackageExports(importPath)
	if err != nil {
		t.Fatalf("PackageExports(%q): %v", importPath, err)
	}
	for _, name := range []string{"MerkleNodeHash", "Receipt.UnsignedBytes", "IsProtocolReceiptShape"} {
		if !exports[name] {
			t.Errorf("PackageExports(%q) missing exported identifier %q", importPath, name)
		}
	}
	if _, present := exports["foldPath"]; present {
		t.Errorf("PackageExports(%q) contains unexported identifier foldPath", importPath)
	}
}
