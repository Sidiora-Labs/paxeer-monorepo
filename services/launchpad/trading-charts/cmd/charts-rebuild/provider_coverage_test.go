package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/chainread"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
	production "github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/jackc/pgx/v5"
)

func rebuildReadyDatabase(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	dsn, pool := internaltest.NewPostgres(t)
	admin, err := pgx.Connect(ctx, os.Getenv(internaltest.PostgresEnv))
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	var nonce [6]byte
	var secret [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := rand.Read(secret[:]); err != nil {
		t.Fatal(err)
	}
	role, password := "charts_rebuild_"+hex.EncodeToString(nonce[:]), hex.EncodeToString(secret[:])
	if _, err := admin.Exec(ctx, "CREATE ROLE "+pgx.Identifier{role}.Sanitize()+" LOGIN PASSWORD '"+password+"'"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DROP OWNED BY "+pgx.Identifier{role}.Sanitize()); err != nil {
			t.Error(err)
			return
		}
		connection, err := pgx.Connect(context.Background(), os.Getenv(internaltest.PostgresEnv))
		if err != nil {
			t.Error(err)
			return
		}
		defer connection.Close(context.Background())
		if _, err := connection.Exec(context.Background(), "DROP ROLE IF EXISTS "+pgx.Identifier{role}.Sanitize()); err != nil {
			t.Error(err)
		}
	})
	for _, sql := range []string{
		`CREATE SCHEMA migration_control`,
		`CREATE SCHEMA recovery`,
		`REVOKE CREATE ON SCHEMA public FROM PUBLIC`,
		`REVOKE EXECUTE ON ALL FUNCTIONS IN SCHEMA candles FROM PUBLIC`,
		`CREATE FUNCTION migration_control.readiness(text) RETURNS TABLE(registry_digest text,interface_digest text,reader_min_version bigint,migrations jsonb) LANGUAGE sql STABLE AS $$ SELECT repeat('a',64),repeat('b',64),1::bigint,jsonb_build_object('1',repeat('c',64)) $$`,
		`CREATE FUNCTION recovery.current_epoch() RETURNS TABLE(epoch bigint,region text) LANGUAGE sql STABLE AS $$ SELECT 1::bigint,'local'::text $$`,
		"GRANT USAGE ON SCHEMA candles,migration_control,recovery TO " + pgx.Identifier{role}.Sanitize(),
		"GRANT SELECT ON candles.candles TO " + pgx.Identifier{role}.Sanitize(),
		"GRANT EXECUTE ON FUNCTION migration_control.readiness(text),recovery.current_epoch() TO " + pgx.Identifier{role}.Sanitize(),
	} {
		if _, err := pool.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, err := production.CatalogFingerprint(ctx, tx, "candles")
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	expected := production.ExpectedSchema{
		Service: "core/trading-charts", RuntimeRole: role,
		RegistryDigest: strings.Repeat("a", 64), InterfaceDigest: strings.Repeat("b", 64), ReaderVersion: 1,
		Migrations: map[int64]string{1: strings.Repeat("c", 64)}, Schemas: map[string]string{"candles": fingerprint}, Epoch: 1, Region: "local",
		Privileges: []production.Grant{
			{Schema: "candles", Object: "candles", Privileges: []string{"SELECT"}},
			{Schema: "migration_control", Object: "readiness", ObjectKind: "function", ArgumentTypes: []string{"text"}, Privileges: []string{"EXECUTE"}},
			{Schema: "recovery", Object: "current_epoch", ObjectKind: "function", Privileges: []string{"EXECUTE"}},
		},
	}
	repair1052Bind(t, "EXPECTED_SCHEMA", expected, true)
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	parsed.User = url.UserPassword(role, password)
	return parsed.String()
}

