package evmrpc_test

import (
	"strings"
	"testing"
	"time"

	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	evmrpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type clientMethodMatrix []struct {
	name      string
	websocket bool
}

var clientMatrix = clientMethodMatrix{
	{"eth_call", true},
	{"eth_getBlockByNumber", true},
	{"eth_getBlockByHash", true},
	{"eth_chainId", true},
	{"eth_blockNumber", true},
	{"eth_sendRawTransaction", true},
	{"eth_sendRawTransactionSync", true},
	{"eth_getTransactionReceipt", true},
	{"eth_getTransactionByHash", true},
	{"eth_getRawTransactionByHash", true},
	{"eth_getTransactionCount", true},
	{"eth_getBalance", true},
	{"eth_getCode", true},
	{"eth_getStorageAt", true},
	{"eth_getProof", true},
	{"eth_getLogs", true},
	{"eth_getBlockReceipts", true},
	{"eth_getBlockTransactionCountByNumber", true},
	{"eth_getUncleByBlockHashAndIndex", true},
	{"eth_estimateGas", true},
	{"eth_gasPrice", true},
	{"eth_maxPriorityFeePerGas", true},
	{"eth_feeHistory", true},
	{"eth_syncing", true},
	{"eth_accounts", true},
	{"eth_coinbase", true},
	{"eth_sign", true},
	{"eth_signTransaction", true},
	{"eth_sendTransaction", true},
	{"net_version", true},
	{"net_listening", true},
	{"web3_clientVersion", true},
	{"debug_traceTransaction", false},
	{"debug_traceBlockByNumber", false},
	{"debug_traceCall", false},
	{"debug_traceTransactionProfile", false},
	{"txpool_content", false},
	{"pax_sequencerReceipt", false},
	{"pax_getEvmTx", false},
	{"pax_getCosmosTx", false},
	{"pax_getBlockByNumber", false},
}

func (matrix clientMethodMatrix) HTTPMethods() []string {
	methods := make([]string, 0, len(matrix))
	for _, method := range matrix {
		methods = append(methods, method.name)
	}
	return methods
}

func (matrix clientMethodMatrix) WSMethods() []string {
	methods := []string{
		"eth_subscribe:newHeads",
		"eth_subscribe:logs",
		"eth_subscribe:newPendingTransactions",
	}
	for _, method := range matrix {
		if method.websocket {
			methods = append(methods, method.name)
		}
	}
	return methods
}

func newClientMatrixServer(t *testing.T, websocket bool) *evmrpc.HTTPServer {
	t.Helper()
	testApp := app.Setup(t, false, false, false)
	config := evmrpcconfig.DefaultConfig
	config.HTTPAddress = TestAddr
	config.HTTPPort = 0
	config.WSPort = 0
	config.FilterTimeout = 500 * time.Millisecond
	config.MaxLogNoBlock = 10
	config.EnabledLegacyPaxApis = evmrpc.PaxLegacyAllGatedMethodNames()
	ctxProvider := func(height int64) sdk.Context {
		if height == MockHeight2 {
			return MultiTxCtx.WithIsTracing(true)
		}
		if height == evmrpc.LatestCtxHeight {
			return Ctx.WithIsTracing(true).WithClosestUpgradeName(LatestCtxUpgradeName)
		}
		return Ctx.WithIsTracing(true)
	}
	txConfigProvider := func(int64) client.TxConfig { return TxConfig }
	var service evmrpc.EVMServer
	var err error
	if websocket {
		service, err = evmrpc.NewEVMWebSocketServer(config, &MockClient{}, EVMKeeper,
			testApp.BeginBlockKeepers, testApp.BaseApp, testApp.TracerAnteHandler,
			ctxProvider, txConfigProvider, "", nil, nil)
	} else {
		service, err = evmrpc.NewEVMHTTPServer(config, &MockClient{}, EVMKeeper,
			testApp.BeginBlockKeepers, testApp.BaseApp, testApp.TracerAnteHandler,
			ctxProvider, txConfigProvider, "", nil, evmrpc.SequencerReceiptOption{})
	}
	require.NoError(t, err)
	t.Cleanup(service.Stop)
	server, ok := service.(*evmrpc.HTTPServer)
	require.True(t, ok)
	require.NoError(t, server.Start())
	return server
}

func TestClientMatrixMethodsServedHTTP(t *testing.T) {
	server := newClientMatrixServer(t, false)
	methods := server.HTTPMethods()
	for _, method := range clientMatrix.HTTPMethods() {
		t.Run(method, func(t *testing.T) {
			assert.Contains(t, methods, method, "client-called method must be served over HTTP")
		})
	}
}

func TestClientMatrixMethodsServedWS(t *testing.T) {
	server := newClientMatrixServer(t, true)
	methods := server.WSMethods()
	for _, method := range clientMatrix.WSMethods() {
		t.Run(method, func(t *testing.T) {
			assert.Contains(t, methods, method, "client-called method must be served over WebSocket")
		})
	}
	for _, method := range clientMatrix {
		if !method.websocket {
			t.Run(method.name, func(t *testing.T) {
				assert.NotContains(t, methods, method.name, "decision 708 excludes this method from WebSocket")
			})
		}
	}
	for _, method := range methods {
		for _, prefix := range []string{"debug_", "txpool_", "pax_"} {
			assert.False(t, strings.HasPrefix(method, prefix), "decision 708 excludes %s from WebSocket", method)
		}
	}
}
