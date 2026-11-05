// Package reconciliation supplies a transactional in-memory storage peer whose
// committed snapshots survive reopening during the repair's recovery proofs.
package reconciliation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	storage "github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/storage"
	"github.com/jackc/pgx/v5"
)

type Restriction struct {
	Revision int64
	State    string
}
type Lineage struct{ Hash, Kind string }
type State struct {
	Tombstones    map[string]storage.Tombstone
	Restrictions  map[string]Restriction
	Operations    map[string]storage.Operation
	Cursors       map[string]string
	Objects       map[string]Lineage
	QuarantinedAt time.Time
}

type Store struct {
	mu       sync.Mutex
	path     string
	state    State
	closed   bool
	MaxBatch int
	Commits  int
}

func Open(path string) (*Store, error) {
	s := &Store{path: path, state: State{
		Tombstones: map[string]storage.Tombstone{}, Restrictions: map[string]Restriction{},
		Operations: map[string]storage.Operation{}, Cursors: map[string]string{}, Objects: map[string]Lineage{},
	}}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(raw, &s.state); err != nil {
		return nil, err
	}
	return s, nil
}

func operationKey(hash string, revision int64, kind string) string {
	return fmt.Sprintf("%s:%d:%s", hash, revision, kind)
}
func scope(bucket, prefix string) string { return fmt.Sprintf("%d:%s:%s", len(bucket), bucket, prefix) }

func (s *Store) Update(ctx context.Context, update func(*State)) error {
	tx, err := s.begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	update(&tx.state)
	return tx.Commit(ctx)
}

func (s *Store) Snapshot() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return clone(s.state)
}

func clone(state State) State {
	raw, err := json.Marshal(state)
	if err != nil {
		panic(err)
	}
	var result State
	if err = json.Unmarshal(raw, &result); err != nil {
		panic(err)
	}
	return result
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return os.ErrClosed
	}
	s.closed = true
	return nil
}

type transaction struct {
	store          *Store
	state          State
	done           bool
	bucket, prefix string
}

func (s *Store) begin(ctx context.Context) (*transaction, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, os.ErrClosed
	}
	return &transaction{store: s, state: clone(s.state)}, nil
}

func (s *Store) BeginReplay(ctx context.Context) (storage.TombstoneBatch, error) { return s.begin(ctx) }
func (s *Store) BeginInventory(ctx context.Context, bucket, prefix string) (storage.InventoryPass, error) {
	tx, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	tx.bucket, tx.prefix = bucket, prefix
	return tx, nil
}

func (tx *transaction) Pending(ctx context.Context, limit int) ([]storage.Tombstone, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var keys []string
	for hash, t := range tx.state.Tombstones {
		r, exists := tx.state.Restrictions[hash]
		_, hasDelete := tx.state.Operations[operationKey(hash, t.Revision, "delete")]
		_, hasPurge := tx.state.Operations[operationKey(hash, t.Revision, "purge")]
		if !exists || r.State != "deleted" || r.Revision < t.Revision || !hasDelete || !hasPurge {
			keys = append(keys, hash)
		}
	}
	sort.Strings(keys)
	if len(keys) > limit {
		keys = keys[:limit]
	}
	result := make([]storage.Tombstone, 0, len(keys))
	for _, key := range keys {
		result = append(result, tx.state.Tombstones[key])
	}
	tx.store.MaxBatch = max(tx.store.MaxBatch, len(result))
	return result, nil
}

func (tx *transaction) RestoreRestriction(ctx context.Context, t storage.Tombstone) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r := tx.state.Restrictions[t.Hash]
	tx.state.Restrictions[t.Hash] = Restriction{Revision: max(r.Revision, t.Revision), State: "deleted"}
	return nil
}
func (tx *transaction) EnsureOperation(ctx context.Context, op storage.Operation) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	key := operationKey(op.Hash, op.Revision, op.Kind)
	if _, exists := tx.state.Operations[key]; !exists {
		op.RequestedAt = time.Now().UTC()
		tx.state.Operations[key] = op
	}
	return nil
}
func (tx *transaction) Cursor(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return tx.state.Cursors[scope(tx.bucket, tx.prefix)], nil
}
func (tx *transaction) Lineage(ctx context.Context, key string) (string, string, error) {
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	l, exists := tx.state.Objects[key]
	if !exists {
		return "", "", pgx.ErrNoRows
	}
	return l.Hash, l.Kind, nil
}
func (tx *transaction) Advance(ctx context.Context, token string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	tx.state.Cursors[scope(tx.bucket, tx.prefix)] = token
	return nil
}
func (tx *transaction) Commit(ctx context.Context) error {
	if tx.done {
		return pgx.ErrTxClosed
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	raw, err := json.Marshal(tx.state)
	if err != nil {
		return err
	}
	// A single atomic snapshot is the fixture's committed storage boundary.
	temporary := tx.store.path + ".pending"
	if err = os.WriteFile(temporary, raw, 0600); err != nil {
		return err
	}
	if err = os.Rename(temporary, tx.store.path); err != nil {
		return err
	}
	tx.store.state = tx.state
	tx.store.Commits++
	tx.done = true
	tx.store.mu.Unlock()
	return nil
}
func (tx *transaction) Rollback(context.Context) error {
	if tx.done {
		return pgx.ErrTxClosed
	}
	tx.done = true
	tx.store.mu.Unlock()
	return nil
}

func (s *Store) MediaBacklogAges(ctx context.Context) (float64, float64, error) {
	if err := ctx.Err(); err != nil {
		return 0, 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, 0, os.ErrClosed
	}
	var oldest time.Time
	for _, op := range s.state.Operations {
		if op.Kind == "purge" && op.State != "complete" && (oldest.IsZero() || op.RequestedAt.Before(oldest)) {
			oldest = op.RequestedAt
		}
	}
	var purge, quarantine float64
	if !oldest.IsZero() {
		purge = time.Since(oldest).Seconds()
	}
	if !s.state.QuarantinedAt.IsZero() {
		quarantine = time.Since(s.state.QuarantinedAt).Seconds()
	}
	return purge, quarantine, nil
}

func (s *Store) Allowed(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return storage.ErrUnavailable
	}
	l, exists := s.state.Objects[key]
	if !exists || !strings.HasPrefix(key, "approved/") {
		return storage.ErrRestricted
	}
	if _, deleted := s.state.Tombstones[l.Hash]; deleted {
		return storage.ErrRestricted
	}
	if r, exists := s.state.Restrictions[l.Hash]; !exists || r.State != "approved" {
		return storage.ErrRestricted
	}
	return nil
}
