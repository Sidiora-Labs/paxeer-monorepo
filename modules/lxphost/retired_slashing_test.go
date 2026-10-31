//go:build cgo

package lxphost

import "testing"

func TestRetiredSlashingSymbolsAbsent(t *testing.T) {
	members := kernelArchiveMembers(t)
	for _, name := range []string{"lxp_da_challenge_record.o", "lxp_slashing_submit.o"} {
		if _, present := members[name]; present {
			t.Errorf("kernel archive contains retired member %s", name)
		}
	}

	symbols := kernelArchiveSymbols(t)
	for _, name := range []string{
		"lxp_da_challenge_registry_init",
		"lxp_da_challenge_record_success",
		"lxp_da_challenge_record_failure",
		"lxp_slashing_submit",
	} {
		if symbols[name] {
			t.Errorf("kernel archive defines retired slashing symbol %s", name)
		}
	}
	if !symbols["lxp_batch_header_decode"] {
		t.Error("kernel archive does not define retained symbol lxp_batch_header_decode")
	}
}
