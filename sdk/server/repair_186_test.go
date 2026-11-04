package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server"
	servertypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	paramstypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func repair186App(t *testing.T, dir string, withParams bool) (*baseapp.BaseApp, paramstypes.Subspace) {
	t.Helper()
	db, err := dbm.NewDB("application", dbm.GoLevelDBBackend, dir)
	require.NoError(t, err)
	opts := viper.New()
	opts.Set("chain-id", "repair-186")
	app := baseapp.NewBaseApp("repair-186", db, nil, tmconfig.DefaultConfig(), opts)
	key := sdk.NewKVStoreKey(paramstypes.StoreKey)
	tkey := sdk.NewTransientStoreKey(paramstypes.TStoreKey)
	app.MountKVStores(map[string]*sdk.KVStoreKey{key.Name(): key})
	app.MountTransientStores(map[string]*sdk.TransientStoreKey{tkey.Name(): tkey})
	space := paramstypes.NewSubspace(codec.NewProtoCodec(codectypes.NewInterfaceRegistry()), codec.NewLegacyAmino(), key, tkey, baseapp.Paramspace).
		WithKeyTable(paramskeeper.ConsensusParamsKeyTable())
	if withParams {
		app.SetParamStore(space)
	}
	require.NoError(t, app.LoadLatestVersion())
	return app, space
}

func repair186Export(t *testing.T, app *baseapp.BaseApp, streaming bool) *tmtypes.GenesisDoc {
	t.Helper()
	home := t.TempDir()
	cfg := tmconfig.DefaultConfig().SetRoot(home)
	require.NoError(t, os.MkdirAll(filepath.Dir(cfg.GenesisFile()), 0755))
	original := &tmtypes.GenesisDoc{
		ChainID: "repair-186", GenesisTime: time.Unix(1700000000, 0).UTC(),
		InitialHeight: 1, ConsensusParams: tmtypes.DefaultConsensusParams(),
	}
	require.NoError(t, original.SaveAs(cfg.GenesisFile()))
	calls := 0
	exporter := func(_ dbm.DB, _ io.Writer, _ int64, _ bool, _ []string, _ servertypes.AppOptions, file *os.File) (servertypes.ExportedApp, error) {
		calls++
		require.Equal(t, streaming, file != nil)
		return servertypes.ExportedApp{
			AppState: json.RawMessage(`{}`), Height: app.LastBlockHeight() + 1,
			ConsensusParams: app.GetConsensusParams(app.NewContext(true, tmproto.Header{})),
		}, nil
	}
	cmd := server.ExportCmd(exporter, home)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	streamFile := filepath.Join(home, "export.json")
	args := []string{}
	if streaming {
		args = []string{"--streaming", "--streaming-file", streamFile}
	}
	cmd.SetArgs(args)
	ctx := context.WithValue(context.Background(), server.ServerContextKey, server.NewContext(viper.New(), cfg))
	err := cmd.ExecuteContext(ctx)
	var encoded []byte
	if streaming {
		require.NoError(t, err)
		encoded, err = os.ReadFile(streamFile)
		require.NoError(t, err)
	} else {
		require.NoError(t, err)
		encoded = output.Bytes()
	}
	require.Equal(t, 1, calls)
	doc, err := tmtypes.GenesisDocFromJSON(encoded)
	require.NoError(t, err)
	require.Equal(t, original.GenesisTime, doc.GenesisTime)
	require.Equal(t, original.ChainID, doc.ChainID)
	require.Equal(t, app.LastBlockHeight()+1, doc.InitialHeight)
	return doc
}

func repair186ImportAndReopen(t *testing.T, doc *tmtypes.GenesisDoc, expected *tmtypes.ConsensusParams) {
	t.Helper()
	require.Equal(t, expected, doc.ConsensusParams)
	request := doc.ToRequestInitChain()
	require.Equal(t, expected.ToProto(), *request.ConsensusParams)
	dir := t.TempDir()
	app, _ := repair186App(t, dir, true)
	_, err := app.InitChain(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, expected.ToProto(), *app.GetConsensusParamsForStateToCommit())
	_, err = app.Commit(context.Background())
	require.NoError(t, err)
	height := app.LastBlockHeight()
	require.NoError(t, app.CommitMultiStore().Close())
	app, _ = repair186App(t, dir, true)
	defer func() { require.NoError(t, app.CommitMultiStore().Close()) }()
	require.Equal(t, height, app.LastBlockHeight())
	recovered := app.GetConsensusParams(app.NewContext(true, tmproto.Header{}))
	require.Equal(t, expected.ToProto(), *recovered)
	params := tmtypes.ConsensusParamsFromProto(*recovered)
	require.NoError(t, params.ValidateConsensusParams())
	require.Equal(t, expected.Timeout.Propose+3*expected.Timeout.ProposeDelta, params.Timeout.ProposeTimeout(3))
	require.Equal(t, expected.Timeout.Vote+2*expected.Timeout.VoteDelta, params.Timeout.VoteTimeout(2))
	require.Equal(t, doc.GenesisTime.Add(expected.Timeout.Commit), params.Timeout.CommitTime(doc.GenesisTime))
}

