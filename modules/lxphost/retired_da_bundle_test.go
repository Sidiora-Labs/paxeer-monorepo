//go:build cgo

package lxphost

import "testing"

func TestRetiredDaBundleSymbolsAbsent(t *testing.T) {
	symbols := kernelArchiveSymbols(t)
	for _, name := range []string{
		"lxp_da_chunk_hash",
		"lxp_da_recovery_metadata_encode",
		"lxp_da_bundle_build",
		"lxp_da_bundle_root",
		"lxp_batch_availability_root",
		"lxp_da_receipt_section_encode",
		"lxp_da_receipt_section_decode",
		"lxp_da_bundle_body",
	} {
		if symbols[name] {
			t.Errorf("kernel archive defines retired data-availability bundle symbol %s", name)
		}
	}
	if !symbols["lxp_log_append"] {
		t.Error("kernel archive does not define retained symbol lxp_log_append")
	}
}
