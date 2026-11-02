package exportcheck_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/exportcheck"
)

func TestCustodyproofLightExportsAbsent(t *testing.T) {
	const importPath = "github.com/Sidiora-Labs/Paxeer-X-Network/custodyproof"
	exports, err := exportcheck.PackageExports(importPath)
	if err != nil {
		t.Fatalf("PackageExports(%q): %v", importPath, err)
	}
	for _, name := range []string{"LightProfile", "LightEvidence", "LightTrust"} {
		if exports[name] {
			t.Errorf("PackageExports(%q) contains retired exported identifier %q", importPath, name)
		}
	}
	if !exports["Decode"] {
		t.Errorf("PackageExports(%q) missing exported identifier Decode", importPath)
	}
}
