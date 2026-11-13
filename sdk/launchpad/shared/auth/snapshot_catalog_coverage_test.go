package auth

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/jackc/pgx/v5"
)

func TestAuthorizationSnapshotsRejectUnavailableCatalogFields(t *testing.T) {
	ctx := context.Background()
	pool, _ := auditCoverageDatabase(t)
	req, verify := signedOperation(t)
	ledger := Ledger{Pool: pool, Environment: "test", AuditKey: bytes.Repeat([]byte("a"), 32)}
	if _, _, err := ledger.ExecuteValidated(ctx, req, verify, func(context.Context, pgx.Tx, []byte) ([]byte, error) { return []byte(`{}`), nil }); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, sql, restore string }{
		{"ledger query", "ALTER TABLE operation_ledger RENAME COLUMN envelope_digest TO unbound_digest", "ALTER TABLE operation_ledger RENAME COLUMN unbound_digest TO envelope_digest"},
		{"ledger scan", "ALTER TABLE operation_ledger ALTER COLUMN retain_until DROP NOT NULL;UPDATE operation_ledger SET retain_until=NULL", "UPDATE operation_ledger SET retain_until=created_at+interval '24 hours';ALTER TABLE operation_ledger ALTER COLUMN retain_until SET NOT NULL"},
		{"audit query", "ALTER TABLE security_audit_outbox RENAME COLUMN actor_pseudonym TO unbound_actor", "ALTER TABLE security_audit_outbox RENAME COLUMN unbound_actor TO actor_pseudonym"},
		{"audit scan", "ALTER TABLE security_audit_outbox ALTER COLUMN actor_pseudonym DROP NOT NULL;UPDATE security_audit_outbox SET actor_pseudonym=NULL", "UPDATE security_audit_outbox SET actor_pseudonym='" + strings.Repeat("a", 64) + "';ALTER TABLE security_audit_outbox ALTER COLUMN actor_pseudonym SET NOT NULL"},
		{"audit join", "ALTER TABLE security_audit_outbox RENAME COLUMN result TO unbound_result", "ALTER TABLE security_audit_outbox RENAME COLUMN unbound_result TO result"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := pool.Exec(ctx, tc.sql); err != nil {
				t.Fatal(err)
			}
			if _, err := ReadAuthStateSnapshot(ctx, pool); err == nil {
				t.Fatal("unavailable catalog certified authorization state")
			}
			if _, err := pool.Exec(ctx, tc.restore); err != nil {
				t.Fatal(err)
			}
		})
	}
	if snapshot, err := ReadAuthStateSnapshot(ctx, pool); err != nil || snapshot.LedgerRecords != 1 || snapshot.AuditRecords != 1 || snapshot.MissingOperationAudits != 0 {
		t.Fatalf("restored catalog snapshot: %+v %v", snapshot, err)
	}
}

func TestAuditBridgePreservesUnstagedRowsOnCatalogRefusal(t *testing.T) {
	ctx := context.Background()
	pool, schema := auditCoverageDatabase(t)
	ddl, err := delivery.SchemaDDL(schema)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	subject := "security.audit.bound"
	store, err := delivery.NewStore(pool, schema, delivery.Policy{Subjects: map[string]bool{subject: true}, EventTypes: map[string]bool{"security_audit": true}})
	if err != nil {
		t.Fatal(err)
	}
	bridge := AuditBridge{Pool: pool, Delivery: store, Subject: subject, EventType: "security_audit"}
	req, verify := signedOperation(t)
	ledger := Ledger{Pool: pool, Environment: "test", AuditKey: bytes.Repeat([]byte("a"), 32)}
	if _, _, err := ledger.ExecuteValidated(ctx, req, verify, func(context.Context, pgx.Tx, []byte) ([]byte, error) { return []byte(`{}`), nil }); err != nil {
		t.Fatal(err)
	}
	var id int64
	if err := pool.QueryRow(ctx, "SELECT id FROM security_audit_outbox").Scan(&id); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, sql, restore string }{
		{"receipts", "ALTER TABLE security_audit_delivery_receipts RENAME COLUMN audit_id TO unbound_id", "ALTER TABLE security_audit_delivery_receipts RENAME COLUMN unbound_id TO audit_id"},
		{"audit row", "ALTER TABLE security_audit_outbox RENAME COLUMN actor_pseudonym TO unbound_actor", "ALTER TABLE security_audit_outbox RENAME COLUMN unbound_actor TO actor_pseudonym"},
		{"payload", "UPDATE security_audit_outbox SET actor_pseudonym='invalid'", "UPDATE security_audit_outbox SET actor_pseudonym='" + strings.Repeat("a", 64) + "'"},
		{"receipt insert", "ALTER TABLE security_audit_delivery_receipts ADD CHECK(audit_id<0)", "ALTER TABLE security_audit_delivery_receipts DROP CONSTRAINT security_audit_delivery_receipts_audit_id_check"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := pool.Exec(ctx, tc.sql); err != nil {
				t.Fatal(err)
			}
			if staged, err := bridge.stageOne(ctx, id); err == nil || staged {
				t.Fatalf("unstaged audit acknowledged: %t %v", staged, err)
			}
			if tc.name == "receipts" {
				if n, err := bridge.Stage(ctx, 10); err == nil || n != 0 {
					t.Fatalf("unavailable receipt catalog: %d %v", n, err)
				}
			}
			if _, err := pool.Exec(ctx, tc.restore); err != nil {
				t.Fatal(err)
			}
			var n int
			if err := pool.QueryRow(ctx, "SELECT count(*) FROM security_audit_delivery_receipts").Scan(&n); err != nil || n != 0 {
				t.Fatalf("refused audit receipt: %d %v", n, err)
			}
		})
	}
	denied := bridge
	denied.EventType = "unapproved"
	if staged, err := denied.stageOne(ctx, id); err == nil || staged {
		t.Fatalf("unauthorized audit envelope: %t %v", staged, err)
	}
	if staged, err := bridge.stageOne(ctx, id+1); !errors.Is(err, pgx.ErrNoRows) || staged {
		t.Fatalf("missing audit source: %t %v", staged, err)
	}
	if staged, err := bridge.stageOne(ctx, id); err != nil || !staged {
		t.Fatalf("bound audit delivery: %t %v", staged, err)
	}
	if staged, err := bridge.stageOne(ctx, id); err != nil || staged {
		t.Fatalf("duplicate audit delivery: %t %v", staged, err)
	}
}
