package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/cli"
	tmos "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/os"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/spf13/cobra"
)

const wasmDirName = "wasm"

// MakeResetCommand constructs a command that removes the database of
// the specified Tendermint core instance.
func MakeResetCommand(conf *config.Config) *cobra.Command {
	var keyType string

	resetCmd := &cobra.Command{
		Use:   "reset",
		Short: "Set of commands to conveniently reset tendermint related data",
	}

	resetBlocksCmd := &cobra.Command{
		Use:   "blockchain",
		Short: "Removes all blocks, state, transactions and evidence stored by the tendermint node",
		RunE: func(cmd *cobra.Command, args []string) error {
			plan, err := makeResetPlan(cmd, conf)
			if err != nil {
				return err
			}
			return plan.resetState()
		},
	}

	resetPeersCmd := &cobra.Command{
		Use:   "peers",
		Short: "Removes all peer addresses",
		RunE: func(cmd *cobra.Command, args []string) error {
			return ResetPeerStore(conf.DBDir())
		},
	}

	resetSignerCmd := &cobra.Command{
		Use:   "unsafe-signer",
		Short: "esets private validator signer state",
		Long: `Resets private validator signer state.
Only use in testing. This can cause the node to double sign`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ResetFilePV(conf.PrivValidator.KeyFile(), conf.PrivValidator.StateFile(), keyType)
		},
	}

	resetAllCmd := &cobra.Command{
		Use:   "unsafe-all",
		Short: "Removes all tendermint data including signing state",
		Long: `Removes all tendermint data including signing state.
Only use in testing. This can cause the node to double sign`,
		RunE: func(cmd *cobra.Command, args []string) error {
			plan, err := makeResetPlan(cmd, conf)
			if err != nil {
				return err
			}
			return plan.resetAll(keyType)
		},
	}

	resetSignerCmd.Flags().StringVar(&keyType, "key", types.ABCIPubKeyTypeEd25519,
		"Signer key type (ed25519 only)")

	resetAllCmd.Flags().StringVar(&keyType, "key", types.ABCIPubKeyTypeEd25519,
		"Signer key type (ed25519 only)")

	resetCmd.AddCommand(resetBlocksCmd)
	resetCmd.AddCommand(resetPeersCmd)
	resetCmd.AddCommand(resetSignerCmd)
	resetCmd.AddCommand(resetAllCmd)

	return resetCmd
}

// resetPlan contains paths resolved once by config accessors. Only wasm is
// derived from home; database and signer paths must never be rooted again.
type resetPlan struct {
	dbDir        string
	keyFile      string
	stateFile    string
	wasmDir      string
	autobahnDirs []string
}

func makeResetPlan(cmd *cobra.Command, conf *config.Config) (resetPlan, error) {
	if cmd.Flags().Lookup(cli.HomeFlag) != nil {
		home, err := cmd.Flags().GetString(cli.HomeFlag)
		if err != nil {
			return resetPlan{}, err
		}
		if home != "" {
			conf.SetRoot(home)
		}
	}
	plan := resetPlan{
		dbDir:     conf.DBDir(),
		keyFile:   conf.PrivValidator.KeyFile(),
		stateFile: conf.PrivValidator.StateFile(),
		wasmDir:   filepath.Join(conf.RootDir, wasmDirName),
	}
	if conf.AutobahnConfigFile != "" {
		dir, err := config.LoadAutobahnPersistentStateDir(conf.AutobahnConfigFile, conf.RootDir)
		if err != nil {
			return resetPlan{}, fmt.Errorf("loading Autobahn reset path from %q: %w", conf.AutobahnConfigFile, err)
		}
		if path, ok := dir.Get(); ok {
			plan.autobahnDirs = append(plan.autobahnDirs, path)
		}
	}
	return plan, nil
}

// ResetAll removes address book files plus all data, and resets the privValdiator data.
// Exported for extenal CLI usage. All arguments except homeDir are already resolved paths.
// XXX: this is unsafe and should only suitable for testnets.
func ResetAll(dbDir, privValKeyFile, privValStateFile string, keyType string, homeDir string) error {
	return (resetPlan{
		dbDir:     dbDir,
		keyFile:   privValKeyFile,
		stateFile: privValStateFile,
		wasmDir:   filepath.Join(homeDir, wasmDirName),
	}).resetAll(keyType)
}

