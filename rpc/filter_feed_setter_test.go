package evmrpc_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetReceiptLogFeedNotServed(t *testing.T) {
	for _, namespace := range []string{"eth", "pax"} {
		t.Run(namespace, func(t *testing.T) {
			response := sendRequestGoodWithNamespace(t, namespace, "setReceiptLogFeed", map[string]interface{}{})
			require.NotContains(t, response, "result")
			errorObject, ok := response["error"].(map[string]interface{})
			require.True(t, ok, "expected a JSON-RPC error, got %v", response)
			require.Equal(t, float64(-32601), errorObject["code"])
		})
	}
}
