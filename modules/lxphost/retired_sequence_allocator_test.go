//go:build cgo

package lxphost

import "testing"

func TestRetiredSequenceAllocatorSymbolsAbsent(t *testing.T) {
	symbols := kernelArchiveSymbols(t)
	for _, name := range []string{
		"lxp_seq_allocator_init",
		"lxp_seq_assign",
		"lxp_batch_range_check",
	} {
		if symbols[name] {
			t.Errorf("kernel archive defines retired sequence allocator symbol %s", name)
		}
	}
	if !symbols["lxp_admit_activity"] {
		t.Error("kernel archive does not define retained symbol lxp_admit_activity")
	}
}
