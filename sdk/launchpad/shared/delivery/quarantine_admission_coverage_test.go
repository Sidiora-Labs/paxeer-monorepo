package delivery

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/jackc/pgx/v5"
	"github.com/nats-io/nats.go"
)

func TestQuarantineRequiresBoundedConsistentCatalogState(t *testing.T) {
	for _, mode := range []string{"lease scope", "offset scope", "lease scan", "offset scan", "lease table", "offset table", "quarantine table", "record identity", "freeze", "epoch"} {
		t.Run(mode, func(t *testing.T) {
			ctx, pool, _, _ := orderedPostgres(t)
			schema := os.Getenv("DELIVERY_TEST_SCHEMA") + "_ordered"
			store, err := NewStore(pool, schema, fixturePolicy())
			if err != nil {
				t.Fatal(err)
			}
			wire := []byte("{}")
			input := QuarantineInput{ConsumerID: "quarantine_consumer", Generation: "g1", Subject: fixtureSubject, Wire: wire, WireSHA256: accounting.Digest(Hash(wire)), Class: InnerSchema, EvidenceDigest: accounting.Digest(Hash(wire))}
			denied := errors.New("catalog authority unavailable")
			var freeze func(context.Context, pgx.Tx, []string) error
			sql := ""
			var foreignKey string
			if mode == "offset scope" || mode == "offset scan" {
				if err := pool.QueryRow(ctx, "SELECT conname FROM pg_constraint WHERE conrelid=$1::regclass AND contype='f'", store.table("delivery_partition_offsets")).Scan(&foreignKey); err != nil {
					t.Fatal(err)
				}
				if _, err := pool.Exec(ctx, "ALTER TABLE "+store.table("delivery_partition_offsets")+" DROP CONSTRAINT "+pgx.Identifier{foreignKey}.Sanitize()); err != nil {
					t.Fatal(err)
				}
			}
			switch mode {
			case "lease scope", "offset scope":
				sql = "INSERT INTO " + store.table("delivery_partition_leases") + " SELECT 'quarantine_consumer','g1','market','125:pool_'||i::text,'owner',1,clock_timestamp()+interval '1 minute' FROM generate_series(1,1001) i"
				if mode == "offset scope" {
					if _, err := pool.Exec(ctx, sql); err != nil {
						t.Fatal(err)
					}
					input.ConsumerID = "offset_consumer"
					if _, err := pool.Exec(ctx, "UPDATE "+store.table("delivery_partition_leases")+" SET consumer_id='offset_consumer'"); err != nil {
						t.Fatal(err)
					}
					// The offset catalog has its own bounded lock scope.
					sql = "INSERT INTO " + store.table("delivery_partition_offsets") + " SELECT 'offset_consumer','g1','market','125:pool_'||i::text,'" + strings.Repeat("a", 64) + "',1,'',1,'' FROM generate_series(1,1001) i; DELETE FROM " + store.table("delivery_partition_leases")
				}
			case "lease scan", "offset scan":
				table := "delivery_partition_leases"
				if mode == "offset scan" {
					table = "delivery_partition_offsets"
				}
				// Nullable catalog keys cannot authorize quarantine evidence.
				sql = "ALTER TABLE " + store.table(table) + " DROP CONSTRAINT " + table + "_pkey CASCADE; ALTER TABLE " + store.table(table) + " ALTER COLUMN partition_kind DROP NOT NULL"
				if mode == "lease scan" {
					sql += ";INSERT INTO " + store.table(table) + " VALUES('quarantine_consumer','g1',NULL,'key','owner',1,clock_timestamp())"
				} else {
					sql += ";INSERT INTO " + store.table(table) + " VALUES('quarantine_consumer','g1',NULL,'key','" + strings.Repeat("a", 64) + "',1,'',1,'')"
				}
			case "lease table":
				sql = "ALTER TABLE " + store.table("delivery_partition_leases") + " RENAME COLUMN partition_kind TO unbound_kind"
			case "offset table":
				sql = "ALTER TABLE " + store.table("delivery_partition_offsets") + " RENAME COLUMN partition_kind TO unbound_kind"
			case "quarantine table":
				sql = "DROP TABLE " + store.table("delivery_ordered_quarantine")
			case "record identity":
				if err := store.QuarantineAfterRollback(ctx, input, nil); err != nil {
					t.Fatal(err)
				}
				sql = "UPDATE " + store.table("delivery_ordered_quarantine") + " SET subject='other.subject'"
			case "freeze":
				input.AffectedPools = []string{"0x" + strings.Repeat("a", 40)}
				freeze = func(context.Context, pgx.Tx, []string) error { return denied }
			case "epoch":
				store, err = NewStoreWithEpochGuard(pool, schema, fixturePolicy(), func(context.Context, pgx.Tx) error { return denied })
				if err != nil {
					t.Fatal(err)
				}
			}
			if sql != "" {
				if _, err := pool.Exec(ctx, sql); err != nil {
					t.Fatal(err)
				}
			}
			if err := store.QuarantineAfterRollback(ctx, input, freeze); err == nil {
				t.Fatal("unbound quarantine catalog admitted")
			} else if (mode == "freeze" || mode == "epoch") && !errors.Is(err, denied) {
				t.Fatalf("quarantine refusal: %v", err)
			}
			if mode == "freeze" {
				var n int
				if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+store.table("delivery_ordered_quarantine")).Scan(&n); err != nil || n != 0 {
					t.Fatalf("atomic quarantine freeze: %d %v", n, err)
				}
			}
		})
	}
}

