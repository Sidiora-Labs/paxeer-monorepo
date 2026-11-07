package core

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
	"golang.org/x/time/rate"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	atypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/autobahn/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/autobahn/consensus"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/autobahn/producer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/p2p"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/p2p/conn"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/proxy"
	sm "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/tcp"
	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	rpctypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/types"
	e2eapp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/test/e2e/app"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

// This peer runs the persistent example application. The adapter honors request
// cancellation and exposes execution barriers; it never manufactures app state
// or responses. All blocks, certificates and app hashes come from the router.
type repairM1182Application struct {
	abci.Application
	committed   utils.AtomicSend[int64]
	infoCalls   atomic.Int64
	pauseHeight int64
	entered     chan struct{}
	resume      <-chan struct{}
}

func (app *repairM1182Application) Info(ctx context.Context, req *abci.RequestInfo) (*abci.ResponseInfo, error) {
	app.infoCalls.Add(1)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return app.Application.Info(ctx, req)
}

func (app *repairM1182Application) FinalizeBlock(ctx context.Context, req *abci.RequestFinalizeBlock) (*abci.ResponseFinalizeBlock, error) {
	if req.Header.Height == app.pauseHeight {
		close(app.entered)
		select {
		case <-app.resume:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return app.Application.FinalizeBlock(ctx, req)
}

func (app *repairM1182Application) Commit(ctx context.Context) (*abci.ResponseCommit, error) {
	response, err := app.Application.Commit(ctx)
	if err != nil {
		return nil, err
	}
	info, err := app.Application.Info(ctx, &abci.RequestInfo{})
	if err != nil {
		return nil, err
	}
	app.committed.Store(info.LastBlockHeight)
	return response, nil
}

type repairM1182Fixture struct {
	env        *Environment
	giga       *p2p.GigaRouter
	app        *repairM1182Application
	appConfig  *e2eapp.Config
	openRouter func(*repairM1182Application) (*Environment, *p2p.GigaRouter)
}

func newRepairM1182Application(t *testing.T, cfg *e2eapp.Config) *repairM1182Application {
	t.Helper()
	app, err := e2eapp.NewApplication(cfg)
	require.NoError(t, err)
	return &repairM1182Application{Application: app, committed: utils.NewAtomicSend[int64](0)}
}

func newRepairM1182Fixture(t *testing.T) *repairM1182Fixture {
	t.Helper()
	dir := t.TempDir()
	rng := utils.TestRng()
	_, keys := atypes.GenCommittee(rng, 1)
	key := keys[0]
	nodeKey := p2p.NodeSecretKey(ed25519.TestSecretKey(utils.GenBytes(rng, 32)))
	addr := tcp.TestReserveAddr()
	endpoint := p2p.Endpoint{AddrPort: addr}
	genDoc := &types.GenesisDoc{ChainID: "repair-m1182", InitialHeight: 1}
	require.NoError(t, genDoc.ValidateAndComplete())
	nodeInfo := types.NodeInfo{NodeID: nodeKey.Public().NodeID(), Network: genDoc.ChainID, ListenAddr: addr.String(), Moniker: "repair-m1182"}
	fixture := &repairM1182Fixture{appConfig: e2eapp.DefaultConfig(filepath.Join(dir, "app"))}
	fixture.openRouter = func(application *repairM1182Application) (*Environment, *p2p.GigaRouter) {
		t.Helper()
		app := proxy.New(application, proxy.NopMetrics())
		states := sm.NewStore(dbm.NewMemDB())
		t.Cleanup(func() { require.NoError(t, states.Close()) })
		blocks := store.NewBlockStore(dbm.NewMemDB())
		t.Cleanup(func() { require.NoError(t, blocks.Close()) })
		peers := dbm.NewMemDB()
		t.Cleanup(func() { require.NoError(t, peers.Close()) })
		router, err := p2p.NewRouter(p2p.NopMetrics(), nodeKey, func() *types.NodeInfo { return &nodeInfo }, peers, &p2p.RouterOptions{
			SelfAddress:              utils.Some(endpoint.NodeAddress(nodeKey.Public().NodeID())),
			Endpoint:                 endpoint,
			Connection:               conn.DefaultMConnConfig(),
			IncomingConnectionWindow: utils.Some(time.Duration(0)),
			MaxAcceptRate:            utils.Some(rate.Inf),
			MaxDialRate:              utils.Some(rate.Limit(30)),
			Giga: utils.Some(&p2p.GigaRouterConfig{
				DialInterval: time.Second,
				ValidatorAddrs: map[atypes.PublicKey]p2p.GigaNodeAddr{
					key.Public(): {Key: nodeKey.Public(), HostPort: tcp.HostPort{Hostname: addr.Addr().String(), Port: addr.Port()}},
				},
				Consensus: &consensus.Config{
					Key:                key,
					ViewTimeout:        func(atypes.View) time.Duration { return time.Hour },
					PersistentStateDir: utils.Some(filepath.Join(dir, "autobahn")),
				},
				Producer: &producer.Config{
					App:                     app,
					MaxGasWantedPerBlock:    1000,
					MaxGasEstimatedPerBlock: 1000,
					MaxTxsPerBlock:          1,
					MaxTxsPerSecond:         utils.None[uint64](),
					BlockInterval:           time.Hour,
				},
				GenDoc:     genDoc,
				StateStore: states,
			}),
		})
		require.NoError(t, err)
		giga, ok := router.Giga().Get()
		require.True(t, ok)
		return &Environment{App: app, Router: router, StateStore: states, BlockStore: blocks, NodeInfo: nodeInfo}, giga
	}
	fixture.app = newRepairM1182Application(t, fixture.appConfig)
	fixture.env, fixture.giga = fixture.openRouter(fixture.app)
	return fixture
}

func repairM1182Client(t *testing.T, env *Environment, requestContext func(context.Context) (context.Context, context.CancelFunc)) (*rpchttp.HTTP, *httptest.Server) {
	t.Helper()
	mux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(mux, NewRoutesMap(env, nil))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if requestContext != nil {
			ctx, cancel := requestContext(req.Context())
			defer cancel()
			req = req.WithContext(ctx)
		}
		mux.ServeHTTP(w, req)
	}))
	t.Cleanup(server.Close)
	client, err := rpchttp.NewWithClient(server.URL, server.Client())
	require.NoError(t, err)
	return client, server
}

