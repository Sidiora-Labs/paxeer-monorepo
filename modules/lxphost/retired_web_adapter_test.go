//go:build cgo

package lxphost

import "testing"

func TestRetiredWebAdapterSymbolsAbsent(t *testing.T) {
	symbols := kernelArchiveSymbols(t)
	for _, name := range []string{
		"lx_web_observation_encode",
		"lx_web_activity_encode",
		"lx_web_adapter_run",
		"lx_web_submission_prepare",
		"lx_web_submission_check",
		"lx_web_submission_recover",
		"lx_web_adapter_run_durable",
	} {
		if symbols[name] {
			t.Errorf("kernel archive defines retired web adapter symbol %s", name)
		}
	}
	if !symbols["lxp_kernel_create"] {
		t.Error("kernel archive does not define retained symbol lxp_kernel_create")
	}
}
