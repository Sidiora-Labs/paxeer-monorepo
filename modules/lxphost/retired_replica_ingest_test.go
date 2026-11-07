//go:build cgo

package lxphost

import "testing"

func TestRetiredReplicaIngestSymbolsAbsent(t *testing.T) {
	if _, present := kernelArchiveMembers(t)["lxp_replica_ingest.o"]; present {
		t.Error("kernel archive contains retired lxp_replica_ingest.o member; rebuild the archive after removing its source")
	}

	symbols := kernelArchiveSymbols(t)
	for _, name := range []string{
		"lxp_replica_init",
		"lxp_replica_bind_execution",
		"lxp_replica_bind_eligibility",
		"lxp_replica_batch_eligible",
		"lxp_replica_validate_header",
		"lxp_replica_chain_link",
		"lxp_replica_ingest_batch",
	} {
		if symbols[name] {
			t.Errorf("kernel archive defines retired replica ingest symbol %s", name)
		}
	}
	if !symbols["lxp_log_read"] {
		t.Error("kernel archive does not define retained symbol lxp_log_read")
	}
}
