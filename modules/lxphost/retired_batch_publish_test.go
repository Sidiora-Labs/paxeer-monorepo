//go:build cgo

package lxphost

import "testing"

func TestRetiredBatchPublishSymbolsAbsent(t *testing.T) {
	if _, present := kernelArchiveMembers(t)["lxp_batch_publish.o"]; present {
		t.Error("kernel archive contains retired lxp_batch_publish.o member; rebuild the archive after removing its source")
	}
	symbols := kernelArchiveSymbols(t)
	for _, name := range []string{
		"lxp_batch_body_encode",
		"lxp_batch_body_decode",
		"lxp_batch_publish",
	} {
		if symbols[name] {
			t.Errorf("kernel archive defines retired batch body archive codec or publisher symbol %s", name)
		}
	}
	if !symbols["lxp_batch_header_decode"] {
		t.Error("kernel archive does not define lxp_batch_header_decode")
	}
}
