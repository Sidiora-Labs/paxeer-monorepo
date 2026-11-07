//go:build cgo

package lxphost

import "testing"

func TestRetiredGuarantorDissentSymbolsAbsent(t *testing.T) {
	if _, present := kernelArchiveMembers(t)["lxp_dissent.o"]; present {
		t.Error("kernel archive contains retired member lxp_dissent.o")
	}
	symbols := kernelArchiveSymbols(t)
	for _, name := range []string{
		"lxp_guarantor_first_divergence",
		"lxp_guarantor_dissent",
		"lxp_guarantor_dissent_verify",
		"lxp_guarantor_withhold",
	} {
		if symbols[name] {
			t.Errorf("kernel archive defines retired guarantor symbol %s", name)
		}
	}
	if !symbols["lxp_batch_header_decode"] {
		t.Error("kernel archive does not define retained symbol lxp_batch_header_decode")
	}
}
