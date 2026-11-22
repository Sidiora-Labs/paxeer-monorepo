package delivery

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func assertReplayVerificationCatalog(t *testing.T, ctx context.Context, replay *ReplayEngine, manifest ArchiveManifest, digest string) {
	t.Helper()
	store := replay.archive.store
	for _, tc := range []struct {
		name, table, column string
		status              bool
	}{
		{"status progress", store.table("delivery_rebuilds"), "raw_count", true},
		{"source frontier", "indexer.indexer_ingest_watermarks_v1", "typed_complete_height", true},
		{"source completeness", "indexer.indexer_block_completeness_v1", "fetch_complete", true},
		{"verification source", "indexer.indexer_block_completeness_v1", "raw_count", false},
		{"verification projection", store.table("repair_41_projection"), "body", false},
		{"verification publication", store.table("delivery_rebuilds"), "projection_digest", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := store.pool.Exec(ctx, "ALTER TABLE "+tc.table+" RENAME COLUMN "+tc.column+" TO unavailable_column"); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if _, err := store.pool.Exec(ctx, "ALTER TABLE "+tc.table+" RENAME COLUMN unavailable_column TO "+tc.column); err != nil {
					t.Error(err)
				}
			}()
			var err error
			if tc.status {
				_, err = replay.Status(ctx, 0)
			} else {
				err = replay.verify(ctx, manifest, digest)
			}
			var databaseError *pgconn.PgError
			if !errors.As(err, &databaseError) || databaseError.Code != "42703" {
				t.Fatalf("incomplete replay catalog: %v", err)
			}
		})
	}
	var status string
	var next, count int64
	if err := store.pool.QueryRow(ctx, "SELECT status,next_height,raw_count FROM "+store.table("delivery_rebuilds")+" WHERE consumer_id=$1 AND generation=$2", replay.consumer, replay.generation).Scan(&status, &next, &count); err != nil || status != "building" || next != manifest.ToHeight+1 || count != manifest.RawCount {
		t.Fatalf("rejected verification changed progress: %s %d %d %v", status, next, count, err)
	}
	if err := replay.Rebuild(ctx, digest, nil, false, 0); err == nil {
		t.Fatal("building replay admitted without work capacity")
	}
	if err := replay.Rebuild(ctx, "invalid", nil, false, 0); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("unbound startup manifest: %v", err)
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.BindGenerationPathTx(ctx, tx, replay.consumer, replay.generation, valueLegacy); err == nil || err.Error() != "delivery: generation already bound to another authoritative path" {
		t.Fatalf("conflicting generation path: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err := replay.archive.ValidateSourceTx(ctx, tx, manifest); !errors.Is(err, pgx.ErrTxClosed) {
		t.Fatalf("closed source transaction: %v", err)
	}
	if _, err := store.CompareShadowGeneration(ctx, digest, []string{fixtureSubject}, "invalid consumer", replay.generation); !errors.Is(err, ErrSchema) {
		t.Fatalf("unbound comparison consumer: %v", err)
	}
	if _, err := store.CompareShadowRange(ctx, "invalid", []string{fixtureSubject}); !errors.Is(err, ErrSchema) {
		t.Fatalf("unbound comparison manifest: %v", err)
	}
	if err := store.RecordShadowTx(ctx, nil, digest, valueDurable, fixtureSubject, fixtureEnvelope("event")); !errors.Is(err, ErrSchema) {
		t.Fatalf("unbound shadow transaction: %v", err)
	}
	stamp := strconv.FormatInt(time.Now().Unix(), 10)
	admin := context.WithValue(ctx, webhookAdminContextKey{}, webhookAdminCredential{stamp, signWebhookBody(strings.Repeat("admin-secret", 4), stamp, "replay.promote")})
	stale := *replay
	stale.owner = "other_worker"
	if err := stale.Promote(admin, 0); !errors.Is(err, ErrLease) {
		t.Fatalf("unbound promotion owner: %v", err)
	}
	key := ConsumerKey{ConsumerID: "consumer", ProjectionGeneration: "g1", PartitionKind: "inventory", PartitionKey: "125:0x" + strings.Repeat("a", 40) + ":0x" + strings.Repeat("b", 40)}
	if !validConsumerKey(key) {
		t.Fatal("canonical inventory partition rejected")
	}
}
