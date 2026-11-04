package operations

import (
	"errors"
	"fmt"
	"os"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/ss"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/tools/utils"
	"github.com/spf13/cobra"
)

const outputFileName = "db_dump.kv"

func DumpDbCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dump-db",
		Short: "For a given State Store DB, dump-db iterates over all keys and values for a specific store and writes them to a file",
		RunE:  executeDumpDB,
	}
	cmd.PersistentFlags().StringP("output-dir", "o", "", "Output Directory")
	cmd.PersistentFlags().StringP("db-dir", "d", "", "Database Directory")
	cmd.PersistentFlags().StringP("module", "m", "", "Module to export. Leave empty to export all")
	cmd.PersistentFlags().StringP("db-backend", "b", "", "DB Backend")
	return cmd
}

func executeDumpDB(cmd *cobra.Command, _ []string) error {
	outputDir, err := cmd.Flags().GetString("output-dir")
	if err != nil {
		return err
	}
	module, err := cmd.Flags().GetString("module")
	if err != nil {
		return err
	}
	dbDir, err := cmd.Flags().GetString("db-dir")
	if err != nil {
		return err
	}
	dbBackend, err := cmd.Flags().GetString("db-backend")
	if err != nil {
		return err
	}
	return DumpDbData(dbBackend, module, outputDir, dbDir)
}

// DumpDbData outputs the raw keys and values from the explicitly named source.
func DumpDbData(dbBackend, module, outputDir, dbDir string) (retErr error) {
	cfg, err := existingStateStoreConfig(dbBackend, dbDir)
	if err != nil {
		return err
	}
	if outputDir == "" {
		return fmt.Errorf("must provide output dir when generating db export")
	}
	backend, err := ss.NewStateStore("", cfg)
	if err != nil {
		return err
	}
	defer func() { retErr = errors.Join(retErr, backend.Close()) }()
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	currentFile, err := utils.CreateFile(outputDir, outputFileName)
	if err != nil {
		return err
	}
	defer func() { retErr = errors.Join(retErr, currentFile.Close()) }()
	fmt.Printf("Writing db data to %s...\n", outputFileName)
	var writeErr error
	_, err = backend.RawIterate(module, func(key, value []byte, version int64) bool {
		_, writeErr = fmt.Fprintf(currentFile, "Key: %X Val: %X Version: %d\n", key, value, version)
		return writeErr != nil
	})
	return errors.Join(err, writeErr)
}
