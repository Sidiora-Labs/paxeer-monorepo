package inspect

import (
	"context"
	_ "embed"
	"go/doc/comment"
	"go/parser"
	"go/token"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	abcitypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/inspect/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state"
	kvsink "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state/indexer/sink/kv"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/store"
	httpclient "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

//go:embed doc.go
var repairM1105Documentation string

func TestRepairM_1105_OccupiedListenAddress(t *testing.T) {
	for _, tls := range []bool{false, true} {
		name := "http"
		if tls {
			name = "https"
		}
		t.Run(name, func(t *testing.T) {
			cfg, wantState, wantTx := repairM1105Config(t)
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, listener.Close()) })
			cfg.RPC.ListenAddress = "tcp://" + listener.Addr().String()
			if tls {
				// Binding precedes reading TLS files; no certificate fixture is needed.
				cfg.RPC.TLSCertFile = "occupied.crt"
				cfg.RPC.TLSKeyFile = "occupied.key"
			}
			ins, err := NewFromConfig(cfg)
			require.NoError(t, err)
			origin := rpc.Server{Addr: cfg.RPC.ListenAddress, Config: cfg.RPC, Handler: rpc.Handler(cfg.RPC, ins.routes)}
			wantErr := origin.ListenAndServe(t.Context())
			require.Error(t, wantErr)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			finished := make(chan error, 1)
			go func() { finished <- ins.Run(ctx) }()
			select {
			case err = <-finished:
				require.Equal(t, wantErr, err, "Run must preserve the original listener error")
			case <-time.After(3 * time.Second):
				// Release the old implementation's blocked waits before failing.
				cancel()
				select {
				case <-finished:
				case <-time.After(3 * time.Second):
					t.Fatal("Run remained blocked after external cancellation")
				}
				t.Fatal("occupied listen address did not return without external cancellation")
			}
			require.NoError(t, ctx.Err(), "Run must not cancel its caller")
			require.False(t, ins.eventBus.IsRunning())
			require.False(t, ins.indexerService.IsRunning())
			repairM1105Reopen(t, cfg, wantState, wantTx)
		})
	}
}

func TestRepairM_1105_DocumentedExamples(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "doc.go", repairM1105Documentation, parser.ParseComments)
	require.NoError(t, err)
	require.NotNil(t, file.Doc)
	var commentParser comment.Parser
	doc := commentParser.Parse(file.Doc.Text())
	var examples []string
	for _, block := range doc.Content {
		if code, ok := block.(*comment.Code); ok {
			examples = append(examples, code.Text)
		}
	}
	require.Len(t, examples, 2)
	for _, example := range examples {
		_, err := parser.ParseFile(token.NewFileSet(), "example.go",
			"package example\nfunc run() error {\n"+example+"\nreturn nil\n}", 0)
		require.NoError(t, err, "documentation must contain valid Go function bodies")
		require.Contains(t, example, `nodeConfig := config.DefaultConfig().SetRoot("/path/to/node")`)
		require.Contains(t, example, "ins, err := inspect.NewFromConfig(nodeConfig)")
		require.Contains(t, example, "if err != nil {\n\treturn err\n}")
		require.Contains(t, example, "defer cancel()")
	}
	require.Contains(t, examples[0], "go func() { runErr <- ins.Run(ctx) }()")
	require.Contains(t, examples[0], "cancel()\nif err := <-runErr; err != nil {")
	require.Contains(t, examples[1], "if err := ins.Run(ctx); err != nil {\n\treturn err\n}")

	// Exercise the documented full-config constructor, asynchronous Run, returned
	// error and cancellation with real disk stores and a loopback listener.
	cfg, wantState, wantTx := repairM1105Config(t)
	cfg.RPC.ListenAddress = "tcp://127.0.0.1:0"
	ins, err := NewFromConfig(cfg)
	require.NoError(t, err)
	// Consume the configured stores through the inspector's real routes,
	// loopback HTTP transport and production RPC client before shutting down.
	server := httptest.NewServer(rpc.Handler(cfg.RPC, ins.routes))
	t.Cleanup(server.Close)
	client, err := httpclient.New(server.URL)
	require.NoError(t, err)
	height := int64(1)
	params, err := client.ConsensusParams(t.Context(), &height)
	require.NoError(t, err)
	require.Equal(t, wantState.ConsensusParams, params.ConsensusParams)
	hash := types.Tx(wantTx.Tx).Hash()
	result, err := client.Tx(t.Context(), hash[:], false)
	require.NoError(t, err)
	require.Equal(t, types.Tx(wantTx.Tx), result.Tx)
	require.Equal(t, wantTx.Height, result.Height)
	server.Close()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	finished := make(chan error, 1)
	go func() { finished <- ins.Run(ctx) }()
	cancel()
	select {
	case err := <-finished:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("documented cancellation did not stop Run")
	}
	require.False(t, ins.eventBus.IsRunning())
	require.False(t, ins.indexerService.IsRunning())
	repairM1105Reopen(t, cfg, wantState, wantTx)
}

