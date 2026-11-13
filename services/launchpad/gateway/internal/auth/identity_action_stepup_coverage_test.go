package auth

import (
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestIdentityActionStepUpBindsReceiptToSession(t *testing.T) {
	f := ownerHTTPFixture(t)
	var schema string
	if err := f.pool.QueryRow(f.ctx, `SELECT current_schema()`).Scan(&schema); err != nil {
		t.Fatal(err)
	}
	upgrade := strings.ReplaceAll(IdentityActionReceiptUpgradeDDL, "users.", pgx.Identifier{schema}.Sanitize()+".")
	if _, err := f.pool.Exec(f.ctx, upgrade); err != nil {
		t.Fatal(err)
	}
	_, principal := f.loginHTTP(t)
	if _, err := f.s.IssueActionStepUp(f.ctx, principal, "127.0.0.1", "other"); err == nil {
		t.Fatal("unrecognized action received a step-up challenge")
	}
	if _, err := f.s.IssueActionStepUp(f.ctx, principal, "invalid", accountDeleteScope); err == nil {
		t.Fatal("invalid client address received a step-up challenge")
	}
	challenge, err := f.s.IssueActionStepUp(f.ctx, principal, "127.0.0.1", accountDeleteScope)
	if err != nil || challenge.ChallengeID == "" || challenge.Purpose != "step_up:"+accountDeleteScope || challenge.Message == "" {
		t.Fatalf("action challenge: %+v err=%v", challenge, err)
	}
	proof := proofIdentity(t, f.key, challenge)
	if _, err := f.s.VerifyActionStepUp(f.ctx, principal, challenge.ChallengeID, proof, "other"); err == nil {
		t.Fatal("unrecognized action consumed a step-up challenge")
	}
	receipt, err := f.s.VerifyActionStepUp(f.ctx, principal, challenge.ChallengeID, proof, accountDeleteScope)
	if err != nil || len(receipt) != 64 {
		t.Fatalf("action receipt: length=%d err=%v", len(receipt), err)
	}
	var action string
	if err := f.pool.QueryRow(f.ctx, `SELECT action FROM identity_reauth WHERE account_id=$1 AND session_id=$2`, principal.AccountID(), principal.SessionID()).Scan(&action); err != nil || action != accountDeleteScope {
		t.Fatalf("durable action receipt: action=%q err=%v", action, err)
	}
	if _, err := f.s.VerifyActionStepUp(f.ctx, principal, challenge.ChallengeID, proof, accountDeleteScope); err == nil {
		t.Fatal("action challenge replay issued a second receipt")
	}
}
