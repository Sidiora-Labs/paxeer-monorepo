//go:build cgo

package lxphost

import "testing"

func TestRetiredGuarantorAttestationSymbolsAbsent(t *testing.T) {
	if _, present := kernelArchiveMembers(t)["lxp_attestation.o"]; present {
		t.Error("kernel archive contains retired member lxp_attestation.o")
	}
	symbols := kernelArchiveSymbols(t)
	for _, name := range []string{
		"lxp_secp256k1_sign",
		"lxp_guarantor_attest",
		"lxp_guarantor_attestation_verify",
	} {
		if symbols[name] {
			t.Errorf("kernel archive defines retired guarantor attestation symbol %s", name)
		}
	}
	for _, name := range []string{
		"lxp_secp256k1_verify",
		"lxp_batch_header_decode",
	} {
		if !symbols[name] {
			t.Errorf("kernel archive does not define retained symbol %s", name)
		}
	}
}
