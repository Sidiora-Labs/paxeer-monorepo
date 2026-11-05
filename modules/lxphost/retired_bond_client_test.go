//go:build cgo

package lxphost

import "testing"

func TestRetiredBondClientSymbolsAbsent(t *testing.T) {
	if _, present := kernelArchiveMembers(t)["lxp_bond_client.o"]; present {
		t.Error("kernel archive contains retired member lxp_bond_client.o")
	}

	symbols := kernelArchiveSymbols(t)
	for _, name := range []string{
		"lxp_paxeer_bond_init",
		"lxp_paxeer_bond_deposit",
		"lxp_paxeer_bond_deposit_proof",
		"lxp_paxeer_membership_commitment",
		"lxp_paxeer_membership_sync",
		"lxp_paxeer_membership_sync_status",
		"lxp_paxeer_bond_binding_encode",
		"lxp_paxeer_bond_binding_decode",
		"lxp_paxeer_bond_binding_adopt",
		"lxp_paxeer_bond_state_read",
		"lxp_paxeer_slash_submit",
	} {
		if symbols[name] {
			t.Errorf("kernel archive defines retired bond client symbol %s", name)
		}
	}
	if !symbols["lxp_batch_header_decode"] {
		t.Error("kernel archive does not define retained symbol lxp_batch_header_decode")
	}
}
