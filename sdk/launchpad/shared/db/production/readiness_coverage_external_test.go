package production_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCoordinatorRuntimeReadinessAndCatalogIdentity(t *testing.T) {
	dsn := os.Getenv("COORDINATOR_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Fatal("coordinator PostgreSQL fixture is required")
	}
	database := coordinatortest.Bootstrap(t, dsn, "chat_runtime")
	runtime := database.Runtimes["chat_runtime"]
	want, err := production.LoadExpectedSchema(runtime.ExpectedSchema, runtime.ExpectedSchemaSHA256)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, runtime.DSN)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := production.CheckReadiness(ctx, pool, want); err != nil {
		t.Fatalf("runtime schema readiness: %v", err)
	}
	altered := want
	altered.Schemas = make(map[string]string, len(want.Schemas))
	for schema, fingerprint := range want.Schemas {
		altered.Schemas[schema] = fingerprint
	}
	for schema := range altered.Schemas {
		altered.Schemas[schema] = production.Checksum([]byte("different catalog"))
		break
	}
	if err := production.CheckReadiness(ctx, pool, altered); err == nil {
		t.Fatal("changed structural fingerprint passed runtime readiness")
	}
	altered = want
	altered.RuntimeRole = "unrelated_runtime"
	if err := production.CheckReadiness(ctx, pool, altered); err == nil {
		t.Fatal("unrelated runtime role passed readiness")
	}
}

func TestRuntimeReadinessRequiresMatchingCatalogAndRecoveryAuthority(t *testing.T) {
	ctx := context.Background()
	database := coordinatortest.Bootstrap(t, os.Getenv("COORDINATOR_TEST_POSTGRES_DSN"), "chat_runtime")
	runtime := database.Runtimes["chat_runtime"]
	path, digest := database.ExpectedSchemaFile(t, "chat_runtime")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := production.LoadExpectedSchema(raw, digest)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, runtime.DSN)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	admin, err := pgxpool.New(ctx, database.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	conn, err := pgx.Connect(ctx, database.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if err := production.RecordRuntimeReadiness(ctx, conn, expected); err != nil {
		t.Fatalf("reviewed runtime authority: %v", err)
	}
	if err := production.CheckReadiness(ctx, nil, expected); err == nil {
		t.Fatal("readiness without database admitted")
	}
	if err := production.RecordRuntimeReadiness(ctx, nil, expected); err == nil {
		t.Fatal("readiness publication without connection admitted")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := production.CheckReadiness(cancelled, pool, expected); err == nil {
		t.Fatal("cancelled readiness admitted")
	}
	for _, mode := range []string{"registry", "migrations", "epoch", "region", "schema", "missing migration", "missing schema"} {
		t.Run(mode, func(t *testing.T) {
			want := expected
			want.Migrations = make(map[int64]string, len(expected.Migrations))
			for id, sum := range expected.Migrations {
				want.Migrations[id] = sum
			}
			want.Schemas = make(map[string]string, len(expected.Schemas))
			for name, sum := range expected.Schemas {
				want.Schemas[name] = sum
			}
			switch mode {
			case "registry":
				want.RegistryDigest = production.Checksum([]byte("another registry"))
			case "migrations":
				for id := range want.Migrations {
					want.Migrations[id] = production.Checksum([]byte("another migration"))
					break
				}
			case "epoch":
				want.Epoch++
			case "region":
				want.Region = "another-region"
			case "schema":
				for name := range want.Schemas {
					want.Schemas[name] = production.Checksum([]byte("another catalog"))
					break
				}
			case "missing migration":
				want.Migrations[99999999] = production.Checksum([]byte("unpublished migration"))
			case "missing schema":
				want.Schemas["unpublished_schema"] = production.Checksum([]byte("unpublished catalog"))
			}
			if err := production.CheckReadiness(ctx, pool, want); err == nil {
				t.Fatal("unbound runtime authority admitted")
			}
			if mode == "migrations" || mode == "schema" || mode == "missing migration" || mode == "missing schema" {
				if err := production.RecordRuntimeReadiness(ctx, conn, want); err == nil {
					t.Fatal("unverified catalog authority published")
				}
			}
		})
	}
	original, err := json.Marshal(expected.Migrations)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, "UPDATE migration_control.runtime_readiness SET migrations='[]'::jsonb WHERE role_name=$1", runtime.Role); err != nil {
		t.Fatal(err)
	}
	if err := production.CheckReadiness(ctx, pool, expected); err == nil {
		t.Fatal("non-map migration authority admitted")
	}
	if _, err := admin.Exec(ctx, "UPDATE migration_control.runtime_readiness SET migrations=$1 WHERE role_name=$2", original, runtime.Role); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, "ALTER TABLE migration_control.runtime_readiness RENAME COLUMN reader_min_version TO unbound_version"); err != nil {
		t.Fatal(err)
	}
	if err := production.CheckReadiness(ctx, pool, expected); err == nil {
		t.Fatal("unavailable readiness version admitted")
	}
	if err := production.RecordRuntimeReadiness(ctx, conn, expected); err == nil {
		t.Fatal("unavailable readiness catalog accepted publication")
	}
	if _, err := admin.Exec(ctx, "ALTER TABLE migration_control.runtime_readiness RENAME COLUMN unbound_version TO reader_min_version"); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, "ALTER FUNCTION recovery.current_epoch() RENAME TO archived_current_epoch"); err != nil {
		t.Fatal(err)
	}
	if err := production.CheckReadiness(ctx, pool, expected); err == nil {
		t.Fatal("missing recovery authority admitted")
	}
	if _, err := admin.Exec(ctx, "ALTER FUNCTION recovery.archived_current_epoch() RENAME TO current_epoch"); err != nil {
		t.Fatal(err)
	}
	if err := production.CheckReadiness(ctx, pool, expected); err != nil {
		t.Fatalf("restored catalog authority: %v", err)
	}
}
