package delivery

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// StreamReplayWindow matches the lifetime of the readers' resume cursors.
const StreamReplayWindow = 72 * time.Hour

// DefaultStreamRetention preserves the full reader replay window.
const DefaultStreamRetention = StreamReplayWindow

const streamRetentionBatch = 5000
const streamRetentionPassBatches = 4

type streamRetentionLease struct {
	partition int64
	epoch     int64
}

// acquireRetention serializes maintenance on the first durable offset. Its
// schedule survives restarts and coordinates all relay instances.
func (r *StreamOutboxRelay) acquireRetention(ctx context.Context) (*streamRetentionLease, error) {
	var lease *streamRetentionLease
	err := r.mutate(ctx, func(tx pgx.Tx) error {
		var l streamRetentionLease
		err := tx.QueryRow(ctx, `UPDATE api_stream.stream_delivery_offsets SET retention_owner=$1,retention_epoch=retention_epoch+1,retention_expires_at=clock_timestamp()+$2::bigint*interval '1 millisecond',retention_next_at=clock_timestamp()+interval '1 minute' WHERE partition_id=(SELECT partition_id FROM api_stream.stream_delivery_offsets ORDER BY partition_id LIMIT 1) AND (retention_expires_at IS NULL OR retention_expires_at<=clock_timestamp()) AND (retention_next_at IS NULL OR retention_next_at<=clock_timestamp()) AND retention_epoch<9223372036854775807 RETURNING partition_id,retention_epoch`, r.config.Owner, r.config.LeaseDuration.Milliseconds()).Scan(&l.partition, &l.epoch)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		lease = &l
		return nil
	})
	return lease, err
}

func (r *StreamOutboxRelay) pruneRetentionBatch(ctx context.Context, l streamRetentionLease) (int64, error) {
	var removed int64
	err := r.mutate(ctx, func(tx pgx.Tx) error {
		// The retention trigger deletes at most 5,000 eligible frames and their
		// cascading publications with the schema owner's authority.
		err := tx.QueryRow(ctx, `UPDATE api_stream.stream_delivery_offsets SET retention_cutoff=clock_timestamp()-$4::bigint*interval '1 millisecond' WHERE partition_id=$1 AND retention_owner=$2 AND retention_epoch=$3 AND retention_expires_at>clock_timestamp() RETURNING retention_removed`, l.partition, r.config.Owner, l.epoch, r.config.RetentionWindow.Milliseconds()).Scan(&removed)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrLease
		}
		if err != nil {
			return err
		}
		var alive bool
		if err = tx.QueryRow(ctx, `SELECT retention_owner=$2 AND retention_epoch=$3 AND retention_expires_at>clock_timestamp() FROM api_stream.stream_delivery_offsets WHERE partition_id=$1`, l.partition, r.config.Owner, l.epoch).Scan(&alive); err != nil {
			return err
		}
		if !alive {
			return ErrLease
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return removed, nil
}

// Prune removes up to 20,000 old published frames in four bounded transactions,
// at most once per minute across relays, under a lease and the restoration fence.
func (r *StreamOutboxRelay) Prune(ctx context.Context) (int64, error) {
	release, err := r.budget.acquire(ctx)
	if err != nil {
		return 0, err
	}
	defer release()
	l, err := r.acquireRetention(ctx)
	if err != nil || l == nil {
		return 0, err
	}
	var removed int64
	for i := 0; i < streamRetentionPassBatches; i++ {
		if err = r.budget.limiter.Wait(ctx); err != nil {
			return removed, err
		}
		n, err := r.pruneRetentionBatch(ctx, *l)
		if err != nil {
			return removed, err
		}
		removed += n
		if n < streamRetentionBatch {
			break
		}
	}
	err = r.mutate(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE api_stream.stream_delivery_offsets SET retention_expires_at=clock_timestamp() WHERE partition_id=$1 AND retention_owner=$2 AND retention_epoch=$3 AND retention_expires_at>clock_timestamp()`, l.partition, r.config.Owner, l.epoch)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrLease
		}
		return nil
	})
	return removed, err
}
