package delivery

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/jackc/pgx/v5"
)

func TestStoredEnvelopeAndIngestAdmission(t *testing.T) {
	ctx, pool, _, _ := orderedPostgres(t)
	store, err := NewStore(pool, os.Getenv("DELIVERY_TEST_SCHEMA")+"_ordered", fixturePolicy())
	if err != nil {
		t.Fatal(err)
	}
	want := fixtureEnvelope("admission-envelope")
	if err := store.Produce(ctx, []Pending{{fixtureSubject, want}}, func(context.Context, pgx.Tx) error { return nil }); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	subject, got, err := store.OriginalEnvelopeTx(ctx, tx, want.EventID)
	if err != nil || subject != fixtureSubject || got.EventID != want.EventID || got.PayloadHash != want.PayloadHash {
		t.Fatalf("stored envelope = %s, %+v, %v", subject, got, err)
	}
	if _, _, err := store.OriginalEnvelopeTx(ctx, tx, "invalid event id"); !errors.Is(err, ErrSchema) {
		t.Fatalf("invalid envelope identity = %v", err)
	}
	if _, _, err := store.OriginalEnvelopeTx(ctx, tx, "absent-event"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("absent envelope = %v", err)
	}
	policy := AdmissionPolicy{DatabaseCapacityBytes: 1 << 40, DatabaseReserveBytes: 1 << 20, MaxBacklogBytes: 1 << 30, MaxBatchBytes: 1 << 20, MaxConcurrentBatches: 2}
	snapshot, err := store.CheckIngestAdmissionTx(ctx, tx, policy, 1024)
	if err != nil || snapshot.DatabaseBytes < 1 || snapshot.BacklogBytes < 1 || snapshot.ReservedConcurrentBytes != 2<<20 {
		t.Fatalf("ingest admission = %+v, %v", snapshot, err)
	}
	invalid := policy
	invalid.MaxConcurrentBatches = 65
	if _, err := store.CheckIngestAdmissionTx(ctx, tx, invalid, 1024); err == nil {
		t.Fatal("unbounded concurrent batches admitted")
	}
	databaseFull := policy
	databaseFull.DatabaseCapacityBytes = snapshot.DatabaseBytes
	if _, err := store.CheckIngestAdmissionTx(ctx, tx, databaseFull, 1024); err == nil {
		t.Fatal("reserved database capacity was spent")
	}
	backlogFull := policy
	backlogFull.MaxBatchBytes = 1024
	backlogFull.MaxConcurrentBatches = 1
	backlogFull.MaxBacklogBytes = 1025
	if _, err := store.CheckIngestAdmissionTx(ctx, tx, backlogFull, 1024); err == nil {
		t.Fatal("reserved outbox capacity was spent")
	}
}

