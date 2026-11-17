package indexer

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/delivery"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/store"
	"github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
)

func boundedIndexerDB(t *testing.T) *sql.DB {
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
	name := "indexer_bounded_" + strings.ToLower(rand.Text())
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

func TestOutboxBoundedReadinessAndWriter(t *testing.T) {
	db := boundedIndexerDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if _, err := db.ExecContext(ctx, `INSERT INTO perps.delivery_outbox
 (event_id,schema_version,subject,payload_hash,envelope,state,created_at,published_at,broker_sequence)
 SELECT 'readiness-'||g,1,'perps.125.chain.block','',decode('7b7d','hex'),
 CASE WHEN g<=200000 THEN 'published' ELSE 'pending' END,
 statement_timestamp()-interval '2 hours',
 CASE WHEN g<=200000 THEN statement_timestamp()-interval '2 hours' END,
 CASE WHEN g<=200000 THEN g END
 FROM generate_series(1,400000) g ORDER BY g`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `ANALYZE perps.delivery_outbox`); err != nil {
		t.Fatal(err)
	}
	var raw []byte
	if err := db.QueryRowContext(ctx, "EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) "+outboxSQL).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var plan any
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	scans := 0
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
				if strings.Contains(x["Node Type"].(string), "Seq Scan") {
					t.Fatalf("readiness scans outbox sequentially: %s", raw)
				}
			}
			for _, a := range x {
				visit(a)
			}
		}
	}
	visit(plan)
	if scans < 3 {
		t.Fatalf("missing indexed head, tail or age read: %s", raw)
	}
	outbox, err := delivery.NewStore(db, "perps", store.Policy(125))
	if err != nil {
		t.Fatal(err)
	}
	ix := &Indexer{db: db, outbox: outbox, holder: "writer", logger: slog.Default(), outboxPending: prometheus.NewGauge(prometheus.GaugeOpts{Name: "bounded_pending"})}
	span, age, err := ix.observeOutbox(ctx)
	if err != nil || span != 200000 || age < 2*time.Hour || age > 2*time.Hour+time.Minute {
		t.Fatalf("backlog span=%d age=%s: %v", span, age, err)
	}
	// Gaps and resolved rows within the span count toward its estimate.
	if _, err = db.ExecContext(ctx, `DELETE FROM perps.delivery_outbox WHERE perps_seq=200002;
 UPDATE perps.delivery_outbox SET state='published',published_at=statement_timestamp(),broker_sequence=9 WHERE perps_seq=200003`); err != nil {
		t.Fatal(err)
	}
	span, _, err = ix.observeOutbox(ctx)
	if err != nil || span != 200000 {
		t.Fatalf("sequence span with gap: %d %v", span, err)
	}
	// The maintenance loop refuses a process without the live writer lease.
	if err = ix.pruneOutbox(ctx); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("maintenance without lease: %v", err)
	}
	var n int
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM perps.delivery_outbox WHERE perps_seq<=200000`).Scan(&n); err != nil || n != 200000 {
		t.Fatalf("nonwriter removed rows: %d %v", n, err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO perps.writer_lease(name,holder,expires_at) VALUES('indexer','writer',statement_timestamp()+interval '1 minute')`); err != nil {
		t.Fatal(err)
	}
	runCtx, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- ix.pruneOutbox(runCtx) }()
	defer func() {
		stop()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Errorf("maintenance stop: %v", err)
		}
	}()
	for {
		if err = db.QueryRowContext(ctx, `SELECT count(*) FROM perps.delivery_outbox WHERE perps_seq<=200000`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n == 190000 {
			break
		}
		if n < 190000 {
			t.Fatalf("maintenance removed more than one batch: %d", n)
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(5 * time.Millisecond):
		}
	}
	time.Sleep(150 * time.Millisecond)
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM perps.delivery_outbox WHERE perps_seq<=200000`).Scan(&n); err != nil || n != 190000 {
		t.Fatalf("maintenance ignored interval: %d %v", n, err)
	}
	if _, err = db.ExecContext(ctx, `UPDATE perps.delivery_outbox SET state='published',published_at=statement_timestamp(),broker_sequence=1 WHERE state='pending'`); err != nil {
		t.Fatal(err)
	}
	span, age, err = ix.observeOutbox(ctx)
	if err != nil || span != 0 || age != 0 {
		t.Fatalf("drained backlog: %d %s %v", span, age, err)
	}
}
