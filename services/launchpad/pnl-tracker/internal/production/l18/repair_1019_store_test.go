package l18

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// Committed in-memory peer records cross connection restarts. Transactions
// snapshot all durable records together; rollback restores every affected table.
// Production pgx and delivery.Store still own wire encoding and SQL execution.
type repair1019Local struct {
	Sequence                 int64
	Identity, Topic, Payload string
	Published                bool
}
type repair1019Delivery struct {
	ID, Subject, Wire, Owner, State string
	Fence                           int64
	Attempts                        int
	Until                           time.Time
}
type repair1019Durable struct {
	Local    []repair1019Local
	Delivery map[string]repair1019Delivery
	Inbox    map[string]string
	Effects  map[string]string
}
type repair1019Store struct {
	db          *repair62Database
	state       repair1019Durable
	saved       *repair1019Durable
	failHandoff bool
}

func repair1019NewStore(db *repair62Database) *repair1019Store {
	return &repair1019Store{db: db, state: repair1019Durable{Delivery: map[string]repair1019Delivery{}, Inbox: map[string]string{}, Effects: map[string]string{}}}
}

func (s *repair1019Store) query(sql string, args []string) (repair62Result, bool, error) {
	sql = strings.ReplaceAll(sql, `"pnl".`, "")
	ok := repair62Result{tag: "UPDATE 1"}
	insert := repair62Result{tag: "INSERT 0 1"}
	errResult := func(err error) (repair62Result, bool, error) { return ok, true, err }
	lease := func() (repair1019Delivery, error) {
		r, exists := s.state.Delivery[args[0]]
		if !exists || r.Owner != args[1] || strconv.FormatInt(r.Fence, 10) != args[2] || r.State != "pending" || !r.Until.After(time.Now()) {
			return r, &pgconn.PgError{Code: "P0001", Message: "stale delivery lease"}
		}
		return r, nil
	}
	switch {
	case sql == "begin":
		raw, err := json.Marshal(s.state)
		if err != nil {
			return errResult(err)
		}
		var saved repair1019Durable
		if err = json.Unmarshal(raw, &saved); err != nil {
			return errResult(err)
		}
		s.saved = &saved
		return repair62Result{tag: "BEGIN"}, true, nil
	case sql == "rollback":
		if s.saved != nil {
			s.state = *s.saved
			s.saved = nil
		}
		return repair62Result{tag: "ROLLBACK"}, true, nil
	case sql == "commit":
		s.saved = nil
		return repair62Result{tag: "COMMIT"}, true, nil
	case strings.HasPrefix(sql, "SELECT has_column_privilege("):
		permitted := s.db.permit("pnl.l18_outbox", "UPDATE", "published_at") == nil
		return repair62Rows([]uint32{16}, []string{strconv.FormatBool(permitted)}), true, nil
	case strings.HasPrefix(sql, "INSERT INTO l18_outbox("):
		for _, old := range s.state.Local {
			if old.Identity == args[0] {
				return errResult(&pgconn.PgError{Code: "23505", Message: "duplicate local identity"})
			}
		}
		s.state.Local = append(s.state.Local, repair1019Local{Sequence: int64(len(s.state.Local) + 1), Identity: args[0], Topic: args[1], Payload: args[2]})
		return insert, true, nil
	case strings.HasPrefix(sql, "SELECT sequence,identity,topic,payload FROM l18_outbox"):
		if err := repair62Require(sql, "published_at IS NULL", "ORDER BY sequence LIMIT $1 FOR UPDATE SKIP LOCKED"); err != nil {
			return errResult(err)
		}
		if err := s.db.permit("pnl.l18_outbox", "UPDATE", "published_at"); err != nil {
			return errResult(err)
		}
		result := repair62Rows([]uint32{20, 25, 25, 3802})
		for _, row := range s.state.Local {
			if !row.Published && len(result.rows) < int(repair62Uint(args[0])) {
				result.rows = append(result.rows, []string{strconv.FormatInt(row.Sequence, 10), row.Identity, row.Topic, row.Payload})
			}
		}
		result.tag = fmt.Sprintf("SELECT %d", len(result.rows))
		return result, true, nil
	case strings.HasPrefix(sql, "UPDATE l18_outbox SET published_at="):
		if s.failHandoff {
			return errResult(&pgconn.PgError{Code: "P0001", Message: "handoff interrupted"})
		}
		for i, row := range s.state.Local {
			if strconv.FormatInt(row.Sequence, 10) == args[0] && !row.Published {
				s.state.Local[i].Published = true
				return ok, true, nil
			}
		}
		return repair62Result{tag: "UPDATE 0"}, true, nil
	case strings.HasPrefix(sql, "INSERT INTO delivery_outbox("):
		if _, found := s.state.Delivery[args[0]]; found {
			return repair62Result{tag: "INSERT 0 0"}, true, nil
		}
		s.state.Delivery[args[0]] = repair1019Delivery{ID: args[0], Subject: args[2], Wire: args[4], State: "pending"}
		return insert, true, nil
	case strings.HasPrefix(sql, "SELECT subject,envelope FROM delivery_outbox"):
		r, found := s.state.Delivery[args[0]]
		if !found {
			return repair62Rows([]uint32{25, 17}), true, nil
		}
		return repair62Rows([]uint32{25, 17}, []string{r.Subject, r.Wire}), true, nil
	case strings.HasPrefix(sql, "WITH picked AS (SELECT event_id FROM delivery_outbox"):
		if err := repair62Require(sql, "FOR UPDATE SKIP LOCKED LIMIT $1", "o.fence+1", "o.attempts+1"); err != nil {
			return errResult(err)
		}
		result := repair62Rows([]uint32{25, 25, 17, 20, 23})
		for id, row := range s.state.Delivery {
			if row.State != "pending" || row.Until.After(time.Now()) {
				continue
			}
			row.Owner, row.Fence, row.Attempts = args[1], row.Fence+1, row.Attempts+1
			row.Until = time.Now().Add(time.Duration(repair62Uint(args[2])) * time.Millisecond)
			s.state.Delivery[id] = row
			result.rows = append(result.rows, []string{id, row.Subject, row.Wire, strconv.FormatInt(row.Fence, 10), strconv.Itoa(row.Attempts)})
			if len(result.rows) == int(repair62Uint(args[0])) {
				break
			}
		}
		result.tag = fmt.Sprintf("UPDATE %d", len(result.rows))
		return result, true, nil
	case strings.HasPrefix(sql, "UPDATE delivery_outbox SET state=CASE WHEN attempts>"):
		r, err := lease()
		if err != nil {
			return errResult(err)
		}
		exhausted := r.Attempts > int(repair62Uint(args[3]))
		if exhausted {
			r.State = "quarantined"
			s.state.Delivery[r.ID] = r
		}
		return repair62Rows([]uint32{16}, []string{strconv.FormatBool(exhausted)}), true, nil
	case strings.HasPrefix(sql, "UPDATE delivery_outbox SET acknowledgement_latency_ms="):
		r, err := lease()
		if err != nil {
			return errResult(err)
		}
		if args[3] != "L18" || args[4] != "1" {
			return errResult(fmt.Errorf("wrong persistence acknowledgement"))
		}
		r.State, r.Owner = "published", ""
		s.state.Delivery[r.ID] = r
		return ok, true, nil
	case strings.HasPrefix(sql, "UPDATE delivery_outbox SET lease_until="):
		r := s.state.Delivery[args[0]]
		r.Until = time.Now().Add(-time.Second)
		s.state.Delivery[r.ID] = r
		return ok, true, nil
	case strings.HasPrefix(sql, "INSERT INTO delivery_inbox("):
		key := strings.Join(args[:3], ":")
		if _, found := s.state.Inbox[key]; found {
			return repair62Result{tag: "INSERT 0 0"}, true, nil
		}
		s.state.Inbox[key] = args[3]
		return insert, true, nil
	case strings.HasPrefix(sql, "SELECT envelope_hash FROM delivery_inbox"):
		return repair62Text([]string{s.state.Inbox[strings.Join(args[:3], ":")]}), true, nil
	case strings.HasPrefix(sql, "INSERT INTO repair1019_effects("):
		if _, found := s.state.Effects[args[0]]; found {
			return errResult(&pgconn.PgError{Code: "23505", Message: "duplicate consumer effect"})
		}
		s.state.Effects[args[0]] = args[1]
		return insert, true, nil
	case sql == "SELECT payload::text FROM repair1019_effects":
		for _, value := range s.state.Effects {
			return repair62Text([]string{value}), true, nil
		}
		return repair62Rows([]uint32{25}), true, nil
	case strings.HasPrefix(sql, "SELECT count(*)::text FROM "):
		count := 0
		switch {
		case strings.Contains(sql, "repair1019_effects"):
			count = len(s.state.Effects)
		case strings.Contains(sql, "delivery_inbox"):
			count = len(s.state.Inbox)
		case strings.Contains(sql, "delivery_outbox"):
			for _, r := range s.state.Delivery {
				if !strings.Contains(sql, "WHERE") || r.State == "published" && r.Attempts == 2 {
					count++
				}
			}
		case strings.Contains(sql, "l18_outbox"):
			for _, r := range s.state.Local {
				if !strings.Contains(sql, "WHERE") || strings.Contains(sql, "IS NULL") && !r.Published || strings.Contains(sql, "IS NOT NULL") && r.Published {
					count++
				}
			}
		default:
			return errResult(fmt.Errorf("count absent from committed interface: %s", sql))
		}
		return repair62Text([]string{strconv.Itoa(count)}), true, nil
	default:
		return repair62Result{}, false, nil
	}
}

