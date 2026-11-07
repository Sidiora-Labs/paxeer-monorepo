package keys

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofrs/flock"
	"github.com/spf13/cobra"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/legacy"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

type keyReplacement struct {
	keyring       keyring.Keyring
	liveDir       string
	stageDir      string
	stageRoot     string
	originalDir   os.FileInfo
	originalFiles map[string][sha256.Size]byte
	lock          *flock.Flock
	retainBackup  bool
}

func stageKeyReplacement(ctx client.Context, cmd *cobra.Command, original keyring.Keyring, name string) (_ *keyReplacement, err error) {
	backend, err := cmd.Flags().GetString(flags.FlagKeyringBackend)
	if err != nil {
		return nil, err
	}
	if ctx.Simulate || (backend != keyring.BackendFile && backend != keyring.BackendTest) {
		return nil, fmt.Errorf("atomic key replacement is not supported for backend %q; the existing key has been preserved", backend)
	}
	if ctx.KeyringDir == "" {
		return nil, errors.New("atomic key replacement requires a keyring directory")
	}
	root := ctx.KeyringDir
	if strings.HasPrefix(root, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		root = strings.Replace(root, "~", home, 1)
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	r := &keyReplacement{
		liveDir: filepath.Join(root, "keyring-"+backend),
		lock:    flock.New(filepath.Join(root, ".keyring-replacement.lock")),
	}
	locked, err := r.lock.TryLock()
	if err != nil {
		return nil, err
	}
	if !locked {
		return nil, errors.New("another key replacement is in progress")
	}
	defer func() {
		if err != nil {
			r.close(cmd)
		}
	}()

	r.originalDir, err = os.Lstat(r.liveDir)
	if err != nil {
		return nil, err
	}
	if !r.originalDir.IsDir() {
		return nil, errors.New("atomic key replacement requires a regular keyring directory")
	}
	r.stageRoot, err = os.MkdirTemp(root, ".keyring-replacement-")
	if err != nil {
		return nil, err
	}
	r.stageDir = filepath.Join(r.stageRoot, "keyring-"+backend)
	if err = os.Mkdir(r.stageDir, 0700); err != nil {
		return nil, err
	}
	r.originalFiles, err = keyringDirectorySnapshot(r.liveDir, r.stageDir)
	if err != nil {
		return nil, err
	}

	algo, ledgerAlgo := original.SupportedAlgorithms()
	opts := append([]keyring.Option{}, ctx.KeyringOptions...)
	opts = append(opts, func(options *keyring.Options) {
		options.SupportedAlgos = algo
		options.SupportedAlgosLedger = ledgerAlgo
	})
	r.keyring, err = keyring.New(sdk.KeyringServiceName(), backend, r.stageRoot, ctx.Input, opts...)
	if err != nil {
		return nil, err
	}
	expected, err := original.Key(name)
	if err != nil {
		return nil, err
	}
	copied, err := r.keyring.Key(name)
	if err != nil {
		return nil, err
	}
	expectedBytes, err := legacy.Cdc.MarshalLengthPrefixed(expected)
	if err != nil {
		return nil, err
	}
	copiedBytes, err := legacy.Cdc.MarshalLengthPrefixed(copied)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(expectedBytes, copiedBytes) {
		return nil, errors.New("keyring directory does not match the existing signing identity")
	}
	if err = r.keyring.Delete(name); err != nil {
		return nil, err
	}
	return r, nil
}

func keyringDirectorySnapshot(dir, copyTo string) (map[string][sha256.Size]byte, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	snapshot := make(map[string][sha256.Size]byte, len(entries))
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			return nil, fmt.Errorf("keyring entry %q is not a regular file", entry.Name())
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		snapshot[entry.Name()] = sha256.Sum256(data)
		if copyTo != "" {
			if err := os.WriteFile(filepath.Join(copyTo, entry.Name()), data, 0600); err != nil {
				return nil, err
			}
		}
	}
	return snapshot, nil
}

func (r *keyReplacement) commit() error {
	entries, err := os.ReadDir(r.stageDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			return fmt.Errorf("staged keyring entry %q is not a regular file", entry.Name())
		}
		if err := syncKeyringPath(filepath.Join(r.stageDir, entry.Name())); err != nil {
			return err
		}
	}
	for _, dir := range []string{r.stageDir, r.stageRoot, filepath.Dir(r.liveDir)} {
		if err := syncKeyringPath(dir); err != nil {
			return err
		}
	}
	currentDir, err := os.Lstat(r.liveDir)
	if err != nil {
		return err
	}
	if !os.SameFile(r.originalDir, currentDir) {
		return errors.New("keyring directory changed during replacement")
	}
	currentFiles, err := keyringDirectorySnapshot(r.liveDir, "")
	if err != nil {
		return err
	}
	if len(currentFiles) != len(r.originalFiles) {
		return errors.New("keyring contents changed during replacement")
	}
	for name, digest := range r.originalFiles {
		if current, ok := currentFiles[name]; !ok || current != digest {
			return errors.New("keyring contents changed during replacement")
		}
	}
	if err := exchangeKeyringDirectories(r.liveDir, r.stageDir); err != nil {
		return fmt.Errorf("atomic key replacement failed: %w", err)
	}
	for _, dir := range []string{r.stageRoot, filepath.Dir(r.liveDir)} {
		if err := syncKeyringPath(dir); err != nil {
			if rollbackErr := exchangeKeyringDirectories(r.liveDir, r.stageDir); rollbackErr != nil {
				r.retainBackup = true
				return fmt.Errorf("key replacement durability failed: %w; rollback failed: %v; original keyring retained at %s", err, rollbackErr, r.stageDir)
			}
			for _, restoredDir := range []string{r.stageRoot, filepath.Dir(r.liveDir)} {
				if restoreErr := syncKeyringPath(restoredDir); restoreErr != nil {
					r.retainBackup = true
					return fmt.Errorf("key replacement durability failed: %w; original keyring restored but directory sync failed: %v", err, restoreErr)
				}
			}
			return fmt.Errorf("key replacement durability failed; original keyring restored: %w", err)
		}
	}
	return nil
}

func syncKeyringPath(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	return errors.Join(f.Sync(), f.Close())
}

func (r *keyReplacement) close(cmd *cobra.Command) {
	if r.stageRoot != "" && !r.retainBackup {
		if err := os.RemoveAll(r.stageRoot); err != nil {
			cmd.PrintErrf("could not remove private keyring staging directory %s: %v\n", r.stageRoot, err)
		}
	}
	if err := r.lock.Unlock(); err != nil {
		cmd.PrintErrf("could not release keyring replacement lock: %v\n", err)
	}
}
