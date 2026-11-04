package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmcmd "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/cmd/tendermint/commands"
	tmcfg "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/tcp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/node"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/local"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/api"
	serverconfig "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/config"
	servertypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/types"
	legacy "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/rootmulti"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	rootmulti "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	paramstypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	storageconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/google/orderedcode"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

// This application fixture uses BaseApp's ABCI handlers and persistent multistore.
// Its only module records committed block heights, giving recovery an observable
// application value as well as the app hash and consensus height.
type repair1309Application struct {
	*baseapp.BaseApp
	apiReady    chan *api.Server
	localClient client.LocalClient
}

func (app *repair1309Application) RegisterAPIRoutes(s *api.Server, _ serverconfig.APIConfig) {
	s.Router.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ready"))
	})
	if app.apiReady != nil {
		app.apiReady <- s
	}
}

func (app *repair1309Application) RegisterLocalServices(c client.LocalClient, _ client.TxConfig) {
	app.localClient = c
}

func repair1309Options() *viper.Viper {
	v := viper.New()
	v.Set("chain-id", "test-chain")
	v.Set("minimum-gas-prices", "0stake")
	v.Set("telemetry.global-labels", []interface{}{})
	v.Set("pruning", "nothing")
	return v
}

func repair1309App(t *testing.T, cfg *tmcfg.Config, legacyStore bool) (*repair1309Application, *storetypes.KVStoreKey) {
	t.Helper()
	opts := repair1309Options()
	app := &repair1309Application{BaseApp: baseapp.NewBaseApp("repair-1309", dbm.NewMemDB(), nil, cfg, opts)}
	key := storetypes.NewKVStoreKey("blocks")
	if !legacyStore {
		sc := storageconfig.DefaultStateCommitConfig()
		sc.WriteMode = storageconfig.MemiavlOnly
		sc.MemIAVLConfig.SnapshotInterval = 1
		sc.MemIAVLConfig.SnapshotKeepRecent = 100
		sc.MemIAVLConfig.SnapshotMinTimeInterval = 0
		sc.MemIAVLConfig.AsyncCommitBuffer = 0
		app.SetCMS(rootmulti.NewStore(filepath.Join(cfg.RootDir, "application"), sc, storageconfig.StateStoreConfig{}, nil))
	}
	pkey := storetypes.NewKVStoreKey(paramstypes.StoreKey)
	tkey := storetypes.NewTransientStoreKey(paramstypes.TStoreKey)
	app.MountKVStores(map[string]*storetypes.KVStoreKey{key.Name(): key, pkey.Name(): pkey})
	app.MountTransientStores(map[string]*storetypes.TransientStoreKey{tkey.Name(): tkey})
	space := paramstypes.NewSubspace(codec.NewProtoCodec(codectypes.NewInterfaceRegistry()), codec.NewLegacyAmino(), pkey, tkey, baseapp.Paramspace).WithKeyTable(paramskeeper.ConsensusParamsKeyTable())
	app.SetParamStore(space)
	var validators []abci.ValidatorUpdate
	if _, err := os.Stat(cfg.GenesisFile()); err == nil {
		doc, err := tmtypes.GenesisDocFromFile(cfg.GenesisFile())
		require.NoError(t, err)
		validators = doc.ValidatorUpdates()
	}
	app.SetInitChainer(func(_ sdk.Context, _ abci.RequestInitChain) abci.ResponseInitChain {
		return abci.ResponseInitChain{Validators: validators}
	})
	app.SetProcessProposalHandler(func(_ sdk.Context, _ *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
		return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_ACCEPT}, nil
	})
	app.SetFinalizeBlocker(func(ctx sdk.Context, req *abci.RequestFinalizeBlock) (*abci.ResponseFinalizeBlock, error) {
		ctx.KVStore(key).Set([]byte("height"), []byte(fmt.Sprint(req.Header.Height)))
		app.SetDeliverStateToCommit()
		app.WriteState()
		return &abci.ResponseFinalizeBlock{AppHash: app.GetWorkingHash()}, nil
	})
	require.NoError(t, app.LoadLatestVersion())
	return app, key
}

