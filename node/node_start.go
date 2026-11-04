package app

import (
	"bytes"
	"crypto/sha256"
	"fmt"
)

// CommittedAppHash returns the post-commit hash of height, never the parent
// AppHash carried by that block's header. The returned bytes belong to the caller.
func (app *App) CommittedAppHash(height int64) ([]byte, error) {
	commit := app.CommitMultiStore().LastCommitID()
	if height <= 0 || commit.Version != height {
		return nil, fmt.Errorf("application commit height %d does not match requested height %d", commit.Version, height)
	}
	if len(commit.Hash) != sha256.Size {
		return nil, fmt.Errorf("application commit at height %d has hash length %d, want %d", height, len(commit.Hash), sha256.Size)
	}
	return bytes.Clone(commit.Hash), nil
}

// CheckCommittedAppHash binds a consensus committed record to the application's
// durable state at the same height, including during the startup handshake.
func (app *App) CheckCommittedAppHash(height int64, hash []byte) error {
	committed, err := app.CommittedAppHash(height)
	if err != nil {
		return err
	}
	if !bytes.Equal(hash, committed) {
		return fmt.Errorf("application hash %X differs from consensus app hash %X at height %d", committed, hash, height)
	}
	return nil
}
