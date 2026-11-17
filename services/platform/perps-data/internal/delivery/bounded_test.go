package delivery_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/delivery"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/store"
	"github.com/lib/pq"
)

func boundedDatabase(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv(testDatabaseEnv)
	if dsn == "" {
		t.Fatal("PostgreSQL required: " + testDatabaseEnv)
	}
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		var err error
		dsn, err = pq.ParseURL(dsn)
		if err != nil {
			t.Fatal(err)
		}
	}
	admin, err := store.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	name := "outbox_bounded_" + strings.ToLower(rand.Text())
	if _, err = admin.Exec("CREATE DATABASE " + pq.QuoteIdentifier(name)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec("DROP DATABASE " + pq.QuoteIdentifier(name)); err != nil {
			t.Error(err)
		}
		_ = admin.Close()
	})
	db, err := store.Open(dsn + " dbname=" + name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = store.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db
}

func boundedExec(t *testing.T, db *sql.DB, q string, args ...any) {
	t.Helper()
	if _, err := db.Exec(q, args...); err != nil {
		t.Fatal(err)
	}
}

func boundedPlan(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	var raw []byte
	if err = tx.QueryRow("EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) "+query, args...).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var plan any
	if err = json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	scans := 0
	bound := 1000.0
	if strings.Contains(query, "removed AS") {
		bound = 10000
	}
	var visit func(any)
	visit = func(v any) {
		switch x := v.(type) {
		case []any:
			for _, a := range x {
				visit(a)
			}
		case map[string]any:
			if x["Relation Name"] == "delivery_outbox" {
				scans++
				kind, _ := x["Node Type"].(string)
				if strings.Contains(kind, "Seq Scan") {
					t.Fatalf("outbox sequential scan: %s", raw)
				}
				rows, _ := x["Actual Rows"].(float64)
				filtered, _ := x["Rows Removed by Filter"].(float64)
				rechecked, _ := x["Rows Removed by Index Recheck"].(float64)
				loops, _ := x["Actual Loops"].(float64)
				if (rows+filtered+rechecked)*loops > bound {
					t.Fatalf("outbox read exceeds batch bound: %s", raw)
				}
			}
			for _, a := range x {
				visit(a)
			}
		}
	}
	visit(plan)
	if scans == 0 {
		t.Fatalf("plan does not read outbox: %s", raw)
	}
}

