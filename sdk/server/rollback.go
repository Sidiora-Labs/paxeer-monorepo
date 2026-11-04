package server

import (
	"encoding/json"
	"errors"
	"fmt"
	legacy "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/rootmulti"
	rootmulti "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	dbm "github.com/tendermint/tm-db"

	tmcmd "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/cmd/tendermint/commands"
	tmcfg "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/types"
	"github.com/spf13/cobra"
)

// rollbackTendermintState rolls back the tendermint state by one height.
// Returns the new height and app hash.
func rollbackTendermintState(cfg *tmcfg.Config, targetHeight int64) (int64, []byte, error) {
	tmHeight, hash, err := tmcmd.RollbackStateToTargetHeight(cfg, true, targetHeight)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to rollback tendermint state: %w", err)
	}
	return tmHeight, hash, nil
}

// rollbackAppState rolls back the app state to the target height.
// Returns the final app hash.
func rollbackAppState(app types.Application, targetHeight int64) ([]byte, error) {
	lastCommit := app.CommitMultiStore().LastCommitID()
	fmt.Printf("CMS app state height %d and hash %X\n", lastCommit.GetVersion(), lastCommit.GetHash())
	fmt.Printf("Attempting to rollback app state to height=%d\n", targetHeight)

	if err := app.CommitMultiStore().RollbackToVersion(targetHeight); err != nil {
		return nil, fmt.Errorf("failed to rollback to version: %w", err)
	}

	lastCommit = app.CommitMultiStore().LastCommitID()
	fmt.Printf("Rolled back app state to height %d and hash %X\n", lastCommit.GetVersion(), lastCommit.GetHash())

	return lastCommit.GetHash(), nil
}

// NewRollbackCmd creates a command to rollback tendermint and multistore state by one height.
func NewRollbackCmd(appCreator types.AppCreator, defaultNodeHome string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rollback",
		Short: "rollback cosmos-sdk and tendermint state by one height",
		Long: `
A state rollback is performed to recover from an incorrect application state transition,
when Tendermint has persisted an incorrect app hash and is thus unable to make
progress. Rollback overwrites a state at height n with the state at height n - 1.
The application also rolls back to height n - 1. The last block is removed, so upon
restarting Tendermint the node will re-fetch and re-execute the transactions in block n.
`,
		RunE: func(cmd *cobra.Command, args []string) (retErr error) {
			ctx := GetServerContextFromCmd(cmd)
			numBlocks, err := cmd.Flags().GetInt64("num-blocks")
			if err != nil {
				return err
			}
			if numBlocks <= 0 {
				return fmt.Errorf("num-blocks must be positive")
			}
			app := appCreator(nil, nil, ctx.Config, ctx.Viper)
			defer func() { retErr = errors.Join(retErr, app.Close()) }()
			appHeight := app.CommitMultiStore().LastCommitID().Version
			tmState, err := tmcmd.LoadTendermintState(ctx.Config)
			if err != nil {
				return err
			}
			tmHeight := tmState.LastBlockHeight
			intent, err := loadRollbackIntent(ctx.Config)
			if err != nil {
				return err
			}
			if intent == nil {
				target := appHeight
				if tmHeight < target {
					target = tmHeight
				}
				if appHeight == tmHeight {
					if numBlocks > target {
						return fmt.Errorf("cannot rollback below 0 from height %d", target)
					}
					target -= numBlocks
				}
				intent = &rollbackIntent{Target: target, AppHeight: appHeight, ConsensusHeight: tmHeight, ChainID: tmState.ChainID, NumBlocks: numBlocks}
			} else if intent.ChainID != tmState.ChainID || (cmd.Flags().Changed("num-blocks") && intent.NumBlocks != numBlocks) {
				return fmt.Errorf("rollback intent conflicts with chain or num-blocks; resume the recorded rollback")
			}
			target := intent.Target
			if target <= 0 {
				return fmt.Errorf("cannot rollback below 0 or to unavailable genesis state: %d", target)
			}
			if appHeight < target || appHeight > intent.AppHeight || tmHeight < target || tmHeight > intent.ConsensusHeight {
				return fmt.Errorf("store heights conflict with rollback intent to %d", target)
			}
			if err := prepareRollback(ctx.Config, app, intent); err != nil {
				return err
			}
			if appHeight > target {
				if _, err := rollbackAppState(app, target); err != nil {
					return err
				}
			}
			height, _, err := rollbackTendermintState(ctx.Config, target)
			if err != nil {
				return err
			}
			if height != target || app.CommitMultiStore().LastCommitID().Version != target {
				return fmt.Errorf("rollback did not reach intended height %d", target)
			}
			if err := archiveRollbackWAL(ctx.Config, target); err != nil {
				return fmt.Errorf("archive consensus WAL: %w", err)
			}
			// App hashes can differ when repairing an incorrect state transition.
			// Heights and the durable intent determine recovery, not hash equality.
			if err := os.Remove(rollbackIntentPath(ctx.Config)); err != nil {
				return err
			}
			return syncRollbackDirectory(ctx.Config.DBDir())
		},
	}

	cmd.Flags().Int64P("num-blocks", "n", 1, "number of blocks to rollback")
	cmd.Flags().String(flags.FlagChainID, "pax-chain", "genesis file chain-id, if left blank will use pax")
	cmd.Flags().String(flags.FlagHome, defaultNodeHome, "The application home directory")
	return cmd
}

