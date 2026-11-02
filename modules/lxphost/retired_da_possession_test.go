//go:build cgo

package lxphost

import "testing"

func TestRetiredDaPossessionSymbolsAbsent(t *testing.T) {
	if _, present := kernelArchiveMembers(t)["lxp_da_possession.o"]; present {
		t.Error("kernel archive contains retired member lxp_da_possession.o")
	}

	symbols := kernelArchiveSymbols(t)
	for _, name := range []string{
		"lxp_da_store_init",
		"lxp_da_store_bundle",
		"lxp_da_store_read_bundle",
		"lxp_da_possession_verify",
		"lxp_da_possession_attest",
	} {
		if symbols[name] {
			t.Errorf("kernel archive defines retired data-availability possession symbol %s", name)
		}
	}
	if !symbols["lxp_log_read"] {
		t.Error("kernel archive does not define retained symbol lxp_log_read")
	}
}
