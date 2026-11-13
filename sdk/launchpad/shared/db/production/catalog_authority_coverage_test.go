package production

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestCatalogPrivilegesAdmitOnlyReviewedObjectRights(t *testing.T) {
	pool, _ := phase2MigrationFixture(t)
	ctx := context.Background()
	for _, mode := range []string{"table", "column", "function", "sequence", "grant option", "reviewed"} {
		t.Run(mode, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			role := "catalog_" + strings.ReplaceAll(uuid.NewString(), "-", "")
			qrole := pgx.Identifier{role}.Sanitize()
			for _, sql := range []string{"CREATE ROLE " + qrole + " NOLOGIN", "CREATE SCHEMA rights", "CREATE TABLE rights.entries(id bigint,body text)", "CREATE SEQUENCE rights.counter", "CREATE FUNCTION rights.lookup(text) RETURNS text LANGUAGE SQL AS 'SELECT $1'", "REVOKE ALL ON ALL FUNCTIONS IN SCHEMA rights FROM PUBLIC"} {
				if _, err := tx.Exec(ctx, sql); err != nil {
					t.Fatal(err)
				}
			}
			var sql, want string
			switch mode {
			case "table":
				sql = "GRANT SELECT ON rights.entries TO " + qrole
				want = "whole-table"
			case "column":
				sql = "GRANT SELECT(id) ON rights.entries TO " + qrole
				want = "column"
			case "function":
				sql = "GRANT EXECUTE ON FUNCTION rights.lookup(text) TO " + qrole
				want = "function"
			case "sequence":
				sql = "GRANT USAGE ON SEQUENCE rights.counter TO " + qrole
				want = "sequence"
			case "grant option":
				sql = "GRANT SELECT ON rights.entries TO " + qrole + " WITH GRANT OPTION"
				want = "whole-table"
			case "reviewed":
				sql = "GRANT SELECT(id) ON rights.entries TO " + qrole + ";GRANT USAGE ON SEQUENCE rights.counter TO " + qrole + ";GRANT EXECUTE ON FUNCTION rights.lookup(text) TO " + qrole
			}
			if _, err := tx.Exec(ctx, sql); err != nil {
				t.Fatal(err)
			}
			grants := []Grant{}
			if mode == "reviewed" {
				grants = []Grant{{Schema: "rights", Object: "entries", Privileges: []string{"SELECT"}, Columns: []string{"id"}}, {Schema: "rights", Object: "counter", ObjectKind: "sequence", Privileges: []string{"USAGE"}}, {Schema: "rights", Object: "lookup", ObjectKind: "function", Privileges: []string{"EXECUTE"}, ArgumentTypes: []string{"text"}}}
			}
			err = checkUnexpectedPrivileges(ctx, tx, role, grants, map[string]string{"rights": "owner"})
			if mode == "reviewed" {
				if err != nil {
					t.Fatal(err)
				}
				for _, grant := range grants {
					if err := checkPrivilege(ctx, tx, role, grant); err != nil {
						t.Fatal(err)
					}
				}
				if err := checkPrivilege(ctx, tx, role, Grant{Schema: "rights", Object: "entries", Privileges: []string{"SELECT"}, Columns: []string{"body"}}); err == nil {
					t.Fatal("ungranted column admitted")
				}
			} else if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("unreviewed %s: %v", mode, err)
			}
		})
	}
}

