//go:build cgo

package lxphost

import "testing"

func TestRetiredDaKernelSymbolsAbsent(t *testing.T) {
	if _, present := kernelArchiveMembers(t)["lxp_da_kernel.o"]; present {
		t.Error("kernel archive contains retired lxp_da_kernel.o member; rebuild the archive after removing its source")
	}

	symbols := kernelArchiveSymbols(t)
	for _, name := range []string{
		"lxp_da_recovery_from_kernel",
		"lxp_da_recovery_verify_kernel",
		"lxp_da_body_from_kernels",
	} {
		if symbols[name] {
			t.Errorf("kernel archive defines retired data-availability kernel symbol %s", name)
		}
	}
	if !symbols["lxp_log_append"] {
		t.Error("kernel archive does not define retained symbol lxp_log_append")
	}
}