func repair1309Commit(t *testing.T, app *repair1309Application, height int64) []byte {
	t.Helper()
	resp, err := app.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{Header: &tmproto.Header{ChainID: "test-chain", Height: height, Time: time.Now().UTC()}})
	require.NoError(t, err)
	var legacyCommit *storetypes.CommitInfo
	if cms, ok := app.CommitMultiStore().(*legacy.Store); ok {
		// Legacy DB adapters define working and committed hashes separately.
		working := storetypes.CommitInfo{}
		legacyCommit = &storetypes.CommitInfo{Version: height}
		for key, store := range cms.GetStores() {
			if store.GetStoreType() == storetypes.StoreTypeTransient {
				continue
			}
			hash, err := store.GetWorkingHash()
			require.NoError(t, err)
			working.StoreInfos = append(working.StoreInfos, storetypes.StoreInfo{Name: key.Name(), CommitId: storetypes.CommitID{Hash: hash}})
			legacyCommit.StoreInfos = append(legacyCommit.StoreInfos, storetypes.StoreInfo{Name: key.Name(), CommitId: store.LastCommitID()})
		}
		require.Equal(t, working.Hash(), resp.AppHash)
	}
	_, err = app.Commit(context.Background())
	require.NoError(t, err)
	if legacyCommit != nil {
		require.Equal(t, legacyCommit.CommitID(), app.LastCommitID())
	} else {
		require.Equal(t, resp.AppHash, app.LastCommitID().Hash)
	}
	return append([]byte(nil), app.LastCommitID().Hash...)
}

func repair1309Context(cfg *tmcfg.Config, v *viper.Viper) context.Context {
	return context.WithValue(context.Background(), ServerContextKey, NewContext(v, cfg))
}

func TestRepairM_1309_ExportStreamError(t *testing.T) {
	cfg := tmcfg.TestConfig().SetRoot(t.TempDir())
	require.NoError(t, os.MkdirAll(filepath.Dir(cfg.GenesisFile()), 0700))
	doc := &tmtypes.GenesisDoc{ChainID: "test-chain", InitialHeight: 1, GenesisTime: time.Now().UTC(), ConsensusParams: tmtypes.DefaultConsensusParams()}
	require.NoError(t, doc.SaveAs(cfg.GenesisFile()))
	app, key := repair1309App(t, cfg, false)
	defer func() { require.NoError(t, app.Close()) }()
	repair1309Commit(t, app, 1)
	exporter := func(_ dbm.DB, _ io.Writer, _ int64, _ bool, _ []string, _ servertypes.AppOptions, file *os.File) (servertypes.ExportedApp, error) {
		require.NotNil(t, file)
		encoded, err := json.Marshal(map[string]string{"height": string(app.CommitMultiStore().GetKVStore(key).Get([]byte("height")))})
		if err != nil {
			return servertypes.ExportedApp{}, err
		}
		if _, err := fmt.Fprintf(file, "%s\n", encoded); err != nil {
			return servertypes.ExportedApp{}, err
		}
		return servertypes.ExportedApp{Height: app.LastBlockHeight() + 1}, nil
	}
	output := filepath.Join(cfg.RootDir, "export.jsonl")
	cmd := ExportCmd(exporter, cfg.RootDir)
	cmd.SetArgs([]string{"--streaming", "--streaming-file", output})
	require.NoError(t, cmd.ExecuteContext(repair1309Context(cfg, repair1309Options())))
	data, err := os.ReadFile(output)
	require.NoError(t, err)
	lines := bytes.SplitN(data, []byte("\n"), 2)
	require.JSONEq(t, `{"height":"1"}`, string(lines[0]))
	recovered, err := tmtypes.GenesisDocFromJSON(lines[1])
	require.NoError(t, err)
	require.Equal(t, int64(2), recovered.InitialHeight)
	require.Equal(t, doc.ChainID, recovered.ChainID)

	cmd = ExportCmd(func(_ dbm.DB, _ io.Writer, _ int64, _ bool, _ []string, _ servertypes.AppOptions, file *os.File) (servertypes.ExportedApp, error) {
		require.NoError(t, file.Close())
		return servertypes.ExportedApp{Height: 2}, nil
	}, cfg.RootDir)
	cmd.SetArgs([]string{"--streaming", "--streaming-file", output})
	err = cmd.ExecuteContext(repair1309Context(cfg, repair1309Options()))
	require.ErrorIs(t, err, os.ErrClosed)
	require.ErrorContains(t, err, "error writing genesis state to file")
	require.ErrorContains(t, err, "error closing genesis state file")
}

