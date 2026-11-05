//go:build cgo

package lxphost

import "testing"

func TestRetiredOracleAdapterSymbolsAbsent(t *testing.T) {
	symbols := kernelArchiveSymbols(t)
	for _, name := range []string{
		"lx_oracle_activity_encode",
		"lx_oracle_transport_encode",
		"lx_oracle_activity_encode_signed",
		"lx_oracle_adapter_run",
		"lx_oracle_adapter_isolation_check",
	} {
		if symbols[name] {
			t.Errorf("kernel archive defines retired oracle adapter symbol %s", name)
		}
	}
	for _, name := range []string{
		"lx_oracle_observation_encode",
		"lx_oracle_observation_sign",
	} {
		if !symbols[name] {
			t.Errorf("kernel archive does not define retained symbol %s", name)
		}
	}
}
