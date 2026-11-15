package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/jackc/pgx/v5"
)

func TestUsageErrorsExitTwo(t *testing.T) {
	t.Setenv("LAUNCHPAD_ADMIN_DATABASE_URL", "")
	t.Setenv("LAUNCHPAD_RUNTIME_DATABASE_URL", "")
	for _, args := range [][]string{
		nil, {"unknown"}, {"apply"}, {"apply", "--migrations"},
		{"apply", "--migrations", "migrations", "extra"},
		{"apply", "--password", "private-value"},
		{"expected"}, {"expected", "--service", "core/ranking-algo"},
		{"expected", "--service", "core/ranking-algo", "--runtime-role", "invalid role", "--region", "local-region", "--out", "expected.json"},
		{"expected", "--service", "core/ranking-algo", "--runtime-role", "ranking_runtime", "--region", "!", "--out", "expected.json"},
		{"check"}, {"check", "--expected", "expected.json"},
		{"check", "--expected", "expected.json", "--sha256", "invalid"},
		{"apply", "--migrations", "migrations"},
		{"expected", "--service", "core/ranking-algo", "--runtime-role", "ranking_runtime", "--region", "local-region", "--out", "expected.json"},
		{"check", "--expected", "expected.json", "--sha256", strings.Repeat("a", 64)},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(context.Background(), args, os.Getenv, &stdout, &stderr); code != 2 {
			t.Fatalf("usage exit = %d, want 2", code)
		}
		if stdout.Len() != 0 || strings.Count(stderr.String(), "\n") != 1 || strings.Contains(stderr.String(), "private-value") {
			t.Fatal("usage must write one credential-free line to stderr")
		}
	}
}

func TestRepositoryRegistryRequiresApproval(t *testing.T) {
	root := coordinatortest.RepositoryRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "migrations", "registry.json"))
	if err != nil {
		t.Fatal(err)
	}
	var registry production.MigrationRegistry
	if err := json.Unmarshal(raw, &registry); err != nil {
		t.Fatal(err)
	}
	if validDigest(registry.ApprovalDigest) {
		if _, err := loadBundle(filepath.Join(root, "migrations")); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Setenv("LAUNCHPAD_ADMIN_DATABASE_URL", "password=private-value port=invalid")
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"apply", "--migrations", filepath.Join(root, "migrations")}, os.Getenv, &stdout, &stderr)
	if code != 1 || stdout.Len() != 0 || stderr.String() != "registry.json requires an approved digest and complete dependency order\n" {
		t.Fatalf("unapproved registry refusal exit = %d", code)
	}
}

func TestConnectionErrorsDoNotExposeCredentials(t *testing.T) {
	directory, _ := approvedBundle(t, "credential_test", false, "")
	dsn := "password=private-value port=invalid"
	t.Setenv("LAUNCHPAD_ADMIN_DATABASE_URL", dsn)
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"apply", "--migrations", directory}, os.Getenv, &stdout, &stderr)
	if code != 1 || stdout.Len() != 0 || strings.Count(stderr.String(), "\n") != 1 || strings.Contains(stderr.String(), "private-value") || strings.Contains(stderr.String(), dsn) {
		t.Fatal("connection refusal must not expose credentials")
	}
}

