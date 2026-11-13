package auth

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestIdentityLedgerRevocationAndOneTimeReauth(t *testing.T) {
	pool, _ := phase2AuthDatabase(t)
	ctx := context.Background()
	accountID, sessionID := uuid.NewString(), uuid.NewString()
	wallet := strings.ToLower(goldenAddr)
	if _, err := pool.Exec(ctx, `INSERT INTO identity_accounts VALUES($1,0,clock_timestamp())`, accountID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO identity_sessions VALUES($1,$2,$3,0,$4,$4,clock_timestamp(),clock_timestamp()+interval '1 hour',clock_timestamp()+interval '1 day',clock_timestamp(),clock_timestamp(),'ledger',NULL)`, sessionID, accountID, wallet, []byte("session-secret-digest")); err != nil {
		t.Fatal(err)
	}
	proof := strings.Repeat("a", 64)
	proofHash := sha256.Sum256([]byte(proof))
	if _, err := pool.Exec(ctx, `INSERT INTO identity_reauth(proof_hash,account_id,session_id,action,issued_at,expires_at) VALUES($1,$2,$3,'wallet_unlink',clock_timestamp(),clock_timestamp()+interval '1 minute')`, proofHash[:], accountID, sessionID); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := ConsumeIdentityReauthTx(ctx, tx, accountID, sessionID, "wallet_unlink", "bad"); !errors.Is(err, ErrScopeDenied) {
		t.Fatalf("malformed reauthentication proof: %v", err)
	}
	if err := ConsumeIdentityReauthTx(ctx, tx, accountID, sessionID, "wallet_unlink", proof); err != nil {
		t.Fatalf("bound reauthentication proof: %v", err)
	}
	if err := ConsumeIdentityReauthTx(ctx, tx, accountID, sessionID, "wallet_unlink", proof); !errors.Is(err, ErrScopeDenied) {
		t.Fatalf("reused reauthentication proof: %v", err)
	}
	if err := RevokeIdentityAccountTx(ctx, tx, accountID, "wallet_unlink"); err != nil {
		t.Fatalf("account revocation: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var epoch, revoked, outbox int64
	if err := pool.QueryRow(ctx, `SELECT a.auth_epoch, (SELECT count(*) FROM identity_sessions s WHERE s.account_id=a.account_id AND s.revoked_at IS NOT NULL), (SELECT count(*) FROM identity_outbox o WHERE o.account_id=a.account_id AND o.action='wallet_unlink') FROM identity_accounts a WHERE a.account_id=$1`, accountID).Scan(&epoch, &revoked, &outbox); err != nil || epoch != 1 || revoked != 1 || outbox != 1 {
		t.Fatalf("revoked account state: epoch=%d sessions=%d outbox=%d err=%v", epoch, revoked, outbox, err)
	}
	if err := RevokeIdentityAccountTx(ctx, nil, accountID, "wallet_unlink"); !errors.Is(err, ErrScopeDenied) {
		t.Fatalf("transaction requirement: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE identity_accounts SET auth_epoch=9223372036854775807 WHERE account_id=$1`, accountID); err != nil {
		t.Fatal(err)
	}
	overflow, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = overflow.Rollback(context.Background()) }()
	if err := RevokeIdentityAccountTx(ctx, overflow, accountID, "wallet_unlink"); err == nil {
		t.Fatal("account epoch overflow accepted")
	}
	if err := overflow.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		t.Fatal(err)
	}
}
