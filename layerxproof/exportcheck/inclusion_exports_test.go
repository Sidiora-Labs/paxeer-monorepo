package exportcheck_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/exportcheck"
)

func TestVerifyInclusionExportsAbsent(t *testing.T) {
	for _, tc := range []struct {
		importPath string
		absent     []string
		present    []string
	}{
		{
			importPath: "github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify",
			absent: []string{
				"BatchHeader",
				"ReceiptInclusion",
				"StateProofAtHeader",
				"DiscoveryProof",
				"Ed25519Domain",
				"WithdrawalInclusion",
				"ExitBalance",
				"WithdrawalNullifierDomain",
			},
			present: []string{
				"ReceiptSignature",
				"ReceiptAtRoot",
				"StateProof",
				"Ed25519Digest",
				"WithdrawalOutcome",
				"WithdrawalEffect",
			},
		},
		{
			importPath: "github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec",
			absent:     []string{"Domain.Tag"},
			present:    []string{"Domain", "DomainHash"},
		},
	} {
		t.Run(tc.importPath, func(t *testing.T) {
			exports, err := exportcheck.PackageExports(tc.importPath)
			if err != nil {
				t.Fatalf("PackageExports(%q): %v", tc.importPath, err)
			}
			for _, name := range tc.absent {
				if exports[name] {
					t.Errorf("PackageExports(%q) contains retired exported identifier %q", tc.importPath, name)
				}
			}
			for _, name := range tc.present {
				if !exports[name] {
					t.Errorf("PackageExports(%q) missing exported identifier %q", tc.importPath, name)
				}
			}
		})
	}
}
