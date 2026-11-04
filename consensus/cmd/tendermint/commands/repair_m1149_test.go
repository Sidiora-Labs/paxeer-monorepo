package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	atypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/autobahn/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/autobahn/data"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state/indexer/sink/kv"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/test/factory"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func repairM1149NodeConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.ResetTestRoot(t.TempDir(), "repair_m1149")
	require.NoError(t, err)
	cfg.DBBackend = "goleveldb"
	cfg.RPC.ListenAddress = ""
	cfg.P2P.PexReactor = false
	pv, err := privval.LoadFilePV(cfg.PrivValidator.KeyFile(), cfg.PrivValidator.StateFile())
	require.NoError(t, err)
	// These real key writers produce the public-key files consumed by the CLI.
	require.NoError(t, pv.Save())
	nk, err := types.LoadOrGenNodeKey(cfg.NodeKeyFile())
	require.NoError(t, err)
	require.NoError(t, nk.SaveAs(cfg.NodeKeyFile()))
	inputDir := filepath.Dir(cfg.NodeKeyFile())
	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "autobahn_address.txt"), []byte("127.0.0.1:26660"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "evmrpc_url.txt"), []byte("http://127.0.0.1:8545"), 0600))
	return cfg
}

func repairM1149Generate(t *testing.T, cfg *config.Config, flags ...string) string {
	t.Helper()
	path := filepath.Join(cfg.RootDir, "config", "autobahn.json")
	cmd := MakeGenAutobahnConfigCommand()
	cmd.SetContext(t.Context())
	args := append([]string{"--output", path}, flags...)
	cmd.SetArgs(append(args, filepath.Dir(cfg.NodeKeyFile())))
	require.NoError(t, cmd.Execute())
	cfg.AutobahnConfigFile = path
	return path
}

func TestRepairM_1149_AutobahnGeneratorStartupRoundTrip(t *testing.T) {
	for _, name := range []string{"default", "relative", "absolute"} {
		t.Run(name, func(t *testing.T) {
			cfg := repairM1149NodeConfig(t)
			want := filepath.Join(cfg.RootDir, "data", "autobahn")
			var flags []string
			switch name {
			case "relative":
				want = filepath.Join(cfg.RootDir, "durable", "safety")
				flags = []string{"--persistent-state-dir", "durable/safety"}
			case "absolute":
				want = filepath.Join(t.TempDir(), "safety")
				flags = []string{"--persistent-state-dir", want}
			}
			repairM1149Generate(t, cfg, flags...)
			dir, err := config.LoadAutobahnPersistentStateDir(cfg.AutobahnConfigFile, cfg.RootDir)
			require.NoError(t, err)
			got, ok := dir.Get()
			require.True(t, ok)
			require.Equal(t, want, got)

			gen, err := types.GenesisDocFromFile(cfg.GenesisFile())
			require.NoError(t, err)
			gen.ConsensusParams.Block.MaxGas = 50_000_000
			// Exercise the actual startup loader, committee/key matching, and
			// persistence consumers, then close and construct the node again.
			for range 2 {
				app := kvstore.NewApplication()
				app.SetValidators(gen.ValidatorUpdates())
				n, err := node.New(t.Context(), cfg, func() {}, app, gen, nil,
					node.DefaultMetricsProvider(cfg.Instrumentation)(gen.ChainID), types.DefaultConsensusPolicy())
				require.NoError(t, err)
				require.True(t, n.RPCEnvironment().Router.Giga().IsPresent())
				n.(interface{ OnStop() }).OnStop()
				require.NoError(t, app.Close())
			}
			require.DirExists(t, want)
		})
	}
}

func TestRepairM_1149_AutobahnGeneratorRejectsEmptyPersistence(t *testing.T) {
	for _, value := range []string{"", " \t "} {
		t.Run(fmt.Sprintf("path_%q", value), func(t *testing.T) {
			cfg := repairM1149NodeConfig(t)
			path := filepath.Join(cfg.RootDir, "config", "autobahn.json")
			original := []byte("existing operator configuration")
			require.NoError(t, os.WriteFile(path, original, 0600))
			cmd := MakeGenAutobahnConfigCommand()
			cmd.SetArgs([]string{"--output", path, "--persistent-state-dir=" + value, filepath.Dir(cfg.NodeKeyFile())})
			err := cmd.ExecuteContext(t.Context())
			if value == "" {
				require.ErrorIs(t, err, config.ErrPersistentStateDirRequired)
			} else {
				require.ErrorIs(t, err, config.ErrPersistentStateDirBlank)
			}
			got, err := os.ReadFile(path)
			require.NoError(t, err)
			require.Equal(t, original, got)
		})
	}
}

