package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const (
	// PriceRetentionBatch bounds both candidate rows and deletions in a statement.
	PriceRetentionBatch = 1000
	// PriceRetentionBatches bounds the statements per table in one pass.
	PriceRetentionBatches = 16
)

const retainPricesSQL = `WITH lease AS MATERIALIZED (
    SELECT 1 FROM perps.writer_lease
    WHERE name = $4 AND holder = $5 AND expires_at > clock_timestamp() FOR SHARE
), picked AS MATERIALIZED (
    SELECT row_id, market_id, kind, block_time FROM perps.index_prices
    WHERE kind IN ('aggregate', 'skipped') AND block_time < $1
        AND (block_time, row_id) > ($2, $3)
        AND EXISTS (SELECT 1 FROM lease)
    ORDER BY block_time, row_id LIMIT 1000
), doomed AS MATERIALIZED (
    SELECT p.row_id FROM picked p
    WHERE p.row_id <> (SELECT n.row_id FROM perps.index_prices n
        WHERE n.market_id = p.market_id
        ORDER BY n.block DESC, n.tx_index DESC, n.log_index DESC, n.sub DESC LIMIT 1)
    AND (p.kind = 'skipped' OR p.row_id <> (
        SELECT n.row_id FROM perps.index_prices n
        WHERE n.market_id = p.market_id AND n.kind = 'aggregate' AND n.block_time < $1
            AND date_trunc('minute', n.block_time AT TIME ZONE 'UTC') =
                date_trunc('minute', p.block_time AT TIME ZONE 'UTC')
        ORDER BY n.block DESC, n.tx_index DESC, n.log_index DESC, n.sub DESC LIMIT 1))
), removed AS (
    DELETE FROM perps.index_prices WHERE row_id = ANY(ARRAY(SELECT row_id FROM doomed))
    RETURNING row_id
)
SELECT EXISTS (SELECT 1 FROM lease), (SELECT count(*) FROM picked), (SELECT count(*) FROM removed),
    (SELECT block_time FROM picked ORDER BY block_time DESC, row_id DESC LIMIT 1),
    (SELECT row_id FROM picked ORDER BY block_time DESC, row_id DESC LIMIT 1)`

const retainPriceEventsSQL = `WITH lease AS MATERIALIZED (
    SELECT 1 FROM perps.writer_lease
    WHERE name = $2 AND holder = $3 AND expires_at > clock_timestamp() FOR SHARE
), picked AS MATERIALIZED (
    SELECT row_id FROM perps.protocol_events
    WHERE event_name IN ('pricefeed_aggregate', 'pricefeed_aggregate_skipped') AND block_time < $1
        AND EXISTS (SELECT 1 FROM lease)
    ORDER BY block_time, row_id LIMIT 1000
), removed AS (
    DELETE FROM perps.protocol_events WHERE row_id = ANY(ARRAY(SELECT row_id FROM picked))
    RETURNING row_id
)
SELECT EXISTS (SELECT 1 FROM lease), (SELECT count(*) FROM removed)`

// PriceRetention walks old index observations by (block_time, row_id). Its zero
// value starts at the beginning; use one instance in the writer's maintenance
// loop, without concurrent calls. A restart safely starts a new sweep.
type PriceRetention struct {
	afterTime time.Time
	afterID   int64
}

// PriceRetentionResult counts the rows removed by committed statements.
type PriceRetentionResult struct {
	PriceRows int64
	EventRows int64
}

// Pass keeps the last aggregate in each UTC minute of each market before
// cutoff and the newest observation of every market, including a newest skipped
// observation. It removes older raw pricefeed events. Other facts are untouched.
// A replay of an old height range writes full-grain rows again; the next sweep
// reduces those rows too. Each statement commits independently and fences the
// writer lease only until that statement ends. Successful statements advance
// the sweep even if a later statement is cancelled; retained samples never
// prevent a later pass from reaching the rest of the history. On reaching the
// end it wraps to catch replayed rows behind its cursor.
func (p *PriceRetention) Pass(ctx context.Context, db *sql.DB, cutoff time.Time, name, holder string) (PriceRetentionResult, error) {
	var result PriceRetentionResult
	if p == nil || db == nil || cutoff.IsZero() || name == "" || holder == "" {
		return result, errors.New("store: price retention needs a database, cutoff and writer lease")
	}
	for range PriceRetentionBatches {
		var held bool
		var scanned, removed int64
		var at sql.NullTime
		var id sql.NullInt64
		err := db.QueryRowContext(ctx, retainPricesSQL, cutoff.UTC(), p.afterTime, p.afterID, name, holder).
			Scan(&held, &scanned, &removed, &at, &id)
		if err != nil {
			return result, fmt.Errorf("store: retain index prices: %w", err)
		}
		if !held {
			return result, ErrLeaseHeld
		}
		result.PriceRows += removed
		if scanned < PriceRetentionBatch {
			p.afterTime, p.afterID = time.Time{}, 0
			break
		}
		p.afterTime, p.afterID = at.Time, id.Int64
	}
	for range PriceRetentionBatches {
		var held bool
		var removed int64
		if err := db.QueryRowContext(ctx, retainPriceEventsSQL, cutoff.UTC(), name, holder).Scan(&held, &removed); err != nil {
			return result, fmt.Errorf("store: retain pricefeed events: %w", err)
		}
		if !held {
			return result, ErrLeaseHeld
		}
		result.EventRows += removed
		if removed < PriceRetentionBatch {
			break
		}
	}
	return result, nil
}
