package control

import (
	"context"
	"encoding/json"
	"sort"
	"time"
)

type workSchedule struct {
	Attempts   uint
	Last, Next time.Time
}
type workItem struct {
	Key, Scope string
	Body       json.RawMessage
	Schedule   workSchedule
}

// claimWork advances durable scheduling before any external request. Older
// attempted entries sort after never-attempted entries, even after restart.
// Backoff bounds repeated provider uncertainty independently for every key.
func (s *Store) claimWork(ctx context.Context, credential, kind, op string) ([]workItem, error) {
	items, err := s.pendingWork(ctx, kind)
	if err != nil {
		return nil, err
	}
	claimed := make([]workItem, 0, len(items))
	for _, item := range items {
		err = s.run(ctx, credential, op, item.Scope, "claim durable "+kind+" work", func(t *transaction) error {
			var current workSchedule
			if err := t.get("work_schedule", kind+"."+item.Key, &current); err != nil && err != Missing {
				return err
			}
			if current.Next.After(t.now) {
				return Conflict
			}
			var body struct{ State, Scope string }
			if err := t.get(kind, item.Key, &body); err != nil {
				return err
			}
			if body.Scope != item.Scope {
				return Conflict
			}
			if kind == "incident" {
				var outbox map[string]string
				if err := t.get("incident_outbox", item.Key, &outbox); err != nil {
					return err
				}
				if body.State != "contained" || outbox["state"] == "completed" {
					return Conflict
				}
			} else if body.State != "pending" && body.State != "unknown" {
				return Conflict
			}
			if current.Attempts < 5 {
				current.Attempts++
			}
			delay := 30 * time.Second * time.Duration(1<<(current.Attempts-1))
			if delay > 5*time.Minute {
				delay = 5 * time.Minute
			}
			current.Last = t.now
			current.Next = t.now.Add(delay)
			return t.put("work_schedule", kind+"."+item.Key, current)
		})
		if err == Conflict {
			continue
		}
		if err != nil {
			return claimed, err
		}
		claimed = append(claimed, item)
	}
	return claimed, nil
}
func (s *Store) pendingWork(ctx context.Context, kind string) ([]workItem, error) {
	if kind != "incident" && kind != "settlement" && kind != "job" {
		return nil, Invalid
	}
	if s.local != nil {
		l := s.local
		l.mu.Lock()
		defer l.mu.Unlock()
		if l.closed {
			return nil, Unavailable
		}
		items := []workItem{}
		now := time.Now()
		for object, raw := range l.objects {
			prefix := kind + "/"
			if len(object) <= len(prefix) || object[:len(prefix)] != prefix {
				continue
			}
			k := object[len(prefix):]
			var body struct{ State, Scope string }
			if err := json.Unmarshal(raw, &body); err != nil {
				return nil, err
			}
			if kind == "incident" {
				var outbox map[string]string
				if err := json.Unmarshal(l.objects["incident_outbox/"+k], &outbox); err != nil {
					return nil, err
				}
				if body.State != "contained" || outbox["state"] == "completed" {
					continue
				}
			} else if kind == "job" {
				if body.State != "queued" && body.State != "running" {
					continue
				}
			} else if body.State != "pending" && body.State != "unknown" {
				continue
			}
			var schedule workSchedule
			if raw, ok := l.objects["work_schedule/"+kind+"."+k]; ok {
				if err := json.Unmarshal(raw, &schedule); err != nil {
					return nil, err
				}
			}
			if schedule.Next.After(now) {
				continue
			}
			items = append(items, workItem{k, body.Scope, raw, schedule})
		}
		sort.Slice(items, func(i, j int) bool {
			if items[i].Schedule.Last.Equal(items[j].Schedule.Last) {
				return items[i].Key < items[j].Key
			}
			return items[i].Schedule.Last.Before(items[j].Schedule.Last)
		})
		if len(items) > 100 {
			items = items[:100]
		}
		return items, nil
	}
	predicate := `o.body->>'State' IN ('pending','unknown')`
	join := ""
	if kind == "job" {
		predicate = `o.body->>'state' IN ('queued','running') AND o.body->>'kind'='replay'`
	}
	if kind == "incident" {
		predicate = `o.body->>'State'='contained' AND b.body->>'state'<>'completed'`
		join = " JOIN " + s.schema + ".objects b ON b.kind='incident_outbox' AND b.id=o.id"
	}
	rows, err := s.pool.Query(ctx, `SELECT o.id,o.body FROM `+s.schema+`.objects o`+join+` LEFT JOIN `+s.schema+`.objects c ON c.kind='work_schedule' AND c.id=$1||'.'||o.id WHERE o.kind=$1 AND `+predicate+` AND (c.id IS NULL OR (c.body->>'Next')::timestamptz<=clock_timestamp()) ORDER BY COALESCE((c.body->>'Last')::timestamptz,'-infinity'::timestamptz),o.id LIMIT 100`, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []workItem
	for rows.Next() {
		var item workItem
		if err = rows.Scan(&item.Key, &item.Body); err != nil {
			return nil, err
		}
		var body struct{ Scope string }
		if err = json.Unmarshal(item.Body, &body); err != nil {
			return nil, err
		}
		item.Scope = body.Scope
		items = append(items, item)
	}
	return items, rows.Err()
}