// approvedBundle supplies a separate, explicitly approved release of unchanged
// repository SQL. Its manifests exercise the same coordinator admission as an
// operator release, with fixed reader and lock requirements.
func approvedBundle(t *testing.T, tag string, includeLegacy bool, fingerprint string) (string, []production.RoleManifest) {
	t.Helper()
	root := coordinatortest.RepositoryRoot(t)
	directory := filepath.Join(t.TempDir(), "migrations")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	readiness := production.Grant{Schema: "migration_control", Object: "readiness", ObjectKind: "function", Privileges: []string{"EXECUTE"}, ArgumentTypes: []string{"text"}}
	epoch := production.Grant{Schema: "recovery", Object: "current_epoch", ObjectKind: "function", Privileges: []string{"EXECUTE"}}
	roles := []production.RoleManifest{
		{Service: "core/ranking-algo", Schema: "ranking", OwnerRole: "ranking_owner", RuntimeRole: tag + "_ranking", Grants: []production.Grant{{Schema: "ranking", Object: "snapshots", Privileges: []string{"SELECT", "INSERT"}}, readiness, epoch}},
		{Service: "platform/recovery", Schema: "recovery", OwnerRole: "recovery_owner", RuntimeRole: tag + "_recovery", Grants: []production.Grant{readiness, epoch}},
	}
	if includeLegacy {
		roles = append(roles, production.RoleManifest{Service: "media/user", Schema: "users", OwnerRole: "users_owner", RuntimeRole: tag + "_users", Grants: []production.Grant{{Schema: "users", Object: "identity_accounts", Privileges: []string{"SELECT"}}, readiness, epoch}})
	}
	grants, err := json.Marshal(struct {
		SchemaVersion int                       `json:"schema_version"`
		Roles         []production.RoleManifest `json:"roles"`
	}{2, roles})
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(directory, "grants.json"), grants)
	raw, err := os.ReadFile(filepath.Join(root, "migrations", "registry.json"))
	if err != nil {
		t.Fatal(err)
	}
	var reviewed production.MigrationRegistry
	if err := json.Unmarshal(raw, &reviewed); err != nil {
		t.Fatal(err)
	}
	approval := production.Checksum([]byte("launchpad-schema database test release"))
	registry := production.MigrationRegistry{SchemaVersion: 2, ApprovalDigest: approval}
	ids := []int64{440000, 440001, 190000}
	if includeLegacy {
		ids = append(ids, 330000)
	}
	for _, id := range ids {
		var m production.MigrationManifest
		for _, candidate := range reviewed.Registrations {
			if candidate.MigrationID == id {
				m = candidate
				break
			}
		}
		if m.MigrationID != id {
			t.Fatalf("migration %d is not registered", id)
		}
		m.InterfaceDigest = production.Checksum(grants)
		m.ApprovalManifestDigest = approval
		m.ReaderMinVersion = 1
		m.EstimatedLockMS = 2000
		for i := range m.LegacyPredecessors {
			m.LegacyPredecessors[i].StructuralFingerprint = fingerprint
		}
		sql, err := coordinatortest.ReadMigrationSource(root, m.Path, m.SHA256)
		if err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, filepath.Join(filepath.Dir(directory), m.Path), sql)
		registry.ApplicationOrder = append(registry.ApplicationOrder, id)
		registry.Registrations = append(registry.Registrations, m)
	}
	raw, err = json.Marshal(registry)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(directory, "registry.json"), raw)
	histories := []legacyHistory{}
	if includeLegacy {
		raw, err := os.ReadFile(filepath.Join(root, "migrations", "legacy-history-aliases.json"))
		if err != nil {
			t.Fatal(err)
		}
		var original struct {
			Histories []legacyHistory `json:"histories"`
		}
		if err := json.Unmarshal(raw, &original); err != nil {
			t.Fatal(err)
		}
		for _, h := range original.Histories {
			if h.Service != "media/user" {
				continue
			}
			for i := range h.Sources {
				h.Sources[i].StructuralFingerprint = fingerprint
				source, err := coordinatortest.ReadMigrationSource(root, h.Sources[i].Path, h.Sources[i].SourceSHA256)
				if err != nil {
					t.Fatal(err)
				}
				writeTestFile(t, filepath.Join(filepath.Dir(directory), h.Sources[i].Path), source)
			}
			histories = append(histories, h)
		}
		if len(histories) != 1 {
			t.Fatal("user legacy history is not registered")
		}
	}
	raw, err = json.Marshal(struct {
		SchemaVersion int             `json:"schema_version"`
		Histories     []legacyHistory `json:"histories"`
	}{2, histories})
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(directory, "legacy-history-aliases.json"), raw)
	return directory, roles
}