func repair1019Signature(sql string) ([]uint32, []uint32, bool) {
	sql = strings.ReplaceAll(sql, `"pnl".`, "")
	switch {
	case strings.HasPrefix(sql, "SELECT has_column_privilege("):
		return []uint32{25}, []uint32{16}, true
	case strings.HasPrefix(sql, "SELECT sequence,identity,topic,payload FROM l18_outbox"):
		return []uint32{20}, []uint32{20, 25, 25, 3802}, true
	case strings.HasPrefix(sql, "UPDATE l18_outbox SET published_at="):
		return []uint32{20}, nil, true
	case strings.HasPrefix(sql, "INSERT INTO delivery_outbox("):
		return []uint32{25, 23, 25, 25, 17}, nil, true
	case strings.HasPrefix(sql, "SELECT subject,envelope FROM delivery_outbox"):
		return []uint32{25}, []uint32{25, 17}, true
	case strings.HasPrefix(sql, "WITH picked AS (SELECT event_id FROM delivery_outbox"):
		return []uint32{23, 25, 20}, []uint32{25, 25, 17, 20, 23}, true
	case strings.HasPrefix(sql, "UPDATE delivery_outbox SET state=CASE WHEN attempts>"):
		return []uint32{25, 25, 20, 23}, []uint32{16}, true
	case strings.HasPrefix(sql, "UPDATE delivery_outbox SET acknowledgement_latency_ms="):
		return []uint32{25, 25, 20, 25, 20}, nil, true
	case strings.HasPrefix(sql, "UPDATE delivery_outbox SET lease_until="):
		return []uint32{25}, nil, true
	case strings.HasPrefix(sql, "INSERT INTO delivery_inbox("):
		return []uint32{25, 25, 25, 25}, nil, true
	case strings.HasPrefix(sql, "SELECT envelope_hash FROM delivery_inbox"):
		return []uint32{25, 25, 25}, []uint32{25}, true
	case strings.HasPrefix(sql, "INSERT INTO repair1019_effects("):
		return []uint32{25, 3802}, nil, true
	case strings.HasPrefix(sql, "SELECT count(*)::text FROM "), sql == "SELECT payload::text FROM repair1019_effects":
		return nil, []uint32{25}, true
	default:
		return nil, nil, false
	}
}