func TestStoreLeaseAndQuarantineAdmission(t *testing.T) {
	ctx, pool, _, _ := orderedPostgres(t)
	schema := os.Getenv("DELIVERY_TEST_SCHEMA") + "_ordered"
	store, err := NewStore(pool, schema, fixturePolicy())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SchemaDDL("bad schema"); err == nil {
		t.Fatal("invalid delivery schema admitted")
	}
	if _, err := NewStoreWithEpochGuard(pool, schema, fixturePolicy(), nil); err == nil {
		t.Fatal("unfenced delivery store admitted")
	}
	if err := store.Produce(ctx, nil, func(context.Context, pgx.Tx) error { return nil }); err == nil {
		t.Fatal("empty production batch admitted")
	}
	envelope := fixtureEnvelope("lease-admission")
	if err := store.Produce(ctx, []Pending{{fixtureSubject, envelope}}, func(context.Context, pgx.Tx) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Claim(ctx, "invalid owner", 1, time.Minute); err == nil {
		t.Fatal("invalid lease owner admitted")
	}
	leases, err := store.Claim(ctx, "lease_owner", 1, time.Minute)
	if err != nil || len(leases) != 1 || leases[0].EventID != envelope.EventID || leases[0].Attempts != 1 {
		t.Fatalf("claimed outbox = %+v, %v", leases, err)
	}
	lease := leases[0]
	if _, err := store.CheckAttemptBudget(ctx, lease, 0); err == nil {
		t.Fatal("zero attempt budget admitted")
	}
	allowed, err := store.CheckAttemptBudget(ctx, lease, 1)
	if err != nil || !allowed {
		t.Fatalf("first lease budget = %t, %v", allowed, err)
	}
	if err := store.Retry(ctx, lease, Transient, 0, time.Second); err == nil {
		t.Fatal("zero retry budget admitted")
	}
	if err := store.Retry(ctx, lease, "unknown", 1, time.Second); err == nil {
		t.Fatal("unknown failure class admitted")
	}
	if err := store.MarkPublished(ctx, lease, PersistenceAck{}); err == nil {
		t.Fatal("empty persistence acknowledgement admitted")
	}
	if err := store.MarkPublished(ctx, lease, PersistenceAck{Stream: "recorded", Sequence: 1}); err != nil {
		t.Fatalf("persisted outbox acknowledgement = %v", err)
	}
	if err := store.Retry(ctx, lease, Transient, 1, time.Second); !errors.Is(err, ErrLease) {
		t.Fatalf("published lease retried: %v", err)
	}
	if allowed, err := store.CheckAttemptBudget(ctx, lease, 1); allowed || !errors.Is(err, ErrLease) {
		t.Fatalf("published lease retained attempt budget: %t, %v", allowed, err)
	}
	wire, err := envelope.Encode(fixturePolicy(), fixtureSubject)
	if err != nil {
		t.Fatal(err)
	}
	input := QuarantineInput{ConsumerID: "lease_consumer", Generation: "generation1", Subject: fixtureSubject, Wire: wire, WireSHA256: accounting.Digest(Hash(wire)), Class: InnerSchema, EvidenceDigest: accounting.Digest(Hash(wire))}
	invalid := input
	invalid.WireSHA256 = "changed"
	if err := store.QuarantineAfterRollback(ctx, invalid, nil); err == nil {
		t.Fatal("changed quarantine envelope admitted")
	}
	if err := store.QuarantineAfterRollback(ctx, input, nil); err != nil {
		t.Fatalf("committed quarantine evidence = %v", err)
	}
	var recorded string
	if err := pool.QueryRow(ctx, "SELECT failure_class FROM "+store.table("delivery_ordered_quarantine")+" WHERE consumer_id=$1 AND envelope_hash=$2", input.ConsumerID, string(input.WireSHA256)).Scan(&recorded); err != nil || recorded != string(InnerSchema) {
		t.Fatalf("quarantine classification = %s, %v", recorded, err)
	}
	if err := store.Consume(ctx, "", "generation1", fixtureSubject, wire, nil, nil); err == nil {
		t.Fatal("invalid consumer configuration admitted")
	}
}

func TestStoreAttemptBudgetAndPoolQuarantine(t *testing.T) {
	ctx, pool, _, _ := orderedPostgres(t)
	store, err := NewStore(pool, os.Getenv("DELIVERY_TEST_SCHEMA")+"_ordered", fixturePolicy())
	if err != nil {
		t.Fatal(err)
	}
	envelope := fixtureEnvelope("attempt-budget")
	if err := store.Produce(ctx, []Pending{{fixtureSubject, envelope}}, func(context.Context, pgx.Tx) error { return nil }); err != nil {
		t.Fatal(err)
	}
	first, err := store.Claim(ctx, "budget_owner", 1, time.Minute)
	if err != nil || len(first) != 1 || first[0].Attempts != 1 {
		t.Fatalf("first claim = %+v, %v", first, err)
	}
	if err := store.Retry(ctx, first[0], Transient, 3, time.Second); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("delivery_outbox")+" SET available_at=clock_timestamp() WHERE event_id=$1", envelope.EventID); err != nil {
		t.Fatal(err)
	}
	second, err := store.Claim(ctx, "budget_owner", 1, time.Minute)
	if err != nil || len(second) != 1 || second[0].Attempts != 2 {
		t.Fatalf("second claim = %+v, %v", second, err)
	}
	allowed, err := store.CheckAttemptBudget(ctx, second[0], 1)
	if err != nil || allowed {
		t.Fatalf("exhausted attempt budget = %t, %v", allowed, err)
	}
	stats, err := store.Stats(ctx)
	if err != nil || stats.PendingRows != 0 || stats.QuarantinedRows != 1 || stats.PendingBytes == 0 {
		t.Fatalf("quarantined outbox stats = %+v, %v", stats, err)
	}
	wire, err := envelope.Encode(fixturePolicy(), fixtureSubject)
	if err != nil {
		t.Fatal(err)
	}
	const poolAddress = "0x1111111111111111111111111111111111111111"
	input := QuarantineInput{ConsumerID: "pool_consumer", Generation: "generation1", Subject: fixtureSubject, Wire: wire, WireSHA256: accounting.Digest(Hash(wire)), Class: IdentityConflict, AffectedPools: []string{poolAddress, poolAddress}, EvidenceDigest: accounting.Digest(Hash(wire))}
	if err := store.QuarantineAfterRollback(ctx, input, nil); err == nil {
		t.Fatal("identity conflict without pool freeze admitted")
	}
	var frozen []string
	freeze := func(ctx context.Context, tx pgx.Tx, pools []string) error {
		frozen = append(frozen, pools...)
		var database string
		if err := tx.QueryRow(ctx, "SELECT current_database()").Scan(&database); err != nil {
			return err
		}
		if database == "" {
			return ErrIntegrity
		}
		return nil
	}
	if err := store.QuarantineAfterRollback(ctx, input, freeze); err != nil {
		t.Fatal(err)
	}
	if len(frozen) != 1 || frozen[0] != poolAddress {
		t.Fatalf("affected pools were not deduplicated: %v", frozen)
	}
	var recordedPools []string
	if err := pool.QueryRow(ctx, "SELECT affected_pools FROM "+store.table("delivery_ordered_quarantine")+" WHERE consumer_id=$1 AND envelope_hash=$2", input.ConsumerID, string(input.WireSHA256)).Scan(&recordedPools); err != nil || len(recordedPools) != 1 || recordedPools[0] != poolAddress {
		t.Fatalf("durable affected pools = %v, %v", recordedPools, err)
	}
	invalid := input
	invalid.AffectedPools = []string{"invalid"}
	if err := store.QuarantineAfterRollback(ctx, invalid, freeze); err == nil {
		t.Fatal("invalid affected pool admitted")
	}
}
