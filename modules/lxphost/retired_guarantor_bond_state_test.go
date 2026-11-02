//go:build cgo

package lxphost

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRetiredGuarantorBondStateSymbolsAbsent(t *testing.T) {
	source := filepath.Join("..", "..", "src", "guarantor", "lxp_bond_state.c")
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("retired guarantor bond state source must be absent: %s (stat error: %v)", source, err)
	}

	if _, present := kernelArchiveMembers(t)["lxp_bond_state.o"]; present {
		t.Error("kernel archive contains retired member lxp_bond_state.o")
	}

	symbols := kernelArchiveSymbols(t)
	for _, name := range []string{
		"lxp_guarantor_set_validate",
		"lxp_guarantor_set_init",
		"lxp_guarantor_set_apply",
		"lxp_guarantor_set_rotate_signer",
		"lxp_guarantor_signer_authorized",
		"lxp_guarantor_signer_at_epoch",
		"lxp_guarantor_eligible",
	} {
		if symbols[name] {
			t.Errorf("kernel archive defines retired guarantor bond state symbol %s", name)
		}
	}
	if !symbols["lxp_batch_header_decode"] {
		t.Error("kernel archive does not define retained symbol lxp_batch_header_decode")
	}
}
