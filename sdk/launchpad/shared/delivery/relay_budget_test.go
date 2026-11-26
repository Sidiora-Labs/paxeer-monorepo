package delivery

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nats-io/nats.go"
)

func relayBudgetSetup(t *testing.T) (context.Context, Relay, nats.JetStreamContext) {
	t.Helper()
	ctx, pool, _, _ := orderedPostgres(t)
	store, err := NewStore(pool, os.Getenv("DELIVERY_TEST_SCHEMA")+"_ordered", fixturePolicy())
	if err != nil {
		t.Fatal(err)
	}
	broker, err := orderedBroker(t).JetStream()
	if err != nil {
		t.Fatal(err)
	}
	event := fixtureEnvelope("relay_budget")
	if err := store.Produce(ctx, []Pending{{fixtureSubject, event}}, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, "INSERT INTO "+store.table("ordered_effects")+" VALUES($1)", event.EventID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return ctx, Relay{Store: store, Publisher: persistedRelayPublisher{broker}, Owner: "relay_budget", Batch: 2, LeaseDuration: time.Minute, PublishTimeout: time.Second, RetryDelay: time.Second, MaxAttempts: 3}, broker
}

func relayBudgetAssertRow(t *testing.T, ctx context.Context, relay Relay, wantState string, wantAttempts int, wantClass FailureClass, wantDelay time.Duration) {
	t.Helper()
	var state, class string
	var attempts int
	var unleased bool
	var wire []byte
	var delaySeconds float64
	if err := relay.Store.pool.QueryRow(ctx, "SELECT state,COALESCE(failure_class,''),attempts,lease_owner IS NULL AND lease_until IS NULL,envelope,EXTRACT(EPOCH FROM (available_at-clock_timestamp()))::double precision FROM "+relay.Store.table("delivery_outbox")+" WHERE event_id='relay_budget'").Scan(&state, &class, &attempts, &unleased, &wire, &delaySeconds); err != nil {
		t.Fatal(err)
	}
	expected, err := fixtureEnvelope("relay_budget").Encode(fixturePolicy(), fixtureSubject)
	if err != nil {
		t.Fatal(err)
	}
	if state != wantState || class != string(wantClass) || attempts != wantAttempts || !unleased || !bytes.Equal(wire, expected) {
		t.Fatalf("durable row: state=%s class=%s attempts=%d unleased=%t immutable=%t", state, class, attempts, unleased, bytes.Equal(wire, expected))
	}
	if wantDelay > 0 && (delaySeconds < wantDelay.Seconds()-0.5 || delaySeconds > wantDelay.Seconds()+0.5) {
		t.Fatalf("retry delay: got %.3fs want %s within 500ms", delaySeconds, wantDelay)
	}
}

func relayBudgetMakeAvailable(t *testing.T, ctx context.Context, relay Relay) {
	t.Helper()
	if _, err := relay.Store.pool.Exec(ctx, "UPDATE "+relay.Store.table("delivery_outbox")+" SET available_at=clock_timestamp()-interval '1 second' WHERE event_id='relay_budget'"); err != nil {
		t.Fatal(err)
	}
}

func TestRelayTransientBudgetGrowingDelay(t *testing.T) {
	ctx, relay, _ := relayBudgetSetup(t)
	for i, delay := range []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 5 * time.Second, 5 * time.Second} {
		if err := relay.RunOnce(ctx); !errors.Is(err, nats.ErrNoStreamResponse) {
			t.Fatalf("attempt %d publication cause: %v", i+1, err)
		}
		relayBudgetAssertRow(t, ctx, relay, "pending", i+1, Transient, delay)
		relayBudgetMakeAvailable(t, ctx, relay)
	}
}

func TestRelayTransientBudgetHundredAttempts(t *testing.T) {
	for _, attempts := range []int{98, 100} {
		t.Run(fmt.Sprintf("attempts_%d", attempts), func(t *testing.T) {
			ctx, relay, broker := relayBudgetSetup(t)
			if _, err := relay.Store.pool.Exec(ctx, "UPDATE "+relay.Store.table("delivery_outbox")+" SET attempts=$1 WHERE event_id='relay_budget'", attempts); err != nil {
				t.Fatal(err)
			}
			if attempts == 98 {
				for _, attempt := range []int{99, 100} {
					if err := relay.RunOnce(ctx); !errors.Is(err, nats.ErrNoStreamResponse) {
						t.Fatalf("attempt %d publication cause: %v", attempt, err)
					}
					state := "pending"
					if attempt == 100 {
						state = "quarantined"
					}
					relayBudgetAssertRow(t, ctx, relay, state, attempt, Transient, 5*time.Second)
					relayBudgetMakeAvailable(t, ctx, relay)
				}
			}
			if _, err := broker.AddStream(&nats.StreamConfig{Name: "RELAY_BUDGET", Subjects: []string{fixtureSubject}, Storage: nats.FileStorage}); err != nil {
				t.Fatal(err)
			}
			if err := relay.RunOnce(ctx); err != nil {
				t.Fatal(err)
			}
			wantAttempts := 100
			if attempts == 100 {
				wantAttempts = 101
			}
			relayBudgetAssertRow(t, ctx, relay, "quarantined", wantAttempts, Transient, 0)
			info, err := broker.StreamInfo("RELAY_BUDGET")
			if err != nil {
				t.Fatal(err)
			}
			if info.State.Msgs != 0 {
				t.Fatalf("exhausted row published: %d messages", info.State.Msgs)
			}
		})
	}
}

