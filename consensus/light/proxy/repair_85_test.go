package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	appProxy "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/proxy"
	rpccore "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/rpc/core"
	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	rpctypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/version"
)

func repair85Endpoint(t *testing.T) (*kvstore.Application, *httptest.Server, *rpchttp.HTTP) {
	t.Helper()
	app := kvstore.NewApplication()
	t.Cleanup(func() { require.NoError(t, app.Close()) })
	env := &rpccore.Environment{App: appProxy.New(app, appProxy.NopMetrics())}
	routes := rpccore.NewRoutesMap(env, nil)
	providerMux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(providerMux, routes)
	providerMux.HandleFunc("/websocket", rpcserver.NewWebsocketManager(routes).WebsocketHandler)
	provider := httptest.NewServer(providerMux)
	t.Cleanup(provider.Close)

	config := rpcserver.DefaultConfig()
	config.WriteTimeout = 5 * time.Second
	p, err := NewProxy(nil, "tcp://127.0.0.1:0", provider.URL, config)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(func() {
		cancel()
		p.Client.Stop()
	})
	listener, mux, err := p.listen(ctx)
	require.NoError(t, err)
	endpoint := &httptest.Server{Listener: listener, Config: &http.Server{Handler: mux}}
	endpoint.Start()
	t.Cleanup(endpoint.Close)
	consumerHTTP := endpoint.Client()
	consumerHTTP.Timeout = 5 * time.Second
	consumer, err := rpchttp.NewWithClient(endpoint.URL, consumerHTTP)
	require.NoError(t, err)
	return app, provider, consumer
}

func TestRepair_85_ABCIInfoRoundTrip(t *testing.T) {
	app, provider, consumer := repair85Endpoint(t)
	providerClient, err := rpchttp.NewWithTimeout(provider.URL, 5*time.Second)
	require.NoError(t, err)
	for height := int64(1); height <= 2; height++ {
		finalized, err := app.FinalizeBlock(t.Context(), &abci.RequestFinalizeBlock{
			Txs: [][]byte{[]byte("repair85=value")},
		})
		require.NoError(t, err)
		require.Len(t, finalized.TxResults, 1)
		require.Zero(t, finalized.TxResults[0].Code)
		_, err = app.Commit(t.Context())
		require.NoError(t, err)
		produced, err := app.Info(t.Context(), &version.RequestInfo)
		require.NoError(t, err)
		require.Equal(t, height, produced.LastBlockHeight)
		require.NotEmpty(t, produced.Data)
		require.NotEmpty(t, produced.Version)
		require.NotZero(t, produced.AppVersion)
		require.Equal(t, finalized.AppHash, produced.LastBlockAppHash)

		upstream, err := providerClient.ABCIInfo(t.Context())
		require.NoError(t, err)
		require.NotNil(t, upstream)
		require.Equal(t, *produced, upstream.Response)
		result, err := consumer.ABCIInfo(t.Context())
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, *produced, result.Response)
	}
}

func TestRepair_85_UnavailableProviderRefusal(t *testing.T) {
	_, provider, consumer := repair85Endpoint(t)
	result, err := consumer.ABCIInfo(t.Context())
	require.NoError(t, err)
	require.NotNil(t, result)
	providerClient, err := rpchttp.NewWithTimeout(provider.URL, 5*time.Second)
	require.NoError(t, err)
	provider.Close()

	upstream, err := providerClient.ABCIInfo(t.Context())
	require.Nil(t, upstream)
	var unavailable *url.Error
	require.ErrorAs(t, err, &unavailable)
	result, err = consumer.ABCIInfo(t.Context())
	require.Nil(t, result)
	var refusal *rpctypes.RPCError
	require.ErrorAs(t, err, &refusal)
	require.Equal(t, int(rpctypes.CodeInternalError), refusal.Code)
}
