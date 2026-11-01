//go:build cgo

package lxphost

import "testing"

func TestRetiredOracleRootSymbolsAbsent(t *testing.T) {
	symbols := kernelArchiveSymbols(t)
	for _, name := range []string{
		"lx_oracle_leaf_encode",
		"lx_oracle_availability_bundle_build",
		"lx_oracle_root_from_availability",
		"lx_oracle_root_compute",
		"lx_batch_header_set_oracle_root",
	} {
		if symbols[name] {
			t.Errorf("kernel archive defines retired oracle root symbol %s", name)
		}
	}
	for _, name := range []string{
		"lx_oracle_observation_encode",
		"lxp_batch_header_decode",
	} {
		if !symbols[name] {
			t.Errorf("kernel archive does not define retained symbol %s", name)
		}
	}
}
