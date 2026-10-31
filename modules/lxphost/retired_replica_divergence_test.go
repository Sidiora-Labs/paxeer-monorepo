//go:build cgo

package lxphost

import "testing"

func TestRetiredReplicaDivergenceSymbolsAbsent(t *testing.T) {
	symbols := kernelArchiveSymbols(t)
	for _, name := range []string{
		"lxp_divergence_detect",
		"lxp_divergence_report",
		"lxp_divergence_report_verify",
		"lxp_replica_halt",
	} {
		if symbols[name] {
			t.Errorf("kernel archive defines retired replica divergence symbol %s", name)
		}
	}
	if !symbols["lxp_log_read"] {
		t.Error("kernel archive does not define retained symbol lxp_log_read")
	}
}