func TestRebuildUsesOwnedProvidersForArchiveAdmission(t *testing.T) {
	repair1052Environment(t, true)
	t.Setenv("DATABASE_URL", rebuildReadyDatabase(t))
	rpc, first, rpcURL := internaltest.NewChainSpanAtWithURL(t, 1703980800, 8*24*time.Hour)
	t.Setenv("RPC_URL", rpcURL)
	last, err := (chainread.Reader{RPC: rpc}).Block(context.Background(), 2)
	if err != nil || first.Time >= 1704067200 || last.Time < 1704672000 {
		t.Fatalf("canonical archive boundary blocks: first=%+v last=%+v err=%v", first, last, err)
	}
	if stamp, err := rebuildBlockTime(context.Background(), rpc, 1, strings.ToUpper(first.Hash)); err != nil || stamp != first.Time {
		t.Fatalf("canonical archive block time: %d, %v", stamp, err)
	}
	if stamp, err := rebuildBlockTime(context.Background(), rpc, 100000, first.Hash); stamp != 0 || err == nil || !strings.Contains(err.Error(), "archive boundary header unavailable") {
		t.Fatalf("missing archive block: %d, %v", stamp, err)
	}
	if stamp, err := rebuildBlockTime(context.Background(), rpc, 1, "0x"+strings.Repeat("f", 64)); stamp != 0 || !errors.Is(err, store.ErrIntegrity) {
		t.Fatalf("different archive block: %d, %v", stamp, err)
	}
	_, objects := internaltest.NewObjectStoreWithConfig(t)
	for key, value := range map[string]string{
		"S3_ENDPOINT": objects.Endpoint, "S3_REGION": objects.Region, "S3_BUCKET": objects.Bucket,
		"S3_ACCESS_KEY_ID": objects.AccessKeyID, "S3_SECRET_ACCESS_KEY": objects.SecretAccessKey,
	} {
		t.Setenv(key, value)
	}
	manifest := filepath.Join(t.TempDir(), "manifests.json")
	if err := os.WriteFile(manifest, []byte("[]"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := runArgs(repair1052Args(manifest)); err == nil || !strings.Contains(err.Error(), "reconstruction failed") {
		t.Fatalf("archive reconstruction admission: %v", err)
	}
	for _, tc := range []struct {
		name, variable, value, want string
	}{
		{"invalid database configuration", "DATABASE_URL", "not a database URL", "DATABASE_URL: url_invalid"},
		{"invalid canonical RPC configuration", "RPC_URL", "not a URL", "RPC_URL: url_invalid"},
		{"unavailable canonical chain", "RPC_URL", "http://127.0.0.1:1", "canonical chain125 RPC required"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.variable, tc.value)
			if err := runArgs(repair1052Args(manifest)); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("provider admission: %v", err)
			}
		})
	}
	digest := strings.Repeat("a", 64)
	approved := store.ArchiveManifest{FormatVersion: "raw-archive-v2", ChainID: 125, SourceStream: "canonical", FromHeight: 1, ToHeight: 2, ByteLength: 1, SHA256: digest, Bucket: objects.Bucket, ObjectKey: "indexer/raw/v2/canonical/" + digest + ".json", Blocks: []store.ArchiveBlock{{Height: 1, Hash: first.Hash, RawCount: 0, FetchComplete: true, RequiredDecodeComplete: true}, {Height: 2, Hash: last.Hash, RawCount: 0, FetchComplete: true, RequiredDecodeComplete: true}}}
	for _, tc := range []struct {
		name  string
		index int
	}{
		{"first block authority", 0},
		{"last block authority", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := approved
			candidate.Blocks = append([]store.ArchiveBlock(nil), approved.Blocks...)
			candidate.Blocks[tc.index].Hash = "0x" + strings.Repeat("f", 64)
			wire, err := json.Marshal([]store.ArchiveManifest{candidate})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(manifest, wire, 0600); err != nil {
				t.Fatal(err)
			}
			if err := runArgs(repair1052Args(manifest)); err == nil || !strings.Contains(err.Error(), store.ErrIntegrity.Error()) {
				t.Fatalf("canonical archive block authority: %v", err)
			}
		})
	}
	t.Setenv("S3_ENDPOINT", " ")
	if err := runArgs(repair1052Args(manifest)); err == nil || !strings.Contains(err.Error(), "archive object reader configuration unavailable") {
		t.Fatalf("archive provider admission: %v", err)
	}
}
