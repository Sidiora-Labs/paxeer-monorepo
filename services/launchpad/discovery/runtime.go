package discovery

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type Publisher interface {
	Publish(context.Context, string, string, json.RawMessage) error
}
type Runtime struct {
	Store     Store
	Publisher Publisher
}

func (r Runtime) PublishBatch(ctx context.Context, limit int) (int, error) {
	if r.Publisher == nil || limit < 1 || limit > 100 {
		return 0, ErrInvalid
	}
	count := 0
	for count < limit {
		empty := false
		e := r.Store.transaction(ctx, func(tx *sql.Tx) error {
			var id, kind string
			var body []byte
			e := tx.QueryRowContext(ctx, `SELECT id,kind,document FROM narrative_discovery.outbox WHERE NOT published ORDER BY id FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&id, &kind, &body)
			if e == sql.ErrNoRows {
				empty = true
				return nil
			}
			if e != nil {
				return e
			}
			// The notification worker consumes the outbox after arbitrary delays.
			// A queued creation must not resurrect a target deleted in the meantime.
			if kind == "callout-target-ref-v2" {
				var event CalloutTargetEvent
				if e = json.Unmarshal(body, &event); e != nil {
					return e
				}
				var raw []byte
				if e = tx.QueryRowContext(ctx, `SELECT document FROM narrative_discovery.callouts WHERE id=$1`, event.TargetID).Scan(&raw); e != nil {
					return e
				}
				var c Callout
				if e = json.Unmarshal(raw, &c); e != nil {
					return e
				}
				if c.Deleted {
					event.Actor, event.Removed, event.Revision = c.AuthorAccountID, true, c.Revision
					body = encode(event)
				}
			}
			send, cancel := context.WithTimeout(ctx, 5*time.Second)
			e = r.Publisher.Publish(send, kind, id, body)
			cancel()
			if e != nil {
				return e
			}
			_, e = tx.ExecContext(ctx, `UPDATE narrative_discovery.outbox SET published=true WHERE id=$1`, id)
			return e
		})
		if e != nil {
			return count, e
		}
		if empty {
			break
		}
		count++
	}
	return count, nil
}
func (r Runtime) Run(ctx context.Context) error {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		evaluation, cancel := context.WithTimeout(ctx, 5*time.Second)
		_, e := r.EvaluateBatch(evaluation, 100, time.Now().UTC())
		cancel()
		if e != nil {
			return e
		}
		if _, e := r.PublishBatch(ctx, 100); e != nil {
			return e
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
}
func (s Store) ExpireSnapshots(ctx context.Context, now time.Time) error {
	_, e := s.DB.ExecContext(ctx, `DELETE FROM narrative_discovery.snapshots WHERE id IN(SELECT id FROM narrative_discovery.snapshots WHERE expires_at<=$1 LIMIT 1000)`, now)
	return e
}

// EvaluateBatch uses a durable round-robin cursor so a large rule inventory
// cannot starve later rules, including when the worker restarts between batches.
func (r Runtime) EvaluateBatch(ctx context.Context, limit int, now time.Time) (int, error) {
	if limit < 1 || limit > 100 || now.IsZero() {
		return 0, ErrInvalid
	}
	count := 0
	err := r.Store.transaction(ctx, func(tx *sql.Tx) error {
		var cursor string
		e := tx.QueryRowContext(ctx, `SELECT last_rule_id FROM narrative_discovery.evaluation_cursor WHERE id=true FOR UPDATE SKIP LOCKED`).Scan(&cursor)
		if e == sql.ErrNoRows {
			return nil
		}
		if e != nil {
			return e
		}
		rows, e := tx.QueryContext(ctx, `SELECT document FROM narrative_discovery.rules WHERE document->>'Disabled'='false' ORDER BY (id>$1) DESC,id LIMIT $2`, cursor, limit)
		if e != nil {
			return e
		}
		rules := []AlertRule{}
		for rows.Next() {
			var raw []byte
			var rule AlertRule
			if e = rows.Scan(&raw); e == nil {
				e = json.Unmarshal(raw, &rule)
			}
			if e != nil {
				rows.Close()
				return e
			}
			rules = append(rules, rule)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		for _, rule := range rules {
			// Pre-migration rules have no denomination. They remain explicitly held
			// until their owner replaces them with a price-qualified rule.
			if rule.Price == nil {
				if _, e = tx.ExecContext(ctx, `UPDATE narrative_discovery.rules SET last_reason='price_context_missing' WHERE id=$1`, rule.ID); e != nil {
					return e
				}
			} else {
				metric, e := canonicalPrice(ctx, tx, rule.PoolAddress)
				if errors.Is(e, ErrUnavailable) {
					if _, e = tx.ExecContext(ctx, `UPDATE narrative_discovery.rules SET last_reason='price_unavailable' WHERE id=$1`, rule.ID); e != nil {
						return e
					}
				} else if e != nil {
					return e
				} else if _, _, e = evaluateRuleTx(ctx, tx, rule.ID, metric, now); e != nil {
					return e
				}
			}
			cursor = rule.ID
			count++
		}
		_, e = tx.ExecContext(ctx, `UPDATE narrative_discovery.evaluation_cursor SET last_rule_id=$1 WHERE id=true`, cursor)
		return e
	})
	return count, err
}
