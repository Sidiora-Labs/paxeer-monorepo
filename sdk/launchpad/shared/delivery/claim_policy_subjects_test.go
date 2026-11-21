package delivery

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nats-io/nats.go"
)

func TestClaimPolicySubjectsRelays(t *testing.T) {
	for _, first := range []int{0, 1} {
		t.Run(fmt.Sprintf("first_%d", first), func(t *testing.T) {
			ctx, pool, _, q := orderedPostgres(t)
			schema := os.Getenv("DELIVERY_TEST_SCHEMA") + "_ordered"
			broker, err := orderedBroker(t).JetStream()
			if err != nil {
				t.Fatal(err)
			}
			subjects := [2][2]string{{"policy.identity.revision", "policy.identity.session"}, {"policy.account.preferences", "policy.account.profile"}}
			if _, err := broker.AddStream(&nats.StreamConfig{Name: "POLICY_SUBJECTS", Subjects: []string{"policy.>"}, Storage: nats.FileStorage}); err != nil {
				t.Fatal(err)
			}
			var stores [2]*Store
			var policies [2]Policy
			var expectedBytes [2]int64
			var wires [2][2][]byte
			var subscriptions [2][2]*nats.Subscription
			for i := range stores {
				policies[i] = Policy{Subjects: map[string]bool{subjects[i][0]: true, subjects[i][1]: true, subjects[1-i][0]: false}, EventTypes: map[string]bool{"Swap": true}}
				stores[i], err = NewStore(pool, schema, policies[i])
				if err != nil {
					t.Fatal(err)
				}
				for j, subject := range subjects[i] {
					subscriptions[i][j], err = broker.PullSubscribe(subject, fmt.Sprintf("policy_%d_%d", i, j), nats.BindStream("POLICY_SUBJECTS"))
					if err != nil {
						t.Fatal(err)
					}
					event := fixtureEnvelope(fmt.Sprintf("policy_%d_%d", i, j))
					if err := stores[i].Produce(ctx, []Pending{{subject, event}}, func(ctx context.Context, tx pgx.Tx) error {
						_, err := tx.Exec(ctx, "INSERT INTO "+q+".ordered_effects VALUES($1)", "source_"+event.EventID)
						return err
					}); err != nil {
						t.Fatal(err)
					}
					if err := pool.QueryRow(ctx, "SELECT envelope FROM "+q+".delivery_outbox WHERE event_id=$1", event.EventID).Scan(&wires[i][j]); err != nil {
						t.Fatal(err)
					}
					expectedBytes[i] += int64(len(wires[i][j]))
				}
			}
			snapshot := func(i int) []byte {
				t.Helper()
				var rows []byte
				if err := pool.QueryRow(ctx, "SELECT jsonb_agg(to_jsonb(o) ORDER BY event_id) FROM "+q+".delivery_outbox o WHERE subject=ANY($1::text[])", subjects[i][:]).Scan(&rows); err != nil {
					t.Fatal(err)
				}
				return rows
			}
			for i, store := range stores {
				stats, err := store.Stats(ctx)
				if err != nil || stats.PendingRows != 2 || stats.PendingBytes != expectedBytes[i] || stats.QuarantinedRows != 0 || stats.OldestUnpublishedAgeSeconds <= 0 || stats.LeaseExpiryCount != 0 || stats.MaxAcknowledgementLatencyMS != 0 {
					t.Fatalf("store %d pending statistics: %+v %v", i, stats, err)
				}
			}
			otherBefore := snapshot(1 - first)
			for _, i := range []int{first, 1 - first} {
				stores[i], err = NewStore(pool, schema, policies[i])
				if err != nil {
					t.Fatal(err)
				}
				relay := Relay{Store: stores[i], Publisher: persistedRelayPublisher{broker}, Owner: fmt.Sprintf("relay_%d", i), Batch: 10, LeaseDuration: time.Minute, PublishTimeout: time.Second, RetryDelay: time.Second, MaxAttempts: 3}
				if err := relay.RunOnce(ctx); err != nil {
					t.Fatal(err)
				}
				if i == first && !bytes.Equal(otherBefore, snapshot(1-first)) {
					t.Fatal("other policy rows changed before their relay ran")
				}
				stats, err := stores[i].Stats(ctx)
				if err != nil || stats.PendingRows != 0 || stats.PendingBytes != 0 || stats.QuarantinedRows != 0 || stats.OldestUnpublishedAgeSeconds != 0 || stats.LeaseExpiryCount != 0 {
					t.Fatalf("store %d published statistics: %+v %v", i, stats, err)
				}
				for j, subject := range subjects[i] {
					eventID := fmt.Sprintf("policy_%d_%d", i, j)
					var state, stream string
					var sequence uint64
					var attempts, fence int
					var unleased bool
					if err := pool.QueryRow(ctx, "SELECT state,broker_stream,broker_sequence,attempts,fence,lease_owner IS NULL AND lease_until IS NULL FROM "+q+".delivery_outbox WHERE event_id=$1", eventID).Scan(&state, &stream, &sequence, &attempts, &fence, &unleased); err != nil {
						t.Fatal(err)
					}
					if state != "published" || stream != "POLICY_SUBJECTS" || sequence == 0 || attempts != 1 || fence != 1 || !unleased {
						t.Fatalf("published row %s: %s %s %d %d %d %t", eventID, state, stream, sequence, attempts, fence, unleased)
					}
					persisted, err := broker.GetMsg(stream, sequence)
					if err != nil || persisted.Subject != subject || !bytes.Equal(persisted.Data, wires[i][j]) || persisted.Header.Get(nats.MsgIdHdr) != eventID {
						t.Fatalf("persisted publication %s: %+v %v", eventID, persisted, err)
					}
					messages, err := subscriptions[i][j].Fetch(1, nats.MaxWait(time.Second))
					if err != nil || len(messages) != 1 {
						t.Fatalf("consumer delivery %s: %d %v", eventID, len(messages), err)
					}
					message := messages[0]
					if err := stores[i].Consume(ctx, fmt.Sprintf("consumer_%d", i), "g1", message.Subject, message.Data, orderedBrokerMessage{message}, func(ctx context.Context, tx pgx.Tx, event Envelope) error {
						_, err := tx.Exec(ctx, "INSERT INTO "+q+".ordered_effects VALUES($1)", "consumer_"+event.EventID)
						return err
					}); err != nil {
						t.Fatal(err)
					}
				}
			}
			var effects, publications int
			if err := pool.QueryRow(ctx, "SELECT (SELECT count(*) FROM "+q+".ordered_effects),(SELECT count(*) FROM "+q+".delivery_outbox WHERE state='published')").Scan(&effects, &publications); err != nil || effects != 8 || publications != 4 {
				t.Fatalf("committed source and consumer effects: %d %d %v", effects, publications, err)
			}
		})
	}
}

