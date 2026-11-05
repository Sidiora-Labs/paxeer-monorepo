package evmrpc_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	evmrpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
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

func TestHTTPAppliesDenyListMixedBatches(t *testing.T) {
	testApp := app.Setup(t, false, false, false)
	config := evmrpcconfig.DefaultConfig
	config.HTTPAddress = TestAddr
	config.HTTPPort = 0
	config.FilterTimeout = time.Minute
	config.DenyList = []string{"eth_chainId", "eth_getBalance", "eth_subscribe", "eth_unsubscribe"}
	ctxProvider := func(int64) sdk.Context {
		return Ctx.WithBlockHeight(MockHeight103).WithIsTracing(true).WithClosestUpgradeName(LatestCtxUpgradeName)
	}
	txConfigProvider := func(int64) client.TxConfig { return TxConfig }
	service, err := evmrpc.NewEVMHTTPServer(config, &MockClient{}, EVMKeeper, testApp.BeginBlockKeepers,
		testApp.BaseApp, testApp.TracerAnteHandler, ctxProvider, txConfigProvider, "", nil)
	require.NoError(t, err)
	t.Cleanup(service.Stop)
	server := service.(*evmrpc.HTTPServer)
	require.NoError(t, server.Start())
	endpoint := "http://" + server.ListenAddr()
	rpcClient, err := rpc.DialHTTP(endpoint)
	require.NoError(t, err)
	t.Cleanup(rpcClient.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var blockNumber hexutil.Uint64
	require.NoError(t, rpcClient.CallContext(ctx, &blockNumber, "eth_blockNumber"))
	blockResult, err := json.Marshal(blockNumber)
	require.NoError(t, err)
	httpClient := &http.Client{Timeout: 5 * time.Second}
	t.Cleanup(httpClient.CloseIdleConnections)
	exchange := func(t *testing.T, body, want string) {
		t.Helper()
		response, err := httpClient.Post(endpoint, "application/json", strings.NewReader(body))
		require.NoError(t, err)
		defer response.Body.Close()
		reply, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode, "%s", reply)
		decode := func(body string) interface{} {
			t.Helper()
			decoder := json.NewDecoder(strings.NewReader(body))
			decoder.UseNumber()
			var value interface{}
			require.NoError(t, decoder.Decode(&value), "%s", body)
			var extra interface{}
			require.ErrorIs(t, decoder.Decode(&extra), io.EOF, "%s", body)
			return value
		}
		require.Equal(t, decode(want), decode(string(reply)), "%s", reply)
	}
	for _, tc := range []struct{ name, body, want string }{
		{
			name: "denied_then_allowed",
			body: `[{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]},{"jsonrpc":"2.0","id":2,"method":"eth_blockNumber","params":[]}]`,
			want: fmt.Sprintf(`[{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"the method eth_chainId does not exist/is not available"}},{"jsonrpc":"2.0","id":2,"result":%s}]`, blockResult),
		},
		{
			name: "allowed_denied_allowed_duplicate_ids",
			body: `[{"jsonrpc":"2.0","id":"same","method":"echo_echo","params":["before"]},{"jsonrpc":"2.0","id":"same","method":"eth_chainId"},{"jsonrpc":"2.0","id":"same","method":"echo_echo","params":["after"]}]`,
			want: `[{"jsonrpc":"2.0","id":"same","result":"before"},{"jsonrpc":"2.0","id":"same","error":{"code":-32601,"message":"the method eth_chainId does not exist/is not available"}},{"jsonrpc":"2.0","id":"same","result":"after"}]`,
		},
		{
			name: "all_denied_distinct_methods_and_id_types",
			body: `[{"jsonrpc":"2.0","id":null,"method":"eth_chainId"},{"jsonrpc":"2.0","id":9007199254740993,"method":"eth_getBalance"},{"jsonrpc":"2.0","id":"text","method":"eth_chainId"}]`,
			want: `[{"jsonrpc":"2.0","id":null,"error":{"code":-32601,"message":"the method eth_chainId does not exist/is not available"}},{"jsonrpc":"2.0","id":9007199254740993,"error":{"code":-32601,"message":"the method eth_getBalance does not exist/is not available"}},{"jsonrpc":"2.0","id":"text","error":{"code":-32601,"message":"the method eth_chainId does not exist/is not available"}}]`,
		},
		{
			name: "denied_subscription_builtins",
			body: `[{"jsonrpc":"2.0","id":1,"method":"eth_subscribe","params":["newHeads"]},{"jsonrpc":"2.0","id":2,"method":"eth_unsubscribe","params":["0x1"]},{"jsonrpc":"2.0","id":3,"method":"echo_echo","params":["allowed"]}]`,
			want: `[{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"the method eth_subscribe does not exist/is not available"}},{"jsonrpc":"2.0","id":2,"error":{"code":-32601,"message":"the method eth_unsubscribe does not exist/is not available"}},{"jsonrpc":"2.0","id":3,"result":"allowed"}]`,
		},
		{
			name: "single_denied_request",
			body: `{"jsonrpc":"2.0","id":"single","method":"eth_chainId","params":[]}`,
			want: `{"jsonrpc":"2.0","id":"single","error":{"code":-32601,"message":"the method eth_chainId does not exist/is not available"}}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exchange(t, tc.body, tc.want)
			exchange(t, `{"jsonrpc":"2.0","id":"next","method":"echo_echo","params":["still open"]}`, `{"jsonrpc":"2.0","id":"next","result":"still open"}`)
		})
	}
}