func TestOutboxBoundedClaimsAndRetention(t *testing.T) {
	db := boundedDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	s, err := delivery.NewStore(db, "perps", testPolicy())
	if err != nil {
		t.Fatal(err)
	}
	e := testEnvelope("bounded", "fill", 1)
	wire, err := e.Encode(testPolicy(), fillSubject)
	if err != nil {
		t.Fatal(err)
	}
	// Published rows precede the pending backlog in sequence order.
	boundedExec(t, db, `INSERT INTO perps.delivery_outbox
 (event_id,schema_version,subject,payload_hash,envelope,state,created_at,published_at,broker_sequence)
 SELECT 'bounded-'||g,1,$1,$2,$3,
 CASE WHEN g<=200000 THEN 'published' ELSE 'pending' END,
 statement_timestamp()-interval '2 hours',
 CASE WHEN g<=100000 THEN statement_timestamp()-interval '2 hours'
 WHEN g<=200000 THEN statement_timestamp()-interval '30 minutes' END,
 CASE WHEN g<=200000 THEN g END
 FROM generate_series(1,400000) g ORDER BY g`, fillSubject, e.PayloadHash, wire)
	boundedExec(t, db, `ANALYZE perps.delivery_outbox`)
	for _, relay := range []bool{false, true} {
		boundedPlan(t, db, s.ClaimStatement(relay), 1000, "plan-owner", 30000)
	}
	boundedPlan(t, db, s.HeadAgeStatement())
	boundedExec(t, db, `INSERT INTO perps.writer_lease(name,holder,expires_at) VALUES('indexer','writer',statement_timestamp()+interval '5 minutes')`)
	boundedPlan(t, db, s.PrunePublishedStatement(), "indexer", "writer")
	age, err := s.HeadAge(ctx)
	if err != nil || age < 2*time.Hour || age > 2*time.Hour+time.Minute {
		t.Fatalf("head age %s: %v", age, err)
	}
	leases, err := s.Claim(ctx, "bounded", 1000, 30*time.Second)
	if err != nil || len(leases) != 1000 {
		t.Fatalf("first batch: %d %v", len(leases), err)
	}
	for i, l := range leases {
		if l.PerpsSeq != int64(200001+i) || l.Fence != 1 || l.Attempts != 1 {
			t.Fatalf("lease %d: %+v", i, l)
		}
	}
	if err = s.ReleaseBatch(ctx, leases); err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		name, update string
		relay        bool
	}{
		{"leased", `lease_owner='other',lease_until=statement_timestamp()+interval '1 minute'`, false},
		{"backoff", `available_at=statement_timestamp()+interval '1 minute'`, false},
		{"relay_leased", `lease_owner='other',lease_until=statement_timestamp()+interval '1 minute'`, true},
		{"relay_backoff", `available_at=statement_timestamp()+interval '1 minute'`, true},
		{"transient", `state='quarantined',failure_class='transient',available_at=statement_timestamp()+interval '1 minute'`, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			boundedExec(t, db, `UPDATE perps.delivery_outbox SET `+c.update+` WHERE perps_seq=200501`)
			boundedPlan(t, db, s.ClaimStatement(c.relay), 1000, "plan-owner", 30000)
			var got []delivery.Lease
			if c.relay {
				got, err = s.RelayClaim(ctx, "bounded", 1000, 30*time.Second)
			} else {
				got, err = s.Claim(ctx, "bounded", 1000, 30*time.Second)
			}
			if err != nil || len(got) != 500 {
				t.Fatalf("barrier batch: %d %v", len(got), err)
			}
			for i, l := range got {
				if l.PerpsSeq != int64(200001+i) {
					t.Fatalf("barrier sequence %d", l.PerpsSeq)
				}
			}
			if err = s.ReleaseBatch(ctx, got); err != nil {
				t.Fatal(err)
			}
			boundedExec(t, db, `UPDATE perps.delivery_outbox SET state='pending',failure_class=NULL,lease_owner=NULL,lease_until=NULL,available_at=statement_timestamp() WHERE perps_seq=200501`)
		})
	}
	// Reclaims count expired leases and use the same instant for every row.
	boundedExec(t, db, `UPDATE perps.delivery_outbox SET lease_owner='expired',lease_until=statement_timestamp()-interval '1 second' WHERE perps_seq IN (200001,200002)`)
	leases, err = s.Claim(ctx, "bounded", 2, 30*time.Second)
	if err != nil || len(leases) != 2 {
		t.Fatalf("reclaim: %d %v", len(leases), err)
	}
	if n := count(t, db, `SELECT count(*) FROM perps.delivery_outbox WHERE perps_seq IN (200001,200002) AND lease_expiry_count=1 AND lease_until=claimed_at+interval '30 seconds'`); n != 2 {
		t.Fatalf("expired lease counters: %d", n)
	}
	if n := count(t, db, `SELECT count(DISTINCT claimed_at) FROM perps.delivery_outbox WHERE perps_seq IN (200001,200002)`); n != 1 {
		t.Fatalf("claim instants: %d", n)
	}
	stale := leases[0]
	stale.Fence--
	if err = s.MarkPublished(ctx, stale, delivery.PersistenceAck{Stream: "perps", Sequence: 1}); !errors.Is(err, delivery.ErrLease) {
		t.Fatalf("stale fence: %v", err)
	}
	if err = s.ReleaseBatch(ctx, leases); err != nil {
		t.Fatal(err)
	}

	t.Run("serialized_claims", func(t *testing.T) {
		var wg sync.WaitGroup
		batches := make(chan []delivery.Lease, 2)
		errs := make(chan error, 2)
		for _, owner := range []string{"replica-a", "replica-b"} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ls, err := s.Claim(ctx, owner, 100, 30*time.Second)
				batches <- ls
				errs <- err
			}()
		}
		wg.Wait()
		close(batches)
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}
		total := 0
		var got []delivery.Lease
		for ls := range batches {
			total += len(ls)
			if len(ls) > 0 {
				got = ls
			}
		}
		if total != 100 {
			t.Fatalf("concurrent claims leased %d rows", total)
		}
		if err = s.ReleaseBatch(ctx, got); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("retention", func(t *testing.T) {
		boundedExec(t, db, `UPDATE perps.delivery_outbox SET lease_owner='active',lease_until=statement_timestamp()+interval '1 minute' WHERE perps_seq=200002`)
		boundedExec(t, db, `UPDATE perps.delivery_outbox SET state='quarantined',failure_class='transient' WHERE perps_seq=200003`)
		boundedExec(t, db, `UPDATE perps.delivery_outbox SET state='quarantined',failure_class='exhausted',broker_sequence=7 WHERE perps_seq=200004`)
		boundedExec(t, db, `UPDATE perps.delivery_outbox SET broker_sequence=NULL WHERE perps_seq=1`)
		if n, err := s.PrunePublished(ctx, "indexer", "other"); !errors.Is(err, delivery.ErrLease) || n != 0 {
			t.Fatalf("unheld writer cleanup: %d %v", n, err)
		}
		total, passes := int64(0), 0
		for {
			n, err := s.PrunePublished(ctx, "indexer", "writer")
			if err != nil {
				t.Fatal(err)
			}
			if n < 0 || n > 10000 {
				t.Fatalf("cleanup size %d", n)
			}
			total += n
			passes++
			if n < 10000 {
				break
			}
		}
		if total != 99999 || passes != 10 {
			t.Fatalf("cleanup removed %d in %d statements", total, passes)
		}
		if n := count(t, db, `SELECT count(*) FROM perps.delivery_outbox WHERE state='published' AND published_at<statement_timestamp()-interval '1 hour'`); n != 1 {
			t.Fatalf("unacknowledged published rows: %d", n)
		}
		if n := count(t, db, `SELECT count(*) FROM perps.delivery_outbox WHERE state='published' AND published_at>=statement_timestamp()-interval '1 hour'`); n != 100000 {
			t.Fatalf("young published rows: %d", n)
		}
		if n := count(t, db, `SELECT count(*) FROM perps.delivery_outbox WHERE state='pending'`); n != 199998 {
			t.Fatalf("pending rows: %d", n)
		}
		if n := count(t, db, `SELECT count(*) FROM perps.delivery_outbox WHERE state='quarantined'`); n != 2 {
			t.Fatalf("quarantined rows: %d", n)
		}
		if n := count(t, db, `SELECT count(*) FROM perps.delivery_outbox WHERE perps_seq=200002 AND lease_owner='active'`); n != 1 {
			t.Fatalf("leased rows: %d", n)
		}
		// A producer write of a removed identity gets a new pending sequence.
		old := testEnvelope("bounded-2", "fill", 1)
		enqueueCommitted(t, ctx, db, s, fillSubject, old)
		var seq int64
		if err = db.QueryRow(`SELECT perps_seq FROM perps.delivery_outbox WHERE event_id='bounded-2' AND state='pending'`).Scan(&seq); err != nil || seq <= 400000 {
			t.Fatalf("old identity sequence %d: %v", seq, err)
		}
		boundedExec(t, db, `UPDATE perps.writer_lease SET expires_at=statement_timestamp()-interval '1 second' WHERE name='indexer'`)
		if _, err = s.PrunePublished(ctx, "indexer", "writer"); !errors.Is(err, delivery.ErrLease) {
			t.Fatalf("expired writer cleanup: %v", err)
		}
	})
}