func (plan resetPlan) resetAll(keyType string) error {
	if err := removeIfExists(plan.dbDir, "all blockchain history"); err != nil {
		return err
	}
	if err := tmos.EnsureDir(plan.dbDir, 0700); err != nil {
		return err
	}
	for _, path := range plan.autobahnDirs {
		if err := removeIfExists(path, "Autobahn persistent state"); err != nil {
			return err
		}
	}
	if err := removeIfExists(plan.wasmDir, "wasm directory"); err != nil {
		return err
	}
	// Recreate the signer state's parent, which may be outside dbDir.
	if err := tmos.EnsureDir(filepath.Dir(plan.stateFile), 0700); err != nil {
		return err
	}
	return ResetFilePV(plan.keyFile, plan.stateFile, keyType)
}

// removeIfExists removes a path if it exists, logging the result.
func removeIfExists(path, label string) error {
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("removing %s at %q: %w", label, path, err)
	}
	logger.Info("Removed "+label, "dir", path)
	return nil
}

// ResetState removes all blocks, tendermint state, indexed transactions and evidence.
// It handles both the legacy flat layout and the new subdirectory layout.
// It also removes the default Autobahn persistent state directory under dbDir.
// Commands additionally include the configured persistence directory in their reset plan.
func ResetState(dbDir string) error {
	return (resetPlan{dbDir: dbDir}).resetState()
}

func (plan resetPlan) resetState() error {
	paths := []string{
		// Legacy paths (flat under data/).
		"blockstore.db", "state.db", "cs.wal", "evidence.db", "tx_index.db",
		// New paths (all tendermint DBs under data/tendermint/).
		"tendermint/blockstore.db", "tendermint/tx_index.db", "tendermint/state.db",
		"tendermint/cs.wal", "tendermint/evidence.db", "tendermint/peerstore.db",
		"autobahn",
	}
	for _, path := range paths {
		if err := removeIfExists(filepath.Join(plan.dbDir, path), path); err != nil {
			return err
		}
	}
	for _, path := range plan.autobahnDirs {
		if err := removeIfExists(path, "Autobahn persistent state"); err != nil {
			return err
		}
	}
	return tmos.EnsureDir(plan.dbDir, 0700)
}

// ResetFilePV loads the file private validator and resets the watermark to 0. If used on an existing network,
// this can cause the node to double sign.
// XXX: this is unsafe and should only suitable for testnets.
func ResetFilePV(privValKeyFile, privValStateFile string, keyType string) error {
	if _, err := os.Stat(privValKeyFile); err == nil {
		pv, err := privval.LoadFilePVEmptyState(privValKeyFile, privValStateFile)
		if err != nil {
			return err
		}
		if err := pv.Reset(); err != nil {
			return err
		}
		logger.Info("Reset private validator file to genesis state", "keyFile", privValKeyFile,
			"stateFile", privValStateFile)
	} else {
		pv, err := privval.GenFilePV(privValKeyFile, privValStateFile, keyType)
		if err != nil {
			return err
		}
		if err := pv.Save(); err != nil {
			return err
		}
		logger.Info("Generated private validator file", "keyFile", privValKeyFile,
			"stateFile", privValStateFile)
	}
	return nil
}

// ResetPeerStore removes the peer store containing all information used by the tendermint networking layer.
// In the case of a reset, new peers will need to be set either via the config or through the discovery mechanism.
// It checks both legacy (data/peerstore.db) and new (data/tendermint/peerstore.db) locations.
func ResetPeerStore(dbDir string) error {
	legacy := filepath.Join(dbDir, "peerstore.db")
	if tmos.FileExists(legacy) {
		if err := os.RemoveAll(legacy); err != nil {
			return err
		}
	}
	newPath := filepath.Join(dbDir, "tendermint", "peerstore.db")
	if tmos.FileExists(newPath) {
		if err := os.RemoveAll(newPath); err != nil {
			return err
		}
	}
	return nil
}

func MakeUnsafeResetAllCommand(conf *config.Config) *cobra.Command {
	var keyType string

	resetAllCmd := &cobra.Command{
		Use:   "unsafe-reset-all",
		Short: "Removes all tendermint data, wasm directory, and signing state",
		Long: `Removes all tendermint data including signing state, and the wasm directory (contract blobs and compiled modules).
Only use in testing. This can cause the node to double sign`,
		RunE: func(cmd *cobra.Command, args []string) error {
			plan, err := makeResetPlan(cmd, conf)
			if err != nil {
				return err
			}
			return plan.resetAll(keyType)
		},
	}

	resetAllCmd.Flags().StringVar(&keyType, "key", types.ABCIPubKeyTypeEd25519,
		"Signer key type (ed25519 only)")

	return resetAllCmd
}
