package evmrpc_test

// Task 22.33.1291 repair record (scope excludes spec/):
// net-listening-proof -> TestRepairM_1291_NetListening
// net-peers-proof -> TestRepairM_1291_NetPeers
// test-api-path-proof -> TestRepairM_1291_TestAPIPath
// test-version-global-race -> TestRepairM_1291_VersionLifetime
// client-version-proof -> TestRepairM_1291_ClientVersion
// ws-handshake-count, legacy-connect-policy -> TestRepairM_1291_HandshakeMetrics
// Wiring request, domain 7/dashboard migration owner: reconcile the keep policy
// in spec/layerx-merge/interfaces.kvx and spec/layerx-merge/contracts/evm-rpc.md;
// retain both sinks until recorded dashboard migration evidence exists.
// Wiring request, domain 7/integration RPC owner: update tests/integration/evm_module/
// rpc_io_test/testdata/web3_clientVersion/clientVersion.io and its specOnly consumer
// to assert Geth/<runtime.GOOS>-<runtime.GOARCH>/<runtime.Version()> exactly.
// Baseline verify was not run: the bounded gate order requires implementation
// before hf-build/hf-done, and permits no preliminary verify run.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmnode "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/local"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/artifacts/cw20"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	rpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

// Node.New builds the production Environment and Router without crossing Go's
// consensus/internal import boundary. Start/Stop releases the real node's resources;
// its in-memory peer inventory remains available to the local RPC client.
func newRepair1291Consensus(t *testing.T, peers string) *local.Local {
	t.Helper()
	cfg, err := tmconfig.ResetTestRoot(t.TempDir(), "rpc-repair-1291")
	require.NoError(t, err)
	cfg.Mode = tmconfig.ModeSeed
	cfg.DBBackend = "memdb"
	cfg.RPC.ListenAddress, cfg.RPC.PprofListenAddress = "", ""
	cfg.P2P.PersistentPeers = peers
	cfg.P2P.BootstrapPeers, cfg.P2P.BlockSyncPeers = "", ""
	nodeCtx, cancel := context.WithCancel(t.Context())
	defer cancel()
	node, err := tmnode.New(nodeCtx, cfg, func() {}, abci.BaseApplication{}, nil, nil,
		tmnode.NoOpMetricsProvider(), tmtypes.DefaultConsensusPolicy())
	require.NoError(t, err)
	require.NoError(t, node.Start(nodeCtx))
	cancel()
	node.Stop()
	node.Wait()
	consensus, err := local.New(node)
	require.NoError(t, err)
	return consensus
}

type repair1291Fixture struct {
	app       *app.App
	consensus *local.Local
	ctx       sdk.Context
	config    rpcconfig.Config
}

func newRepair1291Fixture(t *testing.T, chain string, enabled bool) *repair1291Fixture {
	t.Helper()
	application := app.Setup(t, false, false, false)
	t.Cleanup(func() { require.NoError(t, application.Close()) })
	cfg := rpcconfig.DefaultConfig
	cfg.HTTPAddress = "127.0.0.1"
	cfg.HTTPPort, cfg.WSPort, cfg.IPRateLimitRPS = 0, 0, 0
	cfg.EnableTestAPI = enabled
	cfg.FilterTimeout = time.Minute
	return &repair1291Fixture{app: application, consensus: newRepair1291Consensus(t, ""),
		ctx: application.GetContextForDeliverTx(nil).WithChainID(chain).WithBlockHeight(1), config: cfg}
}

func (f *repair1291Fixture) server(t *testing.T, ws bool) (*evmrpc.HTTPServer, *gethrpc.Client) {
	t.Helper()
	provider := func(int64) sdk.Context { return f.ctx }
	txProvider := func(int64) client.TxConfig { return f.app.GetTxConfig() }
	var service evmrpc.EVMServer
	var err error
	if ws {
		service, err = evmrpc.NewEVMWebSocketServer(f.config, f.consensus, &f.app.EvmKeeper,
			f.app.BeginBlockKeepers, f.app.BaseApp, f.app.TracerAnteHandler, provider, txProvider, "", nil, nil)
	} else {
		service, err = evmrpc.NewEVMHTTPServer(f.config, f.consensus, &f.app.EvmKeeper,
			f.app.BeginBlockKeepers, f.app.BaseApp, f.app.TracerAnteHandler, provider, txProvider, "", nil)
	}
	require.NoError(t, err)
	t.Cleanup(service.Stop)
	listener := httptest.NewServer(service.(http.Handler))
	t.Cleanup(listener.Close)
	var consumer *gethrpc.Client
	if ws {
		consumer, err = gethrpc.DialWebsocket(t.Context(), "ws"+strings.TrimPrefix(listener.URL, "http"), "http://localhost")
	} else {
		consumer, err = gethrpc.DialHTTP(listener.URL)
	}
	require.NoError(t, err)
	t.Cleanup(consumer.Close)
	return service.(*evmrpc.HTTPServer), consumer
}

func repair1291RPCError(t *testing.T, err error, code int) {
	t.Helper()
	var rpcError gethrpc.Error
	require.ErrorAs(t, err, &rpcError)
	require.Equal(t, code, rpcError.ErrorCode())
}

