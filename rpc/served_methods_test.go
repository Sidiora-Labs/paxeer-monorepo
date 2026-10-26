package evmrpc_test

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"testing"
	"time"

	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	evmrpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func TestServedMethodsIncludesPromotedMethods(t *testing.T) {
	api := rpc.API{Namespace: "pax", Service: &evmrpc.PaxBlockAPI{}}
	methods := evmrpc.ServedMethods([]rpc.API{api, api})
	require.Contains(t, methods, "pax_getBlockByNumber")
	require.Contains(t, methods, "pax_getBlockByNumberExcludeTraceFail")
	require.True(t, sort.StringsAreSorted(methods))
	for i := 1; i < len(methods); i++ {
		require.NotEqual(t, methods[i-1], methods[i])
	}

	subscriptions := evmrpc.ServedMethods([]rpc.API{{Namespace: "eth", Service: &evmrpc.SubscriptionAPI{}}})
	require.Contains(t, subscriptions, "eth_subscribe:newHeads")
	require.Contains(t, subscriptions, "eth_subscribe:logs")
	require.NotContains(t, subscriptions, "eth_newHeads")
	require.NotContains(t, subscriptions, "eth_logs")
	managerMethods := evmrpc.ServedMethods([]rpc.API{{Namespace: "eth", Service: &evmrpc.SubscriptionManager{}}})
	require.NotContains(t, managerMethods, "eth_subscribe")
	require.Contains(t, managerMethods, "eth_unsubscribe")

	server := evmrpc.NewHTTPServer(rpc.DefaultHTTPTimeouts)
	apis := []rpc.API{api, {Namespace: "echo", Service: evmrpc.NewEchoAPI()}}
	require.NoError(t, server.EnableRPC(apis, evmrpc.HTTPConfig{Modules: []string{"pax"}}))
	require.NoError(t, server.EnableWS(apis, evmrpc.WsConfig{Modules: []string{"echo"}}))
	apis[0].Namespace = "changed"
	require.Equal(t, methods, server.HTTPMethods())
	require.Equal(t, []string{"echo_echo"}, server.WSMethods())

	gated := evmrpc.NewHTTPServer(rpc.DefaultHTTPTimeouts)
	gatedAPIs := []rpc.API{api, {Namespace: "eth", Service: &evmrpc.SubscriptionAPI{}}}
	require.NoError(t, gated.EnableRPC(gatedAPIs, evmrpc.HTTPConfig{
		PaxLegacyAllowlist: evmrpc.BuildPaxLegacyEnabledSet([]string{"pax_getBlockByNumber"}),
	}))
	require.NoError(t, gated.EnableWS(gatedAPIs, evmrpc.WsConfig{}))
	require.Equal(t, []string{"pax_getBlockByNumber"}, gated.HTTPMethods())
	require.Equal(t, evmrpc.ServedMethods(gatedAPIs), gated.WSMethods())
}

func TestServedMethodsMatchServerDispatch(t *testing.T) {
	testApp := app.Setup(t, false, false, false)
	config := evmrpcconfig.DefaultConfig
	config.HTTPPort = TestPort
	config.WSPort = TestWSPort
	config.FilterTimeout = 500 * time.Millisecond
	config.MaxLogNoBlock = 10
	config.EnabledLegacyPaxApis = evmrpc.PaxLegacyAllGatedMethodNames()
	ctxProvider := func(height int64) sdk.Context {
		if height == evmrpc.LatestCtxHeight {
			return Ctx.WithIsTracing(true).WithClosestUpgradeName(LatestCtxUpgradeName)
		}
		return Ctx.WithIsTracing(true)
	}
	server, err := evmrpc.NewEVMHTTPServer(config, &MockClient{}, EVMKeeper, testApp.BeginBlockKeepers,
		testApp.BaseApp, testApp.TracerAnteHandler, ctxProvider, func(int64) client.TxConfig { return TxConfig }, "", nil)
	require.NoError(t, err)
	httpServer, ok := server.(*evmrpc.HTTPServer)
	require.True(t, ok)
	methods := httpServer.HTTPMethods()
	require.NotEmpty(t, methods)
	require.Contains(t, methods, "pax_getBlockByNumber")
	require.Contains(t, methods, "pax_getBlockByNumberExcludeTraceFail")

	rpcClient, err := rpc.DialHTTP(fmt.Sprintf("http://%s:%d", TestAddr, TestPort))
	require.NoError(t, err)
	t.Cleanup(rpcClient.Close)
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), config.WriteTimeout)
			defer cancel()
			var result interface{}
			err := rpcClient.CallContext(ctx, &result, method, []interface{}{}...)
			if err != nil {
				var rpcError rpc.Error
				require.True(t, errors.As(err, &rpcError), "expected JSON-RPC response: %v", err)
				require.NotEqual(t, -32601, rpcError.ErrorCode(), "advertised method is not dispatched")
			}
		})
	}
}