func TestRepairM_1309_StartGenesisError(t *testing.T) {
	for _, name := range []string{"missing", "malformed", "mismatch"} {
		t.Run(name, func(t *testing.T) {
			cfg := tmcfg.TestConfig().SetRoot(t.TempDir())
			require.NoError(t, os.MkdirAll(filepath.Dir(cfg.GenesisFile()), 0700))
			require.NoError(t, os.WriteFile(filepath.Join(cfg.RootDir, "config/client.toml"), []byte(`chain-id = "test-chain"
keyring-backend = "test"
node = "tcp://127.0.0.1:26657"
output = "json"
broadcast-mode = "sync"
`), 0600))
			if name == "malformed" {
				require.NoError(t, os.WriteFile(cfg.GenesisFile(), []byte("{bad json"), 0600))
			}
			if name == "mismatch" {
				doc := &tmtypes.GenesisDoc{ChainID: "other-chain", GenesisTime: time.Now().UTC(), ConsensusParams: tmtypes.DefaultConsensusParams()}
				require.NoError(t, doc.SaveAs(cfg.GenesisFile()))
			}
			created := false
			cmd := StartCmd(func(dbm.DB, io.Writer, *tmcfg.Config, servertypes.AppOptions) servertypes.Application {
				created = true
				t.Fatal("app must not be created")
				return nil
			}, cfg.RootDir, nil)
			cmd.SetArgs([]string{})
			v := repair1309Options()
			cctx := repair1309ClientContext().WithHomeDir(cfg.RootDir).WithViper("")
			ctx := context.WithValue(repair1309Context(cfg, v), client.ClientContextKey, &cctx)
			if name == "mismatch" {
				require.PanicsWithValue(t, "genesis file chain-id=other-chain does not equal config.toml chain-id=test-chain", func() { _ = cmd.ExecuteContext(ctx) })
			} else {
				var err error
				require.NotPanics(t, func() { err = cmd.ExecuteContext(ctx) })
				require.ErrorContains(t, err, "load genesis")
				if name == "missing" {
					require.ErrorIs(t, err, os.ErrNotExist)
				} else {
					require.ErrorContains(t, err, "invalid character")
				}
			}
			require.False(t, created)
		})
	}
}

func repair1309ClientContext() client.Context {
	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)
	return client.Context{}.WithChainID("test-chain").WithInterfaceRegistry(registry).WithCodec(cdc).WithTxConfig(authtx.NewTxConfig(cdc, authtx.DefaultSignModes))
}

func TestRepairM_1309_StartValidationIgnored(t *testing.T) {
	for _, name := range []string{"gas-prices", "snapshot-pruning"} {
		t.Run(name, func(t *testing.T) {
			cfg := tmcfg.TestConfig().SetRoot(t.TempDir())
			v := repair1309Options()
			if name == "gas-prices" {
				v.Set("minimum-gas-prices", "")
			} else {
				v.Set("pruning", "everything")
				v.Set("state-sync.snapshot-interval", 1)
			}
			parsed, err := serverconfig.GetConfig(v)
			require.NoError(t, err)
			expected := parsed.ValidateBasic(cfg)
			require.Error(t, expected)
			created := false
			err = startInProcess(NewContext(v, cfg), repair1309ClientContext(), func(dbm.DB, io.Writer, *tmcfg.Config, servertypes.AppOptions) servertypes.Application {
				created = true
				t.Fatal("invalid configuration reached appCreator")
				return nil
			}, nil, nil, nil)
			require.ErrorContains(t, err, expected.Error())
			require.False(t, created)
		})
	}
}

