//go:build cgo

package lxphost

import "testing"

func TestRetiredReplaySectionsSymbolsAbsent(t *testing.T) {
	members := kernelArchiveMembers(t)
	for _, name := range []string{"lxp_replay_checkpoint.o", "lxp_replay_section.o"} {
		if _, exists := members[name]; exists {
			t.Errorf("kernel archive contains retired member %s; rebuild build/liblayerx.a", name)
		}
	}
	symbols := kernelArchiveSymbols(t)
	for _, name := range []string{
		"lxp_replay_checkpoint_destroy",
		"lxp_replay_checkpoint_create",
		"lxp_replay_checkpoint_restore",
		"lxp_replay_section_encode",
		"lxp_replay_section_decode",
	} {
		if symbols[name] {
			t.Errorf("kernel archive defines retired symbol %s", name)
		}
	}
	if !symbols["lxp_log_read"] {
		t.Error("kernel archive does not define lxp_log_read")
	}
}
