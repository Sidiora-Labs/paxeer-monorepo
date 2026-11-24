package coordinatortest

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
)

// PostgresDSNEnv names the already running PostgreSQL the bootstrap uses; its
// role may CREATE DATABASE and CREATE ROLE.
const PostgresDSNEnv = "COORDINATOR_TEST_POSTGRES_DSN"

// The registered migrations and reviewed grants pass production readiness
// for every connected service runtime role.
func TestPlatformRuntimeRolesPassReadiness(t *testing.T) {
	admin := os.Getenv(PostgresDSNEnv)
	if admin == "" {
		t.Skipf("%s is not set: point it at a running PostgreSQL whose role may CREATE DATABASE and CREATE ROLE; tests never start containers", PostgresDSNEnv)
	}
	roles := []string{"chat_runtime", "livestream_runtime", "candles_runtime", "stats_runtime", "identity_runtime", "control_runtime", "operator_runtime", "operator_registrar", "ranking_runtime", "pnl_runtime", "indexer_runtime", "api_runtime", "api_rest_runtime", "api_sockets_runtime", "metadata_runtime", "users_runtime"}
	db := Bootstrap(t, admin, roles...)
	ctx := context.Background()
	for _, role := range roles {
		rt := db.Runtimes[role]
		want, err := production.LoadExpectedSchema(rt.ExpectedSchema, rt.ExpectedSchemaSHA256)
		if err != nil {
			t.Fatal(err)
		}
		pool, err := pgxpool.New(ctx, rt.DSN)
		if err != nil {
			t.Fatal(err)
		}
		if err = production.CheckReadiness(ctx, pool, want); err != nil {
			pool.Close()
			t.Fatalf("%s readiness: %v", role, err)
		}
		var owns, creates bool
		if err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_class WHERE relowner=(SELECT oid FROM pg_roles WHERE rolname=current_user)), EXISTS(SELECT 1 FROM pg_namespace n WHERE has_schema_privilege(current_user,n.oid,'CREATE'))`).Scan(&owns, &creates); err != nil || owns || creates {
			pool.Close()
			t.Fatalf("%s owns objects=%v or may CREATE=%v (%v)", role, owns, creates, err)
		}
		pool.Close()
	}
	// The objects the platform runtimes need exist only through registered
	// migrations: the v2 social tables (370002), the moderation view
	// media/metadata reads (370001) and the safety bans (380000).
	conn, err := pgxpool.New(ctx, db.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for id, object := range map[int64]string{370002: "chat.v2_bounties", 370001: "chat.safety_visibility_v2", 380000: "chat.safety_bans"} {
		var present bool
		if err = conn.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL AND EXISTS(SELECT 1 FROM migration_control.applied_migrations WHERE migration_id=$2)`, object, id).Scan(&present); err != nil || !present {
			t.Fatalf("%s from migration %d: present=%v %v", object, id, present, err)
		}
	}
	assertConnectedInterfaces(t, db, conn)
	assertMountedReadiness(t, db)
	assertOrderedBatchRecovery(t, db, conn)
	var goose bool
	if err = conn.QueryRow(ctx, `SELECT to_regclass('public.goose_db_version') IS NOT NULL`).Scan(&goose); err != nil || goose {
		t.Fatalf("coordinator bootstrap left a goose history: %v %v", goose, err)
	}
}
