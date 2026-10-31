//go:build cgo

package lxphost

import "testing"

func TestRetiredWebRootSymbolsAbsent(t *testing.T) {
	symbols := kernelArchiveSymbols(t)
	for _, name := range []string{
		"lx_web_leaf_encode",
		"lx_web_availability_bundle_build",
		"lx_web_root_from_availability",
		"lx_web_root",
		"lx_batch_header_set_web_root",
	} {
		if symbols[name] {
			t.Errorf("kernel archive defines retired web root symbol %s", name)
		}
	}
	if !symbols["lxp_kernel_create"] {
		t.Error("kernel archive does not define retained symbol lxp_kernel_create")
	}
}
