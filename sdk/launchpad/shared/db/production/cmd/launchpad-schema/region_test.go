package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/jackc/pgx/v5"
)

func TestRegionCommandPreservesRecordedRecoveryAuthority(t *testing.T) {
	ctx := context.Background()
	tag := testTag(t)
	directory, roles := approvedBundle(t, tag, false, "")
	dsn, conn := emptyDatabase(t, tag, roles)
	bundle, err := loadBundle(directory)
	if err != nil {
		t.Fatal(err)
	}
	if err := bundle.region(ctx, conn, "local-region"); err == nil {
		t.Fatal("region admitted without applied migrations")
	}
	if err := bundle.apply(ctx, conn); err != nil {
		t.Fatal(err)
	}
	if err := ensureRole(ctx, conn, conn.Config().User, true); err == nil || err.Error() != "existing role has unreviewed authority" {
		t.Fatalf("administrator role reused as owner: %v", err)
	}
	if err := ensureHistory(ctx, conn, "unregistered", "ranking"); err == nil || err.Error() != "service history is unregistered" {
		t.Fatalf("unregistered history: %v", err)
	}
	if _, err := conn.Exec(ctx, "ALTER TABLE recovery.recovery_control RENAME COLUMN checkpoint_sha256 TO unavailable_checkpoint"); err != nil {
		t.Fatal(err)
	}
	if err := bundle.region(ctx, conn, "local-region"); err == nil || !strings.HasPrefix(err.Error(), "recovery region recording failed") {
		t.Fatalf("unavailable checkpoint catalog: %v", err)
	}
	if _, err := conn.Exec(ctx, "ALTER TABLE recovery.recovery_control RENAME COLUMN unavailable_checkpoint TO checkpoint_sha256"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LAUNCHPAD_ADMIN_DATABASE_URL", dsn)
	command(t, "region", "--migrations", directory, "--region", "local-region")
	var epoch, revision int64
	var region, checkpoint string
	if err := conn.QueryRow(ctx, "SELECT epoch,region,revision,checkpoint_sha256 FROM recovery.recovery_control WHERE singleton").Scan(&epoch, &region, &revision, &checkpoint); err != nil || epoch != 1 || region != "local-region" || revision != 1 || checkpoint != bundle.registryDigest {
		t.Fatalf("recorded recovery authority: %d %s %d %s %v", epoch, region, revision, checkpoint, err)
	}
	before := databaseSnapshot(t, conn)
	command(t, "region", "--migrations", directory, "--region", "local-region")
	if after := databaseSnapshot(t, conn); before != after {
		t.Fatal("matching region changed the database")
	}
	if err := bundle.region(ctx, conn, "other-region"); err == nil || err.Error() != "recovery region differs from the recorded authority" {
		t.Fatalf("conflicting region admitted: %v", err)
	}
	if err := conn.QueryRow(ctx, "SELECT epoch,region,revision,checkpoint_sha256 FROM recovery.recovery_control WHERE singleton").Scan(&epoch, &region, &revision, &checkpoint); err != nil || epoch != 1 || region != "local-region" || revision != 1 || checkpoint != bundle.registryDigest {
		t.Fatalf("conflict changed recovery authority: %v", err)
	}
	if _, err := conn.Exec(ctx, "ALTER TABLE recovery.recovery_control RENAME COLUMN epoch TO unavailable_epoch"); err != nil {
		t.Fatal(err)
	}
	if err := bundle.region(ctx, conn, "local-region"); err == nil || !strings.HasPrefix(err.Error(), "recovery region lookup failed") {
		t.Fatalf("unavailable recovery catalog: %v", err)
	}
	if _, err := conn.Exec(ctx, "ALTER TABLE recovery.recovery_control RENAME COLUMN unavailable_epoch TO epoch"); err != nil {
		t.Fatal(err)
	}
	other, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close(ctx)
	if _, err := other.Exec(ctx, "SELECT pg_advisory_lock($1)", production.CoordinatorKey); err != nil {
		t.Fatal(err)
	}
	if err := bundle.region(ctx, conn, "local-region"); err == nil || err.Error() != "migration in progress" {
		t.Fatalf("concurrent region admitted: %v", err)
	}
	if _, err := other.Exec(ctx, "SELECT pg_advisory_unlock($1)", production.CoordinatorKey); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "expected.json")
	if _, err := bundle.expected(ctx, conn, "unregistered", roles[0].RuntimeRole, "local-region", path); err == nil || err.Error() != "service and runtime role are not registered together" {
		t.Fatalf("unregistered readiness: %v", err)
	}
	if _, err := bundle.expected(ctx, conn, roles[0].Service, roles[0].RuntimeRole, "other-region", path); err == nil || err.Error() != "catalog or recovery authority is unavailable or mismatched" {
		t.Fatalf("foreign readiness region: %v", err)
	}
	digest, err := bundle.expected(ctx, conn, roles[0].Service, roles[0].RuntimeRole, "local-region", path)
	if err != nil {
		t.Fatal(err)
	}
	if err := check(ctx, "port=invalid", path, digest); err == nil || err.Error() != "runtime connection configuration is invalid" {
		t.Fatalf("runtime configuration: %v", err)
	}
	if err := check(ctx, dsn, path, strings.Repeat("0", 64)); err == nil || err.Error() != "expected schema document or digest is invalid" {
		t.Fatalf("digest binding: %v", err)
	}
	if err := check(ctx, dsn, filepath.Join(t.TempDir(), "missing.json"), digest); err == nil || err.Error() != "expected schema file is unavailable" {
		t.Fatalf("missing schema: %v", err)
	}
	major := bundle.registry.ServerMajorVersion
	bundle.registry.ServerMajorVersion = 1
	for _, apply := range []func() error{func() error { return bundle.apply(ctx, conn) }, func() error { return bundle.applyInternal(ctx, conn, false) }} {
		if err := apply(); err == nil || err.Error() != "database server major version differs from the approved registry" {
			t.Fatalf("server compatibility: %v", err)
		}
	}
	bundle.registry.ServerMajorVersion = major
	if err := bundle.applyInternal(ctx, conn, true); err == nil || err.Error() != "approval requires an empty release database" {
		t.Fatalf("approval on populated database: %v", err)
	}
}