func TestOrderedDeliveryQuarantinesMalformedEnvelopesWithoutAcknowledging(t *testing.T) {
	ctx, pool, _, _ := orderedPostgres(t)
	store, err := NewStore(pool, os.Getenv("DELIVERY_TEST_SCHEMA")+"_ordered", fixturePolicy())
	if err != nil {
		t.Fatal(err)
	}
	nc := orderedBroker(t)
	js, err := nc.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := js.AddStream(&nats.StreamConfig{Name: "MALFORMED_DELIVERIES", Subjects: []string{fixtureSubject}}); err != nil {
		t.Fatal(err)
	}
	sub, err := js.PullSubscribe(fixtureSubject, "malformed_reader", nats.BindStream("MALFORMED_DELIVERIES"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sub.Unsubscribe() }()
	hooks := ConsumeHooks{Guard: func(context.Context, pgx.Tx, Envelope) (GuardedPosition, error) {
		t.Error("malformed envelope reached partition guard")
		return GuardedPosition{}, ErrIntegrity
	}, Effect: func(context.Context, pgx.Tx, Envelope, GuardedPosition) error {
		t.Error("malformed envelope reached effect")
		return ErrIntegrity
	}, Advance: func(context.Context, pgx.Tx, Envelope, GuardedPosition) error {
		t.Error("malformed envelope advanced offset")
		return ErrIntegrity
	}}
	for _, batch := range []bool{false, true} {
		for i, mode := range []string{"schema", "identity", "authority", "encoding", "subject", "wire limit"} {
			t.Run(fmt.Sprintf("batch_%t/%s", batch, mode), func(t *testing.T) {
				e := fixtureEnvelope("malformed_publication")
				subject := fixtureSubject
				wire, err := e.Encode(fixturePolicy(), subject)
				if err != nil {
					t.Fatal(err)
				}
				switch mode {
				case "schema":
					wire = []byte(strings.Replace(string(wire), `"schema_version":1`, `"schema_version":2`, 1))
				case "identity":
					wire = []byte(strings.Replace(string(wire), e.PayloadHash, strings.Repeat("0", 64), 1))
				case "authority":
					subject = "unapproved.subject"
				case "encoding":
					wire = []byte("{")
				case "subject":
					subject = "invalid subject"
				case "wire limit":
					wire = []byte(strings.Repeat("x", MaxEnvelopeBytes+1))
				}
				if _, err := js.Publish(fixtureSubject, []byte("delivery")); err != nil {
					t.Fatal(err)
				}
				messages, err := sub.Fetch(1, nats.MaxWait(time.Second))
				if err != nil {
					t.Fatal(err)
				}
				d := orderedBrokerMessage{messages[0]}
				consumer := fmt.Sprintf("poison_%t_%d", batch, i)
				if batch {
					err = store.ConsumeOrderedBatch(ctx, consumer, "g1", []OrderedDelivery{{Subject: subject, Wire: wire, Delivery: d}}, OrderedBatchHooks{Guard: hooks.Guard, Effect: func(context.Context, pgx.Tx, []OrderedEffect) error {
						t.Error("malformed batch reached effect")
						return ErrIntegrity
					}})
				} else {
					err = store.ConsumeOrdered(ctx, consumer, "g1", subject, wire, d, hooks)
				}
				if err == nil {
					t.Fatal("malformed envelope admitted")
				}
				var n int
				if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+store.table("delivery_ordered_quarantine")+" WHERE consumer_id=$1", consumer).Scan(&n); err != nil {
					t.Fatal(err)
				}
				want := 1
				if mode == "subject" || mode == "wire limit" {
					want = 0
				}
				if n != want {
					t.Fatalf("bounded quarantine rows: %d want %d", n, want)
				}
			})
		}
	}
	info, err := sub.ConsumerInfo()
	if err != nil || info.NumAckPending != 12 {
		t.Fatalf("malformed deliveries acknowledged: %+v %v", info, err)
	}
}
