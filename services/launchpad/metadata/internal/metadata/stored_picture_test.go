package metadata

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func storedPictureStore(t *testing.T) (*ProductionStore, context.Context, AssetRecord) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	db, err := pgxpool.New(ctx, "postgres://kindle_fixture:kindle_fixture_password@127.0.0.1:25432/kindle_fixture?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	schema := fmt.Sprintf("metadata_stored_picture_%d", time.Now().UnixNano())
	if _, err = db.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), time.Second)
		defer done()
		if _, err := db.Exec(cleanup, "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE"); err != nil {
			t.Error(err)
		}
	})
	store, err := NewProductionStore(db, schema, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, store.SchemaDDL()+store.V2SchemaDDL()+store.ConnectedSchemaDDL()); err != nil {
		t.Fatal(err)
	}
	pool := "0x" + strings.Repeat("1", 40)
	if _, err = db.Exec(ctx, "INSERT INTO "+store.table("pools")+" VALUES($1,$1,$2)", pool, "0x"+strings.Repeat("2", 40)); err != nil {
		t.Fatal(err)
	}
	body := pngBytes(t, 48, 32)
	hash := sha256.Sum256(body)
	id := uuid.NewString()
	asset := AssetRecord{ID: id, Pool: pool, Hash: hex.EncodeToString(hash[:]), Size: int64(len(body)), ObjectKey: "quarantine/" + id}
	if err = store.CreateConnectedAsset(ctx, asset, "image/png", 48, 32); err != nil {
		t.Fatal(err)
	}
	return store, ctx, asset
}

func TestStoredPictureFoundByPoolAndContent(t *testing.T) {
	store, ctx, asset := storedPictureStore(t)
	stored, state, err := store.FindConnectedAsset(ctx, asset.Pool, asset.Hash)
	if err != nil || stored != asset || state != "CREATED" {
		t.Fatalf("stored picture = %+v, %q, %v", stored, state, err)
	}
	duplicate := asset
	duplicate.ID = uuid.NewString()
	duplicate.ObjectKey = "quarantine/" + duplicate.ID
	if err = store.CreateConnectedAsset(ctx, duplicate, "image/png", 48, 32); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate admission = %v", err)
	}
	var count int
	if err = store.Pool.QueryRow(ctx, "SELECT count(*) FROM "+store.table("assets")+" WHERE pool=$1", asset.Pool).Scan(&count); err != nil || count != 1 {
		t.Fatalf("pool assets = %d, %v", count, err)
	}
	for _, tc := range []struct {
		name, mime    string
		width, height int
		change        func(*AssetRecord)
	}{
		{"other bytes", "image/png", 48, 32, func(a *AssetRecord) { a.Hash = strings.Repeat("a", 64) }},
		{"other pool", "image/png", 48, 32, func(a *AssetRecord) { a.Pool = "0x" + strings.Repeat("3", 40) }},
		{"other kind", "image/jpeg", 48, 32, func(a *AssetRecord) {}},
		{"other width", "image/png", 49, 32, func(a *AssetRecord) {}},
		{"other height", "image/png", 48, 33, func(a *AssetRecord) {}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := asset
			tc.change(&changed)
			if err := store.CreateConnectedAsset(ctx, changed, tc.mime, tc.width, tc.height); !errors.Is(err, ErrConflict) {
				t.Fatalf("changed identity admission = %v", err)
			}
		})
	}
}

func TestStoredPictureKeepsItsState(t *testing.T) {
	store, ctx, asset := storedPictureStore(t)
	snapshot := func() string {
		t.Helper()
		var encoded string
		if err := store.Pool.QueryRow(ctx, "SELECT jsonb_build_object('asset',to_jsonb(a),'details',to_jsonb(d))::text FROM "+store.table("assets")+" a JOIN "+store.table("asset_details_v2")+" d ON d.asset_id=a.id WHERE a.id=$1", asset.ID).Scan(&encoded); err != nil {
			t.Fatal(err)
		}
		return encoded
	}
	for _, state := range []string{"CREATED", "UPLOADING", "QUARANTINED", "VALIDATING", "APPROVED", "REJECTED", "DELETED"} {
		t.Run(state, func(t *testing.T) {
			if _, err := store.Pool.Exec(ctx, "UPDATE "+store.table("assets")+" SET state=$2 WHERE id=$1", asset.ID, state); err != nil {
				t.Fatal(err)
			}
			before := snapshot()
			stored, got, err := store.FindConnectedAsset(ctx, asset.Pool, asset.Hash)
			if err != nil || stored != asset || got != state {
				t.Fatalf("stored picture = %+v, %q, %v", stored, got, err)
			}
			if after := snapshot(); after != before {
				t.Fatalf("lookup changed stored picture: %s", after)
			}
		})
	}
}

func TestStoredPictureOtherPoolOrContent(t *testing.T) {
	store, ctx, asset := storedPictureStore(t)
	otherPool := "0x" + strings.Repeat("3", 40)
	for _, key := range [][2]string{{otherPool, asset.Hash}, {asset.Pool, strings.Repeat("f", 64)}} {
		stored, state, err := store.FindConnectedAsset(ctx, key[0], key[1])
		if !errors.Is(err, ErrMissing) || stored != (AssetRecord{}) || state != "" {
			t.Fatalf("missing picture = %+v, %q, %v", stored, state, err)
		}
	}
	if _, err := store.Pool.Exec(ctx, "INSERT INTO "+store.table("pools")+" VALUES($1,$1,$2)", otherPool, "0x"+strings.Repeat("4", 40)); err != nil {
		t.Fatal(err)
	}
	other := asset
	other.ID = uuid.NewString()
	other.Pool = otherPool
	other.ObjectKey = "quarantine/" + other.ID
	if err := store.CreateConnectedAsset(ctx, other, "image/png", 48, 32); err != nil {
		t.Fatal(err)
	}
	stored, state, err := store.FindConnectedAsset(ctx, otherPool, asset.Hash)
	if err != nil || stored != other || state != "CREATED" {
		t.Fatalf("other pool picture = %+v, %q, %v", stored, state, err)
	}
	original, _, err := store.FindConnectedAsset(ctx, asset.Pool, asset.Hash)
	if err != nil || original != asset {
		t.Fatalf("original pool picture = %+v, %v", original, err)
	}
	var count int
	if err = store.Pool.QueryRow(ctx, "SELECT count(*) FROM "+store.table("assets")).Scan(&count); err != nil || count != 2 {
		t.Fatalf("pool assets = %d, %v", count, err)
	}
}
