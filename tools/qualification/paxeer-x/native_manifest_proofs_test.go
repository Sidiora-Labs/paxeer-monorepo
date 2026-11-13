package qualification

import (
	"os"
	"regexp"
	"testing"
)

// The task's corrected gate is Python (gotcha 2984). Keep the generated Go
// path as the finding-to-proof record; native lifecycle gates are not run here.
var nativeManifestProofs = map[string]string{
	"f014": "test_f014_retained_transport",
	"f060": "test_f060_manifest_round_trip",
	"f015": "test_f015_chain_commitment",
	"f045": "test_f045_projection_manifest",
	"f020": "test_f020_transitive_inventory",
	"f023": "test_f023_retained_executor",
	"f046": "test_f046_peer_manifest",
	"f047": "test_f047_presentation_manifest",
}

func TestNativeManifestProofInventory(t *testing.T) {
	source, err := os.ReadFile("test_native_manifest_contracts.py")
	if err != nil {
		t.Fatal(err)
	}
	if len(nativeManifestProofs) != 8 {
		t.Fatal("all eight findings require a declared behavioral proof")
	}
	for finding, name := range nativeManifestProofs {
		t.Run(finding, func(t *testing.T) {
			declaration := regexp.MustCompile(`(?m)^def ` + regexp.QuoteMeta(name) + `\(`)
			if !declaration.Match(source) {
				t.Fatalf("%s has no declared Python proof %s", finding, name)
			}
		})
	}
}
