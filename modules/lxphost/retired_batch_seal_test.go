//go:build cgo

package lxphost

import "testing"

func TestRetiredBatchSealSymbolsAbsent(t *testing.T) {
	members := kernelArchiveMembers(t)
	if _, present := members["lxp_batch_seal.o"]; present {
		t.Error("kernel archive contains retired lxp_batch_seal.o member; rebuild the archive after removing its source")
	}

	symbols := kernelArchiveSymbols(t)
	for _, name := range []string{"lxp_batch_seal", "lxp_batch_sign"} {
		if symbols[name] {
			t.Errorf("kernel archive defines retired batch sealing symbol %s", name)
		}
	}
	for _, name := range []string{"lxp_batch_header_decode", "lxp_batch_verify_signature"} {
		if !symbols[name] {
			t.Errorf("kernel archive does not define retained symbol %s", name)
		}
		if !members["lxp_batch_header.o"][name] {
			t.Errorf("header codec archive member does not define retained symbol %s", name)
		}
	}
}
