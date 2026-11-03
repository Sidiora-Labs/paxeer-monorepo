//go:build cgo

package lxphost

import "testing"

func TestRetiredHistorySymbolsAbsent(t *testing.T) {
	members := kernelArchiveMembers(t)
	for _, name := range []string{"lxp_history.o", "lxp_history_service.o"} {
		if _, present := members[name]; present {
			t.Errorf("kernel archive contains retired history member %s", name)
		}
	}
	symbols := kernelArchiveSymbols(t)
	for _, name := range []string{
		"lxp_history_index_rebuild",
		"lxp_history_open",
		"lxp_history_close",
		"lxp_history_query",
		"lxp_receipt_lookup",
		"lxp_history_serve_range",
	} {
		if symbols[name] {
			t.Errorf("kernel archive defines retired history symbol %s", name)
		}
	}
	if !symbols["lxp_log_read"] {
		t.Error("kernel archive does not define retained symbol lxp_log_read")
	}
}
