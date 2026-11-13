package delivery

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/jackc/pgx/v5"
)

func TestDeliveryDatabaseCancellationAndInputBounds(t *testing.T) {
	ctx, pool, partitions, _ := orderedPostgres(t)
	store, err := NewStore(pool, os.Getenv("DELIVERY_TEST_SCHEMA")+"_ordered", fixturePolicy())
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	envelope := fixtureEnvelope("cancelled_event")
	pending := []Pending{{fixtureSubject, envelope}}
	lease := Lease{EventID: envelope.EventID, Owner: "owner", Fence: 1}
	for name, call := range map[string]func() error{
		"produce": func() error {
			return store.Produce(cancelled, pending, func(context.Context, pgx.Tx) error { return nil })
		},
		"claim": func() error { _, err := store.Claim(cancelled, "owner", 1, time.Minute); return err },
		"publish": func() error {
			return store.MarkPublished(cancelled, lease, PersistenceAck{Stream: "stream", Sequence: 1})
		},
		"retry":          func() error { return store.Retry(cancelled, lease, Transient, 2, time.Second) },
		"attempt budget": func() error { _, err := store.CheckAttemptBudget(cancelled, lease, 2); return err },
		"statistics":     func() error { _, err := store.Stats(cancelled); return err },
		"partition acquire": func() error {
			_, err := partitions.Acquire(cancelled, ConsumerKey{ConsumerID: "consumer", ProjectionGeneration: "g1", PartitionKind: "market", PartitionKey: "125:0x" + strings.Repeat("a", 40)}, "owner", time.Minute)
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); !errors.Is(err, context.Canceled) {
				t.Fatalf("cancelled database operation: %v", err)
			}
		})
	}
	for name, call := range map[string]func() error{
		"store": func() error { _, err := NewStore(nil, "schema", fixturePolicy()); return err },
		"guarded store": func() error {
			_, err := NewStoreWithEpochGuard(nil, "schema", fixturePolicy(), func(context.Context, pgx.Tx) error { return nil })
			return err
		},
		"partition store":  func() error { _, err := NewPartitionStore(nil, "schema"); return err },
		"partition schema": func() error { _, err := PartitionSchemaDDL("invalid schema"); return err },
		"replay schema":    func() error { _, err := ReplayDDL("invalid schema"); return err },
		"webhook schema":   func() error { _, err := WebhookDDL("invalid schema"); return err },
		"transaction":      func() error { return store.assertEpochTx(ctx, nil) },
		"enqueue":          func() error { return store.Enqueue(ctx, nil, fixtureSubject, envelope) },
		"partition lease":  func() error { _, err := partitions.Acquire(ctx, ConsumerKey{}, "owner", time.Minute); return err },
		"archive":          func() error { _, err := NewArchive(nil, nil, "prefix", "stream", "deployment", "", "", ""); return err },
		"replay": func() error {
			_, err := NewReplayEngine(nil, "consumer", "g1", "owner", nil, nil, time.Minute)
			return err
		},
		"work budget":     func() error { _, err := NewWorkBudget(0, 1, 1); return err },
		"consumer status": func() error { _, err := store.ConsumerReplayStatus(ctx, ConsumerKey{}, "stream", 0); return err },
		"ordered consumer": func() error {
			return store.ConsumeOrdered(ctx, "consumer", "g1", fixtureSubject, nil, nil, ConsumeHooks{})
		},
		"watermark consumer": func() error {
			return store.ConsumeOrderedWithWatermark(ctx, "consumer", "g1", fixtureSubject, nil, nil, ConsumeHooks{})
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Fatal("invalid delivery input admitted")
			}
		})
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.OriginalEnvelopeTx(ctx, tx, envelope.EventID); !errors.Is(err, pgx.ErrTxClosed) {
		t.Fatalf("closed transaction envelope: %v", err)
	}
	if _, err := partitions.AppendTx(ctx, tx, orderedAppendInput("closed_event", "1")); !errors.Is(err, pgx.ErrTxClosed) {
		t.Fatalf("closed transaction append: %v", err)
	}
	if err := store.Enqueue(ctx, tx, fixtureSubject, envelope); !errors.Is(err, pgx.ErrTxClosed) {
		t.Fatalf("closed transaction enqueue: %v", err)
	}
	if _, err := positiveSequence("01"); err == nil {
		t.Fatal("noncanonical sequence admitted")
	}
	if validConsumerKey(ConsumerKey{ConsumerID: "consumer", ProjectionGeneration: "g1", PartitionKind: "inventory", PartitionKey: "other:pool:wallet"}) {
		t.Fatal("foreign partition chain admitted")
	}
	if poisonClass(Transient) {
		t.Fatal("transient operation classified as poison")
	}
	failure := &IntegrityFailure{Class: Authority, Cause: ErrAuthorization}
	if !strings.Contains(failure.Error(), string(Authority)) || !errors.Is(failure, ErrAuthorization) {
		t.Fatalf("integrity classification: %v", failure)
	}
	input := QuarantineInput{ConsumerID: "consumer", Generation: "g1", Subject: fixtureSubject, Wire: []byte("{}"), WireSHA256: accounting.Digest(Hash([]byte("{}"))), Class: InnerSchema, EvidenceDigest: accounting.Digest(Hash([]byte("evidence")))}
	input.Lease = &PartitionLease{Key: ConsumerKey{ConsumerID: "other"}}
	if err := store.QuarantineAfterRollback(ctx, input, nil); !errors.Is(err, ErrLease) {
		t.Fatalf("foreign quarantine lease: %v", err)
	}
}
