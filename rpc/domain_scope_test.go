package evmrpc_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	chainstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	evmrpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

type domainScopeHarness struct {
	app      *app.App
	ctx      sdk.Context
	operator common.Address
	http     *evmrpc.HTTPServer
	ws       *evmrpc.HTTPServer
}

func newDomainScopeHarness(t *testing.T) *domainScopeHarness {
	t.Helper()
	testApp := app.Setup(t, false, false, false)
	ctx := testApp.GetContextForDeliverTx(nil).WithBlockHeight(MockHeight8).WithChainID("hyperpax_125-1")
	h := &domainScopeHarness{app: testApp, ctx: ctx, operator: common.HexToAddress("0x1234")}
	params := testApp.LayerXChainsKeeper.GetParams(ctx)
	value := params.MinBond.Add(params.CreationFee)
	coins := sdk.NewCoins(sdk.NewCoin(testApp.EvmKeeper.GetBaseDenom(ctx), value.MulRaw(4)))
	require.NoError(t, testApp.BankKeeper.MintCoins(ctx, "evm", coins))
	require.NoError(t, testApp.BankKeeper.SendCoinsFromModuleToAccount(ctx, "evm", sdk.AccAddress(h.operator.Bytes()), coins))
	domain := h.create(t, "rpc-chain")
	require.Equal(t, uint32(1), domain.Id)

	config := evmrpcconfig.DefaultConfig
	config.HTTPAddress = TestAddr
	config.HTTPPort = 0
	config.WSPort = 0
	config.IPRateLimitRPS = 0
	config.FilterTimeout = time.Minute
	ctxProvider := func(height int64) sdk.Context {
		if height == evmrpc.LatestCtxHeight {
			return h.ctx
		}
		return h.ctx.WithBlockHeight(height)
	}
	txConfigProvider := func(int64) client.TxConfig { return TxConfig }
	httpService, err := evmrpc.NewEVMHTTPServer(config, &MockClient{}, &testApp.EvmKeeper, testApp.BeginBlockKeepers,
		testApp.BaseApp, testApp.TracerAnteHandler, ctxProvider, txConfigProvider, "", nil)
	require.NoError(t, err)
	t.Cleanup(httpService.Stop)
	h.http = httpService.(*evmrpc.HTTPServer)
	require.NoError(t, h.http.Start())
	wsService, err := evmrpc.NewEVMWebSocketServer(config, &MockClient{}, &testApp.EvmKeeper, testApp.BeginBlockKeepers,
		testApp.BaseApp, testApp.TracerAnteHandler, ctxProvider, txConfigProvider, "", nil, nil)
	require.NoError(t, err)
	t.Cleanup(wsService.Stop)
	h.ws = wsService.(*evmrpc.HTTPServer)
	require.NoError(t, h.ws.Start())
	return h
}

func (h *domainScopeHarness) create(t *testing.T, name string) chainstypes.Domain {
	t.Helper()
	params := h.app.LayerXChainsKeeper.GetParams(h.ctx)
	domain, err := h.app.LayerXChainsKeeper.Create(h.ctx, h.operator, name, params.MinGasAllowance, params.MinByteAllowance,
		0, params.MinBond.Add(params.CreationFee), h.app.EvmKeeper.GetNextBaseFeePerGas(h.ctx))
	require.NoError(t, err)
	return domain
}

func domainHTTPClient(t *testing.T, server *evmrpc.HTTPServer, path string) *rpc.Client {
	t.Helper()
	client, err := rpc.DialHTTP("http://" + server.ListenAddr() + path)
	require.NoError(t, err)
	t.Cleanup(client.Close)
	return client
}

func domainWSClient(t *testing.T, server *evmrpc.HTTPServer, path string) *rpc.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := rpc.DialWebsocket(ctx, "ws://"+server.ListenAddr()+path, "http://localhost")
	require.NoError(t, err)
	t.Cleanup(client.Close)
	return client
}

func assertDomainChainID(t *testing.T, client *rpc.Client, chainID, version string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, call := range []struct{ method, want string }{{"eth_chainId", chainID}, {"net_version", version}} {
		var result string
		require.NoError(t, client.CallContext(ctx, &result, call.method))
		require.Equal(t, call.want, result, call.method)
	}
}

func TestChainPathServesTheDerivedChainId(t *testing.T) {
	h := newDomainScopeHarness(t)
	for _, path := range []string{"/chain/1", "/chain/1/"} {
		t.Run(path, func(t *testing.T) {
			assertDomainChainID(t, domainHTTPClient(t, h.http, path), "0x7735941", "125000001")
		})
	}
	require.NoError(t, h.app.LayerXChainsKeeper.Pause(h.ctx, h.operator, 1))
	assertDomainChainID(t, domainHTTPClient(t, h.http, "/chain/1"), "0x7735941", "125000001")
	params := h.app.LayerXChainsKeeper.GetParams(h.ctx)
	require.NoError(t, h.app.LayerXChainsKeeper.GovernancePause(h.ctx, params.Authority, 1, true))
	assertDomainChainID(t, domainHTTPClient(t, h.http, "/chain/1"), "0x7735941", "125000001")
	domain := h.create(t, "second-chain")
	require.Equal(t, uint32(2), domain.Id)
	assertDomainChainID(t, domainHTTPClient(t, h.http, "/chain/2"), "0x7735942", "125000002")
}

