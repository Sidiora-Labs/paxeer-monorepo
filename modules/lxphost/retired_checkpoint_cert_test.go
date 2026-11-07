//go:build cgo

package lxphost

import "testing"

func TestRetiredCheckpointCertSymbolsAbsent(t *testing.T) {
	symbols := kernelArchiveSymbols(t)
	for _, name := range []string{
		"lxp_checkpoint_maximum_attestation_delay_ms",
		"lxp_guarantor_attestation_verify",
		"lxp_checkpoint_certificate_hash",
		"lxp_guarantor_cert_assemble",
		"lxp_guarantor_cert_verify",
		"lxp_receipt_augment",
	} {
		if symbols[name] {
			t.Errorf("kernel archive defines retired checkpoint certificate symbol %s", name)
		}
	}
	for _, name := range []string{"lxp_batch_header_decode", "lxp_receipt_decode"} {
		if !symbols[name] {
			t.Errorf("kernel archive does not define %s", name)
		}
	}
}
