package operations

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/proto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/ss/backend"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/wal"
)

func ReplayChangelogCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "replay-changelog", Short: "Scan the changelog to replay and recover pebbledb data", RunE: executeReplayChangelog}
	cmd.PersistentFlags().StringP("db-dir", "d", "", "Database Directory")
	cmd.PersistentFlags().Uint64P("start-offset", "s", 0, "Start offset, default to earliest offset")
	cmd.PersistentFlags().Uint64P("end-offset", "e", 0, "End offset, default to latest offset")
	cmd.PersistentFlags().Bool("no-dry-run", false, "Whether to dry run or re-apply the changelog to DB")
	return cmd
}

func executeReplayChangelog(cmd *cobra.Command, _ []string) (retErr error) {
	dbDir, err := cmd.Flags().GetString("db-dir")
	if err != nil {
		return err
	}
	start, err := cmd.Flags().GetUint64("start-offset")
	if err != nil {
		return err
	}
	end, err := cmd.Flags().GetUint64("end-offset")
	if err != nil {
		return err
	}
	noDryRun, err := cmd.Flags().GetBool("no-dry-run")
	if err != nil {
		return err
	}
	if dbDir == "" {
		return fmt.Errorf("must provide database dir")
	}
	if start > 0 && end > 0 && start > end {
		return fmt.Errorf("start offset must not exceed end offset")
	}
	logDir := filepath.Join(dbDir, "changelog")
	info, err := os.Stat(logDir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("changelog is not a directory")
	}
	stream, err := wal.NewChangelogWAL(logDir, wal.Config{})
	if err != nil {
		return err
	}
	defer func() { retErr = errors.Join(retErr, stream.Close()) }()
	first, err := stream.FirstOffset()
	if err != nil {
		return err
	}
	last, err := stream.LastOffset()
	if err != nil {
		return err
	}
	if first == 0 && last == 0 && start == 0 && end == 0 {
		return nil
	}
	if start == 0 {
		start = first
	}
	if end == 0 {
		end = last
	}
	if start < first || end > last || start == 0 || start > end {
		return fmt.Errorf("invalid replay offsets [%d,%d], available [%d,%d]", start, end, first, last)
	}
	var store types.StateStore
	if noDryRun {
		cfg, err := existingStateStoreConfig(config.PebbleDBBackend, dbDir)
		if err != nil {
			return err
		}
		// Opening the composite store would auto-recover the entire WAL, bypassing
		// these bounds (and open a second WAL handle). Replay the MVCC store directly.
		store, err = backend.ResolveBackend(cfg.Backend)(cfg.DBDirectory, cfg)
		if err != nil {
			return err
		}
		defer func() { retErr = errors.Join(retErr, store.Close()) }()
	}
	return stream.Replay(start, end, func(index uint64, entry proto.ChangelogEntry) error {
		return processChangelogEntry(cmd.OutOrStdout(), store, index, entry)
	})
}

func processChangelogEntry(out io.Writer, store types.StateStore, index uint64, entry proto.ChangelogEntry) error {
	if _, err := fmt.Fprintf(out, "Offset: %d, Height: %d\n", index, entry.Version); err != nil {
		return err
	}
	if store != nil {
		if err := store.ApplyChangesetSync(entry.Version, entry.Changesets); err != nil {
			return err
		}
		_, err := fmt.Fprintf(out, "Re-applied changeset for height %d\n", entry.Version)
		return err
	}
	return nil
}
