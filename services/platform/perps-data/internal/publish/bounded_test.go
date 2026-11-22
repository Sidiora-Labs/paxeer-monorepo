package publish

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/delivery"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/store"
	"github.com/prometheus/client_golang/prometheus"
)

// boundedPublisher acknowledges each ordered batch and records the stream
// position that the real relay must persist before the next claim.
type boundedPublisher struct {
	next    int64
	batches chan int
}

func (b *boundedPublisher) PublishBatch(ctx context.Context, leases []delivery.Lease) []delivery.Publication {
	out := make([]delivery.Publication, len(leases))
	for i, l := range leases {
		if l.PerpsSeq != b.next || l.Control {
			out[i].Err = errors.New("publication sequence is not contiguous")
			return out
		}
		b.next++
		out[i].Ack = delivery.PersistenceAck{Stream: "PERPS_V1", Sequence: uint64(b.next - 200001)}
	}
	b.batches <- len(leases)
	return out
}

func TestOutboxBoundedPublisherPace(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	db, err := store.Open(testDatabase(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = store.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	policy := testPolicy(t)
	outbox, err := delivery.NewStore(db, "perps", policy)
	if err != nil {
		t.Fatal(err)
	}
	e := fillEvent(t, 1, hashA, 0, 0)
	wire := wireOf(t, policy, e)
	if _, err = db.ExecContext(ctx, `INSERT INTO perps.delivery_outbox
 (event_id,schema_version,subject,payload_hash,envelope,state,published_at,broker_sequence)
 SELECT 'pace-'||g,1,$1,$2,
 convert_to(replace($3::text,$4::text,'"event_id":"pace-'||g||'"'),'UTF8'),
 CASE WHEN g<=200000 THEN 'published' ELSE 'pending' END,
 CASE WHEN g<=200000 THEN statement_timestamp()-interval '2 hours' END,
 CASE WHEN g<=200000 THEN g END
 FROM generate_series(1,400000) g ORDER BY g`, e.subject, e.env.PayloadHash, string(wire), `"event_id":"`+e.env.EventID+`"`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `ANALYZE perps.delivery_outbox;
 UPDATE perps.delivery_outbox SET available_at=statement_timestamp()+interval '1 hour' WHERE perps_seq=220004`); err != nil {
		t.Fatal(err)
	}
	var first []byte
	if err = db.QueryRowContext(ctx, `SELECT envelope FROM perps.delivery_outbox WHERE perps_seq=200001`).Scan(&first); err != nil {
		t.Fatal(err)
	}
	decoded, err := delivery.Decode(first, policy, e.subject)
	if err != nil || decoded.EventID != "pace-200001" || decoded.PayloadHash != e.env.PayloadHash || string(decoded.Payload) != string(e.env.Payload) {
		t.Fatalf("bulk envelope changed identity or payload: %v", err)
	}
	broker := &boundedPublisher{next: 200001, batches: make(chan int, 25)}
	p := &Publisher{relay: delivery.Relay{Store: outbox, Publisher: broker, Owner: "pace", Batch: 1000, MaxAttempts: 3,
		LeaseDuration: 30 * time.Second, PublishTimeout: 5 * time.Second, RetryDelay: time.Second},
		idle: time.Minute, logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		brokerDown: prometheus.NewGauge(prometheus.GaugeOpts{Name: "bounded_broker_down"})}
	runCtx, stop := context.WithTimeout(ctx, 45*time.Second)
	done := make(chan error, 1)
	go func() { done <- p.Run(runCtx) }()
	defer func() {
		stop()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Errorf("publisher stop: %v", err)
		}
	}()
	for pass := 0; pass < 21; pass++ {
		select {
		case n := <-broker.batches:
			want := 1000
			if pass == 20 {
				want = 3
			}
			if n != want {
				t.Fatalf("pass %d claimed %d, want %d", pass, n, want)
			}
		case <-runCtx.Done():
			t.Fatalf("full batches waited the idle interval at pass %d: %v", pass, runCtx.Err())
		}
	}
	// Await the real acknowledgement write after the short final batch.
	for {
		if count(t, db, `SELECT count(*) FROM perps.delivery_outbox WHERE perps_seq=220003 AND state='published'`) == 1 {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(time.Millisecond):
		}
	}
	if n := count(t, db, `SELECT count(*) FROM perps.delivery_outbox WHERE perps_seq BETWEEN 200001 AND 220003
 AND state='published' AND broker_sequence=perps_seq-200000 AND attempts=1`); n != 20003 {
		t.Fatalf("ordered acknowledged rows: %d", n)
	}
	if _, err = db.ExecContext(ctx, `UPDATE perps.delivery_outbox SET available_at=statement_timestamp() WHERE perps_seq=220004`); err != nil {
		t.Fatal(err)
	}
	select {
	case n := <-broker.batches:
		t.Fatalf("short batch did not wait idle interval: next size %d", n)
	case <-time.After(150 * time.Millisecond):
	}
}
