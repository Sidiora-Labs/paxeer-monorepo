//go:build cgo

package lxphost

import "testing"

func TestRetiredGuarantorProcessSymbolsAbsent(t *testing.T) {
	if _, present := kernelArchiveMembers(t)["lxp_guarantor.o"]; present {
		t.Error("kernel archive contains retired member lxp_guarantor.o")
	}

	symbols := kernelArchiveSymbols(t)
	for _, name := range []string{
		"lxp_guarantor_verify_signatures",
		"lxp_guarantor_recompute_roots",
		"lxp_guarantor_process_batch",
	} {
		if symbols[name] {
			t.Errorf("kernel archive defines retired guarantor symbol %s", name)
		}
	}
	if !symbols["lxp_batch_header_decode"] {
		t.Error("kernel archive does not define retained symbol lxp_batch_header_decode")
	}
}
