//go:build cgo

package lxphost

import "testing"

func TestRetiredDaVerifiedSymbolsAbsent(t *testing.T) {
	if _, exists := kernelArchiveMembers(t)["lxp_da_verified.o"]; exists {
		t.Error("kernel archive contains retired storage member lxp_da_verified.o")
	}
	symbols := kernelArchiveSymbols(t)
	for _, name := range []string{
		"lxp_da_store_read_verified",
		"lxp_da_log_read_body",
		"lxp_da_log_store_body",
	} {
		if symbols[name] {
			t.Errorf("kernel archive defines retired verified data-availability storage symbol %s", name)
		}
	}
	for _, name := range []string{"lxp_log_append", "lxp_log_read"} {
		if !symbols[name] {
			t.Errorf("kernel archive does not define retained symbol %s", name)
		}
	}
}
