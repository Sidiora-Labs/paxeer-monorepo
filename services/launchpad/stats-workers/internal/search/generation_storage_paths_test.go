package search

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestGenerationStorageRejectionsPreserveActivePointer(t *testing.T) {
	for _, tc := range []struct{ name, stage, sql, code string }{
		{"begin pointer relation", "begin", "ALTER TABLE stats_search.active_v2 RENAME TO saved_active", "42P01"},
		{"begin generation insertion", "begin", "ALTER TABLE stats_search.generations_v2 ADD CHECK(generation <> 'g2') NOT VALID", "23514"},
		{"begin document insertion", "begin", "ALTER TABLE stats_search.documents_v2 ADD CHECK(generation <> 'g2') NOT VALID", "23514"},
		{"activate pointer relation", "activate", "ALTER TABLE stats_search.active_v2 RENAME TO saved_active", "42P01"},
		{"activate generation relation", "activate", "ALTER TABLE stats_search.generations_v2 RENAME TO saved_generations", "42P01"},
		{"activate source relation", "activate", "ALTER TABLE stats_search.source_documents_v2 RENAME TO saved_source", "42P01"},
		{"activate retained state", "activate", "ALTER TABLE stats_search.generations_v2 ADD CHECK(state <> 'retained') NOT VALID", "23514"},
		{"activate active state", "activate", "ALTER TABLE stats_search.generations_v2 ADD CHECK(state <> 'active') NOT VALID", "23514"},
		{"activate pointer update", "activate", "ALTER TABLE stats_search.active_v2 ADD CHECK(generation <> 'g2') NOT VALID", "23514"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			db := integratedDatabase(t)
			seedIndexed(t, db, 901)
			store := Store{Pool: db}
			observed := time.Now().UTC()
			var proof GenerationProof
			if tc.stage == "activate" {
				if err := store.BeginGeneration(ctx, "g2", 1, observed); err != nil {
					t.Fatal(err)
				}
				tx, err := db.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				count, digest, err := GenerationDigest(ctx, tx, "g2")
				if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
					t.Fatal(rollbackErr)
				}
				if err != nil {
					t.Fatal(err)
				}
				proof = GenerationProof{Generation: "g2", SourceWatermark: 1, SourceObservedAt: observed, Rows: count, Digest: digest}
			}
			if _, err := db.Exec(ctx, tc.sql); err != nil {
				t.Fatal(err)
			}
			var err error
			if tc.stage == "begin" {
				err = store.BeginGeneration(ctx, "g2", 1, observed)
			} else {
				err = store.ActivateGeneration(ctx, proof)
			}
			var storage *pgconn.PgError
			if !errors.As(err, &storage) || storage.Code != tc.code {
				t.Fatalf("generation storage: %v, want SQLSTATE %s", err, tc.code)
			}
			pointer := "stats_search.active_v2"
			if strings.Contains(tc.sql, "saved_active") {
				pointer = "stats_search.saved_active"
			}
			var active string
			if err = db.QueryRow(ctx, "SELECT generation FROM "+pointer+" WHERE singleton").Scan(&active); err != nil || active != "g1" {
				t.Fatalf("active pointer changed: %s %v", active, err)
			}
		})
	}
}

func TestGenerationBoundsRejectInvalidAuthority(t *testing.T) {
	ctx := context.Background()
	db := integratedDatabase(t)
	store := Store{Pool: db}
	now := time.Now().UTC()
	if err := store.BeginGeneration(ctx, "invalid-generation", 1, now); !errors.Is(err, ErrInvalid) {
		t.Fatalf("generation name: %v", err)
	}
	if err := store.AdvanceGenerationBoundary(ctx, "g2", 0, now); !errors.Is(err, ErrInvalid) {
		t.Fatalf("generation boundary: %v", err)
	}
	proof := GenerationProof{Generation: "g2", SourceWatermark: 1, SourceObservedAt: now, Digest: strings.Repeat("z", 64)}
	if err := store.ActivateGeneration(ctx, proof); !errors.Is(err, ErrInvalid) {
		t.Fatalf("generation digest encoding: %v", err)
	}
	proof.Digest = strings.Repeat("0", 64)
	proof.Rows = -1
	if err := store.ActivateGeneration(ctx, proof); !errors.Is(err, ErrInvalid) {
		t.Fatalf("generation row count: %v", err)
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, _, err = GenerationDigest(ctx, tx, "g1"); err == nil {
		t.Fatal("closed digest transaction accepted")
	}
	db.Close()
	proof.Rows = 0
	if err = store.ActivateGeneration(ctx, proof); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("closed generation store: %v", err)
	}
}
