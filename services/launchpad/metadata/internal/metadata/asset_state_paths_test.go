package metadata

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/internaltest"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestAssetRecordStateTransitions(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.NewPostgres(t)
	const schema = "metadata_asset_state_paths"
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	store, err := NewProductionStore(pool, schema, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, store.SchemaDDL()); err != nil {
		t.Fatal(err)
	}
	address := "0x" + strings.Repeat("a", 40)
	if _, err := pool.Exec(ctx, "INSERT INTO "+store.table("pools")+"(pool,creator,token) VALUES($1,$1,$1)", address); err != nil {
		t.Fatal(err)
	}
	id := uuid.NewString()
	record := AssetRecord{ID: id, Pool: address, Hash: strings.Repeat("b", 64), Size: 10, ObjectKey: "quarantine/" + id}
	for _, tc := range []struct {
		name   string
		change func(*AssetRecord)
	}{
		{"invalid asset id", func(a *AssetRecord) { a.ID = "invalid" }},
		{"invalid hash", func(a *AssetRecord) { a.Hash = strings.Repeat("z", 64) }},
		{"unscoped key", func(a *AssetRecord) { a.ObjectKey = "public/" + id }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := record
			tc.change(&candidate)
			if err := store.CreateAsset(ctx, candidate); !errors.Is(err, ErrValidation) {
				t.Fatalf("asset admission = %v", err)
			}
		})
	}
	if err := store.CreateAsset(ctx, record); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateAsset(ctx, record); err != nil {
		t.Fatalf("idempotent asset creation = %v", err)
	}
	changed := record
	changed.Hash = strings.Repeat("c", 64)
	if err := store.CreateAsset(ctx, changed); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed immutable asset = %v", err)
	}
	if err := store.TransitionAsset(ctx, id, "CREATED", "APPROVED"); !errors.Is(err, ErrValidation) {
		t.Fatalf("invalid asset transition = %v", err)
	}
	if err := store.TransitionAsset(ctx, id, "CREATED", "UPLOADING"); err != nil {
		t.Fatal(err)
	}
	if err := store.TransitionAsset(ctx, id, "CREATED", "UPLOADING"); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale asset transition = %v", err)
	}
	var state string
	if err := pool.QueryRow(ctx, "SELECT state FROM "+store.table("assets")+" WHERE id=$1", id).Scan(&state); err != nil || state != "UPLOADING" {
		t.Fatalf("asset state = %q, %v", state, err)
	}
	if _, err := pool.Exec(ctx, "ALTER TABLE "+store.table("assets")+" RENAME TO assets_hidden"); err != nil {
		t.Fatal(err)
	}
	newID := uuid.NewString()
	for _, call := range []func() error{
		func() error {
			return store.CreateAsset(ctx, AssetRecord{ID: newID, Pool: address, Hash: record.Hash, Size: 10, ObjectKey: "quarantine/" + newID})
		},
		func() error { return store.TransitionAsset(ctx, id, "UPLOADING", "QUARANTINED") },
	} {
		var databaseError *pgconn.PgError
		if err := call(); !errors.As(err, &databaseError) || databaseError.Code != "42P01" {
			t.Fatalf("unavailable asset table = %v", err)
		}
	}
}