func TestRepair_186_ExportConsensusRoundTrip(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		name := "json"
		if streaming {
			name = "streaming"
		}
		t.Run(name, func(t *testing.T) {
			expected := tmtypes.DefaultConsensusParams()
			expected.Block = tmtypes.BlockParams{MaxBytes: 1234567, MaxGas: 7654321, MinTxsInBlock: 7, MaxGasWanted: 6543210}
			expected.Evidence = tmtypes.EvidenceParams{MaxAgeNumBlocks: 2345, MaxAgeDuration: 19 * time.Hour, MaxBytes: 45678}
			expected.Version.AppVersion = 186
			expected.Synchrony = tmtypes.SynchronyParams{Precision: 321 * time.Millisecond, MessageDelay: 9 * time.Second}
			expected.Timeout = tmtypes.TimeoutParams{
				Propose: 7 * time.Second, ProposeDelta: 230 * time.Millisecond,
				Vote: 340 * time.Millisecond, VoteDelta: 450 * time.Millisecond,
				Commit: 560 * time.Millisecond, BypassCommitTimeout: true,
			}
			expected.ABCI = tmtypes.ABCIParams{VoteExtensionsEnableHeight: 17, RecheckTx: true}
			app, _ := repair186App(t, t.TempDir(), true)
			defer func() { require.NoError(t, app.CommitMultiStore().Close()) }()
			doc := &tmtypes.GenesisDoc{
				ChainID: "repair-186", GenesisTime: time.Unix(1700000000, 0).UTC(),
				InitialHeight: 4, ConsensusParams: expected,
			}
			_, err := app.InitChain(context.Background(), doc.ToRequestInitChain())
			require.NoError(t, err)
			_, err = app.Commit(context.Background())
			require.NoError(t, err)
			repair186ImportAndReopen(t, repair186Export(t, app, streaming), expected)
		})
	}
}

func TestRepair_186_ExportConsensusNilAware(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		for _, name := range []string{"nil", "empty", "legacy", "partial_timing"} {
			mode := "json/"
			if streaming {
				mode = "streaming/"
			}
			t.Run(mode+name, func(t *testing.T) {
				app, space := repair186App(t, t.TempDir(), name != "nil")
				defer func() { require.NoError(t, app.CommitMultiStore().Close()) }()
				ctx := sdk.NewContext(app.CommitMultiStore(), tmproto.Header{ChainID: "repair-186"}, false)
				expected := tmtypes.DefaultConsensusParams()
				if name == "legacy" || name == "partial_timing" {
					expected.Block.MaxGas = 987654
					pb := expected.ToProto()
					space.Set(ctx, baseapp.ParamStoreKeyBlockParams, pb.Block)
					space.Set(ctx, baseapp.ParamStoreKeyEvidenceParams, pb.Evidence)
					space.Set(ctx, baseapp.ParamStoreKeyValidatorParams, pb.Validator)
				}
				if name == "partial_timing" {
					expected.Synchrony.Precision = 123 * time.Millisecond
					expected.Timeout.Vote = 456 * time.Millisecond
					expected.Timeout.BypassCommitTimeout = true
					space.Set(ctx, baseapp.ParamStoreKeySynchronyParams, tmproto.SynchronyParams{Precision: &expected.Synchrony.Precision})
					space.Set(ctx, baseapp.ParamStoreKeyTimeoutParams, tmproto.TimeoutParams{Vote: &expected.Timeout.Vote, BypassCommitTimeout: true})
				}
				app.CommitMultiStore().Commit(true)
				repair186ImportAndReopen(t, repair186Export(t, app, streaming), expected)
			})
		}
	}
}
