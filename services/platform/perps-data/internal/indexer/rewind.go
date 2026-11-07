package indexer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"

	"github.com/lib/pq"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/source"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/store"
)

// rebuildAffected runs after the follower deletes the fork, inside its
// transaction. Scope must come from the pre-deletion capture: a range starting
// at a.From no longer contains any facts from which Rebuild can infer scope.
//
// The follower deletes snapshots for every market in the fork hour, and its
// scope omits spot entities. Refold the surviving range through the store while
// preserving unrelated account projections exactly. Pass captured scope too,
// including entities whose entire history was on the discarded fork.
func rebuildAffected(ctx context.Context, tx *sql.Tx, a source.Affected) error {
	if tx == nil {
		return errors.New("indexer: rewind rebuild needs a transaction")
	}
	from, recovery, err := rewindRecovery(a)
	if err != nil {
		return err
	}
	accounts := make(pq.ByteaArray, len(a.Accounts))
	for i, account := range a.Accounts {
		accounts[i] = account.Bytes()
	}
	accountTables := []string{"positions_current", "orders", "spot_orders", "account_margin", "user_stats", "user_pnl_daily"}
	for _, table := range accountTables {
		if _, err := tx.ExecContext(ctx, `CREATE TEMP TABLE rewind_`+table+` ON COMMIT DROP AS
SELECT * FROM perps.`+table+` WHERE NOT (account = ANY($1::bytea[]))`, accounts); err != nil {
			return fmt.Errorf("indexer: preserve %s: %w", table, err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM perps.`+table+` WHERE account = ANY($1::bytea[])`, accounts); err != nil {
			return err
		}
	}
	// Include markets whose only facts were on the discarded fork. Rebuild's
	// surviving-range discovery cannot find them.
	if _, err := tx.ExecContext(ctx, `DELETE FROM perps.market_stats_rolling WHERE market_id = ANY($1::numeric[])`, pq.Array(a.Markets)); err != nil {
		return err
	}
	for _, table := range []string{"market_snapshots", "protocol_snapshots"} {
		if _, err := tx.ExecContext(ctx, `DELETE FROM perps.`+table+` WHERE hour >= date_trunc('hour', $1::timestamptz)`, a.FromTime); err != nil {
			return err
		}
	}
	// An empty surviving history must remove orphan boards too. The normal
	// range rebuild has no changing block at which it could do that.
	for _, table := range []string{"leaderboard", "spot_leaderboard", "spot_account_legs", "spot_token_seen"} {
		if _, err := tx.ExecContext(ctx, `DELETE FROM perps.`+table); err != nil {
			return err
		}
	}
	var first, head sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT min(number), max(number) FROM perps.blocks`).Scan(&first, &head); err != nil {
		return err
	}
	to := from
	if head.Valid {
		from, to = first.Int64, head.Int64
	}
	if err := store.Rebuild(ctx, tx, from, to, recovery); err != nil {
		return err
	}
	for _, table := range accountTables {
		if _, err := tx.ExecContext(ctx, `DELETE FROM perps.`+table+` WHERE NOT (account = ANY($1::bytea[]))`, accounts); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO perps.`+table+` SELECT * FROM rewind_`+table); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DROP TABLE rewind_`+table); err != nil {
			return err
		}
	}
	// Tokens and registry changes are not carried by source.Affected yet.
	// Reconcile their actual surviving history, including orphan removals.
	_, err = tx.ExecContext(ctx, `INSERT INTO perps.spot_token_seen (token, first_block)
SELECT token, min(block) FROM (
 SELECT block, v.token FROM perps.spot_swaps s CROSS JOIN LATERAL
 (VALUES (s.stablecoin), (s.token), (s.token_other), (s.token_in), (s.token_out)) v(token)
 UNION ALL SELECT block, v.token FROM perps.spot_order_events s CROSS JOIN LATERAL (VALUES (s.token), (s.stablecoin)) v(token)
 UNION ALL SELECT block, v.token FROM perps.spot_token_registry s CROSS JOIN LATERAL (VALUES (s.token), (s.share_token)) v(token)
) history WHERE token IS NOT NULL GROUP BY token
ON CONFLICT (token) DO UPDATE SET first_block = EXCLUDED.first_block`)
	return err
}

func rewindRecovery(a source.Affected) (int64, store.Recovery, error) {
	if a.From > math.MaxInt64 {
		return 0, store.Recovery{}, fmt.Errorf("indexer: rewind height %d exceeds database range", a.From)
	}
	if a.FromTime.IsZero() && (len(a.Accounts) != 0 || len(a.Markets) != 0) {
		return 0, store.Recovery{}, errors.New("indexer: rewind scope requires the first changed time")
	}
	return int64(a.From), store.Recovery{
		Accounts: a.Accounts,
		Markets:  a.Markets,
		FromTime: a.FromTime,
	}, nil
}
