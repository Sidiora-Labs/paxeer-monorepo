//go:build cgo

package lxphost

import "testing"

func TestRetiredFinalisationSymbolsAbsent(t *testing.T) {
	if _, present := kernelArchiveMembers(t)["lxp_finalisation.o"]; present {
		t.Error("kernel archive contains retired lxp_finalisation.o member; rebuild the archive after removing its source")
	}
	symbols := kernelArchiveSymbols(t)
	for _, name := range []string{
		"lxp_da_withhold_sim",
		"lxp_checkpoint_block_on_da",
		"lxp_da_unavailable_mode",
		"lxp_checkpoint_finalisable",
	} {
		if symbols[name] {
			t.Errorf("kernel archive defines retired checkpoint finalisation symbol %s", name)
		}
	}
	if !symbols["lxp_batch_header_decode"] {
		t.Error("kernel archive does not define retained symbol lxp_batch_header_decode")
	}
}