func writeTestFile(t *testing.T, path string, raw []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func testTag(t *testing.T) string {
	t.Helper()
	var random [6]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	return "schema_" + hex.EncodeToString(random[:])
}

func emptyDatabase(t *testing.T, tag string, roles []production.RoleManifest) (string, *pgx.Conn) {
	t.Helper()
	admin := os.Getenv("COORDINATOR_TEST_POSTGRES_DSN")
	if admin == "" {
		t.Skip("COORDINATOR_TEST_POSTGRES_DSN is not set: configure a running PostgreSQL whose role may CREATE DATABASE and CREATE ROLE; tests never start containers")
	}
	ctx := context.Background()
	control, err := pgx.Connect(ctx, admin)
	if err != nil {
		t.Fatal("administrator connection failed")
	}
	t.Cleanup(func() { _ = control.Close(ctx) })
	names := map[string]bool{"migration_control_owner": true}
	for _, m := range roles {
		names[m.OwnerRole], names[m.RuntimeRole] = true, true
	}
	created := []string{}
	for role := range names {
		var exists bool
		if err := control.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname=$1)", role).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if !exists {
			created = append(created, role)
		}
	}
	if _, err := control.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{tag}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := control.Exec(ctx, "DROP DATABASE "+pgx.Identifier{tag}.Sanitize()+" WITH (FORCE)"); err != nil {
			t.Error(err)
		}
		for _, role := range created {
			if _, err := control.Exec(ctx, "DROP ROLE IF EXISTS "+pgx.Identifier{role}.Sanitize()); err != nil {
				t.Error(err)
			}
		}
	})
	parsed, err := url.Parse(admin)
	if err != nil || parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
		t.Fatal("administrator connection must be a PostgreSQL URL")
	}
	parsed.Path = "/" + tag
	dsn := parsed.String()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal("private database connection failed")
	}
	t.Cleanup(func() { _ = conn.Close(ctx) })
	var empty bool
	if err := conn.QueryRow(ctx, "SELECT to_regnamespace('migration_control') IS NULL").Scan(&empty); err != nil || !empty {
		t.Fatal("private database is not empty")
	}
	return dsn, conn
}

func command(t *testing.T, args ...string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), args, os.Getenv, &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("%s exit %d: %s", args[0], code, stderr.String())
	}
	if args[0] != "expected" && stdout.Len() != 0 {
		t.Fatal("command printed unexpected output")
	}
	return stdout.String()
}

