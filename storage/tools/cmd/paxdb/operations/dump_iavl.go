package operations

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/memiavl"
	"github.com/spf13/cobra"
)

func DumpIAVLCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "dump-iavl", Short: "Iterate and dump memIAVL data", RunE: executeDumpIAVL}
	cmd.PersistentFlags().StringP("db-dir", "d", "", "Database Directory")
	cmd.PersistentFlags().StringP("output-dir", "o", "", "Output Directory")
	cmd.PersistentFlags().Int64("height", 0, "Block Height")
	cmd.PersistentFlags().StringP("module", "m", "", "Module to export. Default to export all")
	return cmd
}

func executeDumpIAVL(cmd *cobra.Command, _ []string) (retErr error) {
	module, err := cmd.Flags().GetString("module")
	if err != nil {
		return err
	}
	dbDir, err := cmd.Flags().GetString("db-dir")
	if err != nil {
		return err
	}
	outputDir, err := cmd.Flags().GetString("output-dir")
	if err != nil {
		return err
	}
	height, err := cmd.Flags().GetInt64("height")
	if err != nil {
		return err
	}
	if dbDir == "" {
		return fmt.Errorf("must provide database dir")
	}
	if outputDir == "" {
		return fmt.Errorf("must provide output dir")
	}
	db, err := memiavl.OpenDB(height, memiavl.Options{Dir: dbDir, ZeroCopy: true, CreateIfMissing: false})
	if err != nil {
		return err
	}
	defer func() { retErr = errors.Join(retErr, db.Close()) }()
	return DumpIAVLData(module, db, outputDir)
}

// DumpIAVLData publishes each module only after all selected trees are written and closed.
func DumpIAVLData(module string, db *memiavl.DB, outputDir string) error {
	if outputDir == "" {
		return fmt.Errorf("must provide output dir")
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(outputDir, ".iavl-dump-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(staging) }()
	var modules []string
	for _, name := range selectedModules(module, db) {
		if name == "." || name == ".." || filepath.Base(name) != name {
			return fmt.Errorf("invalid module filename %q", name)
		}
		tree := db.TreeByName(name)
		if tree == nil {
			return fmt.Errorf("tree does not exist for module %s", name)
		}
		file, err := os.Create(filepath.Join(staging, name))
		if err != nil {
			return err
		}
		if err := writeIAVLTree(file, name, tree); err != nil {
			return fmt.Errorf("dump module %s: %w", name, err)
		}
		modules = append(modules, name)
	}
	for _, name := range modules {
		if err := os.Rename(filepath.Join(staging, name), filepath.Join(outputDir, name)); err != nil {
			return err
		}
	}
	for _, name := range modules {
		fmt.Printf("Finished dumping module: %s \n", name)
	}
	return nil
}

// writeIAVLTree owns the destination and stops reading at the first write error.
func writeIAVLTree(dst io.WriteCloser, name string, tree *memiavl.Tree) (retErr error) {
	defer func() { retErr = errors.Join(retErr, dst.Close()) }()
	if _, err := fmt.Fprintf(dst, "Tree %s has version %d and root hash: %X \n", name, tree.Version(), tree.RootHash()); err != nil {
		return err
	}
	iter := tree.Iterator(nil, nil, true)
	defer func() { retErr = errors.Join(retErr, iter.Close()) }()
	for ; iter.Valid(); iter.Next() {
		if _, err := fmt.Fprintf(dst, "Key: %X, Value: %X \n", iter.Key(), iter.Value()); err != nil {
			return err
		}
	}
	return iter.Error()
}