func TestMigrationSessionsRejectUnavailableCatalogs(t *testing.T) {
	pool, dsn := phase2MigrationFixture(t)
	ctx := context.Background()
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(ctx) }()
	plan := OnlineIndexPlan{Schema: "stats", Table: "entries", Name: "entries_idx", Columns: []string{"id"}}
	sql, err := plan.CreateSQL()
	if err != nil {
		t.Fatal(err)
	}
	m := phase2RegisteredMigration(70001, sql, nil)
	m.TransactionMode = "nontransactional"
	registry := map[int64]MigrationManifest{m.MigrationID: m}
	for name, call := range map[string]func() error{
		"online plan": func() error { _, err := (OnlineIndexPlan{}).CreateSQL(); return err },
		"online connection": func() error {
			return ApplyOnlineIndexMigration(ctx, nil, m, plan, registry, m.InterfaceDigest, m.ApprovalManifestDigest, 1, nil)
		},
		"online cancelled": func() error {
			return ApplyOnlineIndexMigration(cancelled, conn, m, plan, registry, m.InterfaceDigest, m.ApprovalManifestDigest, 1, nil)
		},
		"inspection cancelled": func() error { _, err := InspectOnlineIndex(cancelled, conn, plan); return err },
		"registered cancelled": func() error {
			return ApplyRegisteredMigration(cancelled, conn, m, []byte(sql), registry, m.InterfaceDigest, m.ApprovalManifestDigest, 1)
		},
		"coordinated connection": func() error {
			return ApplyCoordinatedMigration(ctx, nil, m, []byte(sql), registry, m.InterfaceDigest, m.ApprovalManifestDigest, 1)
		},
		"lock configuration": func() error { _, err := (PostgresCoordinatorLock{}).TryLock(ctx); return err },
		"lock cancelled": func() error {
			_, err := (PostgresCoordinatorLock{Pool: pool, Key: CoordinatorKey, AcquireTimeout: time.Second}).TryLock(cancelled)
			return err
		},
		"numeric budget":    func() error { _, err := SortRawBalances(ctx, pool, make([]string, 10001)); return err },
		"numeric cancelled": func() error { _, err := SortRawBalances(cancelled, pool, []string{"1"}); return err },
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Fatal("unavailable catalog admitted work")
			}
		})
	}
	if conn.IsClosed() {
		conn, err = pgx.Connect(ctx, dsn)
		if err != nil {
			t.Fatal(err)
		}
	}
	lock := PostgresCoordinatorLock{Pool: pool, Key: CoordinatorKey, AcquireTimeout: time.Second}
	lease, err := lock.TryLock(ctx)
	if err != nil {
		t.Fatal(err)
	}
	raw := lease.(*postgresLease)
	if _, err := raw.conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", CoordinatorKey); err != nil {
		t.Fatal(err)
	}
	if err := lease.Release(ctx); err == nil || !strings.Contains(err.Error(), "ownership lost") {
		t.Fatalf("lost lock ownership: %v", err)
	}
	if err := lease.Release(ctx); err != nil {
		t.Fatal(err)
	}
	lease, err = lock.TryLock(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.Release(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled lock release: %v", err)
	}
	if _, err := conn.Exec(ctx, "ALTER TABLE migration_control.applied_migrations RENAME TO archived_migrations"); err != nil {
		t.Fatal(err)
	}
	if err := ApplyOnlineIndexMigration(ctx, conn, m, plan, registry, m.InterfaceDigest, m.ApprovalManifestDigest, 1, nil); err == nil {
		t.Fatal("missing approval catalog admitted index")
	}
	if err := ApplyRegisteredMigration(ctx, conn, m, []byte(sql), registry, m.InterfaceDigest, m.ApprovalManifestDigest, 1); err == nil {
		t.Fatal("missing approval catalog admitted migration")
	}
	if _, err := conn.Exec(ctx, "ALTER TABLE migration_control.archived_migrations RENAME TO applied_migrations"); err != nil {
		t.Fatal(err)
	}
}

func TestOnlineIndexReconcilesInterruptedSessions(t *testing.T) {
	pool, dsn := phase2MigrationFixture(t)
	ctx := context.Background()
	exec(t, pool, "SET ROLE stats_owner;CREATE TABLE stats.entries(id bigint);INSERT INTO stats.entries VALUES(1);RESET ROLE")
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(ctx) }()
	plan := OnlineIndexPlan{Schema: "stats", Table: "entries", Name: "entries_idx", Columns: []string{"id"}}
	sql, err := plan.CreateSQL()
	if err != nil {
		t.Fatal(err)
	}
	m := phase2RegisteredMigration(70001, sql, nil)
	m.TransactionMode = "nontransactional"
	registry := map[int64]MigrationManifest{m.MigrationID: m}
	exec(t, pool, "INSERT INTO migration_control.migration_attempts(migration_id,checksum,registry_digest,state,lock_wait_ms) VALUES(70001,'"+Checksum([]byte("other"))+"','"+m.ApprovalManifestDigest+"','started',0)")
	if err := ApplyOnlineIndexMigration(ctx, conn, m, plan, registry, m.InterfaceDigest, m.ApprovalManifestDigest, 1, nil); err == nil || !strings.Contains(err.Error(), "checksum differs") {
		t.Fatalf("changed interrupted checksum: %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE migration_control.migration_attempts SET checksum=$1", m.SHA256); err != nil {
		t.Fatal(err)
	}
	if err := ApplyOnlineIndexMigration(ctx, conn, m, plan, registry, m.InterfaceDigest, m.ApprovalManifestDigest, 1, nil); !errors.Is(err, ErrInvalidOnlineIndex) {
		t.Fatalf("interrupted index authority: %v", err)
	}
	var state, failure string
	if err := pool.QueryRow(ctx, "SELECT state,failure_class FROM migration_control.migration_attempts WHERE migration_id=70001").Scan(&state, &failure); err != nil || state != "failed" || failure != "interrupted" {
		t.Fatalf("interrupted attempt: %s %s %v", state, failure, err)
	}
	authority := []byte(`{"version":1,"approvals":[{"migrationId":70001,"schema":"stats","index":"entries_idx","operation":"drop_recreate","failedChecksum":"` + m.SHA256 + `","failureClass":"interrupted"}]}`)
	repairs, err := LoadIndexRepairRegistry(authority, Checksum(authority))
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyOnlineIndexMigration(ctx, conn, m, plan, registry, m.InterfaceDigest, m.ApprovalManifestDigest, 1, &repairs); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT state FROM migration_control.migration_attempts WHERE migration_id=70001 ORDER BY attempt DESC LIMIT 1").Scan(&state); err != nil || state != "reconciled" {
		t.Fatalf("reconciled attempt: %s %v", state, err)
	}
}