func databaseSnapshot(t *testing.T, conn *pgx.Conn) string {
	t.Helper()
	var snapshot string
	err := conn.QueryRow(context.Background(), `SELECT jsonb_build_object(
 'migrations',(SELECT jsonb_agg(to_jsonb(m) ORDER BY migration_id) FROM migration_control.applied_migrations m),
 'baselines',(SELECT jsonb_agg(to_jsonb(b) ORDER BY service,original_version) FROM migration_control.legacy_baselines b),
 'readiness',(SELECT jsonb_agg(to_jsonb(r) ORDER BY role_name,service) FROM migration_control.runtime_readiness r),
 'relations',(SELECT jsonb_agg(jsonb_build_array(c.oid,c.relname,c.relowner,c.relacl) ORDER BY c.oid) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname IN ('ranking','recovery','users','migration_control')),
 'functions',(SELECT jsonb_agg(jsonb_build_array(p.oid,p.proname,p.proowner,p.proacl) ORDER BY p.oid) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname IN ('ranking','recovery','users','migration_control')),
 'namespaces',(SELECT jsonb_agg(jsonb_build_array(n.oid,n.nspname,n.nspowner,n.nspacl) ORDER BY n.oid) FROM pg_namespace n WHERE n.nspname IN ('ranking','recovery','users','migration_control')),
 'defaults',(SELECT jsonb_agg(to_jsonb(d) ORDER BY d.oid) FROM pg_default_acl d)
 )::text`).Scan(&snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestCommandApplyExpectedCheck(t *testing.T) {
	tag := testTag(t)
	directory, roles := approvedBundle(t, tag, false, "")
	dsn, conn := emptyDatabase(t, tag, roles)
	t.Setenv("LAUNCHPAD_ADMIN_DATABASE_URL", dsn)
	command(t, "apply", "--migrations", directory)
	var count int
	if err := conn.QueryRow(context.Background(), "SELECT count(*) FROM migration_control.applied_migrations").Scan(&count); err != nil || count != 3 {
		t.Fatalf("applied migration count = %d: %v", count, err)
	}
	checkpoint := production.Checksum([]byte("restoration checkpoint"))
	if _, err := conn.Exec(context.Background(), "INSERT INTO recovery.recovery_control(singleton,epoch,region,revision,checkpoint_sha256) VALUES(true,7,'local-region',1,$1)", checkpoint); err != nil {
		t.Fatal(err)
	}
	paths, digests := []string{}, []string{}
	for _, m := range roles {
		path := filepath.Join(t.TempDir(), "expected.json")
		printed := command(t, "expected", "--migrations", directory, "--service", m.Service, "--runtime-role", m.RuntimeRole, "--region", "local-region", "--out", path)
		digest := strings.TrimSuffix(printed, "\n")
		if !validDigest(digest) || printed != digest+"\n" {
			t.Fatal("expected must print only its digest")
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		want, err := production.LoadExpectedSchema(raw, digest)
		if err != nil {
			t.Fatal(err)
		}
		if want.Service != m.Service || want.RuntimeRole != m.RuntimeRole || want.Epoch != 7 || want.Region != "local-region" || len(want.Migrations) != 3 {
			t.Fatal("expected schema did not preserve the database authority")
		}
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatal("expected schema must have private file permissions")
		}
		var random [24]byte
		if _, err := rand.Read(random[:]); err != nil {
			t.Fatal(err)
		}
		password := hex.EncodeToString(random[:])
		var literal string
		if err := conn.QueryRow(context.Background(), "SELECT quote_literal($1)", password).Scan(&literal); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(context.Background(), "ALTER ROLE "+pgx.Identifier{m.RuntimeRole}.Sanitize()+" LOGIN PASSWORD "+literal); err != nil {
			t.Fatal("runtime credential provisioning failed")
		}
		runtimeURL, err := url.Parse(dsn)
		if err != nil {
			t.Fatal("runtime connection URL is invalid")
		}
		runtimeURL.User = url.UserPassword(m.RuntimeRole, password)
		t.Setenv("LAUNCHPAD_RUNTIME_DATABASE_URL", runtimeURL.String())
		command(t, "check", "--expected", path, "--sha256", digest)
		paths, digests = append(paths, path), append(digests, digest)
	}
	before := databaseSnapshot(t, conn)
	command(t, "apply", "--migrations", directory)
	if after := databaseSnapshot(t, conn); after != before {
		t.Fatal("second apply changed database objects, grants or ledger records")
	}
	if _, err := conn.Exec(context.Background(), "ALTER TABLE ranking.snapshots ADD COLUMN operator_note text"); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	// The administrator can authenticate and delegate to the exact runtime role.
	runtimeURL, err := url.Parse(dsn)
	if err != nil {
		t.Fatal("runtime connection URL is invalid")
	}
	query := runtimeURL.Query()
	query.Set("options", "-c role="+roles[0].RuntimeRole)
	runtimeURL.RawQuery = query.Encode()
	t.Setenv("LAUNCHPAD_RUNTIME_DATABASE_URL", runtimeURL.String())
	if code := run(context.Background(), []string{"check", "--expected", paths[0], "--sha256", digests[0]}, os.Getenv, &stdout, &stderr); code != 1 || stderr.String() != "structural fingerprint mismatch\n" || stdout.Len() != 0 {
		t.Fatalf("schema drift exit %d: %s", code, stderr.String())
	}
}

func TestApplyImportsLegacyBaseline(t *testing.T) {
	tag := testTag(t)
	_, basicRoles := approvedBundle(t, tag, false, "")
	roles := append(basicRoles, production.RoleManifest{OwnerRole: "users_owner", RuntimeRole: tag + "_users"})
	dsn, conn := emptyDatabase(t, tag, roles)
	ctx := context.Background()
	if err := ensureRole(ctx, conn, "users_owner", true); err != nil {
		t.Fatal(err)
	}
	root := coordinatortest.RepositoryRoot(t)
	full, err := coordinatortest.ReadMigrationSource(root, "services/launchpad/user/migrations/00001_init.sql", "")
	if err != nil {
		t.Fatal(err)
	}
	up := strings.SplitN(string(full), "-- +goose Down", 2)[0]
	if _, err := conn.Exec(ctx, "GRANT CREATE ON DATABASE "+pgx.Identifier{tag}.Sanitize()+" TO users_owner; SET ROLE users_owner; "+up+"; RESET ROLE; REVOKE CREATE ON DATABASE "+pgx.Identifier{tag}.Sanitize()+" FROM users_owner"); err != nil {
		t.Fatal(err)
	}
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, err := production.CatalogFingerprint(ctx, tx, "users")
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	directory, _ := approvedBundle(t, tag, true, fingerprint)
	// Preserve the service's exact original version proof before admission.
	if err := ensureRole(ctx, conn, "migration_control_owner", true); err != nil {
		t.Fatal(err)
	}
	ddl := production.MigrationControlDDL()
	if _, err := conn.Exec(ctx, ddl[strings.Index(ddl, ";")+1:]); err != nil {
		t.Fatal(err)
	}
	if err := ensureHistory(ctx, conn, "media/user", "users"); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, "INSERT INTO migration_control.user_versions(version_id,is_applied) VALUES(1,true)"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LAUNCHPAD_ADMIN_DATABASE_URL", dsn)
	command(t, "apply", "--migrations", directory)
	var source, upDigest, actual string
	if err := conn.QueryRow(ctx, "SELECT source_checksum,up_checksum,structural_fingerprint FROM migration_control.legacy_baselines WHERE service='media/user' AND original_version=1").Scan(&source, &upDigest, &actual); err != nil {
		t.Fatal(err)
	}
	if source != production.Checksum(full) || upDigest != production.Checksum([]byte(up)) || actual != fingerprint {
		t.Fatal("imported baseline differs from the exact source and reviewed structure")
	}
	before := databaseSnapshot(t, conn)
	command(t, "apply", "--migrations", directory)
	if databaseSnapshot(t, conn) != before {
		t.Fatal("recorded baseline changed on second apply")
	}
	// The same approved source initializes a fresh database before registration.
	freshDSN, fresh := emptyDatabase(t, testTag(t), roles)
	t.Setenv("LAUNCHPAD_ADMIN_DATABASE_URL", freshDSN)
	command(t, "apply", "--migrations", directory)
	var imported int
	if err := fresh.QueryRow(ctx, "SELECT count(*) FROM migration_control.legacy_baselines WHERE service='media/user' AND structural_fingerprint=$1", fingerprint).Scan(&imported); err != nil || imported != 1 {
		t.Fatalf("fresh baseline import = %d: %v", imported, err)
	}
}

func TestMigrationSourceChecksExactBytes(t *testing.T) {
	directory, _ := approvedBundle(t, "source_test", false, "")
	path := filepath.Join(directory, "190000.sql")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, path, append(raw, []byte("\nSELECT 1;\n")...))
	if _, err := loadBundle(directory); err == nil {
		t.Fatal("changed registered SQL was admitted")
	}
	if _, err := coordinatortest.ReadMigrationSource(filepath.Dir(directory), "../unbound.sql", ""); err == nil {
		t.Fatal("source outside the repository was admitted")
	}
}
