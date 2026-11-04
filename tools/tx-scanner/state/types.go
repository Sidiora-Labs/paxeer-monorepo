package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const checkpointName = "tx-scanner-state.json"

// ErrRecoveryRequired distinguishes a damaged checkpoint from a first start.
var ErrRecoveryRequired = errors.New("scanner checkpoint requires explicit recovery")

type State struct {
	LastProcessedHeight int64   `json:"last_processed_height"`
	BlocksMissingTxs    []int64 `json:"blocks_missing_txs"`
}

// WriteState writes a synced checkpoint, preserving the previous valid checkpoint
// in tx-scanner-state.json.previous before atomically replacing the current one.
// Calls for the same directory must be serialized by the caller.
func WriteState(dir string, s State) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	dir = filepath.Clean(dir)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}
	filename := filepath.Join(dir, checkpointName)
	_, previous, err := readCheckpoint(filename)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("refusing to replace checkpoint %q: %w", filename, err)
		}
		if _, backupErr := os.Lstat(filename + ".previous"); !errors.Is(backupErr, os.ErrNotExist) {
			if backupErr != nil {
				return backupErr
			}
			return fmt.Errorf("%w: checkpoint %q is missing; restore %q before writing", ErrRecoveryRequired, filename, filename+".previous")
		}
	} else {
		if err := writeCheckpoint(filename+".previous", previous); err != nil {
			return fmt.Errorf("preserve previous checkpoint: %w", err)
		}
	}
	return writeCheckpoint(filename, data)
}

func writeCheckpoint(filename string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(filename), ".tx-scanner-state-*.tmp")
	if err != nil {
		return err
	}
	tempName := file.Name()
	defer func() { _ = os.Remove(tempName) }()
	if _, err := file.Write(data); err != nil {
		return errors.Join(err, file.Close())
	}
	if err := file.Sync(); err != nil {
		return errors.Join(err, file.Close())
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempName, filename); err != nil {
		return err
	}
	// Persist the rename before acknowledging the checkpoint or replacing the
	// primary after publishing its backup.
	dir, err := os.Open(filepath.Dir(filename))
	if err != nil {
		return err
	}
	return errors.Join(dir.Sync(), dir.Close())
}

func readCheckpoint(filename string) (State, []byte, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return State{}, nil, err
	}
	// Both fields are emitted by WriteState. A syntactically valid but empty or
	// incomplete object must not erase the scanner's history on restart.
	var fields struct {
		LastProcessedHeight *int64          `json:"last_processed_height"`
		BlocksMissingTxs    json.RawMessage `json:"blocks_missing_txs"`
	}
	if err := json.Unmarshal(data, &fields); err != nil {
		return State{}, nil, fmt.Errorf("%w: decode %q: %w", ErrRecoveryRequired, filename, err)
	}
	if fields.LastProcessedHeight == nil || len(fields.BlocksMissingTxs) == 0 {
		return State{}, nil, fmt.Errorf("%w: checkpoint %q lacks required state fields", ErrRecoveryRequired, filename)
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{}, nil, fmt.Errorf("%w: decode %q: %w", ErrRecoveryRequired, filename, err)
	}
	return s, data, nil
}

// ReadState reads the current checkpoint without silently falling back to older
// state. os.ErrNotExist identifies a first start only when no backup exists.
func ReadState(dir string) (State, error) {
	filename := filepath.Join(filepath.Clean(dir), checkpointName)
	s, _, err := readCheckpoint(filename)
	if errors.Is(err, os.ErrNotExist) {
		if _, backupErr := os.Lstat(filename + ".previous"); !errors.Is(backupErr, os.ErrNotExist) {
			if backupErr != nil {
				return State{}, backupErr
			}
			return State{}, fmt.Errorf("%w: checkpoint %q is missing; inspect %q", ErrRecoveryRequired, filename, filename+".previous")
		}
	}
	if errors.Is(err, ErrRecoveryRequired) {
		return State{}, fmt.Errorf("%w; inspect previous checkpoint %q", err, filename+".previous")
	}
	return s, err
}
