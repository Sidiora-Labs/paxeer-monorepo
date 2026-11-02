//go:build cgo

package lxphost

import "testing"

func TestRetiredPaxeerClientSymbolsAbsent(t *testing.T) {
	if _, present := kernelArchiveMembers(t)["lxp_paxeer_client.o"]; present {
		t.Error("kernel archive contains retired member lxp_paxeer_client.o")
	}
	symbols := kernelArchiveSymbols(t)
	for _, name := range []string{
		"lxp_paxeer_guarantor_attestation_from_core",
		"lxp_paxeer_custody_abi_init",
		"lxp_paxeer_verify_cert",
		"lxp_checkpoint_register",
		"lxp_paxeer_anchor_address",
		"lxp_paxeer_abi_selector",
		"lxp_paxeer_checkpoint_certificate_encode",
		"lxp_paxeer_abi_encode_bytes",
		"lxp_checkpoint_submit_calldata",
	} {
		if symbols[name] {
			t.Errorf("kernel archive defines retired Paxeer client symbol %s", name)
		}
	}
	if !symbols["lxp_batch_header_decode"] {
		t.Error("kernel archive does not define retained symbol lxp_batch_header_decode")
	}
}