func TestRelayTransientBudgetRecovery(t *testing.T) {
	ctx, relay, broker := relayBudgetSetup(t)
	for attempt := 1; attempt <= 2; attempt++ {
		if err := relay.RunOnce(ctx); !errors.Is(err, nats.ErrNoStreamResponse) {
			t.Fatalf("attempt %d publication cause: %v", attempt, err)
		}
		relayBudgetAssertRow(t, ctx, relay, "pending", attempt, Transient, time.Duration(attempt)*time.Second)
		relayBudgetMakeAvailable(t, ctx, relay)
	}
	if _, err := broker.AddStream(&nats.StreamConfig{Name: "RELAY_BUDGET", Subjects: []string{fixtureSubject}, Storage: nats.FileStorage}); err != nil {
		t.Fatal(err)
	}
	if err := relay.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	relayBudgetAssertRow(t, ctx, relay, "published", 3, "", 0)
	var stream string
	var sequence uint64
	if err := relay.Store.pool.QueryRow(ctx, "SELECT broker_stream,broker_sequence FROM "+relay.Store.table("delivery_outbox")+" WHERE event_id='relay_budget'").Scan(&stream, &sequence); err != nil {
		t.Fatal(err)
	}
	message, err := broker.GetMsg(stream, sequence)
	if err != nil {
		t.Fatal(err)
	}
	wire, err := fixtureEnvelope("relay_budget").Encode(fixturePolicy(), fixtureSubject)
	if err != nil {
		t.Fatal(err)
	}
	if stream != "RELAY_BUDGET" || sequence == 0 || message.Subject != fixtureSubject || !bytes.Equal(message.Data, wire) || message.Header.Get(nats.MsgIdHdr) != "relay_budget" {
		t.Fatal("persisted acknowledgement does not match immutable row")
	}
}

type relayBudgetClassifiedPublisher struct {
	persistedRelayPublisher
	cause error
}

func (p relayBudgetClassifiedPublisher) Publish(ctx context.Context, subject, eventID string, wire []byte) (PersistenceAck, error) {
	ack, err := p.persistedRelayPublisher.Publish(ctx, subject, eventID, wire)
	if err != nil {
		return ack, errors.Join(p.cause, err)
	}
	return ack, nil
}

func TestRelayTransientBudgetPermanentFailures(t *testing.T) {
	for _, test := range []struct {
		class FailureClass
		cause error
	}{
		{Authorization, ErrAuthorization},
		{IntegrityConflict, ErrIntegrity},
		{SchemaViolation, ErrSchema},
	} {
		t.Run(string(test.class), func(t *testing.T) {
			ctx, relay, broker := relayBudgetSetup(t)
			relay.Publisher = relayBudgetClassifiedPublisher{persistedRelayPublisher{broker}, test.cause}
			if err := relay.RunOnce(ctx); !errors.Is(err, test.cause) || !errors.Is(err, nats.ErrNoStreamResponse) {
				t.Fatalf("permanent publication cause: %v", err)
			}
			relayBudgetAssertRow(t, ctx, relay, "quarantined", 1, test.class, time.Second)
		})
	}
}

func TestRelayTransientBudgetLargerBaseDelay(t *testing.T) {
	ctx, relay, _ := relayBudgetSetup(t)
	relay.RetryDelay = 10 * time.Second
	for attempt := 1; attempt <= 5; attempt++ {
		if err := relay.RunOnce(ctx); !errors.Is(err, nats.ErrNoStreamResponse) {
			t.Fatalf("attempt %d publication cause: %v", attempt, err)
		}
		relayBudgetAssertRow(t, ctx, relay, "pending", attempt, Transient, 10*time.Second)
		relayBudgetMakeAvailable(t, ctx, relay)
	}
}

func TestRelayTransientBudgetCauseAndPassBoundary(t *testing.T) {
	ctx, relay, _ := relayBudgetSetup(t)
	event := fixtureEnvelope("relay_budget_following")
	if err := relay.Store.Produce(ctx, []Pending{{fixtureSubject, event}}, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, "INSERT INTO "+relay.Store.table("ordered_effects")+" VALUES($1)", event.EventID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := relay.RunOnce(ctx); !errors.Is(err, nats.ErrNoStreamResponse) {
		t.Fatalf("publication cause: %v", err)
	}
	relayBudgetAssertRow(t, ctx, relay, "pending", 1, Transient, time.Second)
	var attempts int
	if err := relay.Store.pool.QueryRow(ctx, "SELECT attempts FROM "+relay.Store.table("delivery_outbox")+" WHERE event_id=$1", event.EventID).Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if attempts != 0 {
		t.Fatalf("pass continued after publication failure: following attempts=%d", attempts)
	}
}

func TestRelayTransientBudgetDelayBounds(t *testing.T) {
	for _, test := range []struct {
		base     time.Duration
		attempts int
		want     time.Duration
	}{
		{time.Second, 1, time.Second},
		{time.Second, 2, 2 * time.Second},
		{time.Second, 3, 4 * time.Second},
		{time.Second, 4, 5 * time.Second},
		{2 * time.Second, 3, 5 * time.Second},
		{1500 * time.Millisecond, 2, 3 * time.Second},
		{time.Second, 100, 5 * time.Second},
		{time.Second, int(^uint(0) >> 1), 5 * time.Second},
		{10 * time.Second, 100, 10 * time.Second},
		{time.Hour, int(^uint(0) >> 1), time.Hour},
	} {
		t.Run(fmt.Sprintf("%s_attempts_%d", test.base, test.attempts), func(t *testing.T) {
			if delay := relayBudgetRetryDelay(test.base, test.attempts); delay != test.want {
				t.Fatalf("retry delay: got %s want %s", delay, test.want)
			}
		})
	}
}
