//go:build cgo

package lxphost

import "testing"

func TestRetiredAdmissionQueueSymbolsAbsent(t *testing.T) {
	symbols := kernelArchiveSymbols(t)
	for _, name := range []string{
		"lxp_admission_queue_init",
		"lxp_admission_queue_push",
		"lxp_admission_queue_pop",
	} {
		if symbols[name] {
			t.Errorf("kernel archive defines retired admission queue symbol %s", name)
		}
	}
	for _, name := range []string{
		"lxp_admit_activity",
		"lxp_activity_check_timestamp_bound",
	} {
		if !symbols[name] {
			t.Errorf("kernel archive does not define retained admission symbol %s", name)
		}
	}
}
