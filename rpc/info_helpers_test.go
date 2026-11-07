package evmrpc_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInfoHelpersNotServed(t *testing.T) {
	for _, method := range []string{"gasPriceHelper", "calculateGasUsedRatio"} {
		t.Run(method, func(t *testing.T) {
			response := sendRequestGood(t, method)
			require.Equal(t, "2.0", response["jsonrpc"])
			require.Equal(t, "test", response["id"])
			require.NotContains(t, response, "result")
			errorObject, ok := response["error"].(map[string]interface{})
			require.True(t, ok, "expected a JSON-RPC error, got %v", response)
			require.Equal(t, float64(-32601), errorObject["code"])
		})
	}
}
