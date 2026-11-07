//go:build cgo

package lxphost

import "testing"

func TestRetiredBatchTimeSymbolsAbsent(t *testing.T) {
	symbols := kernelArchiveSymbols(t)
	for _, name := range []string{
		"lxp_batch_timestamp_select",
		"lxp_batch_timestamp_validate",
	} {
		if symbols[name] {
			t.Errorf("kernel archive defines retired batch timestamp symbol %s", name)
		}
	}
	if !symbols["lxp_admit_activity"] {
		t.Error("kernel archive does not define retained symbol lxp_admit_activity")
	}
}