func TestRepairM_1105_ConstructorFailureClosesDatabases(t *testing.T) {
	for _, stage := range []string{"state", "genesis", "sink"} {
		t.Run(stage, func(t *testing.T) {
			cfg, wantState, wantTx := repairM1105Config(t)
			var locked dbm.DB
			switch stage {
			case "state":
				var err error
				locked, err = config.DefaultDBProvider(&config.DBContext{ID: "state", Config: cfg})
				require.NoError(t, err)
			case "genesis":
				require.NoError(t, os.Remove(cfg.GenesisFile()))
			case "sink":
				cfg.TxIndex.Indexer = []string{"unsupported"}
			}
			ins, err := NewFromConfig(cfg)
			if locked != nil {
				require.NoError(t, locked.Close())
			}
			require.Error(t, err)
			require.Nil(t, ins)
			if stage == "genesis" {
				require.ErrorIs(t, err, os.ErrNotExist)
			}
			repairM1105Reopen(t, cfg, wantState, wantTx)
		})
	}
}

func TestRepairM_1105_CallerRetainsStoreOwnership(t *testing.T) {
	nodeConfig, _, _ := repairM1105Config(t)
	stateDB, err := config.DefaultDBProvider(&config.DBContext{ID: "state", Config: nodeConfig})
	require.NoError(t, err)
	blockDB, err := config.DefaultDBProvider(&config.DBContext{ID: "blockstore", Config: nodeConfig})
	require.NoError(t, err)
	states := state.NewStore(stateDB)
	blocks := store.NewBlockStore(blockDB)
	t.Cleanup(func() { require.NoError(t, states.Close()) })
	t.Cleanup(func() { require.NoError(t, blocks.Close()) })
	cfg := config.DefaultRPCConfig()
	// With no RPC listeners Run returns normally while the caller remains live.
	cfg.ListenAddress = ""
	ins := New(cfg, blocks, states, nil)
	finished := make(chan error, 1)
	go func() { finished <- ins.Run(t.Context()) }()
	select {
	case err := <-finished:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("Run waited for caller cancellation after servers returned")
	}
	require.NoError(t, t.Context().Err())
	require.False(t, ins.eventBus.IsRunning())
	require.False(t, ins.indexerService.IsRunning())
	key, value := []byte("caller-owned"), []byte("still open")
	for _, db := range []dbm.DB{stateDB, blockDB} {
		require.NoError(t, db.SetSync(key, value))
		got, err := db.Get(key)
		require.NoError(t, err)
		require.Equal(t, value, got)
	}
}

func repairM1105Config(t *testing.T) (*config.Config, state.State, *abcitypes.TxResultV2) {
	t.Helper()
	cfg := config.DefaultConfig().SetRoot(t.TempDir())
	cfg.DBBackend = "goleveldb"
	cfg.TxIndex.Indexer = []string{"kv"}
	require.NoError(t, os.MkdirAll(filepath.Dir(cfg.GenesisFile()), 0700))
	genesis := &types.GenesisDoc{
		ChainID: "repair-m1105", GenesisTime: time.Unix(1, 0).UTC(),
		ConsensusParams: types.DefaultConsensusParams(),
		Validators: []types.GenesisValidator{{
			PubKey: ed25519.TestSecretKey([]byte("repair-m1105-validator")).Public(), Power: 10,
		}},
	}
	initial, err := state.MakeGenesisState(genesis)
	require.NoError(t, err)
	require.NoError(t, genesis.SaveAs(cfg.GenesisFile()))
	stateDB, err := config.DefaultDBProvider(&config.DBContext{ID: "state", Config: cfg})
	require.NoError(t, err)
	states := state.NewStore(stateDB)
	require.NoError(t, states.Save(initial))
	wantState, err := states.Load()
	require.NoError(t, err)
	require.NoError(t, states.Close())
	txDB, err := config.DefaultDBProvider(&config.DBContext{ID: "tx_index", Config: cfg})
	require.NoError(t, err)
	sink := kvsink.NewEventSink(txDB)
	tx := &abcitypes.TxResultV2{Height: 1, Index: 0, Tx: []byte("repair-m1105-persisted-tx")}
	require.NoError(t, sink.IndexTxEvents([]*abcitypes.TxResultV2{tx}))
	require.NoError(t, sink.Stop())
	return cfg, wantState, tx
}

func repairM1105Reopen(t *testing.T, cfg *config.Config, wantState state.State, wantTx *abcitypes.TxResultV2) {
	t.Helper()
	// Reopening the real on-disk databases proves that no owner retains a lock.
	for _, id := range []string{"blockstore", "state", "tx_index"} {
		db, err := config.DefaultDBProvider(&config.DBContext{ID: id, Config: cfg})
		require.NoError(t, err, "%s was not released", id)
		switch id {
		case "blockstore":
			require.Zero(t, store.NewBlockStore(db).Height())
		case "state":
			got, err := state.NewStore(db).Load()
			require.NoError(t, err)
			require.Equal(t, wantState, got)
		case "tx_index":
			hash := types.Tx(wantTx.Tx).Hash()
			got, err := kvsink.NewEventSink(db).GetTxByHash(hash[:])
			require.NoError(t, err)
			require.Equal(t, wantTx, got)
		}
		require.NoError(t, db.Close())
	}
}