func repairM1182Run(t *testing.T, ctx context.Context, env *Environment) func() {
	t.Helper()
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- utils.IgnoreCancel(env.Router.Run(runCtx)) }()
	stop := sync.OnceFunc(func() {
		cancel()
		require.NoError(t, <-done)
	})
	t.Cleanup(stop)
	return stop
}

func repairM1182Produce(t *testing.T, ctx context.Context, giga *p2p.GigaRouter, count int) {
	t.Helper()
	for i := range count {
		response, err := giga.InsertTx(ctx, types.Tx(fmt.Sprintf("key%d=value%d", i, i)))
		require.NoError(t, err)
		require.True(t, response.IsOK())
	}
}

func repairM1182WaitHeight(t *testing.T, ctx context.Context, app *repairM1182Application, height int64) {
	t.Helper()
	_, err := app.committed.Subscribe().Wait(ctx, func(actual int64) bool { return actual >= height })
	require.NoError(t, err)
}

func repairM1182RequireRPCError(t *testing.T, err error, details ...string) {
	t.Helper()
	var rpcError *rpctypes.RPCError
	require.ErrorAs(t, err, &rpcError, "the client must receive the server's JSON-RPC error, not a transport failure")
	require.Equal(t, int(rpctypes.CodeInternalError), rpcError.Code)
	for _, detail := range details {
		require.Contains(t, rpcError.Data, detail)
	}
}

func TestRepairM_1182_StartupRPC(t *testing.T) {
	fixture := newRepairM1182Fixture(t)
	client, _ := repairM1182Client(t, fixture.env, nil)
	before := fixture.app.infoCalls.Load()
	status, err := client.Status(t.Context())
	require.NoError(t, err)
	require.Equal(t, before+1, fixture.app.infoCalls.Load(), "all app status fields must use the same Info response")
	require.Zero(t, status.SyncInfo.LatestBlockHeight)
	require.Zero(t, status.SyncInfo.LastCommittedBlockHeight)
	require.Empty(t, status.SyncInfo.LatestBlockHash)
	require.True(t, status.SyncInfo.LatestBlockTime.Equal(time.Unix(0, 0)))
	require.Empty(t, status.SyncInfo.EarliestBlockHash)
	require.Equal(t, int64(1), status.SyncInfo.EarliestBlockHeight)
	require.Equal(t, "1", status.ApplicationInfo.Version)
	require.True(t, status.SyncInfo.CatchingUp)
}

func TestRepairM_1182_PopulatedHeadRPC(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	fixture := newRepairM1182Fixture(t)
	stop := repairM1182Run(t, ctx, fixture.env)
	defer stop()
	repairM1182Produce(t, ctx, fixture.giga, 4)
	repairM1182WaitHeight(t, ctx, fixture.app, 3)
	client, _ := repairM1182Client(t, fixture.env, nil)
	status, err := client.Status(ctx)
	require.NoError(t, err)
	info, err := fixture.app.Info(ctx, &abci.RequestInfo{})
	require.NoError(t, err)
	block, err := fixture.giga.BlockByNumber(ctx, 3)
	require.NoError(t, err)
	require.Equal(t, int64(3), status.SyncInfo.LatestBlockHeight)
	require.Equal(t, info.LastBlockAppHash, []byte(status.SyncInfo.LatestAppHash))
	require.Equal(t, block.BlockID.Hash, status.SyncInfo.LatestBlockHash)
	require.True(t, block.Block.Time.Equal(status.SyncInfo.LatestBlockTime))
	require.Equal(t, int64(3), status.SyncInfo.LastCommittedBlockHeight)
	require.False(t, status.SyncInfo.CatchingUp)
	require.Zero(t, fixture.env.BlockStore.Height())
	require.False(t, fixture.env.ConsensusReactor.IsPresent())
}