func TestRepairM_1309_StartLateServiceError(t *testing.T) {
	cfg := tmcfg.TestConfig().SetRoot(t.TempDir())
	v := repair1309Options()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	grpcListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	grpcAddress := grpcListener.Addr().String()
	require.NoError(t, listener.Close())
	require.NoError(t, grpcListener.Close())
	v.Set("grpc-only", true)
	v.Set("grpc.address", grpcAddress)
	v.Set("grpc-web.enable", false)
	v.Set("api.enable", true)
	v.Set("api.address", "tcp://"+address)
	v.Set("api.max-open-connections", 10)
	v.Set("api.rpc-max-body-bytes", 1000)
	app, _ := repair1309App(t, cfg, false)
	app.apiReady = make(chan *api.Server, 1)
	done := make(chan error, 1)
	go func() {
		done <- startInProcess(NewContext(v, cfg), repair1309ClientContext(), func(dbm.DB, io.Writer, *tmcfg.Config, servertypes.AppOptions) servertypes.Application { return app }, nil, nil, nil)
	}()
	var apiServer *api.Server
	select {
	case apiServer = <-app.apiReady:
	case <-time.After(10 * time.Second):
		t.Fatal("API registration timed out")
	}
	t.Cleanup(func() { _ = apiServer.Close() })
	httpClient := &http.Client{Timeout: time.Second}
	require.Eventually(t, func() bool {
		resp, err := httpClient.Get("http://" + address + "/health")
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		return err == nil && resp.StatusCode == http.StatusOK && string(body) == "ready"
	}, 10*time.Second, 20*time.Millisecond)
	// Both API and gRPC have passed their full startup windows before failure.
	timer := time.NewTimer(2*servertypes.ServerStartTime + 100*time.Millisecond)
	defer timer.Stop()
	select {
	case err := <-done:
		t.Fatalf("start returned before listener failure: %v", err)
	case <-timer.C:
	}
	require.NoError(t, apiServer.Close())
	select {
	case err := <-done:
		require.ErrorContains(t, err, "api server")
		require.ErrorIs(t, err, net.ErrClosed)
	case <-time.After(10 * time.Second):
		t.Fatal("late API failure did not shut down startInProcess")
	}
	// Reopening the persistent store proves shutdown released application locks.
	reopened, _ := repair1309App(t, cfg, false)
	require.NoError(t, reopened.Close())
	for _, addr := range []string{address, grpcAddress} {
		listener, err := net.Listen("tcp", addr)
		require.NoError(t, err, "shutdown must release every service listener")
		require.NoError(t, listener.Close())
	}
}

func TestRepairM_1309_RollbackPreflight(t *testing.T) {
	for _, name := range []string{"zero", "negative", "below-zero", "missing-consensus-block", "unsupported-app"} {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			cfg := setupTendermintStateDB(t, home, 3)
			app, _ := repair1309App(t, cfg, name == "unsupported-app")
			for h := int64(1); h <= 3; h++ {
				repair1309Commit(t, app, h)
			}
			before := app.LastCommitID()
			if name == "missing-consensus-block" {
				db, err := dbm.NewDB("blockstore", dbm.GoLevelDBBackend, tmcfg.ResolveDBDir("blockstore", cfg.DBDir()))
				require.NoError(t, err)
				key, err := orderedcode.Append(nil, int64(0), int64(2))
				require.NoError(t, err)
				require.NoError(t, db.DeleteSync(key))
				require.NoError(t, db.Close())
			}
			count := "1"
			switch name {
			case "zero":
				count = "0"
			case "negative":
				count = "-1"
			case "below-zero":
				count = "4"
			}
			created := false
			cmd := NewRollbackCmd(func(dbm.DB, io.Writer, *tmcfg.Config, servertypes.AppOptions) servertypes.Application {
				created = true
				return app
			}, home)
			cmd.SetArgs([]string{"--num-blocks", count})
			err := cmd.ExecuteContext(repair1309Context(cfg, repair1309Options()))
			require.Error(t, err)
			switch name {
			case "zero", "negative":
				require.ErrorContains(t, err, "must be positive")
			case "below-zero":
				require.ErrorContains(t, err, "cannot rollback below 0")
			case "missing-consensus-block":
				require.ErrorContains(t, err, "consensus rollback preflight")
			case "unsupported-app":
				require.ErrorIs(t, err, legacy.ErrVersionedStateUnsupported)
			}
			require.Equal(t, before, app.LastCommitID())
			state, err := tmcmd.LoadTendermintState(cfg)
			require.NoError(t, err)
			require.Equal(t, int64(3), state.LastBlockHeight)
			_, err = os.Stat(rollbackIntentPath(cfg))
			require.ErrorIs(t, err, os.ErrNotExist)
			if !created {
				require.NoError(t, app.Close())
			}
			if name != "unsupported-app" {
				reopened, _ := repair1309App(t, cfg, false)
				require.Equal(t, before, reopened.LastCommitID())
				require.NoError(t, reopened.Close())
			}
		})
	}
}

