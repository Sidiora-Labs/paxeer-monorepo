package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

type persistedRelayPublisher struct{ broker nats.JetStreamContext }

func (p persistedRelayPublisher) Publish(ctx context.Context, subject, eventID string, wire []byte) (PersistenceAck, error) {
	ack, err := p.broker.Publish(subject, wire, nats.Context(ctx), nats.MsgId(eventID))
	if err != nil {
		return PersistenceAck{}, err
	}
	return PersistenceAck{Stream: ack.Stream, Sequence: ack.Sequence}, nil
}

func TestRelayPublishesOnlyValidatedDurableEnvelopes(t *testing.T) {
	connection := orderedBroker(t)
	broker, err := connection.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := broker.AddStream(&nats.StreamConfig{Name: "RELAY_PERSISTENCE", Subjects: []string{fixtureSubject}}); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"persisted", "empty", "attempt limit", "schema", "digest", "unavailable subject", "cancelled budget"} {
		t.Run(mode, func(t *testing.T) {
			ctx, pool, _, _ := orderedPostgres(t)
			store, err := NewStore(pool, os.Getenv("DELIVERY_TEST_SCHEMA")+"_ordered", fixturePolicy())
			if err != nil {
				t.Fatal(err)
			}
			relay := Relay{Store: store, Publisher: persistedRelayPublisher{broker}, Owner: "relay", Batch: 2, LeaseDuration: time.Minute, PublishTimeout: time.Second, RetryDelay: time.Second, MaxAttempts: 3}
			envelope := fixtureEnvelope("relay_" + Hash([]byte(mode))[:12])
			subject := fixtureSubject
			policy := fixturePolicy()
			if mode == "unavailable subject" {
				subject = "relay.unavailable"
				policy.Subjects[subject] = true
				store.policy = policy
			}
			wire, err := envelope.Encode(policy, subject)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "schema" {
				envelope.SchemaVersion = 2
				wire, err = json.Marshal(envelope)
			}
			if mode == "digest" {
				envelope.PayloadHash = Hash([]byte("another payload"))
				wire, err = json.Marshal(envelope)
			}
			if err != nil {
				t.Fatal(err)
			}
			attempts := 0
			if mode == "attempt limit" {
				attempts = 3
			}
			if mode != "empty" {
				if _, err := pool.Exec(ctx, "INSERT INTO "+store.table("delivery_outbox")+"(event_id,schema_version,subject,payload_hash,envelope,attempts) VALUES($1,$2,$3,$4,$5,$6)", envelope.EventID, envelope.SchemaVersion, subject, envelope.PayloadHash, wire, attempts); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "cancelled budget" {
				budget, err := NewWorkBudget(1, 1, 1)
				if err != nil {
					t.Fatal(err)
				}
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				relay.Budget = budget
				if err := relay.RunOnce(cancelled); !errors.Is(err, context.Canceled) {
					t.Fatalf("cancelled relay: %v", err)
				}
				return
			}
			err = relay.RunOnce(ctx)
			switch mode {
			case "schema":
				if !errors.Is(err, ErrSchema) {
					t.Fatalf("schema admission: %v", err)
				}
			case "digest":
				if !errors.Is(err, ErrIntegrity) {
					t.Fatalf("digest admission: %v", err)
				}
			case "unavailable subject":
				if err == nil {
					t.Fatal("publication without a stream admitted")
				}
			default:
				if err != nil {
					t.Fatal(err)
				}
			}
			if mode == "empty" {
				return
			}
			var state string
			var sequence *int64
			if err := pool.QueryRow(ctx, "SELECT state,broker_sequence FROM "+store.table("delivery_outbox")+" WHERE event_id=$1", envelope.EventID).Scan(&state, &sequence); err != nil {
				t.Fatal(err)
			}
			want := "quarantined"
			if mode == "persisted" {
				want = "published"
				if sequence == nil || *sequence < 1 {
					t.Fatal("persisted sequence absent")
				}
				message, err := broker.GetMsg("RELAY_PERSISTENCE", uint64(*sequence))
				if err != nil || string(message.Data) != string(wire) || message.Header.Get(nats.MsgIdHdr) != envelope.EventID {
					t.Fatalf("immutable persisted message: %+v %v", message, err)
				}
			}
			if mode == "unavailable subject" {
				want = "pending"
			}
			if state != want {
				t.Fatalf("durable relay state: got %s want %s", state, want)
			}
		})
	}
}