func TestSchemaDatabaseOperationsRejectClosedConnections(t *testing.T) {
	ctx := context.Background()
	tag := testTag(t)
	directory, roles := approvedBundle(t, tag, false, "")
	_, conn := emptyDatabase(t, tag, roles)
	bundle, err := loadBundle(directory)
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(ctx); err != nil {
		t.Fatal(err)
	}
	for name, run := range map[string]func() error{
		"server version":        func() error { _, err := serverMajorVersion(ctx, conn); return err },
		"role authority":        func() error { return ensureRole(ctx, conn, "ranking_owner", true) },
		"history catalog":       func() error { return ensureHistory(ctx, conn, "core/ranking-algo", "ranking") },
		"owner interfaces":      func() error { return bundle.ownerInterfaces(ctx, conn, "ranking_owner") },
		"apply":                 func() error { return bundle.apply(ctx, conn) },
		"apply validation":      func() error { return bundle.applyInternal(ctx, conn, false) },
		"approval":              func() error { return bundle.approve(ctx, conn, directory) },
		"approval finalization": func() error { return bundle.finalizeApproval(ctx, conn) },
		"legacy history": func() error {
			return bundle.importHistory(ctx, conn, &legacyHistory{Service: "media/user", Schema: "users", Sources: []legacySource{{OriginalVersion: 1}}}, false)
		},
		"statement": func() error { return execute(ctx, conn, "operation refused", "SELECT 1") },
	} {
		t.Run(name, func(t *testing.T) {
			if err := run(); err == nil || strings.Contains(err.Error(), conn.Config().Password) && conn.Config().Password != "" {
				t.Fatalf("closed connection: %v", err)
			}
		})
	}
}

func TestAppliedLedgerRequiresExactCatalogAndPredecessors(t *testing.T) {
	ctx := context.Background()
	tag := testTag(t)
	directory, roles := approvedBundle(t, tag, false, "")
	_, conn := emptyDatabase(t, tag, roles)
	bundle, err := loadBundle(directory)
	if err != nil {
		t.Fatal(err)
	}
	if err := bundle.apply(ctx, conn); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"unregistered migration", "unapplied predecessor", "incomplete release"} {
		t.Run(mode, func(t *testing.T) {
			copyBundle := *bundle
			copyBundle.catalog = make(map[int64]production.MigrationManifest)
			for id, manifest := range bundle.catalog {
				copyBundle.catalog[id] = manifest
			}
			want := "migration ledger differs from the approved registry"
			switch mode {
			case "unregistered migration":
				delete(copyBundle.catalog, 440000)
			case "unapplied predecessor":
				manifest := copyBundle.catalog[440000]
				manifest.PredecessorIDs = []int64{999999}
				copyBundle.catalog[440000] = manifest
				copyBundle.catalog[999999] = production.MigrationManifest{MigrationID: 999999, SHA256: strings.Repeat("a", 64)}
				want = "migration ledger has an unapplied predecessor"
			case "incomplete release":
				copyBundle.catalog[999999] = production.MigrationManifest{MigrationID: 999999, SHA256: strings.Repeat("a", 64)}
				want = "registered migrations are not all applied"
			}
			if _, err := copyBundle.applied(ctx, conn, true); err == nil || err.Error() != want {
				t.Fatalf("ledger admission: %v", err)
			}
		})
	}
}