func repairM1149BlockStores(t *testing.T, cfg *config.Config, pruneBase int64) map[int64]types.Tx {
	t.Helper()
	bsdb, err := config.DefaultDBProvider(&config.DBContext{ID: "blockstore", Config: cfg})
	require.NoError(t, err)
	bs := store.NewBlockStore(bsdb)
	ssdb, err := config.DefaultDBProvider(&config.DBContext{ID: "state", Config: cfg})
	require.NoError(t, err)
	ss := state.NewStore(ssdb)
	s, err := state.MakeGenesisStateFromFile(cfg.GenesisFile())
	require.NoError(t, err)
	pv, err := privval.LoadFilePV(cfg.PrivValidator.KeyFile(), cfg.PrivValidator.StateFile())
	require.NoError(t, err)
	app := kvstore.NewApplication()
	defer app.Close()
	lastCommit := new(types.Commit)
	txs := map[int64]types.Tx{}
	for h := int64(1); h <= 4; h++ {
		tx := types.Tx(fmt.Sprintf("height_%d=value_%d", h, h))
		txs[h] = tx
		block := s.MakeBlock(h, []types.Tx{tx}, lastCommit, nil, s.Validators.GetProposer().Address)
		parts, err := block.MakePartSet(types.BlockPartSizeBytes)
		require.NoError(t, err)
		id := types.BlockID{Hash: block.Hash(), PartSetHeader: parts.Header()}
		votes := types.NewVoteSet(s.ChainID, h, 0, tmproto.PrecommitType, s.Validators)
		commit, err := factory.MakeCommit(t.Context(), id, h, 0, votes, []types.PrivValidator{pv}, block.Time)
		require.NoError(t, err)
		bs.SaveBlock(block, parts, commit)
		response, err := app.FinalizeBlock(t.Context(), &abci.RequestFinalizeBlock{Header: block.Header.ToProto(), Hash: block.Hash(), Txs: [][]byte{tx}})
		require.NoError(t, err)
		require.NoError(t, ss.SaveFinalizeBlockResponses(h, response))
		lastCommit = commit
		s.LastBlockID = id
		s.LastBlockHeight = h
		s.AppHash = response.AppHash
	}
	if pruneBase > 1 {
		_, err := bs.PruneBlocks(pruneBase)
		require.NoError(t, err)
		require.Equal(t, pruneBase, bs.Base())
	}
	require.NoError(t, bs.Close())
	require.NoError(t, ss.Close())
	return txs
}

func TestRepairM_1149_ReindexNormalizedHeights(t *testing.T) {
	for _, tc := range []struct {
		name        string
		base        int64
		flags       []string
		first, last int64
		wantError   bool
	}{
		{name: "both_defaults", base: 1, first: 1, last: 4},
		{name: "omitted_end", base: 1, flags: []string{"--start-height", "2"}, first: 2, last: 4},
		{name: "omitted_start", base: 1, flags: []string{"--end-height", "3"}, first: 1, last: 3},
		{name: "clamped_end", base: 1, flags: []string{"--end-height", "99"}, first: 1, last: 4},
		{name: "pruned_defaults", base: 3, first: 3, last: 4},
		{name: "pruned_omitted_end", base: 3, flags: []string{"--start-height", "4"}, first: 4, last: 4},
		{name: "pruned_omitted_start", base: 3, flags: []string{"--end-height", "3"}, first: 3, last: 3},
		{name: "below_pruned_base", base: 3, flags: []string{"--start-height", "2"}, wantError: true},
		{name: "reversed_interval", base: 1, flags: []string{"--start-height", "3", "--end-height", "2"}, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := repairM1149NodeConfig(t)
			cfg.TxIndex.Indexer = []string{"kv"}
			txs := repairM1149BlockStores(t, cfg, tc.base)
			cmd := setupReIndexEventCmd(t.Context(), cfg)
			cmd.SetArgs(tc.flags)
			err := cmd.Execute()
			if tc.wantError {
				require.Error(t, err)
				require.NoDirExists(t, filepath.Join(config.ResolveDBDir("tx_index", cfg.DBDir()), "tx_index.db"))
				return
			}
			require.NoError(t, err)
			// The command must close its stores, so a fresh process can reopen
			// the persistent consumer and observe exactly the replayed interval.
			for range 2 {
				db, err := config.DefaultDBProvider(&config.DBContext{ID: "tx_index", Config: cfg})
				require.NoError(t, err)
				sink := kv.NewEventSink(db)
				for h := int64(1); h <= 4; h++ {
					want := tc.first <= h && h <= tc.last
					indexed, err := sink.HasBlock(h)
					require.NoError(t, err)
					require.Equal(t, want, indexed, "height %d", h)
					hash := txs[h].Hash()
					result, err := sink.GetTxByHash(hash[:])
					require.NoError(t, err)
					if want {
						require.NotNil(t, result)
						require.Equal(t, h, result.Height)
						require.Equal(t, []byte(txs[h]), result.Tx)
						require.NotEmpty(t, result.Result.Events)
					} else {
						require.Nil(t, result)
					}
				}
				require.NoError(t, sink.Stop())
			}
		})
	}
}

