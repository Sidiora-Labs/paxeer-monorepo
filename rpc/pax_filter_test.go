package evmrpc

import (
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func TestPaxFilterAPIMethodSetMatchesGate(t *testing.T) {
	f := &FilterAPI{}
	api := NewPaxFilterAPI(f)
	require.Same(t, f, api.f)

	want := []string{
		"pax_getFilterChanges",
		"pax_getFilterLogs",
		"pax_getLogs",
		"pax_newBlockFilter",
		"pax_newFilter",
		"pax_uninstallFilter",
	}
	var gated []string
	for _, method := range PaxLegacyAllGatedMethodNames() {
		if strings.HasPrefix(method, "pax_") && (strings.Contains(method, "Filter") || method == "pax_getLogs") {
			gated = append(gated, method)
		}
	}
	require.Equal(t, want, gated)
	require.Equal(t, gated, ServedMethods([]rpc.API{{Namespace: PaxNamespace, Service: api}}))
}
