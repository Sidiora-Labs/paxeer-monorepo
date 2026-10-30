//go:build cgo

package lxphost

import "testing"

func TestRetiredDaChallengeSymbolsAbsent(t *testing.T) {
	symbols := kernelArchiveSymbols(t)
	for _, name := range []string{
		"lxp_da_challenge_indices",
		"lxp_da_challenge_issue",
		"lxp_da_challenge_respond",
		"lxp_da_challenge_judge",
	} {
		if symbols[name] {
			t.Errorf("kernel archive defines retired data-availability challenge symbol %s", name)
		}
	}
	if !symbols["lxp_log_read"] {
		t.Error("kernel archive does not define retained symbol lxp_log_read")
	}
}