func TestRepairM_1291_NetListening(t *testing.T)     { TestNetListeningReportsNetInfo(t) }
func TestRepairM_1291_NetPeers(t *testing.T)         { TestNetPeerCountReportsPeers(t) }
func TestRepairM_1291_ClientVersion(t *testing.T)    { TestClientVersion(t) }
func TestRepairM_1291_HandshakeMetrics(t *testing.T) { evmrpc.TestWSConnectEmitsBothCounters(t) }

func TestRepairM_1291_TestAPIPath(t *testing.T) {
	f := newRepair1291Fixture(t, "rpc-test-network", true)
	server, httpClient := f.server(t, false)
	_, wsClient := f.server(t, true)
	require.Contains(t, server.HTTPMethods(), "test_incrementPointerVersion")
	for _, scenario := range []struct {
		offset  int16
		version uint16
	}{
		{-32768, 32770}, {32767, 32769}, {-2, 2}, {-1, 1}, {0, 2}, {1, 3},
	} {
		var result json.RawMessage
		require.NoError(t, httpClient.CallContext(t.Context(), &result, "test_incrementPointerVersion", "cw20", scenario.offset))
		require.JSONEq(t, "null", string(result))
		require.Equal(t, scenario.version, cw20.CurrentVersion(f.ctx))
		require.Equal(t, uint16(2), cw20.CurrentVersion(f.ctx.WithChainID("hyperpax_125-1")))
	}
	// The supported pointer type and the rejected type both traverse JSON dispatch.
	var result json.RawMessage
	err := httpClient.CallContext(t.Context(), &result, "test_incrementPointerVersion", "cw721", 1)
	require.EqualError(t, err, "invalid pointer type")
	require.Equal(t, uint16(3), cw20.CurrentVersion(f.ctx))
	for _, offset := range []int{32768, -32769} {
		repair1291RPCError(t, httpClient.CallContext(t.Context(), &result,
			"test_incrementPointerVersion", "cw20", offset), -32602)
		require.Equal(t, uint16(3), cw20.CurrentVersion(f.ctx))
	}
	repair1291RPCError(t, wsClient.CallContext(t.Context(), &result,
		"test_incrementPointerVersion", "cw20", 1), -32601)
	server.Stop()
	require.Equal(t, uint16(2), cw20.CurrentVersion(f.ctx))

	for _, scenario := range []struct {
		chain   string
		enabled bool
	}{
		{"rpc-test-network", false}, {"hyperpax_125-1", true},
	} {
		t.Run(fmt.Sprintf("chain_%s_enabled_%t", scenario.chain, scenario.enabled), func(t *testing.T) {
			other := newRepair1291Fixture(t, scenario.chain, scenario.enabled)
			for _, ws := range []bool{false, true} {
				srv, consumer := other.server(t, ws)
				if ws {
					require.NotContains(t, srv.WSMethods(), "test_incrementPointerVersion")
				} else {
					require.NotContains(t, srv.HTTPMethods(), "test_incrementPointerVersion")
				}
				repair1291RPCError(t, consumer.CallContext(t.Context(), &result,
					"test_incrementPointerVersion", "cw20", 1), -32601)
			}
		})
	}
}

func TestRepairM_1291_VersionLifetime(t *testing.T) {
	f := newRepair1291Fixture(t, "rpc-test-network", true)
	oldServer, consumer := f.server(t, false)
	require.Equal(t, uint16(2), cw20.CurrentVersion(f.ctx))
	var wg sync.WaitGroup
	failures := make(chan error, 8)
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 32; i++ {
				var result json.RawMessage
				if err := consumer.CallContext(t.Context(), &result, "test_incrementPointerVersion", "cw20", 1+(worker+i)%2); err != nil {
					failures <- err
					return
				}
				if string(result) != "null" {
					failures <- fmt.Errorf("unexpected result %s", result)
					return
				}
				if version := cw20.CurrentVersion(f.ctx); version != 3 && version != 4 {
					failures <- fmt.Errorf("unexpected version %d", version)
					return
				}
			}
		}(worker)
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		require.NoError(t, err)
	}

	replacement, replacementClient := f.server(t, false)
	require.Equal(t, uint16(2), cw20.CurrentVersion(f.ctx), "replacement resets old process state")
	var result json.RawMessage
	require.NoError(t, replacementClient.CallContext(t.Context(), &result, "test_incrementPointerVersion", "cw20", 7))
	require.Equal(t, uint16(9), cw20.CurrentVersion(f.ctx))
	require.EqualError(t, consumer.CallContext(t.Context(), &result, "test_incrementPointerVersion", "cw20", 20),
		"pointer version override owner has stopped or been replaced")
	require.Equal(t, uint16(9), cw20.CurrentVersion(f.ctx))
	oldServer.Stop()
	require.Equal(t, uint16(9), cw20.CurrentVersion(f.ctx), "stopping old server cannot reset replacement")
	replacement.Stop()
	require.Equal(t, uint16(2), cw20.CurrentVersion(f.ctx), "stop resets owner even before Listen")
	reopened, reopenedClient := f.server(t, false)
	require.Equal(t, uint16(2), cw20.CurrentVersion(f.ctx))
	require.NoError(t, reopenedClient.CallContext(t.Context(), &result, "test_incrementPointerVersion", "cw20", -1))
	require.Equal(t, uint16(1), cw20.CurrentVersion(f.ctx))
	reopened.Stop()
	require.Equal(t, uint16(2), cw20.CurrentVersion(f.ctx))
}
