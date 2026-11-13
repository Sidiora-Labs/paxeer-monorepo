package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/jackc/pgx/v5"
	"github.com/nats-io/nats.go"
)

func TestOrderedAdmissionRejectsUnboundDeliveryBeforeProjection(t *testing.T) {
	ctx, pool, partitions, _ := orderedPostgres(t)
	envelope, _, wire, policy := orderedPublicationFixture(t, ctx, pool, partitions)
	store, err := NewStore(pool, os.Getenv("DELIVERY_TEST_SCHEMA")+"_ordered", policy)
	if err != nil {
		t.Fatal(err)
	}
	connection := orderedBroker(t)
	broker, err := connection.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := broker.AddStream(&nats.StreamConfig{Name: "ORDERED_ADMISSION", Subjects: []string{accounting.MarketSubject}}); err != nil {
		t.Fatal(err)
	}
	subscription, err := broker.PullSubscribe(accounting.MarketSubject, "admission", nats.BindStream("ORDERED_ADMISSION"))
	if err != nil {
		t.Fatal(err)
	}
	defer subscription.Unsubscribe()
	if _, err := broker.Publish(accounting.MarketSubject, wire); err != nil {
		t.Fatal(err)
	}
	messages, err := subscription.Fetch(1, nats.MaxWait(time.Second))
	if err != nil || len(messages) != 1 {
		t.Fatalf("committed message: %d %v", len(messages), err)
	}
	delivery := orderedBrokerMessage{messages[0]}
	for _, mode := range []string{"digest", "schema", "malformed", "oversize", "unapproved subject", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			candidate := envelope
			input := append([]byte(nil), wire...)
			subject := accounting.MarketSubject
			callCtx := ctx
			switch mode {
			case "digest":
				candidate.PayloadHash = Hash([]byte("another payload"))
				input, err = json.Marshal(candidate)
			case "schema":
				candidate.SchemaVersion = 2
				input, err = json.Marshal(candidate)
			case "malformed":
				input = []byte("{")
			case "oversize":
				input = make([]byte, MaxEnvelopeBytes+1)
			case "unapproved subject":
				subject = "accounting.unknown.v1"
			case "cancelled":
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				callCtx = cancelled
			}
			if err != nil {
				t.Fatal(err)
			}
			hooks := ConsumeHooks{Guard: func(context.Context, pgx.Tx, Envelope) (GuardedPosition, error) {
				t.Fatal("invalid delivery reached projection guard")
				return GuardedPosition{}, ErrIntegrity
			}, Effect: func(context.Context, pgx.Tx, Envelope, GuardedPosition) error {
				t.Fatal("invalid delivery projected")
				return ErrIntegrity
			}, Advance: func(context.Context, pgx.Tx, Envelope, GuardedPosition) error {
				t.Fatal("invalid delivery advanced position")
				return ErrIntegrity
			}}
			if err := store.ConsumeOrdered(callCtx, "admission_reader", "g1", subject, input, delivery, hooks); err == nil {
				t.Fatal("invalid ordered delivery admitted")
			}
			batch := []OrderedDelivery{{Subject: subject, Wire: input, Delivery: delivery}}
			if err := store.ConsumeOrderedBatch(callCtx, "admission_reader", "g1", batch, OrderedBatchHooks{Guard: hooks.Guard, Effect: func(context.Context, pgx.Tx, []OrderedEffect) error {
				t.Fatal("invalid batch projected")
				return ErrIntegrity
			}}); err == nil {
				t.Fatal("invalid ordered batch admitted")
			}
		})
	}
	hooks := OrderedBatchHooks{Guard: func(context.Context, pgx.Tx, Envelope) (GuardedPosition, error) {
		t.Fatal("unbound broker reached projection")
		return GuardedPosition{}, ErrIntegrity
	}, Effect: func(context.Context, pgx.Tx, []OrderedEffect) error {
		t.Fatal("unbound broker projected")
		return ErrIntegrity
	}}
	for _, input := range []OrderedDelivery{{Subject: accounting.MarketSubject, Wire: wire}, {Subject: accounting.MarketSubject, Wire: make([]byte, 8*MaxEnvelopeBytes+1), Delivery: delivery}} {
		if err := store.ConsumeOrderedBatch(ctx, "admission_reader", "g1", []OrderedDelivery{input}, hooks); err == nil {
			t.Fatal("unbound batch delivery admitted")
		}
	}
	changed := envelope
	changed.TraceID = "another_trace"
	changedWire, err := changed.Encode(policy, accounting.MarketSubject)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ConsumeOrderedBatch(ctx, "admission_reader", "g1", []OrderedDelivery{{Subject: accounting.MarketSubject, Wire: changedWire, Delivery: delivery}}, hooks); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("broker content binding: %v", err)
	}
	if err := store.ConsumeOrderedWithWatermark(ctx, "admission_reader", "g1", accounting.MarketSubject, changedWire, delivery, ConsumeHooks{Advance: func(context.Context, pgx.Tx, Envelope, GuardedPosition) error {
		t.Fatal("unbound watermark advanced")
		return ErrIntegrity
	}}); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("watermark content binding: %v", err)
	}
	info, err := subscription.ConsumerInfo()
	if err != nil || info.NumAckPending != 1 {
		t.Fatalf("rejected deliveries acknowledged: %+v %v", info, err)
	}
}