func TestRepairM_1309_RollbackRecovery(t *testing.T) {
	cfg, err := tmcfg.ResetTestRootWithChainID(t.TempDir(), "repair1309", "test-chain")
	require.NoError(t, err)
	// ResetTestRoot uses memdb; rollback and restart must reopen on-disk stores.
	cfg.DBBackend = string(dbm.GoLevelDBBackend)
	cfg.RPC.ListenAddress = "tcp://127.0.0.1:0"
	cfg.P2P.ListenAddress = "tcp://" + tcp.TestReserveAddr().String()
	cfg.P2P.PexReactor = false
	cfg.TxIndex.Indexer = []string{"null"}
	cfg.Consensus.CreateEmptyBlocksInterval = 100 * time.Millisecond
	app, key := repair1309App(t, cfg, false)
	n, err := node.New(context.Background(), cfg, func() {}, app, nil, nil, node.DefaultMetricsProvider(cfg.Instrumentation)("test-chain"), tmtypes.DefaultConsensusPolicy())
	require.NoError(t, err)
	require.NoError(t, n.Start(context.Background()))
	t.Cleanup(func() {
		if n.IsRunning() {
			n.Stop()
		}
		n.Wait()
	})
	rpc, err := local.New(n)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		block, err := rpc.Block(context.Background(), nil)
		return err == nil && block != nil && block.Block != nil && block.Block.Height >= 4
	}, 20*time.Second, 20*time.Millisecond)
	n.Stop()
	n.Wait()
	for _, name := range []string{"state", "blockstore"} {
		require.DirExists(t, filepath.Join(tmcfg.ResolveDBDir(name, cfg.DBDir()), name+".db"))
	}
	state, err := tmcmd.LoadTendermintState(cfg)
	require.NoError(t, err)
	walPath := cfg.Consensus.WalFile()
	walData, err := os.ReadFile(walPath)
	require.NoError(t, err)
	require.NotEmpty(t, walData)
	height := app.LastBlockHeight()
	require.Equal(t, height, state.LastBlockHeight)
	target := height - 2
	intent := &rollbackIntent{Target: target, AppHeight: height, ConsensusHeight: height, NumBlocks: 2, ChainID: "test-chain"}
	require.NoError(t, prepareRollback(cfg, app, intent))
	_, err = rollbackAppState(app, target)
	require.NoError(t, err)
	// Stop after the real application mutation, before consensus mutation.
	require.NoError(t, app.Close())
	state, err = tmcmd.LoadTendermintState(cfg)
	require.NoError(t, err)
	require.Equal(t, height, state.LastBlockHeight)
	app, key = repair1309App(t, cfg, false)
	require.Equal(t, target, app.LastBlockHeight())
	require.Equal(t, fmt.Sprint(target), string(app.CommitMultiStore().GetKVStore(key).Get([]byte("height"))))
	recoveredHash := append([]byte(nil), app.LastCommitID().Hash...)
	pending, err := loadRollbackIntent(cfg)
	require.NoError(t, err)
	require.Equal(t, intent, pending)
	startErr := startInProcess(NewContext(repair1309Options(), cfg), repair1309ClientContext(), func(dbm.DB, io.Writer, *tmcfg.Config, servertypes.AppOptions) servertypes.Application {
		t.Fatal("pending rollback must refuse startup")
		return nil
	}, nil, nil, nil)
	require.ErrorContains(t, startErr, "unfinished rollback")
	cmd := NewRollbackCmd(func(dbm.DB, io.Writer, *tmcfg.Config, servertypes.AppOptions) servertypes.Application { return app }, cfg.RootDir)
	cmd.SetArgs([]string{})
	require.NoError(t, cmd.ExecuteContext(repair1309Context(cfg, repair1309Options())))
	_, err = os.Stat(walPath)
	require.ErrorIs(t, err, os.ErrNotExist)
	archives, err := filepath.Glob(filepath.Join(filepath.Dir(walPath), fmt.Sprintf(".rollback-wal-to-%d-*", target), filepath.Base(walPath)))
	require.NoError(t, err)
	require.Len(t, archives, 1)
	archivedWAL, err := os.ReadFile(archives[0])
	require.NoError(t, err)
	require.Equal(t, walData, archivedWAL)
	state, err = tmcmd.LoadTendermintState(cfg)
	require.NoError(t, err)
	require.Equal(t, target, state.LastBlockHeight)
	_, err = os.Stat(rollbackIntentPath(cfg))
	require.ErrorIs(t, err, os.ErrNotExist)
	app, key = repair1309App(t, cfg, false)
	require.Equal(t, target, app.LastBlockHeight())
	require.Equal(t, recoveredHash, app.LastCommitID().Hash)
	require.Equal(t, fmt.Sprint(target), string(app.CommitMultiStore().GetKVStore(key).Get([]byte("height"))))
	cfg.P2P.ListenAddress = "tcp://" + tcp.TestReserveAddr().String()
	n2, err := node.New(context.Background(), cfg, func() {}, app, nil, nil, node.DefaultMetricsProvider(cfg.Instrumentation)("test-chain"), tmtypes.DefaultConsensusPolicy())
	require.NoError(t, err)
	require.NoError(t, n2.Start(context.Background()))
	t.Cleanup(func() {
		if n2.IsRunning() {
			n2.Stop()
		}
		n2.Wait()
		require.NoError(t, app.Close())
	})
	rpc2, err := local.New(n2)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		block, err := rpc2.Block(context.Background(), nil)
		return err == nil && block != nil && block.Block != nil && block.Block.Height > height
	}, 20*time.Second, 20*time.Millisecond, "recovered node must resume real consensus block production")
	first := target + 1
	block, err := rpc2.Block(context.Background(), &first)
	require.NoError(t, err)
	require.Equal(t, recoveredHash, []byte(block.Block.AppHash))
}

