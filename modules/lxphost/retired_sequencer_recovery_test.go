//go:build cgo

package lxphost

import "testing"

func TestRetiredSequencerRecoverySymbolsAbsent(t *testing.T) {
	if _, present := kernelArchiveMembers(t)["lxp_recovery.o"]; present {
		t.Error("kernel archive contains retired lxp_recovery.o member; rebuild the archive after removing its source")
	}

	symbols := kernelArchiveSymbols(t)
	for _, name := range []string{
		"lxp_sequencer_recover",
		"lxp_sequencer_header_registry_init",
		"lxp_sequencer_equivocation_detect",
		"lxp_sequencer_loss",
		"lxp_sequencer_handover_authorize",
		"lxp_sequencer_can_seal",
	} {
		if symbols[name] {
			t.Errorf("kernel archive defines retired sequencer recovery symbol %s", name)
		}
	}
	if !symbols["lxp_admit_activity"] {
		t.Error("kernel archive does not define retained symbol lxp_admit_activity")
	}
}