func repairM1149SignedValidator(t *testing.T, cfg *config.Config) []byte {
	t.Helper()
	for _, path := range []string{cfg.PrivValidator.KeyFile(), cfg.PrivValidator.StateFile()} {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	}
	pv, err := privval.GenFilePV(cfg.PrivValidator.KeyFile(), cfg.PrivValidator.StateFile(), types.ABCIPubKeyTypeEd25519)
	require.NoError(t, err)
	require.NoError(t, pv.Save())
	require.NoError(t, pv.SignVote(t.Context(), "repair-m1149", &tmproto.Vote{
		Type: tmproto.PrevoteType, Height: 7, Round: 1,
		Timestamp:        time.Unix(1_700_000_000, 0).UTC(),
		ValidatorAddress: pv.GetAddress(),
	}))
	reopened, err := privval.LoadFilePV(cfg.PrivValidator.KeyFile(), cfg.PrivValidator.StateFile())
	require.NoError(t, err)
	require.Equal(t, int64(7), reopened.LastSignState.Height)
	key, err := os.ReadFile(cfg.PrivValidator.KeyFile())
	require.NoError(t, err)
	return key
}

func repairM1149ResetCLI(t *testing.T, cfg *config.Config, args ...string) error {
	t.Helper()
	cmd := &cobra.Command{Use: "tendermint", SilenceErrors: true, SilenceUsage: true}
	cmd.PersistentFlags().String(cli.HomeFlag, cfg.RootDir, "node home")
	cmd.AddCommand(MakeResetCommand(cfg), MakeUnsafeResetAllCommand(cfg))
	cmd.SetArgs(args)
	return cmd.ExecuteContext(t.Context())
}

func repairM1149CheckSigner(t *testing.T, cfg *config.Config, key []byte, height int64) {
	t.Helper()
	actual, err := os.ReadFile(cfg.PrivValidator.KeyFile())
	require.NoError(t, err)
	require.Equal(t, key, actual)
	pv, err := privval.LoadFilePV(cfg.PrivValidator.KeyFile(), cfg.PrivValidator.StateFile())
	require.NoError(t, err)
	require.Equal(t, height, pv.LastSignState.Height)
}

func TestRepairM_1149_ResetResolvedPaths(t *testing.T) {
	for _, homeKind := range []string{"absolute", "relative"} {
		for _, command := range [][]string{{"reset", "unsafe-all"}, {"unsafe-reset-all"}} {
			t.Run(homeKind+"/"+strings.Join(command, "_"), func(t *testing.T) {
				t.Chdir(t.TempDir())
				home := "node"
				if homeKind == "absolute" {
					cwd, err := os.Getwd()
					require.NoError(t, err)
					home = filepath.Join(cwd, home)
				}
				cfg := config.TestConfig().SetRoot(home)
				cfg.DBPath = "custom-db"
				cfg.PrivValidator.Key = "signer/custom-key.json"
				cfg.PrivValidator.State = "custom-db/custom-watermark.json"
				key := repairM1149SignedValidator(t, cfg)
				createDirs(t, cfg.DBDir(), append(append([]string{}, legacyDBs...), newTendermintDBs...))
				createDirs(t, home, []string{"wasm", "unrelated"})
				marker := filepath.Join(home, "unrelated", "keep")
				require.NoError(t, os.WriteFile(marker, []byte("keep"), 0600))
				require.NoError(t, repairM1149ResetCLI(t, cfg, command...))
				require.DirExists(t, cfg.DBDir())
				for _, path := range append(append([]string{}, legacyDBs...), newTendermintDBs...) {
					require.NoDirExists(t, filepath.Join(cfg.DBDir(), path))
				}
				require.NoDirExists(t, filepath.Join(home, "wasm"))
				require.FileExists(t, marker)
				repairM1149CheckSigner(t, cfg, key, 0)
				require.NoFileExists(t, filepath.Join(home, cfg.PrivValidator.KeyFile()))
				require.NoDirExists(t, filepath.Join(home, cfg.DBDir()))
			})
		}
	}
}

