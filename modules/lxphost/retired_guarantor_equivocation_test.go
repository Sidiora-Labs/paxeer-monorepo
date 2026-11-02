//go:build cgo

package lxphost

import "testing"

func TestRetiredGuarantorEquivocationSymbolsAbsent(t *testing.T) {
	if _, present := kernelArchiveMembers(t)["lxp_equivocation.o"]; present {
		t.Error("kernel archive contains retired member lxp_equivocation.o")
	}
	symbols := kernelArchiveSymbols(t)
	for _, name := range []string{
		"lxp_equivocation_detect",
		"lxp_equivocation_encode",
		"lxp_equivocation_verify",
	} {
		if symbols[name] {
			t.Errorf("kernel archive defines retired guarantor equivocation symbol %s", name)
		}
	}
	if !symbols["lxp_batch_header_decode"] {
		t.Error("kernel archive does not define retained symbol lxp_batch_header_decode")
	}
}
