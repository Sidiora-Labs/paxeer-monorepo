package evmrpc_test

import (
	"context"
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

func newRegistrationServers(t *testing.T) (*evmrpc.HTTPServer, *evmrpc.HTTPServer, *rpc.Client, *rpc.Client) {
	t.Helper()
	testApp := app.Setup(t, false, false, false)
	config := evmrpcconfig.DefaultConfig
	config.HTTPAddress = TestAddr
	config.HTTPPort = 0
	config.WSPort = 0
	config.FilterTimeout = time.Minute
	config.EnabledLegacyPaxApis = evmrpc.PaxLegacyAllGatedMethodNames()
	ctxProvider := func(height int64) sdk.Context {
		ctx := Ctx
		if height == MockHeight2 {
			ctx = MultiTxCtx
		}
		if height == evmrpc.LatestCtxHeight {
			height = MockHeight103
		}
		return ctx.WithBlockHeight(height).WithIsTracing(true).WithClosestUpgradeName(LatestCtxUpgradeName)
	}
	txConfigProvider := func(int64) client.TxConfig { return TxConfig }
	httpService, err := evmrpc.NewEVMHTTPServer(config, &MockClient{}, EVMKeeper, testApp.BeginBlockKeepers,
		testApp.BaseApp, testApp.TracerAnteHandler, ctxProvider, txConfigProvider, "", nil)
	require.NoError(t, err)
	t.Cleanup(httpService.Stop)
	wsService, err := evmrpc.NewEVMWebSocketServer(config, &MockClient{}, EVMKeeper, testApp.BeginBlockKeepers,
		testApp.BaseApp, testApp.TracerAnteHandler, ctxProvider, txConfigProvider, "", nil, nil)
	require.NoError(t, err)
	t.Cleanup(wsService.Stop)
	httpServer := httpService.(*evmrpc.HTTPServer)
	wsServer := wsService.(*evmrpc.HTTPServer)
	require.NoError(t, wsServer.SetListenAddr(TestAddr, 0))
	require.NoError(t, httpServer.Start())
	require.NoError(t, wsServer.Start())
	httpClient, err := rpc.DialHTTP("http://" + httpServer.ListenAddr())
	require.NoError(t, err)
	t.Cleanup(httpClient.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	wsClient, err := rpc.DialWebsocket(ctx, "ws://"+wsServer.ListenAddr(), "http://localhost")
	require.NoError(t, err)
	t.Cleanup(wsClient.Close)
	return httpServer, wsServer, httpClient, wsClient
}

func TestRawAndUncleMethodsServedHTTPAndWS(t *testing.T) {
	httpServer, wsServer, httpClient, wsClient := newRegistrationServers(t)
	raw, err := multiTxBlockTx1.MarshalBinary()
	require.NoError(t, err)
	for _, transport := range []struct {
		name    string
		methods []string
		client  *rpc.Client
	}{
		{name: "HTTP", methods: httpServer.HTTPMethods(), client: httpClient},
		{name: "WebSocket", methods: wsServer.WSMethods(), client: wsClient},
	} {
		t.Run(transport.name, func(t *testing.T) {
			for _, call := range []struct {
				method string
				params []interface{}
				want   interface{}
			}{
				{method: "eth_getRawTransactionByHash", params: []interface{}{multiTxBlockTx1.Hash()}, want: hexutil.Encode(raw)},
				{method: "eth_getRawTransactionByBlockNumberAndIndex", params: []interface{}{"0x2", "0x0"}, want: hexutil.Encode(raw)},
				{method: "eth_getRawTransactionByBlockHashAndIndex", params: []interface{}{MultiTxBlockHash, "0x0"}, want: hexutil.Encode(raw)},
				{method: "eth_getUncleByBlockNumberAndIndex", params: []interface{}{"0x2", "0x0"}, want: nil},
				{method: "eth_getUncleByBlockHashAndIndex", params: []interface{}{MultiTxBlockHash, "0x0"}, want: nil},
				{method: "eth_getUncleCountByBlockNumber", params: []interface{}{"0x2"}, want: "0x0"},
				{method: "eth_getUncleCountByBlockHash", params: []interface{}{MultiTxBlockHash}, want: "0x0"},
			} {
				t.Run(call.method, func(t *testing.T) {
					require.Contains(t, transport.methods, call.method)
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					var result interface{}
					require.NoError(t, transport.client.CallContext(ctx, &result, call.method, call.params...))
					require.Equal(t, call.want, result)
				})
			}
			t.Run("eth_sendRawTransactionSync", func(t *testing.T) {
				require.Contains(t, transport.methods, "eth_sendRawTransactionSync")
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				var receipt map[string]interface{}
				require.NoError(t, transport.client.CallContext(ctx, &receipt, "eth_sendRawTransactionSync", hexutil.Bytes(raw)))
				require.Equal(t, multiTxBlockTx1.Hash().Hex(), receipt["transactionHash"])
				var standard map[string]interface{}
				require.NoError(t, transport.client.CallContext(ctx, &standard, "eth_getTransactionReceipt", multiTxBlockTx1.Hash()))
				require.Equal(t, standard, receipt)
			})
		})
	}
}

func TestWSServesEthGetLogs(t *testing.T) {
	httpServer, wsServer, httpClient, wsClient := newRegistrationServers(t)
	require.Contains(t, httpServer.HTTPMethods(), "eth_getLogs")
	require.Contains(t, wsServer.WSMethods(), "eth_getLogs")
	criteria := map[string]interface{}{
		"fromBlock": "0x2",
		"toBlock":   "0x2",
		"address":   "0x1111111111111111111111111111111111111112",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var httpLogs, wsLogs []map[string]interface{}
	require.NoError(t, httpClient.CallContext(ctx, &httpLogs, "eth_getLogs", criteria))
	require.Len(t, httpLogs, 2)
	for _, log := range httpLogs {
		require.Equal(t, criteria["address"], log["address"])
		require.Equal(t, "0x2", log["blockNumber"])
	}
	require.NoError(t, wsClient.CallContext(ctx, &wsLogs, "eth_getLogs", criteria))
	require.Equal(t, httpLogs, wsLogs)
}

func TestPaxNewPendingTransactionFilterNotServed(t *testing.T) {
	httpServer, _, httpClient, _ := newRegistrationServers(t)
	methods := httpServer.HTTPMethods()
	require.NotContains(t, methods, "pax_newPendingTransactionFilter")
	for _, method := range []string{
		"pax_getFilterChanges", "pax_getFilterLogs", "pax_getLogs",
		"pax_newBlockFilter", "pax_newFilter", "pax_uninstallFilter",
	} {
		require.Contains(t, methods, method)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var result interface{}
	err := httpClient.CallContext(ctx, &result, "pax_newPendingTransactionFilter")
	var rpcError rpc.Error
	require.ErrorAs(t, err, &rpcError)
	require.Equal(t, -32601, rpcError.ErrorCode())
	require.Nil(t, result)
	var filterID rpc.ID
	require.NoError(t, httpClient.CallContext(ctx, &filterID, "pax_newBlockFilter"))
	require.NotEmpty(t, filterID)
	var removed bool
	require.NoError(t, httpClient.CallContext(ctx, &removed, "pax_uninstallFilter", filterID))
	require.True(t, removed)
}
