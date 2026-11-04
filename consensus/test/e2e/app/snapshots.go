// nolint: gosec
package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
)

const (
	snapshotChunkSize = 1e6

	// Keep only the most recent 10 snapshots. Older snapshots are pruned
	maxSnapshotCount = 10
	pruneFileName    = "prune.json"
)

type snapshotPrune struct {
	Retained []abci.Snapshot
	Removed  []abci.Snapshot
}

// SnapshotStore stores state sync snapshots. Snapshots are stored simply as
// JSON files, and chunks are generated on-the-fly by splitting the JSON data
// into fixed-size chunks.
type SnapshotStore struct {
	sync.RWMutex
	dir      string
	metadata []abci.Snapshot
}

// NewSnapshotStore creates a new snapshot store.
func NewSnapshotStore(dir string) (*SnapshotStore, error) {
	store := &SnapshotStore{dir: dir}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	if err := store.loadMetadata(); err != nil {
		return nil, err
	}
	return store, nil
}

// loadMetadata loads snapshot metadata. Does not take out locks, since it's
// called internally on construction.
func (s *SnapshotStore) loadMetadata() error {
	file := filepath.Join(s.dir, "metadata.json")
	metadata := []abci.Snapshot{}

	bz, err := os.ReadFile(file)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return fmt.Errorf("failed to load snapshot metadata from %q: %w", file, err)
	}
	if len(bz) != 0 {
		err = json.Unmarshal(bz, &metadata)
		if err != nil {
			return fmt.Errorf("invalid snapshot data in %q: %w", file, err)
		}
	}
	s.metadata = metadata
	return s.recoverPrune()
}

// saveMetadata saves snapshot metadata. Does not take out locks, since it's
// called internally from e.g. Create().
func (s *SnapshotStore) saveMetadata() error {
	bz, err := json.Marshal(s.metadata)
	if err != nil {
		return err
	}

	return replaceFile(filepath.Join(s.dir, "metadata.json"), bz)
}

// recoverPrune completes a durable prune intent before accepting new writes.
// The journal survives both partial deletion and interrupted metadata replacement.
// The caller holds the store lock, or is constructing the store.
func (s *SnapshotStore) recoverPrune() error {
	file := filepath.Join(s.dir, pruneFileName)
	bz, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var prune snapshotPrune
	if err := json.Unmarshal(bz, &prune); err != nil {
		return fmt.Errorf("invalid snapshot prune journal: %w", err)
	}
	// Once an intent is durable, never advertise snapshots selected for removal.
	s.metadata = prune.Retained
	for _, snapshot := range prune.Removed {
		if err := os.Remove(filepath.Join(s.dir, fmt.Sprintf("%v.json", snapshot.Height))); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := syncDirectory(s.dir); err != nil {
		return err
	}
	if err := s.saveMetadata(); err != nil {
		return err
	}
	if err := os.Remove(file); err != nil {
		return err
	}
	return syncDirectory(s.dir)
}

// Create creates a snapshot of the given application state's key/value pairs.
func (s *SnapshotStore) Create(state *State) (abci.Snapshot, error) {
	s.Lock()
	defer s.Unlock()
	if err := s.recoverPrune(); err != nil {
		return abci.Snapshot{}, err
	}
	bz, err := state.Export()
	if err != nil {
		return abci.Snapshot{}, err
	}
	snapshot := abci.Snapshot{
		Height: state.Height,
		Format: 1,
		Hash:   hashItems(state.Values, state.Height),
		Chunks: byteChunks(bz),
	}
	err = replaceFile(filepath.Join(s.dir, fmt.Sprintf("%v.json", state.Height)), bz)
	if err != nil {
		return abci.Snapshot{}, err
	}
	s.metadata = append(s.metadata, snapshot)
	err = s.saveMetadata()
	if err != nil {
		return abci.Snapshot{}, err
	}
	return snapshot, nil
}

// Prune removes old snapshots ensuring only the most recent n snapshots remain
func (s *SnapshotStore) Prune(n int) error {
	s.Lock()
	defer s.Unlock()
	if n < 0 {
		return fmt.Errorf("snapshot retention count must be non-negative: %d", n)
	}
	if err := s.recoverPrune(); err != nil {
		return err
	}
	if n >= len(s.metadata) {
		return nil
	}
	// Persist the complete intent before removing any snapshot files.
	i := len(s.metadata) - n
	prune := snapshotPrune{Retained: s.metadata[i:], Removed: s.metadata[:i]}
	bz, err := json.Marshal(prune)
	if err != nil {
		return err
	}
	if err := replaceFile(filepath.Join(s.dir, pruneFileName), bz); err != nil {
		return err
	}
	return s.recoverPrune()
}

// List lists available snapshots.
func (s *SnapshotStore) List() ([]*abci.Snapshot, error) {
	s.RLock()
	defer s.RUnlock()
	snapshots := make([]*abci.Snapshot, len(s.metadata))
	for idx := range s.metadata {
		snapshots[idx] = &s.metadata[idx]
	}
	return snapshots, nil
}

// LoadChunk loads a snapshot chunk.
func (s *SnapshotStore) LoadChunk(height uint64, format uint32, chunk uint32) ([]byte, error) {
	s.RLock()
	defer s.RUnlock()
	for _, snapshot := range s.metadata {
		if snapshot.Height == height && snapshot.Format == format {
			bz, err := os.ReadFile(filepath.Join(s.dir, fmt.Sprintf("%v.json", height)))
			if err != nil {
				return nil, err
			}
			return byteChunk(bz, chunk), nil
		}
	}
	return nil, nil
}

// byteChunk returns the chunk at a given index from the full byte slice.
func byteChunk(bz []byte, index uint32) []byte {
	start := int(index * snapshotChunkSize)
	end := int((index + 1) * snapshotChunkSize)
	switch {
	case start >= len(bz):
		return nil
	case end >= len(bz):
		return bz[start:]
	default:
		return bz[start:end]
	}
}

// byteChunks calculates the number of chunks in the byte slice.
func byteChunks(bz []byte) uint32 {
	return uint32(math.Ceil(float64(len(bz)) / snapshotChunkSize))
}
