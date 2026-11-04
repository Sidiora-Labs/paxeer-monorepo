package operations

import (
	"errors"
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/proto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/ss"
	"github.com/spf13/cobra"
)

func PruneCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "prune", Short: "Prune a db at a given height", RunE: executePrune}
	cmd.PersistentFlags().StringP("db-dir", "d", "", "Database Directory")
	cmd.PersistentFlags().StringP("db-backend", "b", "", "DB Backend")
	cmd.PersistentFlags().Int64P("version", "v", 0, "Version to prune at")
	return cmd
}

func executePrune(cmd *cobra.Command, _ []string) error {
	dbDir, err := cmd.Flags().GetString("db-dir")
	if err != nil {
		return err
	}
	dbBackend, err := cmd.Flags().GetString("db-backend")
	if err != nil {
		return err
	}
	version, err := cmd.Flags().GetInt64("version")
	if err != nil {
		return err
	}
	return PruneDB(dbBackend, dbDir, version)
}

// PruneDB prunes only the existing database named by dbDir.
func PruneDB(dbBackend, dbDir string, version int64) (retErr error) {
	if version <= 0 {
		return fmt.Errorf("prune version must be positive")
	}
	cfg, err := existingStateStoreConfig(dbBackend, dbDir)
	if err != nil {
		return err
	}
	var stores []*proto.NamedChangeSet
	if dbBackend == config.PebbleDBBackend {
		stores, err = persistedPruneStores(cfg)
		if err != nil {
			return err
		}
	}
	backend, err := ss.NewStateStore("", cfg)
	if err != nil {
		return err
	}
	defer func() { retErr = errors.Join(retErr, backend.Close()) }()
	if version >= backend.GetLatestVersion() {
		return fmt.Errorf("prune version %d must be below latest version %d", version, backend.GetLatestVersion())
	}
	if version < backend.GetEarliestVersion() {
		return fmt.Errorf("prune version %d is below earliest version %d", version, backend.GetEarliestVersion())
	}
	// Pebble's pruning inventory is process-local. Register the persisted stores
	// after reopening, using empty changesets at the unchanged head: this updates
	// pruning bookkeeping without rewriting any key or advancing the version.
	if len(stores) > 0 {
		if err := backend.ApplyChangesetSync(backend.GetLatestVersion(), stores); err != nil {
			return fmt.Errorf("register stores for pruning: %w", err)
		}
	}
	fmt.Printf("Pruning %s db at path %s at height %d...\n", dbBackend, dbDir, version)
	return backend.Prune(version)
}
