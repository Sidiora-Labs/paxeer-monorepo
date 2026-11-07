// Decision 701: lx_* methods are switched off at the node. The gateway
// switch-off at services/platform/hosted/gateway/src/rpc.rs:237 belongs to domain 8.
package evmrpc_test

import (
	"context"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

// These are the 26 retired names in spec/layerx-merge/contracts/clients.md.
var lxSwitchOffMethods = [26]string{
	"lx_sendActivity",
	"lx_getSequence",
	"lx_getAccount",
	"lx_getBalance",
	"lx_getBalances",
	"lx_getReceipt",
	"lx_getActivityStatus",
	"lx_getBatchHeader",
	"lx_getCheckpoint",
	"lx_getProof",
	"lx_getAsset",
	"lx_listAssets",
	"lx_estimateFee",
	"lx_getProgramEvents",
	"lx_getNodeInfo",
	"lx_getWalletCaps",
	"lx_getHistory",
	"lx_register",
	"lx_requestFunds",
	"lx_subscribe",
	"lx_unsubscribe",
	"lx_subscription",
	"lx_getArchiveNetwork",
	"lx_getArchiveHead",
	"lx_listActivities",
	"lx_listBatches",
}

func TestLxMethodsNotServedHTTP(t *testing.T) {
	type HTTPMethods interface {
		HTTPMethods() []string
	}
	httpServer, _, httpClient, _ := newRegistrationServers(t)
	var server HTTPMethods = httpServer
	methods := server.HTTPMethods()
	require.NotEmpty(t, methods)
	for _, method := range lxSwitchOffMethods {
		t.Run(method, func(t *testing.T) {
			require.NotContains(t, methods, method)
			assertLxMethodNotFound(t, httpClient, method)
		})
	}
}

func TestLxMethodsNotServedWS(t *testing.T) {
	type WSMethods interface {
		WSMethods() []string
	}
	_, wsServer, _, wsClient := newRegistrationServers(t)
	var server WSMethods = wsServer
	methods := server.WSMethods()
	require.NotEmpty(t, methods)
	for _, method := range lxSwitchOffMethods {
		t.Run(method, func(t *testing.T) {
			require.NotContains(t, methods, method)
			assertLxMethodNotFound(t, wsClient, method)

			// The setup helper hardcodes eth_, so also check its prefixed alias.
			// The call above sends the exact lx_* name over WebSocket.
			var params []interface{}
			if method == "lx_subscribe" || method == "lx_unsubscribe" {
				params = []interface{}{method}
			}
			recv, done := sendWSRequestGood(t, method, params...)
			defer close(done)
			select {
			case response, ok := <-recv:
				require.True(t, ok, "WebSocket closed without answering %s", method)
				require.Equal(t, "2.0", response["jsonrpc"])
				require.Equal(t, "test", response["id"])
				require.NotContains(t, response, "result")
				errorObject, ok := response["error"].(map[string]interface{})
				require.True(t, ok, "expected a JSON-RPC error, got %v", response)
				require.Equal(t, float64(-32601), errorObject["code"])
			case <-time.After(5 * time.Second):
				t.Fatalf("timed out waiting for WebSocket refusal of eth_%s", method)
			}
		})
	}
}

func assertLxMethodNotFound(t *testing.T, client *rpc.Client, method string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var params []interface{}
	switch method {
	case "lx_subscribe":
		params = []interface{}{"newHeads"}
	case "lx_unsubscribe":
		params = []interface{}{"0x1"}
	}
	var result interface{}
	err := client.CallContext(ctx, &result, method, params...)
	var rpcError rpc.Error
	require.ErrorAs(t, err, &rpcError, "expected a JSON-RPC refusal of %s", method)
	require.Equal(t, -32601, rpcError.ErrorCode(), method)
	require.Nil(t, result, method)
}
