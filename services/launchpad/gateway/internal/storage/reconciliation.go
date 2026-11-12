package storage

import (
	"context"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/cleanup"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Tombstone struct {
	Hash     string
	Revision int64
}

type TombstoneStore interface {
	BeginReplay(context.Context) (TombstoneBatch, error)
}

type TombstoneBatch interface {
	Pending(context.Context, int) ([]Tombstone, error)
	RestoreRestriction(context.Context, Tombstone) error
	EnsureOperation(context.Context, Operation) error
	Commit(context.Context) error
	Rollback(context.Context) error
}

// ReplayTombstones drains recovery work in bounded, independently committed batches.
func ReplayTombstones(ctx context.Context, store TombstoneStore) error {
	if store == nil {
		return ErrUnavailable
	}
	for {
		if ctx.Err() != nil {
			return ErrUnavailable
		}
		count, err := replayTombstoneBatch(ctx, store)
		if err != nil {
			return err
		}
		if count == 0 {
			return nil
		}
	}
}

func replayTombstoneBatch(ctx context.Context, store TombstoneStore) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	batch, err := store.BeginReplay(ctx)
	if err != nil {
		return 0, ErrUnavailable
	}
	defer cleanup.Rollback(context.WithoutCancel(ctx), batch)
	pending, err := batch.Pending(ctx, 256)
	if err != nil {
		return 0, ErrUnavailable
	}
	if len(pending) > 256 {
		return 0, ErrConflict
	}
	for _, item := range pending {
		if !digest.MatchString(item.Hash) || item.Revision < 1 {
			return 0, ErrConflict
		}
		if err = batch.RestoreRestriction(ctx, item); err != nil {
			return 0, ErrUnavailable
		}
		for _, kind := range []string{deleteKind, purgeKind} {
			if err = batch.EnsureOperation(ctx, Operation{ID: uuid.NewString(), Hash: item.Hash, Revision: item.Revision, Kind: kind, State: requestedState}); err != nil {
				return 0, ErrUnavailable
			}
		}
	}
	if err = batch.Commit(ctx); err != nil {
		return 0, ErrUnavailable
	}
	return len(pending), nil
}

type postgresTombstones struct{ authority *Authority }
type postgresTombstoneBatch struct {
	authority *Authority
	pgx.Tx
}

func (s postgresTombstones) BeginReplay(ctx context.Context) (TombstoneBatch, error) {
	tx, err := s.authority.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &postgresTombstoneBatch{s.authority, tx}, nil
}

func (b *postgresTombstoneBatch) Pending(ctx context.Context, limit int) ([]Tombstone, error) {
	a := b.authority
	rows, err := b.Query(ctx, "SELECT t.hash,t.revision FROM "+a.table("tombstones")+" t LEFT JOIN "+a.table("restrictions")+" r ON r.hash=t.hash WHERE r.hash IS NULL OR r.state<>'deleted' OR r.initial_visibility OR r.revision<t.revision OR NOT EXISTS(SELECT 1 FROM "+a.table("operations")+" o WHERE o.hash=t.hash AND o.revision=t.revision AND o.kind='delete') OR NOT EXISTS(SELECT 1 FROM "+a.table("operations")+" o WHERE o.hash=t.hash AND o.revision=t.revision AND o.kind='purge') ORDER BY t.hash LIMIT $1", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var pending []Tombstone
	for rows.Next() {
		var item Tombstone
		if err = rows.Scan(&item.Hash, &item.Revision); err != nil {
			return nil, err
		}
		pending = append(pending, item)
	}
	return pending, rows.Err()
}

func (b *postgresTombstoneBatch) RestoreRestriction(ctx context.Context, item Tombstone) error {
	table := b.authority.table("restrictions")
	_, err := b.Exec(ctx, "INSERT INTO "+table+"(hash,revision,state) VALUES($1,$2,'deleted') ON CONFLICT(hash) DO UPDATE SET state='deleted',initial_visibility=false,revision=GREATEST("+table+".revision,EXCLUDED.revision)", item.Hash, item.Revision)
	return err
}

func (b *postgresTombstoneBatch) EnsureOperation(ctx context.Context, op Operation) error {
	_, err := b.Exec(ctx, "INSERT INTO "+b.authority.table("operations")+"(id,hash,revision,kind,state) VALUES($1,$2,$3,$4,$5) ON CONFLICT(hash,revision,kind) DO NOTHING", op.ID, op.Hash, op.Revision, op.Kind, op.State)
	return err
}
