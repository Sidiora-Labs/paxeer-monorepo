//go:build cgo

package lxphost

import "testing"

func TestRetiredReplayFixtureSymbolsAbsent(t *testing.T) {
	if _, exists := kernelArchiveMembers(t)["lxp_replay_fixture.o"]; exists {
		t.Error("kernel archive contains retired replay fixture object lxp_replay_fixture.o")
	}
	symbols := kernelArchiveSymbols(t)
	for _, name := range []string{
		"lxp_replay_fixture_load",
		"lxp_replay_digest",
		"lxp_replay_crossarch_case",
	} {
		if symbols[name] {
			t.Errorf("kernel archive defines retired replay fixture symbol %s", name)
		}
	}
	if !symbols["lxp_log_read"] {
		t.Error("kernel archive does not define retained symbol lxp_log_read")
	}
}
