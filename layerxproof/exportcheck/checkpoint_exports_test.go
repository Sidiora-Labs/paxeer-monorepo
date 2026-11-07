package exportcheck_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/exportcheck"
)

func TestCodecCheckpointExportsAbsent(t *testing.T) {
	const importPath = "github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	exports, err := exportcheck.PackageExports(importPath)
	if err != nil {
		t.Fatalf("PackageExports(%q): %v", importPath, err)
	}
	for _, name := range []string{
		"DecodeGuarantorAttestation",
		"DecodeCheckpointCertificate",
		"MaxValidityProofBytes",
		"MaxGuarantorAttestations",
		"AvailabilityClassCount",
	} {
		if exports[name] {
			t.Errorf("PackageExports(%q) contains retired exported identifier %q", importPath, name)
		}
	}
	for _, name := range []string{"CheckpointHash", "GuarantorAttestation", "SettlementReference", "CheckpointCertificate"} {
		if !exports[name] {
			t.Errorf("PackageExports(%q) missing exported identifier %q", importPath, name)
		}
	}
}

func TestVerifyCheckpointExportsAbsent(t *testing.T) {
	const importPath = "github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	exports, err := exportcheck.PackageExports(importPath)
	if err != nil {
		t.Fatalf("PackageExports(%q): %v", importPath, err)
	}
	for _, name := range []string{"SettlementDomain", "Attestation", "CheckpointCertificate", "GuarantorEquivocation"} {
		if exports[name] {
			t.Errorf("PackageExports(%q) contains retired exported identifier %q", importPath, name)
		}
	}
	for _, name := range []string{
		"GuarantorSignature",
		"ErrAttestationBinding",
		"ErrAttestationDomain",
		"ErrAttestationDuty",
		"ErrAttestationFreshness",
		"ErrGuarantorSignature",
		"ErrGuarantorUnknown",
		"ErrSettlementReference",
		"ErrEquivocationShape",
	} {
		if !exports[name] {
			t.Errorf("PackageExports(%q) missing exported identifier %q", importPath, name)
		}
	}
}
