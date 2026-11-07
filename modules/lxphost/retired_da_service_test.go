//go:build cgo

package lxphost

import "testing"

func TestRetiredDaServiceSymbolsAbsent(t *testing.T) {
	if _, present := kernelArchiveMembers(t)["lxp_da_service.o"]; present {
		t.Error("kernel archive contains retired member lxp_da_service.o")
	}
	symbols := kernelArchiveSymbols(t)
	for _, name := range []string{
		"lxp_da_serve_chunk",
		"lxp_da_serve_chunk_proof",
	} {
		if symbols[name] {
			t.Errorf("kernel archive defines retired data-availability service symbol %s", name)
		}
	}
	if !symbols["lxp_log_append"] {
		t.Error("kernel archive does not define retained symbol lxp_log_append")
	}
}