type rollbackIntent struct {
	Target          int64  `json:"target"`
	AppHeight       int64  `json:"app_height"`
	ConsensusHeight int64  `json:"consensus_height"`
	ChainID         string `json:"chain_id"`
	NumBlocks       int64  `json:"num_blocks"`
}

func rollbackIntentPath(cfg *tmcfg.Config) string {
	return filepath.Join(cfg.DBDir(), "rollback-intent.json")
}

func loadRollbackIntent(cfg *tmcfg.Config) (*rollbackIntent, error) {
	data, err := os.ReadFile(rollbackIntentPath(cfg))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read rollback intent: %w", err)
	}
	var intent rollbackIntent
	if err := json.Unmarshal(data, &intent); err != nil {
		return nil, fmt.Errorf("decode rollback intent: %w", err)
	}
	if intent.NumBlocks <= 0 || intent.Target <= 0 || intent.AppHeight < intent.Target || intent.ConsensusHeight < intent.Target || intent.ChainID == "" {
		return nil, fmt.Errorf("invalid rollback intent")
	}
	return &intent, nil
}

func saveRollbackIntent(cfg *tmcfg.Config, intent *rollbackIntent) error {
	if err := os.MkdirAll(cfg.DBDir(), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(cfg.DBDir(), ".rollback-intent-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	data, err := json.Marshal(intent)
	if err == nil {
		_, err = file.Write(data)
	}
	if err == nil {
		err = file.Sync()
	}
	err = errors.Join(err, file.Close())
	if err != nil {
		return err
	}
	if err := os.Rename(file.Name(), rollbackIntentPath(cfg)); err != nil {
		return err
	}
	return syncRollbackDirectory(cfg.DBDir())
}

func syncRollbackDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

// Run the actual consensus rollback against isolated database snapshots so all
// historical blocks, validator sets, parameters and validator files are checked
// before either live store is changed.
func preflightConsensusRollback(cfg *tmcfg.Config, target int64) error {
	if cfg.AutobahnConfigFile != "" {
		return fmt.Errorf("consensus rollback preflight: autobahn offline stores are unsupported")
	}
	dir, err := os.MkdirTemp("", "rollback-preflight-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	copyCfg := *cfg
	copyPV := *cfg.PrivValidator
	copyCfg.PrivValidator = &copyPV
	copyCfg.RootDir = dir
	copyPV.RootDir = dir
	copyCfg.DBPath = "data"
	copyCfg.DBBackend = string(dbm.GoLevelDBBackend)
	if err := os.MkdirAll(copyCfg.DBDir(), 0700); err != nil {
		return err
	}
	for _, name := range []string{"state", "blockstore"} {
		if err := snapshotRollbackDB(cfg, &copyCfg, name); err != nil {
			return fmt.Errorf("consensus rollback preflight: %w", err)
		}
	}
	sourceFiles := []string{cfg.PrivValidator.KeyFile(), cfg.PrivValidator.StateFile()}
	// Absolute validator paths must never point the preflight at live files.
	copyPV.Key = "config/priv_validator_key.json"
	copyPV.State = "data/priv_validator_state.json"
	destFiles := []string{copyPV.KeyFile(), copyPV.StateFile()}
	for i, src := range sourceFiles {
		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("consensus rollback preflight: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(destFiles[i]), 0700); err != nil {
			return err
		}
		if err := os.WriteFile(destFiles[i], data, 0600); err != nil {
			return err
		}
	}
	height, _, err := rollbackTendermintState(&copyCfg, target)
	if err != nil {
		return fmt.Errorf("consensus rollback preflight: %w", err)
	}
	if height != target {
		return fmt.Errorf("consensus rollback preflight: expected height %d, got %d", target, height)
	}
	return nil
}

func snapshotRollbackDB(source, dest *tmcfg.Config, name string) (retErr error) {
	sourceDir := tmcfg.ResolveDBDir(name, source.DBDir())
	if _, err := os.Stat(filepath.Join(sourceDir, name+".db")); err != nil {
		return err
	}
	src, err := dbm.NewDB(name, dbm.BackendType(source.DBBackend), sourceDir)
	if err != nil {
		return err
	}
	defer func() { retErr = errors.Join(retErr, src.Close()) }()
	dst, err := dbm.NewDB(name, dbm.GoLevelDBBackend, dest.DBDir())
	if err != nil {
		return err
	}
	defer func() { retErr = errors.Join(retErr, dst.Close()) }()
	iter, err := src.Iterator(nil, nil)
	if err != nil {
		return err
	}
	defer iter.Close()
	batch := dst.NewBatch()
	defer batch.Close()
	for ; iter.Valid(); iter.Next() {
		if err := batch.Set(iter.Key(), iter.Value()); err != nil {
			return err
		}
	}
	if err := iter.Error(); err != nil {
		return err
	}
	return batch.WriteSync()
}

func prepareRollback(cfg *tmcfg.Config, app types.Application, intent *rollbackIntent) error {
	if app.CommitMultiStore().LastCommitID().Version > intent.Target {
		switch app.CommitMultiStore().(type) {
		case *legacy.Store:
			return fmt.Errorf("application rollback preflight: %w", &legacy.RollbackUnsupportedError{Target: intent.Target})
		case *rootmulti.Store:
			if intent.Target > math.MaxUint32 {
				return fmt.Errorf("application rollback preflight: target exceeds max uint32")
			}
		default:
			return fmt.Errorf("application rollback preflight: multistore %T has no supported rollback capability", app.CommitMultiStore())
		}
		snapshot, err := app.CommitMultiStore().CacheMultiStoreForExport(intent.Target)
		if err != nil {
			return fmt.Errorf("application rollback preflight: %w", err)
		}
		snapshot.Close()
	}
	if err := preflightConsensusRollback(cfg, intent.Target); err != nil {
		return err
	}
	return saveRollbackIntent(cfg, intent)
}

// Consensus messages above the rollback target cannot be replayed on restart.
// Keep their files for diagnosis, and leave the intent in place on any failure
// so recovery can finish archiving the remaining files before allowing startup.
func archiveRollbackWAL(cfg *tmcfg.Config, target int64) error {
	head := cfg.Consensus.WalFile()
	dir, base := filepath.Dir(head), filepath.Base(head)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var files []string
	for _, entry := range entries {
		name := entry.Name()
		if name != base {
			suffix, ok := strings.CutPrefix(name, base+".")
			if !ok || suffix == "lock" {
				continue
			}
			if _, err := strconv.Atoi(suffix); err != nil {
				return fmt.Errorf("invalid consensus WAL segment %q: %w", name, err)
			}
		}
		files = append(files, name)
	}
	if len(files) == 0 {
		return syncRollbackDirectory(dir)
	}
	archive, err := os.MkdirTemp(dir, fmt.Sprintf(".rollback-wal-to-%d-", target))
	if err != nil {
		return err
	}
	if err := syncRollbackDirectory(dir); err != nil {
		return err
	}
	for _, name := range files {
		if err := os.Rename(filepath.Join(dir, name), filepath.Join(archive, name)); err != nil {
			return err
		}
	}
	if err := syncRollbackDirectory(archive); err != nil {
		return err
	}
	return syncRollbackDirectory(dir)
}
