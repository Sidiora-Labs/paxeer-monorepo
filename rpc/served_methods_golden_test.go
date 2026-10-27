package evmrpc_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type HTTPMethods interface {
	HTTPMethods() []string
}

type WSMethods interface {
	WSMethods() []string
}

func TestServedMethodsHTTPGolden(t *testing.T) {
	httpServer, _, _, _ := newRegistrationServers(t)
	var server HTTPMethods = httpServer
	assertServedMethodsGolden(t, "testdata/served_methods_http.golden", server.HTTPMethods())
}

func TestServedMethodsWSGolden(t *testing.T) {
	_, wsServer, _, _ := newRegistrationServers(t)
	var server WSMethods = wsServer
	methods := server.WSMethods()
	assertServedMethodsGolden(t, "testdata/served_methods_ws.golden", methods)
	for _, method := range []string{
		"eth_subscribe:newHeads",
		"eth_subscribe:logs",
		"eth_subscribe:newPendingTransactions",
	} {
		require.Contains(t, methods, method)
	}
}

func assertServedMethodsGolden(t *testing.T, path string, methods []string) {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NotEmpty(t, data, "%s must contain the frozen method set", path)
	require.True(t, strings.HasSuffix(string(data), "\n"), "%s must end with a newline", path)
	golden := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	for i, method := range golden {
		require.NotEmpty(t, method, "%s:%d must contain a method", path, i+1)
		require.Equal(t, method, strings.TrimSpace(method), "%s:%d has surrounding whitespace", path, i+1)
		if i > 0 {
			require.Less(t, golden[i-1], method, "%s:%d must be sorted and unique", path, i+1)
		}
		assertServedMethodAllowed(t, method)
	}
	for _, method := range methods {
		assertServedMethodAllowed(t, method)
	}
	require.Len(t, methods, len(golden), "%s differs from the served method set: %v", path, methods)
	for i, method := range golden {
		require.Equal(t, method, methods[i], "%s:%d differs from the served method", path, i+1)
	}
}

func assertServedMethodAllowed(t *testing.T, method string) {
	t.Helper()
	require.False(t, strings.HasPrefix(method, "lx_"), "decision 701 forbids %s", method)
	require.False(t, strings.HasSuffix(method, "ForTest"), "test helper %s must not be served or frozen in a golden", method)
	switch method {
	case "eth_cleanup", "eth_setReceiptLogFeed", "eth_gasPriceHelper", "eth_calculateGasUsedRatio":
		t.Fatalf("retired helper %s must not be served or frozen in a golden", method)
	}
}