func TestUnknownChainPathIs404(t *testing.T) {
	h := newDomainScopeHarness(t)
	require.NoError(t, h.app.LayerXChainsKeeper.Retire(h.ctx, h.operator, 1))
	for _, server := range []*evmrpc.HTTPServer{h.http, h.ws} {
		for _, path := range []string{"/chain/1", "/chain/999", "/chain/0", "/chain/4294967296", "/chain/-1", "/chain/+1", "/chain/1/extra", "/chain/1//", "/chain/", "/chain"} {
			t.Run(server.ListenAddr()+path, func(t *testing.T) {
				req, err := http.NewRequest(http.MethodPost, "http://"+server.ListenAddr()+path,
					strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}`))
				require.NoError(t, err)
				req.Header.Set("Content-Type", "application/json")
				client := &http.Client{Timeout: 5 * time.Second}
				response, err := client.Do(req)
				require.NoError(t, err)
				defer response.Body.Close()
				require.Equal(t, http.StatusNotFound, response.StatusCode)
				require.Equal(t, "application/json", response.Header.Get("Content-Type"))
				var body map[string]string
				require.NoError(t, json.NewDecoder(response.Body).Decode(&body))
				require.Equal(t, map[string]string{"error": "unknown_chain"}, body)
			})
		}
	}
	conn, response, err := websocket.DefaultDialer.Dial("ws://"+h.ws.ListenAddr()+"/chain/999", nil)
	if conn != nil {
		t.Cleanup(func() { _ = conn.Close() })
	}
	require.Error(t, err)
	require.NotNil(t, response)
	defer response.Body.Close()
	require.Equal(t, http.StatusNotFound, response.StatusCode)
}

func TestRootPathStillServesChain125(t *testing.T) {
	h := newDomainScopeHarness(t)
	assertDomainChainID(t, domainHTTPClient(t, h.http, "/chain/1"), "0x7735941", "125000001")
	assertDomainChainID(t, domainHTTPClient(t, h.http, "/"), "0x7d", "125")
	assertDomainChainID(t, domainWSClient(t, h.ws, "/chain/1"), "0x7735941", "125000001")
	assertDomainChainID(t, domainWSClient(t, h.ws, "/"), "0x7d", "125")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var rootError rpc.Error
	var result string
	err := domainHTTPClient(t, h.http, "/").CallContext(ctx, &result, "eth_sendRawTransaction", "0x")
	require.ErrorAs(t, err, &rootError)
	var chainError rpc.Error
	err = domainHTTPClient(t, h.http, "/chain/1").CallContext(ctx, &result, "eth_sendRawTransaction", "0x")
	require.ErrorAs(t, err, &chainError)
	require.Equal(t, rootError.ErrorCode(), chainError.ErrorCode())
	require.Equal(t, rootError.Error(), chainError.Error())
}

func TestChainPathServesWebSocketUpgrades(t *testing.T) {
	h := newDomainScopeHarness(t)
	for _, path := range []string{"/chain/1", "/chain/1/"} {
		t.Run(path, func(t *testing.T) {
			assertDomainChainID(t, domainWSClient(t, h.ws, path), "0x7735941", "125000001")
		})
	}
	domain := h.create(t, "second-chain")
	require.Equal(t, uint32(2), domain.Id)
	first := domainWSClient(t, h.ws, "/chain/1")
	second := domainWSClient(t, h.ws, "/chain/2")
	assertDomainChainID(t, first, "0x7735941", "125000001")
	assertDomainChainID(t, second, "0x7735942", "125000002")
	assertDomainChainID(t, first, "0x7735941", "125000001")
}

func TestStopClosesScopedWebSocketClients(t *testing.T) {
	h := newDomainScopeHarness(t)
	root := domainWSClient(t, h.ws, "/")
	first := domainWSClient(t, h.ws, "/chain/1")
	second := domainWSClient(t, h.ws, "/chain/1")
	assertDomainChainID(t, root, "0x7d", "125")
	assertDomainChainID(t, first, "0x7735941", "125000001")
	assertDomainChainID(t, second, "0x7735941", "125000001")

	stop := func() {
		t.Helper()
		done := make(chan struct{})
		go func() {
			h.ws.Stop()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("WebSocket server Stop did not return within five seconds")
		}
	}
	stop()

	for _, client := range []struct {
		name string
		rpc  *rpc.Client
	}{{"root", root}, {"first scoped", first}, {"second scoped", second}} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		var result string
		err := client.rpc.CallContext(ctx, &result, "eth_chainId")
		require.Error(t, err, "%s client remained connected after Stop", client.name)
		require.NoError(t, ctx.Err(), "%s client waited for its deadline after Stop", client.name)
		cancel()
	}

	stop()
}