func TestClaimPolicySubjectsStatsAndDisabled(t *testing.T) {
	ctx, pool, _, q := orderedPostgres(t)
	schema := os.Getenv("DELIVERY_TEST_SCHEMA") + "_ordered"
	subjects := []string{"policy.first", "policy.second"}
	var totalBytes int64
	for i, subject := range subjects {
		policy := Policy{Subjects: map[string]bool{subject: true, subjects[1-i]: false}, EventTypes: fixturePolicy().EventTypes}
		store, err := NewStore(pool, schema, policy)
		if err != nil {
			t.Fatal(err)
		}
		var expectedBytes int64
		for j := 0; j < 2; j++ {
			event := fixtureEnvelope(fmt.Sprintf("stats_%d_%d", i, j))
			if err := store.Produce(ctx, []Pending{{subject, event}}, func(ctx context.Context, tx pgx.Tx) error {
				_, err := tx.Exec(ctx, "INSERT INTO "+q+".ordered_effects VALUES($1)", event.EventID)
				return err
			}); err != nil {
				t.Fatal(err)
			}
			wire, err := event.Encode(policy, subject)
			if err != nil {
				t.Fatal(err)
			}
			expectedBytes += int64(len(wire))
		}
		leases, err := store.Claim(ctx, "stats_owner", 1, time.Minute)
		if err != nil || len(leases) != 1 || leases[0].EventID != fmt.Sprintf("stats_%d_0", i) {
			t.Fatalf("oldest policy row: %+v %v", leases, err)
		}
		for expiry := 0; expiry <= i; expiry++ {
			if _, err := pool.Exec(ctx, "UPDATE "+q+".delivery_outbox SET lease_until=clock_timestamp()-interval '1 second' WHERE event_id=$1", leases[0].EventID); err != nil {
				t.Fatal(err)
			}
			previous := leases[0]
			leases, err = store.Claim(ctx, "stats_owner", 1, time.Minute)
			if err != nil || len(leases) != 1 || leases[0].EventID != previous.EventID || leases[0].Fence != previous.Fence+1 || leases[0].Attempts != previous.Attempts+1 {
				t.Fatalf("reclaimed policy row: %+v %v", leases, err)
			}
		}
		if err := store.Retry(ctx, leases[0], SchemaViolation, 10, time.Second); err != nil {
			t.Fatal(err)
		}
		stats, err := store.Stats(ctx)
		if err != nil || stats.PendingRows != 1 || stats.QuarantinedRows != 1 || stats.PendingBytes != expectedBytes || stats.LeaseExpiryCount != int64(i+1) || stats.OldestUnpublishedAgeSeconds <= 0 || stats.MaxAcknowledgementLatencyMS != 0 {
			t.Fatalf("policy quarantine statistics: %+v %v", stats, err)
		}
		totalBytes += expectedBytes
	}
	disabled, err := NewStore(pool, schema, Policy{Subjects: map[string]bool{subjects[0]: false, subjects[1]: false}, EventTypes: fixturePolicy().EventTypes})
	if err != nil {
		t.Fatal(err)
	}
	var before, after []byte
	snapshot := "SELECT jsonb_agg(to_jsonb(o) ORDER BY event_id) FROM " + q + ".delivery_outbox o"
	if err := pool.QueryRow(ctx, snapshot).Scan(&before); err != nil {
		t.Fatal(err)
	}
	leases, err := disabled.Claim(ctx, "disabled_owner", 1000, time.Minute)
	if err != nil || len(leases) != 0 {
		t.Fatalf("disabled policy claimed rows: %+v %v", leases, err)
	}
	stats, err := disabled.Stats(ctx)
	if err != nil || stats != (Stats{}) {
		t.Fatalf("disabled policy statistics: %+v %v", stats, err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer rollbackBounded(tx)
	admission, err := disabled.CheckIngestAdmissionTx(ctx, tx, AdmissionPolicy{DatabaseCapacityBytes: 1 << 40, DatabaseReserveBytes: 1 << 20, MaxBacklogBytes: 1 << 30, MaxBatchBytes: 1 << 20, MaxConcurrentBatches: 2}, 1024)
	if err != nil || admission.BacklogBytes != totalBytes {
		t.Fatalf("schema-wide ingestion backlog: %+v %v", admission, err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, snapshot).Scan(&after); err != nil || !bytes.Equal(before, after) {
		t.Fatalf("pending and quarantined rows changed: %v", err)
	}
}
