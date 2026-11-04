// Package reconcile is the idempotent backfill worker for core/pnl-tracker. On a
// schedule it scans indexer.swaps past the pnl.reconciler_cursor keyset and folds
// any swaps the realtime consumer missed (e.g. during downtime or webhook loss),
// using the SAME store.FoldTrade as the consumer — so backfill can never
// double-count (ON CONFLICT on the trade id) and realtime + backfill converge.
// Ports workers/reconciler.ts.
package reconcile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	goredis "github.com/redis/go-redis/v9"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/constants"
	shareddb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/pnlcache"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/store"
)

// Reconciler folds unreconciled indexer swaps into pnl positions.
type Reconciler struct {
	store     *store.Store
	redis     *goredis.Client
	logger    *slog.Logger
	batchSize int
}

// New builds a Reconciler. A non-positive batch size falls back to 500.
func New(st *store.Store, rdb *goredis.Client, logger *slog.Logger, batchSize int) *Reconciler {
	if batchSize <= 0 {
		batchSize = 500
	}
	return &Reconciler{store: st, redis: rdb, logger: logger, batchSize: batchSize}
}

// RunOnce scans and folds a single batch, advancing the cursor to the last swap
// processed. It returns the number of swaps examined (folded or already-present).
// Forward progress is guaranteed by the (block_number, log_index) keyset, so a
// block larger than the batch is still fully drained over successive ticks.
func (r *Reconciler) RunOnce(ctx context.Context) (int, error) {
	if r.redis == nil {
		return 0, errors.New("reconcile: Redis is required to drain refresh obligations")
	}
	cur, err := r.store.GetCursor(ctx)
	if err != nil {
		return 0, err
	}
	swaps, err := r.store.ListUnreconciledSwaps(ctx, cur, r.batchSize)
	if err != nil {
		return 0, err
	}
	if len(swaps) == 0 {
		return 0, nil
	}

	var last store.IndexerSwap
	processed := 0
	for i := range swaps {
		sw := swaps[i]
		usdl, token := legs(sw)
		_, ferr := r.store.FoldTrade(ctx, store.TradeInput{
			ID:             sw.ID,
			UserAddress:    sw.Sender,
			PoolAddress:    sw.PoolAddress,
			TokenAddress:   sw.TokenAddress,
			IsBuy:          sw.IsBuy,
			UsdlAmount:     usdl,
			TokenAmount:    token,
			Price:          sw.Price,
			Fee:            sw.Fee,
			BlockNumber:    sw.BlockNumber,
			BlockTimestamp: sw.BlockTimestamp,
			TxHash:         sw.TxHash,
		})
		if ferr != nil {
			// Only the prefix whose refresh obligations were drained may advance.
			if processed > 0 {
				if cerr := r.store.AdvanceCursor(ctx, last.BlockNumber, last.LogIndex, last.ID, shareddb.NowSeconds()); cerr != nil {
					return processed, errors.Join(fmt.Errorf("reconcile: fold swap %s: %w", sw.ID, ferr), cerr)
				}
			}
			return processed, fmt.Errorf("reconcile: fold swap %s: %w", sw.ID, ferr)
		}
		processed++
		// The persisted cursor is also the refresh checkpoint: a committed fold
		// remains replayable until its cache bust and notification both succeed.
		// Drain even on inserted=false, including after a stop between the fold
		// commit and Redis, or after an ambiguous cursor-write response.
		if err := r.refreshUser(ctx, sw); err != nil {
			return processed, fmt.Errorf("reconcile: refresh swap %s: %w", sw.ID, err)
		}
		last = sw
	}

	if err := r.store.AdvanceCursor(ctx, last.BlockNumber, last.LogIndex, last.ID, shareddb.NowSeconds()); err != nil {
		return processed, err
	}
	if processed > 0 {
		r.logger.Info("reconciled swaps", slog.Int("count", processed),
			slog.Int64("throughBlock", last.BlockNumber))
	}
	return processed, nil
}

// refreshUser atomically invalidates the read caches and publishes the same
// user-scoped signal as position.Consumer. The receipt survives worker restarts
// and has no TTL: a failed cursor write can replay an arbitrarily old trade.
// WATCH prevents two workers replaying the same trade from both publishing.
func (r *Reconciler) refreshUser(ctx context.Context, sw store.IndexerSwap) error {
	user := strings.ToLower(sw.Sender)
	receipt := "pnl:reconcile:notified:" + sw.ID
	payload, err := json.Marshal(map[string]string{"userAddress": user})
	if err != nil {
		return err
	}
	return r.redis.Watch(ctx, func(tx *goredis.Tx) error {
		sent, err := tx.Exists(ctx, receipt).Result()
		if err != nil {
			return err
		}
		_, err = tx.TxPipelined(ctx, func(pipe goredis.Pipeliner) error {
			pipe.Del(ctx, pnlcache.KeyPositions(user), pnlcache.KeyPortfolio(user))
			if sent == 0 {
				pipe.Set(ctx, receipt, "1", 0)
				pipe.Publish(ctx, constants.ChannelPnlUpdate, payload)
			}
			return nil
		})
		return err
	}, receipt)
}

// legs splits an indexer swap into its USDL and token amounts by direction.
func legs(sw store.IndexerSwap) (usdl, token string) {
	if sw.IsBuy {
		return sw.AmountIn, sw.AmountOut
	}
	return sw.AmountOut, sw.AmountIn
}
