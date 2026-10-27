//go:build cgo

package lxphost

import "testing"

func TestRetiredGatewayGrantSymbolsAbsent(t *testing.T) {
	symbols := kernelArchiveSymbols(t)
	for _, name := range []string{
		"lxp_gateway_receive_claim",
		"lxp_gateway_grant_bounds_check",
		"lxp_gateway_grant_present_test_locked",
	} {
		if symbols[name] {
			t.Errorf("kernel archive defines retired gateway symbol %s", name)
		}
	}
	for _, name := range []string{
		"lxp_gateway_send_settle",
		"lxp_gateway_invoice_registry_create",
		"lxp_gateway_invoice_registry_destroy",
		"lxp_gateway_invoice_state",
	} {
		if !symbols[name] {
			t.Errorf("kernel archive does not define retained gateway symbol %s", name)
		}
	}
}