func TestOutboxBoundedRelayControlsAndBudget(t *testing.T) {
	db := boundedDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	s, err := delivery.NewStore(db, "perps", testPolicy())
	if err != nil {
		t.Fatal(err)
	}
	enqueueCommitted(t, ctx, db, s, fillSubject, testEnvelope("control-head", "fill", 1), testEnvelope("budget-head", "fill", 2), testEnvelope("after-budget", "fill", 3))
	boundedExec(t, db, `UPDATE perps.delivery_outbox SET state='quarantined',failure_class='schema_violation' WHERE event_id='control-head'`)
	boundedExec(t, db, `UPDATE perps.delivery_outbox SET attempts=2 WHERE event_id='budget-head'`)
	js := startBroker(t, ctx)
	relay := delivery.Relay{Store: s, Publisher: jetStreamPublisher{js: js, stream: "PERPS_V1"}, Owner: "bounded", Batch: 100,
		LeaseDuration: 30 * time.Second, PublishTimeout: 5 * time.Second, RetryDelay: time.Second, MaxAttempts: 2}
	claimed, err := relay.PassCount(ctx)
	if err != nil || claimed != 3 {
		t.Fatalf("budget pass: %d %v", claimed, err)
	}
	if r := readOutbox(t, db, "control-head"); r.state != "quarantined" || r.brokerSequence != 1 {
		t.Fatalf("control lease: %+v", r)
	}
	if r := readOutbox(t, db, "budget-head"); r.state != "quarantined" || r.failureClass != "exhausted" || r.attempts != 3 || r.brokerSequence != 0 {
		t.Fatalf("durable budget: %+v", r)
	}
	if r := readOutbox(t, db, "after-budget"); r.state != "pending" || r.attempts != 0 || r.brokerSequence != 0 {
		t.Fatalf("held successor: %+v", r)
	}
	claimed, err = relay.PassCount(ctx)
	if err != nil || claimed != 2 {
		t.Fatalf("control recovery pass: %d %v", claimed, err)
	}
	if r := readOutbox(t, db, "budget-head"); r.state != "quarantined" || r.brokerSequence != 2 {
		t.Fatalf("exhausted control: %+v", r)
	}
	if r := readOutbox(t, db, "after-budget"); r.state != "published" || r.brokerSequence != 3 || r.attempts != 1 {
		t.Fatalf("successor publication: %+v", r)
	}
	if n, err := s.Quarantined(ctx); err != nil || n != 0 {
		t.Fatalf("resolved quarantine: %d %v", n, err)
	}
	if age, err := s.HeadAge(ctx); err != nil || age != 0 {
		t.Fatalf("resolved head: %s %v", age, err)
	}
}