func TestRepairM_1182_MissingLatestBlockRPC(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	fixture := newRepairM1182Fixture(t)
	stop := repairM1182Run(t, ctx, fixture.env)
	repairM1182Produce(t, ctx, fixture.giga, 4)
	repairM1182WaitHeight(t, ctx, fixture.app, 3)
	stop()
	// Reopen the real persisted app with a fresh consensus store, reproducing
	// an executed head whose block metadata has been lost.
	app := newRepairM1182Application(t, fixture.appConfig)
	empty := newRepairM1182Fixture(t)
	empty.env.App = proxy.New(app, proxy.NopMetrics())
	client, _ := repairM1182Client(t, empty.env, func(ctx context.Context) (context.Context, context.CancelFunc) {
		return context.WithTimeout(ctx, 100*time.Millisecond)
	})
	status, err := client.Status(ctx)
	repairM1182RequireRPCError(t, err, "load latest Autobahn block at height 3", "context deadline exceeded")
	require.Nil(t, status)
}

func TestRepairM_1182_AppInfoFailureRPC(t *testing.T) {
	fixture := newRepairM1182Fixture(t)
	client, _ := repairM1182Client(t, fixture.env, func(ctx context.Context) (context.Context, context.CancelFunc) {
		ctx, cancel := context.WithCancel(ctx)
		cancel()
		return ctx, cancel
	})
	status, err := client.Status(t.Context())
	repairM1182RequireRPCError(t, err, "load Autobahn application info", "context canceled")
	require.Nil(t, status)
	require.Equal(t, int64(1), fixture.app.infoCalls.Load())
}

func TestRepairM_1182_CatchUpAndRestartRPC(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	fixture := newRepairM1182Fixture(t)
	resume := make(chan struct{})
	release := sync.OnceFunc(func() { close(resume) })
	fixture.app.pauseHeight = 4
	fixture.app.entered = make(chan struct{})
	fixture.app.resume = resume
	stop := repairM1182Run(t, ctx, fixture.env)
	defer stop()
	defer release()
	repairM1182Produce(t, ctx, fixture.giga, 5)
	select {
	case <-fixture.app.entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	client, server := repairM1182Client(t, fixture.env, nil)
	catchingUp, err := client.Status(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(3), catchingUp.SyncInfo.LatestBlockHeight)
	require.Equal(t, int64(4), catchingUp.SyncInfo.LastCommittedBlockHeight)
	require.True(t, catchingUp.SyncInfo.CatchingUp)
	require.False(t, fixture.env.ConsensusReactor.IsPresent())
	release()
	repairM1182WaitHeight(t, ctx, fixture.app, 4)
	converged, err := client.Status(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(4), converged.SyncInfo.LatestBlockHeight)
	require.Equal(t, int64(4), converged.SyncInfo.LastCommittedBlockHeight)
	require.False(t, converged.SyncInfo.CatchingUp)
	server.Close()
	stop()

	// Reconstruct both producers from their persisted state, then bind a new
	// handler and client. No old Environment, reactor or in-memory watch is reused.
	reopenedApp := newRepairM1182Application(t, fixture.appConfig)
	reopenedEnv, reopenedGiga := fixture.openRouter(reopenedApp)
	require.NotSame(t, fixture.giga, reopenedGiga)
	require.NotSame(t, fixture.app.Application, reopenedApp.Application)
	reopenedClient, _ := repairM1182Client(t, reopenedEnv, nil)
	recovered, err := reopenedClient.Status(ctx)
	require.NoError(t, err)
	require.Equal(t, converged.SyncInfo, recovered.SyncInfo)
	require.False(t, recovered.SyncInfo.CatchingUp)
	require.Zero(t, reopenedEnv.BlockStore.Height())
	require.False(t, reopenedEnv.ConsensusReactor.IsPresent())
	stopReopened := repairM1182Run(t, ctx, reopenedEnv)
	defer stopReopened()
	afterRestart, err := reopenedClient.Status(ctx)
	require.NoError(t, err)
	require.Equal(t, recovered.SyncInfo, afterRestart.SyncInfo)
}
