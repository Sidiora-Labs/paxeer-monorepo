package delivery

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nats-io/nats.go"
)

func TestConsumerTransactionsRequireConsistentInboxAndQuarantine(t *testing.T) {
	ctx, pool, _, table := orderedPostgres(t)
	store, err := NewStore(pool, os.Getenv("DELIVERY_TEST_SCHEMA")+"_ordered", fixturePolicy())
	if err != nil {
		t.Fatal(err)
	}
	connection := orderedBroker(t)
	broker, err := connection.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := broker.AddStream(&nats.StreamConfig{Name: "CONSUMER_CATALOG", Subjects: []string{fixtureSubject}}); err != nil {
		t.Fatal(err)
	}
	sub, err := broker.PullSubscribe(fixtureSubject, "catalog", nats.BindStream("CONSUMER_CATALOG"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := sub.Unsubscribe(); err != nil {
			t.Error(err)
		}
	}()
	envelope := fixtureEnvelope("catalog_consumer")
	wire, err := envelope.Encode(fixturePolicy(), fixtureSubject)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := broker.Publish(fixtureSubject, wire); err != nil {
		t.Fatal(err)
	}
	messages, err := sub.Fetch(1, nats.MaxWait(time.Second))
	if err != nil || len(messages) != 1 {
		t.Fatalf("broker message: %d %v", len(messages), err)
	}
	message := orderedBrokerMessage{messages[0]}
	for _, mode := range []string{"cancelled", "inbox unavailable", "effect unavailable", "inbox identity unavailable", "identity conflict", "quarantine unavailable", "unauthorized subject", "invalid digest"} {
		t.Run(mode, func(t *testing.T) {
			consumer := "catalog_" + strings.ReplaceAll(mode, " ", "_")
			input, subject := wire, fixtureSubject
			bounded := ctx
			effect := func(ctx context.Context, tx pgx.Tx, _ Envelope) error { _, err := tx.Exec(ctx, "SELECT 1"); return err }
			switch mode {
			case "cancelled":
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				bounded = cancelled
			case "inbox unavailable":
				if _, err := pool.Exec(ctx, "ALTER TABLE "+table+".delivery_inbox RENAME TO unavailable_inbox"); err != nil {
					t.Fatal(err)
				}
				defer func() {
					if _, err := pool.Exec(ctx, "ALTER TABLE "+table+".unavailable_inbox RENAME TO delivery_inbox"); err != nil {
						t.Error(err)
					}
				}()
			case "effect unavailable":
				effect = func(ctx context.Context, tx pgx.Tx, _ Envelope) error {
					_, err := tx.Exec(ctx, "SELECT * FROM "+table+".unavailable_effect")
					return err
				}
			case "inbox identity unavailable", "identity conflict":
				if _, err := pool.Exec(ctx, "INSERT INTO "+table+".delivery_inbox(consumer_id,projection_generation,event_id,envelope_hash) VALUES($1,'g1',$2,$3)", consumer, envelope.EventID, strings.Repeat("a", 64)); err != nil {
					t.Fatal(err)
				}
				if mode == "inbox identity unavailable" {
					if _, err := pool.Exec(ctx, "ALTER TABLE "+table+".delivery_inbox ALTER COLUMN envelope_hash DROP NOT NULL; UPDATE "+table+".delivery_inbox SET envelope_hash=NULL"); err != nil {
						t.Fatal(err)
					}
					defer func() {
						if _, err := pool.Exec(ctx, "DELETE FROM "+table+".delivery_inbox WHERE envelope_hash IS NULL; ALTER TABLE "+table+".delivery_inbox ALTER COLUMN envelope_hash SET NOT NULL"); err != nil {
							t.Error(err)
						}
					}()
				}
			case "quarantine unavailable":
				input = []byte(`{}`)
				if _, err := pool.Exec(ctx, "ALTER TABLE "+table+".delivery_quarantine RENAME TO unavailable_quarantine"); err != nil {
					t.Fatal(err)
				}
				defer func() {
					if _, err := pool.Exec(ctx, "ALTER TABLE "+table+".unavailable_quarantine RENAME TO delivery_quarantine"); err != nil {
						t.Error(err)
					}
				}()
			case "unauthorized subject":
				subject = "unauthorized.event"
			case "invalid digest":
				candidate := envelope
				candidate.PayloadHash = strings.Repeat("b", 64)
				input, err = candidate.encodeBytes()
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := store.Consume(bounded, consumer, "g1", subject, input, message, effect); err == nil {
				t.Fatal("unavailable consumer authority admitted")
			}
			if mode != "inbox unavailable" && mode != "inbox identity unavailable" && mode != "identity conflict" {
				var count int
				if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+table+".delivery_inbox WHERE consumer_id=$1", consumer).Scan(&count); err != nil || count != 0 {
					t.Fatalf("refused inbox: %d %v", count, err)
				}
			}
		})
	}
	for _, mode := range []string{"changed subject", "changed class", "changed envelope", "unavailable class"} {
		t.Run(mode, func(t *testing.T) {
			consumer := "evidence_" + strings.ReplaceAll(mode, " ", "_")
			invalid := []byte(`{}`)
			if err := store.quarantineConsumer(ctx, consumer, "g1", fixtureSubject, invalid, SchemaViolation); err != nil {
				t.Fatal(err)
			}
			var mutation string
			switch mode {
			case "changed subject":
				mutation = "subject='other.event'"
			case "changed class":
				mutation = "failure_class='transient'"
			case "changed envelope":
				mutation = "envelope=convert_to('[]','UTF8')"
			case "unavailable class":
				if _, err := pool.Exec(ctx, "ALTER TABLE "+table+".delivery_quarantine ALTER COLUMN failure_class DROP NOT NULL"); err != nil {
					t.Fatal(err)
				}
				mutation = "failure_class=NULL"
				defer func() {
					if _, err := pool.Exec(ctx, "DELETE FROM "+table+".delivery_quarantine WHERE failure_class IS NULL; ALTER TABLE "+table+".delivery_quarantine ALTER COLUMN failure_class SET NOT NULL"); err != nil {
						t.Error(err)
					}
				}()
			}
			if _, err := pool.Exec(ctx, "UPDATE "+table+".delivery_quarantine SET "+mutation+" WHERE consumer_id=$1", consumer); err != nil {
				t.Fatal(err)
			}
			if err := store.quarantineConsumer(ctx, consumer, "g1", fixtureSubject, invalid, SchemaViolation); err == nil {
				t.Fatal("inconsistent quarantine admitted")
			}
		})
	}
	info, err := sub.ConsumerInfo()
	if err != nil || info.NumAckPending != 1 {
		t.Fatalf("refused delivery acknowledgement: %+v %v", info, err)
	}
	if err := store.Consume(ctx, "catalog_success", "g1", fixtureSubject, wire, message, func(ctx context.Context, tx pgx.Tx, _ Envelope) error { _, err := tx.Exec(ctx, "SELECT 1"); return err }); err != nil {
		t.Fatal(err)
	}
	if err := store.quarantineConsumer(ctx, "consumer", "g1", strings.Repeat("x", 257), nil, SchemaViolation); err == nil {
		t.Fatal("unbounded subject evidence admitted")
	}
}
