//go:build cgo

package lxphost

import "testing"

func TestRetiredBatchEligibilitySymbolsAbsent(t *testing.T) {
	if _, present := kernelArchiveMembers(t)["lxp_eligibility.o"]; present {
		t.Error("kernel archive contains retired lxp_eligibility.o member; rebuild the archive after removing its source")
	}
	symbols := kernelArchiveSymbols(t)
	for _, name := range []string{
		"lxp_batch_eligibility_init",
		"lxp_batch_eligibility_reset",
		"lxp_replica_ack",
		"lxp_batch_eligibility",
	} {
		if symbols[name] {
			t.Errorf("kernel archive defines retired replica acknowledgement eligibility symbol %s", name)
		}
	}
	if !symbols["lxp_batch_header_decode"] {
		t.Error("kernel archive does not define lxp_batch_header_decode")
	}
}
