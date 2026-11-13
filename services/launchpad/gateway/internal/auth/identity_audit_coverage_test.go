package auth

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestIdentityAuditRecordsBoundSessionActions(t *testing.T) {
	f := fixtureIdentity(t)
	_, owner := f.login(t)
	f.s.securityAuditKey = []byte(strings.Repeat("a", 32))
	_, err := f.pool.Exec(f.ctx, `CREATE TABLE security_audit_outbox(
		request_id uuid PRIMARY KEY, actor_pseudonym text NOT NULL,
		action text NOT NULL, result text NOT NULL, policy_version text NOT NULL,
		request_digest text NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		action, result string
		lookup         bool
	}{
		{"session_logout", "accepted", false},
		{"session_logout_denied", "denied", true},
	} {
		t.Run(tc.action, func(t *testing.T) {
			principal := owner
			if tc.lookup {
				principal.wallet = ""
			}
			tx, err := f.pool.Begin(f.ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(context.Background()) }()
			if err := f.s.audit(f.ctx, tx, principal, tc.action); err != nil {
				t.Fatalf("audit action: %v", err)
			}
			if err := tx.Commit(f.ctx); err != nil {
				t.Fatal(err)
			}
			var result, pseudonym, digest string
			if err := f.pool.QueryRow(f.ctx, `SELECT result,actor_pseudonym,request_digest FROM security_audit_outbox WHERE action=$1`, tc.action).Scan(&result, &pseudonym, &digest); err != nil {
				t.Fatal(err)
			}
			if result != tc.result || len(pseudonym) != 64 || len(digest) != 64 || strings.Contains(pseudonym, owner.Wallet()) {
				t.Fatalf("audit projection: result=%q pseudonym=%q digest=%q", result, pseudonym, digest)
			}
			var outboxCount int
			if err := f.pool.QueryRow(f.ctx, `SELECT count(*) FROM identity_outbox WHERE action=$1`, tc.action).Scan(&outboxCount); err != nil || outboxCount != 1 {
				t.Fatalf("identity outbox: count=%d err=%v", outboxCount, err)
			}
		})
	}
	missing := owner
	missing.wallet = ""
	missing.sessionID = "00000000-0000-4000-8000-000000000000"
	tx, err := f.pool.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := f.s.audit(f.ctx, tx, missing, "session_logout"); err == nil {
		t.Fatal("unknown session produced an audit row")
	}
	if err := tx.Rollback(f.ctx); err != nil && err != pgx.ErrTxClosed {
		t.Fatal(err)
	}
}
