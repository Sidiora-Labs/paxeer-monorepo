//go:build cgo

package lxphost

import "testing"

func TestRetiredReplayEngineSymbolsAbsent(t *testing.T) {
	if _, exists := kernelArchiveMembers(t)["lxp_replay.o"]; exists {
		t.Error("kernel archive contains retired replay engine object lxp_replay.o")
	}

	symbols := kernelArchiveSymbols(t)
	for _, name := range []string{
		"lxp_replay_engine_init",
		"lxp_replay_engine_bind_transaction",
		"lxp_replay_engine_bind_kernel",
		"lxp_replay_engine_register",
		"lxp_replay_engine_register_batch_finalizer",
		"lxp_replay_batch",
		"lxp_replay_batch_publication",
		"lxp_replay_verify_roots",
	} {
		if symbols[name] {
			t.Errorf("kernel archive defines retired replay engine symbol %s", name)
		}
	}
	for _, name := range []string{"lxp_log_read", "lxp_kernel_create"} {
		if !symbols[name] {
			t.Errorf("kernel archive does not define retained symbol %s", name)
		}
	}
}
