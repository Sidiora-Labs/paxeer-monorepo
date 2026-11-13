package delivery

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestDeliveryEpochGuardFencesEveryTransactionPhase(t *testing.T) {
	ctx, pool, _, _ := orderedPostgres(t)
	denied := errors.New("epoch authority changed")
	for _, phase := range []int{1, 2, 3} {
		calls := 0
		store, err := NewStoreWithEpochGuard(pool, "epoch_guard", fixturePolicy(), func(context.Context, pgx.Tx) error {
			calls++
			if calls == phase {
				return denied
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		effectCalled := false
		err = store.mutate(ctx, func(tx pgx.Tx) error { effectCalled = true; return nil })
		if phase <= 2 {
			if !errors.Is(err, denied) || calls != phase || effectCalled != (phase > 1) {
				t.Fatalf("phase %d: %d %t %v", phase, calls, effectCalled, err)
			}
		} else if err != nil || calls != 2 || !effectCalled {
			t.Fatalf("complete mutation: %d %t %v", calls, effectCalled, err)
		}
	}
}

func TestDeliveryClosedTransactionsPreserveErrors(t *testing.T) {
	ctx, pool, partitions, _ := orderedPostgres(t)
	store, err := NewStore(pool, os.Getenv("DELIVERY_TEST_SCHEMA")+"_ordered", fixturePolicy())
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	envelope := fixtureEnvelope("closed_transaction")
	key := ConsumerKey{ConsumerID: "consumer", ProjectionGeneration: "g1", PartitionKind: "market", PartitionKey: "125:0x" + strings.Repeat("a", 40)}
	lease := PartitionLease{Key: key, Owner: "owner", Epoch: 1}
	for name, call := range map[string]func() error{
		"shadow record": func() error {
			return store.RecordShadowTx(ctx, tx, strings.Repeat("a", 64), valueLegacy, fixtureSubject, envelope)
		},
		"generation binding": func() error { return store.BindGenerationPathTx(ctx, tx, "consumer", "g1", valueDurable) },
		"shadow comparison": func() error {
			_, _, err := store.compareShadowTx(ctx, tx, strings.Repeat("a", 64), []string{fixtureSubject})
			return err
		},
		"lease lock":  func() error { return partitions.lockLeaseTx(ctx, tx, lease) },
		"lease fence": func() error { return partitions.finalFenceTx(ctx, tx, lease) },
		"bootstrap": func() error {
			return partitions.BootstrapTx(ctx, tx, BootstrapInput{Lease: lease}, func(context.Context, pgx.Tx, BootstrapInput) error { return nil })
		},
		"ingest admission": func() error {
			_, err := store.CheckIngestAdmissionTx(ctx, tx, AdmissionPolicy{DatabaseCapacityBytes: 1 << 40, DatabaseReserveBytes: 1 << 20, MaxBacklogBytes: 1 << 20, MaxBatchBytes: 1024, MaxConcurrentBatches: 1}, 1)
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Fatal("closed transaction admitted")
			}
		})
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := store.quarantineConsumer(cancelled, "consumer", "g1", fixtureSubject, []byte("{}"), SchemaViolation); err != nil {
		t.Fatal(err)
	}
	var quarantined int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+store.table("delivery_quarantine")+" WHERE consumer_id='consumer'").Scan(&quarantined); err != nil || quarantined != 1 {
		t.Fatalf("cancellation evidence: %d %v", quarantined, err)
	}
	for name, call := range map[string]func() error{
		"shadow range": func() error {
			_, err := store.CompareShadowRange(cancelled, strings.Repeat("a", 64), []string{fixtureSubject})
			return err
		},
		"consumer replay": func() error { _, err := store.ConsumerReplayStatus(cancelled, key, "source", 0); return err },
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); !errors.Is(err, context.Canceled) {
				t.Fatalf("cancelled operation: %v", err)
			}
		})
	}
}

func TestEnvelopeAdmissionRejectsNoncanonicalIdentitiesAndPayloads(t *testing.T) {
	for name, edit := range map[string]func(*Envelope){
		"chain":              func(e *Envelope) { e.ChainID = 1 },
		"source number":      func(e *Envelope) { e.SourceOrder.Height = "9223372036854775808" },
		"source encoding":    func(e *Envelope) { e.SourceOrder.LogIndex = "01" },
		"clock":              func(e *Envelope) { e.EmittedAt = time.Time{} },
		"clock zone":         func(e *Envelope) { e.EmittedAt = e.EmittedAt.In(time.FixedZone("local", 3600)) },
		"identity":           func(e *Envelope) { e.EventID = "invalid event" },
		"payload array":      func(e *Envelope) { e.Payload = []byte("[]"); e.PayloadHash = Hash(e.Payload) },
		"payload null":       func(e *Envelope) { e.Payload = []byte("null"); e.PayloadHash = Hash(e.Payload) },
		"payload whitespace": func(e *Envelope) { e.Payload = []byte(" {} "); e.PayloadHash = Hash(e.Payload) },
		"wire budget": func(e *Envelope) {
			e.Payload = []byte(`{"data":"` + strings.Repeat("x", MaxEnvelopeBytes-20) + `"}`)
			e.PayloadHash = Hash(e.Payload)
		},
	} {
		t.Run(name, func(t *testing.T) {
			e := fixtureEnvelope("admission")
			edit(&e)
			if _, err := e.Encode(fixturePolicy(), fixtureSubject); err == nil {
				t.Fatal("noncanonical envelope encoded")
			}
		})
	}
	for _, wire := range [][]byte{nil, []byte(`{} {}`), []byte(`{"unknown":true}`), []byte(`{"schemaVersion":1,"payload":null}`)} {
		if _, err := Decode(wire, fixturePolicy(), fixtureSubject); err == nil {
			t.Fatal("invalid wire decoded")
		}
	}
	for _, test := range []struct {
		err  error
		want FailureClass
	}{{ErrAuthorization, Authorization}, {ErrIntegrity, IntegrityConflict}, {ErrSchema, SchemaViolation}, {context.Canceled, Transient}} {
		if got := publicationFailureClass(test.err); got != test.want {
			t.Fatalf("publication classification: %s", got)
		}
	}
	if got := (SourceOrder{"1", "2", "3"}).Compare(SourceOrder{"1", "2", "4"}); got != -1 {
		t.Fatal(got)
	}
	if _, err := CompareShadow([]ShadowRecord{{"event", strings.Repeat("a", 64)}}, []ShadowRecord{{"event", strings.Repeat("a", 64)}, {"event", strings.Repeat("a", 64)}}); !errors.Is(err, ErrIntegrity) {
		t.Fatal(err)
	}
	if _, err := CompareShadow([]ShadowRecord{{"invalid event", strings.Repeat("a", 64)}}, nil); !errors.Is(err, ErrSchema) {
		t.Fatal(err)
	}
}

func TestWorkBudgetLimitsConcurrentDeliveryAndCancelledWaits(t *testing.T) {
	budget, err := NewWorkBudget(1, 10000, 10)
	if err != nil {
		t.Fatal(err)
	}
	release, err := budget.acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := budget.acquire(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	release()
	release, err = budget.acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	release()
	if err := (Relay{}).RunOnceWithBudget(context.Background(), budget); err == nil {
		t.Fatal("unconfigured relay admitted")
	}
	if len(budget.slots) != 0 {
		t.Fatal("work slot leaked")
	}
	if err := (&WebhookBridge{}).RunOnceWithBudget(context.Background(), nil); err == nil {
		t.Fatal("webhook without budget admitted")
	}
}
