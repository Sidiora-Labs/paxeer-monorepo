package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPostgresIdentityResolverBindsCurrentSession(t *testing.T) {
	pool, _ := phase2AuthDatabase(t)
	ctx := context.Background()
	accountID, sessionID := uuid.NewString(), uuid.NewString()
	wallet := strings.ToLower(goldenAddr)
	if _, err := pool.Exec(ctx, `INSERT INTO identity_accounts VALUES($1,2,clock_timestamp())`, accountID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO identity_wallet_links VALUES($1,$2,125,$3,'private','verified',clock_timestamp())`, uuid.NewString(), accountID, wallet); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO identity_sessions VALUES($1,$2,$3,2,$4,$4,clock_timestamp(),clock_timestamp()+interval '1 hour',clock_timestamp()+interval '1 day',clock_timestamp(),clock_timestamp(),'coverage',NULL)`, sessionID, accountID, wallet, []byte("session-secret-digest")); err != nil {
		t.Fatal(err)
	}
	principal := Principal{SessionID: sessionID, Wallet: wallet}
	resolver := PostgresIdentityResolver{Pool: pool}
	identity, err := resolver.Resolve(ctx, principal)
	if err != nil || identity.AccountID != accountID || identity.SessionID != sessionID || identity.Wallet != wallet || identity.AuthEpoch != 2 || identity.ProofAt.IsZero() || !identity.AccessExpiresAt.After(time.Now()) {
		t.Fatalf("current identity = %+v, %v", identity, err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	locked, err := resolver.ResolveTx(ctx, tx, principal)
	if err != nil || locked.AccountID != identity.AccountID || locked.SessionID != identity.SessionID || locked.AuthEpoch != identity.AuthEpoch {
		t.Fatalf("locked identity = %+v, %v", locked, err)
	}
	if _, err := resolver.ResolveTx(ctx, tx, Principal{SessionID: uuid.NewString(), Wallet: wallet}); !errors.Is(err, ErrScopeDenied) {
		t.Fatalf("unregistered session = %v", err)
	}
	if _, err := resolver.ResolveTx(ctx, tx, Principal{SessionID: sessionID, Wallet: "not-a-wallet"}); !errors.Is(err, ErrScopeDenied) {
		t.Fatalf("invalid wallet = %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := resolver.Resolve(ctx, Principal{SessionID: uuid.NewString(), Wallet: wallet}); !errors.Is(err, ErrScopeDenied) {
		t.Fatalf("unregistered current session = %v", err)
	}
	guarded := PostgresIdentityResolver{Pool: pool, GuardFunction: "users.identity_guard_v2"}
	guardedIdentity, err := guarded.Resolve(ctx, principal)
	if err != nil || guardedIdentity.AccountID != accountID || guardedIdentity.AuthEpoch != identity.AuthEpoch {
		t.Fatalf("guarded identity = %+v, %v", guardedIdentity, err)
	}
	if _, err := (PostgresIdentityResolver{Pool: pool, GuardFunction: "users.other_guard"}).Resolve(ctx, principal); !errors.Is(err, ErrScopeDenied) {
		t.Fatalf("unapproved identity guard = %v", err)
	}
	if _, err := (PostgresIdentityResolver{GuardFunction: "users.identity_guard_v2"}).Resolve(ctx, principal); err == nil {
		t.Fatal("guarded identity resolved without a database")
	}
	if _, err := resolver.Resolve(ctx, Principal{SessionID: sessionID, Wallet: "not-a-wallet"}); !errors.Is(err, ErrScopeDenied) {
		t.Fatalf("invalid current wallet = %v", err)
	}
	if _, err := resolver.ResolveTx(ctx, nil, principal); !errors.Is(err, ErrScopeDenied) {
		t.Fatalf("missing transaction = %v", err)
	}
}
