package evmrpc_test

import (
	"strings"
	"testing"
	"time"

	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/stretchr/testify/require"
)

type httpWSParityServers struct {
	http *evmrpc.HTTPServer
	ws   *evmrpc.HTTPServer
}

func (s httpWSParityServers) HTTPMethods() []string {
	return s.http.HTTPMethods()
}

func (s httpWSParityServers) WSMethods() []string {
	return s.ws.WSMethods()
}

func TestHTTPAndWSAnswersMatch(t *testing.T) {
	httpServer, wsServer, _, _ := newRegistrationServers(t)
	servers := httpWSParityServers{http: httpServer, ws: wsServer}
	httpMethods := make(map[string]struct{})
	for _, method := range servers.HTTPMethods() {
		httpMethods[method] = struct{}{}
	}
	sharedMethods := make(map[string]struct{})
	for _, method := range servers.WSMethods() {
		if _, ok := httpMethods[method]; ok {
			sharedMethods[method] = struct{}{}
		}
	}

	for _, call := range []struct {
		method string
		params []interface{}
	}{
		{method: "eth_blockNumber"},
		{method: "eth_chainId"},
		{method: "eth_getBlockByNumber", params: []interface{}{"0x2", false}},
		{method: "eth_getBlockByHash", params: []interface{}{MultiTxBlockHash, false}},
		{method: "eth_getTransactionByHash", params: []interface{}{multiTxBlockTx1.Hash()}},
		{method: "eth_getTransactionReceipt", params: []interface{}{multiTxBlockTx1.Hash()}},
		{method: "eth_getBalance", params: []interface{}{"0x1234567890123456789023456789012345678901", "latest"}},
		{method: "eth_getCode", params: []interface{}{"0x1234567890123456789023456789012345678901", "latest"}},
		{method: "eth_getLogs", params: []interface{}{map[string]interface{}{
			"fromBlock": "0x2",
			"toBlock":   "0x2",
			"address":   "0x1111111111111111111111111111111111111112",
		}}},
		{method: "eth_feeHistory", params: []interface{}{"0x1", "0x2", []interface{}{50.0}}},
	} {
		t.Run(call.method, func(t *testing.T) {
			require.Contains(t, sharedMethods, call.method, "read method must be served on both transports")
			method := strings.TrimPrefix(call.method, "eth_")
			httpAnswer := sendRequestGood(t, method, call.params...)
			recv, done := sendWSRequestGood(t, method, call.params...)
			defer close(done)
			timer := time.NewTimer(3 * time.Second)
			defer timer.Stop()
			var wsAnswer map[string]interface{}
			select {
			case answer, ok := <-recv:
				require.True(t, ok, "WebSocket closed before returning an answer")
				wsAnswer = answer
			case <-timer.C:
				t.Fatal("timed out waiting for the WebSocket answer")
			}

			for transport, answer := range map[string]map[string]interface{}{
				"HTTP": httpAnswer, "WebSocket": wsAnswer,
			} {
				require.NotContains(t, answer, "error", "%s returned an RPC error: %v", transport, answer["error"])
				require.Contains(t, answer, "result", "%s must return a JSON result", transport)
				require.NotEmpty(t, answer["result"], "%s must return fixture data", transport)
			}
			require.Equal(t, httpAnswer["result"], wsAnswer["result"], "HTTP and WebSocket JSON results differ")
		})
	}
}