func TestRepairM_1309_RollbackUnequalHashes(t *testing.T) {
	cfg := setupTendermintStateDB(t, t.TempDir(), 3)
	app, key := repair1309App(t, cfg, false)
	for h := int64(1); h <= 3; h++ {
		repair1309Commit(t, app, h)
	}
	command := func(app *repair1309Application) error {
		cmd := NewRollbackCmd(func(dbm.DB, io.Writer, *tmcfg.Config, servertypes.AppOptions) servertypes.Application { return app }, cfg.RootDir)
		cmd.SetArgs([]string{})
		return cmd.ExecuteContext(repair1309Context(cfg, repair1309Options()))
	}
	require.NoError(t, command(app))
	state, err := tmcmd.LoadTendermintState(cfg)
	require.NoError(t, err)
	require.Equal(t, int64(2), state.LastBlockHeight)
	app, key = repair1309App(t, cfg, false)
	require.Equal(t, int64(2), app.LastBlockHeight())
	require.Equal(t, "2", string(app.CommitMultiStore().GetKVStore(key).Get([]byte("height"))))
	require.NotEqual(t, state.AppHash, app.LastCommitID().Hash, "correcting an app transition need not retain the old consensus hash")
	before := app.LastCommitID()
	// A crash after both stores changed but before intent removal must not
	// start another rollback on recovery.
	require.NoError(t, saveRollbackIntent(cfg, &rollbackIntent{Target: 2, AppHeight: 3, ConsensusHeight: 3, NumBlocks: 1, ChainID: "test-chain"}))
	require.NoError(t, command(app))
	app, _ = repair1309App(t, cfg, false)
	require.Equal(t, before, app.LastCommitID())
	require.NoError(t, app.Close())
	state, err = tmcmd.LoadTendermintState(cfg)
	require.NoError(t, err)
	require.Equal(t, int64(2), state.LastBlockHeight)
}