func TestRepairM_1149_ResetReportsFilesystemFailures(t *testing.T) {
	for _, command := range [][]string{{"reset", "blockchain"}, {"reset", "unsafe-all"}, {"unsafe-reset-all"}} {
		t.Run(strings.Join(command, "_"), func(t *testing.T) {
			cfg := config.TestConfig().SetRoot(t.TempDir())
			cfg.PrivValidator.State = "signer/watermark.json"
			key := repairM1149SignedValidator(t, cfg)
			createDirs(t, cfg.RootDir, []string{"wasm", "data/autobahn"})
			if command[len(command)-1] == "blockchain" {
				require.NoError(t, os.Symlink("tendermint", filepath.Join(cfg.DBDir(), "tendermint")))
			} else {
				require.NoError(t, os.Symlink("loop", filepath.Join(cfg.RootDir, "loop")))
				cfg.DBPath = "loop/data"
			}
			err := repairM1149ResetCLI(t, cfg, command...)
			require.ErrorIs(t, err, syscall.ELOOP)
			var pathErr *os.PathError
			require.ErrorAs(t, err, &pathErr)
			require.True(t, strings.HasPrefix(pathErr.Path, cfg.RootDir))
			require.DirExists(t, filepath.Join(cfg.RootDir, "wasm"))
			require.DirExists(t, filepath.Join(cfg.RootDir, "data", "autobahn"))
			repairM1149CheckSigner(t, cfg, key, 7)
		})
	}
	t.Run("recreation_failure", func(t *testing.T) {
		t.Chdir(t.TempDir())
		cfg := config.TestConfig().SetRoot(t.TempDir())
		key := repairM1149SignedValidator(t, cfg)
		cfg.PrivValidator.Key = cfg.PrivValidator.KeyFile()
		cfg.PrivValidator.State = cfg.PrivValidator.StateFile()
		cfg.SetRoot("")
		cfg.DBPath = ""
		createDirs(t, ".", []string{"wasm"})
		err := repairM1149ResetCLI(t, cfg, "unsafe-reset-all")
		require.Error(t, err)
		var pathErr *os.PathError
		require.ErrorAs(t, err, &pathErr)
		require.Equal(t, "", pathErr.Path)
		require.DirExists(t, "wasm")
		repairM1149CheckSigner(t, cfg, key, 7)
	})
	t.Run("signer_recreation_failure", func(t *testing.T) {
		cfg := config.TestConfig().SetRoot(t.TempDir())
		key := repairM1149SignedValidator(t, cfg)
		blocker := filepath.Join(cfg.RootDir, "blocker")
		require.NoError(t, os.WriteFile(blocker, []byte("not a directory"), 0600))
		cfg.PrivValidator.State = filepath.Join(blocker, "state.json")
		err := repairM1149ResetCLI(t, cfg, "unsafe-reset-all")
		require.ErrorIs(t, err, syscall.ENOTDIR)
		actual, err := os.ReadFile(cfg.PrivValidator.KeyFile())
		require.NoError(t, err)
		require.Equal(t, key, actual)
	})
}

func repairM1149SeedAutobahn(t *testing.T, rng utils.Rng, dir string, committee *atypes.Committee, keys []atypes.SecretKey) *atypes.Block {
	t.Helper()
	wal, err := data.NewDataWAL(utils.Some(dir), committee)
	require.NoError(t, err)
	qc, blocks := data.TestCommitQC(rng, committee, keys, utils.None[*atypes.CommitQC]())
	require.NoError(t, wal.CommitQCs.PersistQC(qc))
	gr := qc.QC().GlobalRange(committee)
	for i, block := range blocks {
		require.NoError(t, wal.Blocks.PersistBlock(gr.First+atypes.GlobalBlockNumber(i), block))
	}
	require.NoError(t, wal.Close())
	// Stop and reopen both WALs and the production data state before reset.
	wal, err = data.NewDataWAL(utils.Some(dir), committee)
	require.NoError(t, err)
	s, err := data.NewState(&data.Config{Committee: committee}, wal)
	require.NoError(t, err)
	require.Equal(t, gr.Next, s.NextBlock())
	got, err := s.TryBlock(gr.First)
	require.NoError(t, err)
	require.Equal(t, blocks[0].Header().Hash(), got.Header().Hash())
	require.NoError(t, wal.Close())
	return blocks[0]
}

