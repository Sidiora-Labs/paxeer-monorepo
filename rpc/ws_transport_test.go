package evmrpc_test

import (
	"context"
	"net"
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

func newWSTransportServers(t *testing.T, denyList []string) (*evmrpc.HTTPServer, *rpc.Client, *rpc.Client) {
	t.Helper()
	testApp := app.Setup(t, false, false, false)
	config := evmrpcconfig.DefaultConfig
	config.HTTPAddress = TestAddr
	config.HTTPPort = 0
	config.WSPort = 0
	config.FilterTimeout = time.Minute
	config.DenyList = denyList
	ctxProvider := func(int64) sdk.Context {
		return Ctx.WithBlockHeight(MockHeight103).WithIsTracing(true).WithClosestUpgradeName(LatestCtxUpgradeName)
	}
	txConfigProvider := func(int64) client.TxConfig { return TxConfig }
	wsService, err := evmrpc.NewEVMWebSocketServer(config, &MockClient{}, EVMKeeper, testApp.BeginBlockKeepers,
		testApp.BaseApp, testApp.TracerAnteHandler, ctxProvider, txConfigProvider, "", nil, nil)
	require.NoError(t, err)
	t.Cleanup(wsService.Stop)
	wsServer := wsService.(*evmrpc.HTTPServer)
	require.NoError(t, wsServer.Start())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	wsClient, err := rpc.DialWebsocket(ctx, "ws://"+wsServer.ListenAddr(), "http://localhost")
	require.NoError(t, err)
	t.Cleanup(wsClient.Close)

	httpService, err := evmrpc.NewEVMHTTPServer(config, &MockClient{}, EVMKeeper, testApp.BeginBlockKeepers,
		testApp.BaseApp, testApp.TracerAnteHandler, ctxProvider, txConfigProvider, "", nil)
	require.NoError(t, err)
	t.Cleanup(httpService.Stop)
	httpServer := httpService.(*evmrpc.HTTPServer)
	require.NoError(t, httpServer.Start())
	httpClient, err := rpc.DialHTTP("http://" + httpServer.ListenAddr())
	require.NoError(t, err)
	t.Cleanup(httpClient.Close)
	return wsServer, wsClient, httpClient
}

func TestWSBindsHTTPAddress(t *testing.T) {
	server, wsClient, _ := newWSTransportServers(t, []string{"eth_chainId"})
	host, port, err := net.SplitHostPort(server.ListenAddr())
	require.NoError(t, err)
	require.Equal(t, TestAddr, host)
	require.True(t, net.ParseIP(host).IsLoopback())
	require.NotEqual(t, "0", port)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var echoed string
	require.NoError(t, wsClient.CallContext(ctx, &echoed, "echo_echo", "loopback"))
	require.Equal(t, "loopback", echoed)
}

func TestWSAppliesDenyList(t *testing.T) {
	_, allowedWS, _ := newWSTransportServers(t, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var chainID hexutil.Big
	require.NoError(t, allowedWS.CallContext(ctx, &chainID, "eth_chainId"))
	require.Equal(t, EVMKeeper.ChainID(Ctx), chainID.ToInt())

	_, deniedWS, deniedHTTP := newWSTransportServers(t, []string{"eth_chainId"})
	ctx, cancelDenied := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelDenied()
	var wsResult, httpResult interface{}
	wsErr := deniedWS.CallContext(ctx, &wsResult, "eth_chainId")
	httpErr := deniedHTTP.CallContext(ctx, &httpResult, "eth_chainId")
	var wsRPCError, httpRPCError rpc.Error
	require.ErrorAs(t, wsErr, &wsRPCError)
	require.ErrorAs(t, httpErr, &httpRPCError)
	require.Equal(t, -32601, wsRPCError.ErrorCode())
	require.EqualError(t, wsErr, "the method eth_chainId does not exist/is not available")
	require.Equal(t, httpRPCError.ErrorCode(), wsRPCError.ErrorCode())
	require.Equal(t, httpErr.Error(), wsErr.Error())
	require.Nil(t, wsResult)
	require.Nil(t, httpResult)
	var echoed string
	require.NoError(t, deniedWS.CallContext(ctx, &echoed, "echo_echo", "allowed"))
	require.Equal(t, "allowed", echoed)
	batch := []rpc.BatchElem{
		{Method: "eth_chainId", Result: new(hexutil.Big)},
		{Method: "eth_chainId", Result: new(hexutil.Big)},
	}
	require.NoError(t, deniedWS.BatchCallContext(ctx, batch))
	for _, call := range batch {
		var rpcError rpc.Error
		require.ErrorAs(t, call.Error, &rpcError)
		require.Equal(t, -32601, rpcError.ErrorCode())
		require.EqualError(t, call.Error, "the method eth_chainId does not exist/is not available")
	}
}
