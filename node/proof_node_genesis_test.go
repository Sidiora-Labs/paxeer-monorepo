// Proves: AddGenesisReceiptKeyCmd and AddGenesisSequencerLeaderCmd -> paxd subprocesses, genesis.json and app.toml -> ReadGenesisImportConfig, App.InitChain and SequencerKeeper.
//
// Repairs node/genesis.go:ReadGenesisImportConfig accepting an enabled stream
// with no filename, and node/app.go:InitChainer accepting an unreadable stream
// as an empty genesis. The proof also uses App.ExportAppToFileStateAndValidators
// to produce the stream consumed by a fresh App after the exporter has closed.
package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmcfg "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	serverconfig "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/config"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func TestProofNodeGenesis(t *testing.T) {
	root, err := filepath.Abs("..")
	require.NoError(t, err)
	binary := filepath.Join(t.TempDir(), "paxd")
	buildCtx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	build := exec.CommandContext(buildCtx, "go", "build", "-o", binary, "./daemon/paxd")
	build.Dir = root
	output, err := build.CombinedOutput()
	require.NoError(t, err, "build real paxd: %s", output)

	home := t.TempDir()
	run := func(args ...string) ([]byte, error) {
		t.Helper()
		ctx, stop := context.WithTimeout(context.Background(), 30*time.Second)
		defer stop()
		cmd := exec.CommandContext(ctx, binary, append(args, "--home", home)...)
		return cmd.CombinedOutput()
	}
	succeed := func(args ...string) []byte {
		t.Helper()
		out, runErr := run(args...)
		require.NoError(t, runErr, "paxd %v: %s", args, out)
		return out
	}
	const chainID = "proof-node-genesis"
	succeed("init", "proof-genesis", "--chain-id", chainID, "--mode", "validator")
	keyFile := filepath.Join(home, "config", "receipt-key")
	out := succeed("sequencer", "gen-receipt-key", "--output", keyFile)
	var keyOutput struct {
		PublicKey string `json:"public_key"`
	}
	require.NoError(t, json.Unmarshal(out, &keyOutput))
	public, err := hex.DecodeString(keyOutput.PublicKey)
	require.NoError(t, err)
	require.Len(t, public, 32)
	keyID := sha256.Sum256(public)
	receiptArgs := []string{"add-genesis-receipt-key", "--public-key", keyOutput.PublicKey,
		"--first-height", "1", "--last-height", "1000"}
	succeed(receiptArgs...)
	config := tmcfg.DefaultConfig().SetRoot(home)
	validator, err := privval.LoadFilePV(config.PrivValidator.KeyFile(), config.PrivValidator.StateFile())
	require.NoError(t, err)
	leader := validator.Key.PubKey.Bytes()
	require.Len(t, leader, 32)
	leaderArgs := []string{"add-genesis-sequencer-leader", "--pubkey", hex.EncodeToString(leader)}
	succeed(leaderArgs...)
	succeed("validate-genesis")

	// Every producer has exited before the next process reopens its output.
	// A duplicate or malformed producer must refuse without changing that file.
	before, err := os.ReadFile(config.GenesisFile())
	require.NoError(t, err)
	for _, refusal := range []struct {
		args []string
		text string
	}{
		{receiptArgs, "already pinned"},
		{leaderArgs, "repeats an earlier leader"},
		{[]string{"add-genesis-sequencer-leader", "--pubkey", "zz"}, "invalid consensus public key hex"},
	} {
		out, refusalErr := run(refusal.args...)
		require.Error(t, refusalErr, "paxd %v unexpectedly succeeded", refusal.args)
		require.Contains(t, string(out), refusal.text)
		after, readErr := os.ReadFile(config.GenesisFile())
		require.NoError(t, readErr)
		require.Equal(t, before, after, "refusal changed persisted genesis")
	}
	doc, err := tmtypes.GenesisDocFromFile(config.GenesisFile())
	require.NoError(t, err)
	require.Equal(t, chainID, doc.ChainID)

	readOptions := func(path string) *viper.Viper {
		t.Helper()
		opts := viper.New()
		opts.SetConfigFile(path)
		require.NoError(t, opts.ReadInConfig())
		// Use the production disk-backed SC store, with synchronous commits and
		// no periodic snapshots or network listeners during this focused proof.
		opts.Set("chain-id", chainID)
		opts.Set(FlagSCEnable, true)
		opts.Set(FlagSCAsyncCommitBuffer, 0)
		opts.Set(FlagSCSnapshotInterval, 0)
		opts.Set(FlagSSEnable, false)
		opts.Set("evm.http_enabled", false)
		opts.Set("evm.ws_enabled", false)
		return opts
	}
	openApp := func(appHome string, opts *viper.Viper) *App {
		t.Helper()
		db, openErr := dbm.NewDB("application", dbm.GoLevelDBBackend, filepath.Join(appHome, "data"))
		require.NoError(t, openErr)
		return New(db, nil, true, map[int64]bool{}, appHome, 1, false,
			tmcfg.DefaultConfig().SetRoot(appHome), MakeEncodingConfig(), nil, opts, EmptyWasmOpts, nil)
	}
	opts := readOptions(filepath.Join(home, "config", "app.toml"))
	importConfig, err := ReadGenesisImportConfig(opts)
	require.NoError(t, err)
	require.False(t, importConfig.StreamGenesisImport)
	a := openApp(home, opts)
	t.Cleanup(func() {
		if a != nil {
			require.NoError(t, a.Close())
		}
	})
	consensusParams := doc.ConsensusParams.ToProto()
	_, err = a.InitChain(context.Background(), &abci.RequestInitChain{
		Time: doc.GenesisTime, ChainId: doc.ChainID, InitialHeight: doc.InitialHeight,
		ConsensusParams: &consensusParams, AppStateBytes: doc.AppState,
	})
	require.NoError(t, err)
	header := tmproto.Header{ChainID: chainID, Height: 1, Time: doc.GenesisTime}
	ctx := a.NewContext(false, header)
	want := seqtypes.GenesisState{
		Leaders:          [][32]byte{[32]byte(leader)},
		ReceiptKeys:      []seqtypes.ReceiptKey{{KeyId: keyID, PublicKey: [32]byte(public), FirstHeight: 1, LastHeight: 1000}},
		ActivationHeight: 1,
	}
	assertConsumer := func(host *App, consumerCtx sdk.Context) {
		t.Helper()
		got, readErr := host.SequencerKeeper.ExportGenesis(consumerCtx)
		require.NoError(t, readErr)
		require.Equal(t, want, got)
		pins, readErr := host.SequencerKeeper.ReceiptKeyPins(consumerCtx)
		require.NoError(t, readErr)
		require.Len(t, pins, 1)
		require.Equal(t, keyID, pins[0].SequencerID)
		require.Equal(t, [32]byte(public), pins[0].PublicKey)
		require.Equal(t, uint64(1), pins[0].FirstBatchNumber)
		require.Equal(t, uint64(1000), pins[0].LastBatchNumber)
	}
	assertConsumer(a, ctx)
	require.NoError(t, a.SetSealedStateToCommit(ctx, ctx.MultiStore().(sdk.CacheMultiStore)))
	_, err = a.Commit(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(1), a.LastBlockHeight())
	require.NoError(t, a.Close())
	a = nil
	a = openApp(home, readOptions(filepath.Join(home, "config", "app.toml")))
	require.Equal(t, int64(1), a.LastBlockHeight())
	assertConsumer(a, a.NewUncachedContext(true, header))

	streamPath := filepath.Join(t.TempDir(), "genesis-stream.json")
	stream, err := os.Create(streamPath)
	require.NoError(t, err)
	exported, err := a.ExportAppToFileStateAndValidators(false, nil, stream)
	require.NoError(t, err)
	require.NoError(t, stream.Close())
	require.NoError(t, a.Close())
	a = nil

	// Persist the actual template key, close its writer, then reopen via Viper
	// and the server config reader as well as the node's own config reader.
	streamHome := t.TempDir()
	appConfig := serverconfig.DefaultConfig()
	appConfig.Genesis.StreamImport = true
	appConfig.Genesis.GenesisStreamFile = streamPath
	tmpl, err := template.New("app").Parse(serverconfig.AutoManagedConfigTemplate)
	require.NoError(t, err)
	configPath := filepath.Join(streamHome, "app.toml")
	configFile, err := os.Create(configPath)
	require.NoError(t, err)
	require.NoError(t, tmpl.Execute(configFile, appConfig))
	require.NoError(t, configFile.Close())
	streamOpts := readOptions(configPath)
	importConfig, err = ReadGenesisImportConfig(streamOpts)
	require.NoError(t, err)
	require.True(t, importConfig.StreamGenesisImport)
	require.Equal(t, streamPath, importConfig.GenesisStreamFile)
	serverConfig, err := serverconfig.GetConfig(streamOpts)
	require.NoError(t, err)
	require.Equal(t, serverConfig.Genesis.StreamImport, importConfig.StreamGenesisImport)
	require.Equal(t, serverConfig.Genesis.GenesisStreamFile, importConfig.GenesisStreamFile)
	a = openApp(streamHome, streamOpts)
	_, err = a.InitChain(context.Background(), &abci.RequestInitChain{
		Time: doc.GenesisTime, ChainId: chainID, InitialHeight: exported.Height,
		ConsensusParams: exported.ConsensusParams,
	})
	require.NoError(t, err)
	assertConsumer(a, a.NewContext(false, tmproto.Header{ChainID: chainID, Height: exported.Height, Time: doc.GenesisTime}))

	// Refuse an absent or non-file stream before any module can overwrite the
	// successfully imported state. This previously silently imported nothing.
	for _, badPath := range []string{filepath.Join(streamHome, "missing"), streamHome} {
		a.genesisImportConfig.GenesisStreamFile = badPath
		consumerCtx := a.NewContext(false, header)
		require.Panics(t, func() { a.InitChainer(consumerCtx, abci.RequestInitChain{}) })
		assertConsumer(a, consumerCtx)
	}
}

func TestProofNodeGenesisConfigRefusals(t *testing.T) {
	for _, bad := range []string{`""`, `"   "`, `["stream.json"]`, `{ path = "stream.json" }`} {
		opts := viper.New()
		opts.SetConfigType("toml")
		require.NoError(t, opts.ReadConfig(strings.NewReader(
			"[genesis]\nstream-import = true\ngenesis-stream-file = "+bad+"\n")))
		_, err := ReadGenesisImportConfig(opts)
		require.Error(t, err, "accepted malformed stream filename %s", bad)
	}
	opts := viper.New()
	opts.Set("genesis.stream-import", "not-a-boolean")
	_, err := ReadGenesisImportConfig(opts)
	require.Error(t, err)
}