func TestRepairM_1149_ResetConfiguredAutobahnDir(t *testing.T) {
	for _, setting := range []string{"default_with_custom_db", "custom_relative", "custom_absolute"} {
		for _, command := range [][]string{{"reset", "blockchain"}, {"reset", "unsafe-all"}, {"unsafe-reset-all"}} {
			t.Run(setting+"/"+strings.Join(command, "_"), func(t *testing.T) {
				rng := utils.TestRng()
				cfg := repairM1149NodeConfig(t)
				cfg.DBPath = "alternate-db"
				var flags []string
				switch setting {
				case "custom_relative":
					flags = []string{"--persistent-state-dir", "durable/safety"}
				case "custom_absolute":
					flags = []string{"--persistent-state-dir", filepath.Join(t.TempDir(), "safety")}
				}
				repairM1149Generate(t, cfg, flags...)
				dir, err := config.LoadAutobahnPersistentStateDir(cfg.AutobahnConfigFile, cfg.RootDir)
				require.NoError(t, err)
				path, ok := dir.Get()
				require.True(t, ok)
				committee, keys := atypes.GenCommittee(rng, 1)
				repairM1149SeedAutobahn(t, rng, path, committee, keys)
				createDirs(t, cfg.DBDir(), []string{"state.db", "autobahn"})
				key, err := os.ReadFile(cfg.PrivValidator.KeyFile())
				require.NoError(t, err)
				require.NoError(t, repairM1149ResetCLI(t, cfg, command...))
				require.NoDirExists(t, path)
				require.NoDirExists(t, filepath.Join(cfg.DBDir(), "state.db"))
				require.NoDirExists(t, filepath.Join(cfg.DBDir(), "autobahn"))
				require.FileExists(t, cfg.AutobahnConfigFile)
				actual, err := os.ReadFile(cfg.PrivValidator.KeyFile())
				require.NoError(t, err)
				require.Equal(t, key, actual)
				wal, err := data.NewDataWAL(utils.Some(path), committee)
				require.NoError(t, err)
				fresh, err := data.NewState(&data.Config{Committee: committee}, wal)
				require.NoError(t, err)
				require.Equal(t, committee.FirstBlock(), fresh.NextBlock())
				block, err := fresh.TryBlock(committee.FirstBlock())
				require.ErrorIs(t, err, data.ErrNotFound)
				require.Nil(t, block)
				require.NoError(t, wal.Close())
			})
		}
	}
}

func TestRepairM_1149_ResetAutobahnPreflightAndFailure(t *testing.T) {
	for _, mode := range []string{"missing_config", "invalid_json", "invalid_persistence", "unremovable_persistence", "explicit_test_opt_out"} {
		t.Run(mode, func(t *testing.T) {
			cfg := repairM1149NodeConfig(t)
			cfg.PrivValidator.State = "signer/watermark.json"
			key := repairM1149SignedValidator(t, cfg)
			configPath := repairM1149Generate(t, cfg)
			createDirs(t, cfg.DBDir(), []string{"state.db"})
			createDirs(t, cfg.RootDir, []string{"wasm"})
			switch mode {
			case "missing_config":
				require.NoError(t, os.Remove(configPath))
			case "invalid_json":
				require.NoError(t, os.WriteFile(configPath, []byte("{"), 0600))
			default:
				raw, err := os.ReadFile(configPath)
				require.NoError(t, err)
				var fc config.AutobahnFileConfig
				require.NoError(t, json.Unmarshal(raw, &fc))
				switch mode {
				case "invalid_persistence":
					fc.PersistentStateDir = utils.None[string]()
				case "unremovable_persistence":
					require.NoError(t, os.Symlink("loop", filepath.Join(cfg.RootDir, "loop")))
					fc.PersistentStateDir = utils.Some("loop/safety")
				case "explicit_test_opt_out":
					fc.PersistentStateDir = utils.None[string]()
					fc.UnsafeTestOnlyDisablePersistence = true
				}
				raw, err = json.Marshal(fc)
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(configPath, raw, 0600))
			}
			err := repairM1149ResetCLI(t, cfg, "reset", "unsafe-all")
			if mode == "explicit_test_opt_out" {
				require.NoError(t, err)
				repairM1149CheckSigner(t, cfg, key, 0)
				return
			}
			require.Error(t, err)
			if mode == "unremovable_persistence" {
				require.ErrorIs(t, err, syscall.ELOOP)
			} else {
				require.DirExists(t, filepath.Join(cfg.DBDir(), "state.db"))
			}
			if mode == "missing_config" {
				require.ErrorIs(t, err, os.ErrNotExist)
			}
			if mode == "invalid_persistence" {
				require.ErrorIs(t, err, config.ErrPersistentStateDirRequired)
			}
			require.DirExists(t, filepath.Join(cfg.RootDir, "wasm"))
			repairM1149CheckSigner(t, cfg, key, 7)
		})
	}
}
