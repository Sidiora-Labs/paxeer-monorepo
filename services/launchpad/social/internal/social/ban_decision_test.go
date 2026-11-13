package social

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
)

func TestLegacyWritesFollowCommittedModerationDecision(t *testing.T) {
	db := internaltest.NewPostgres(t)
	store := &safety.Store{DB: db}
	handlers := New(Deps{})
	issuer := "0x1111111111111111111111111111111111111111"
	target := "0x2222222222222222222222222222222222222222"
	pool := "0x3333333333333333333333333333333333333333"
	principal := func(wallet, role string) context.Context {
		return safety.WithPrincipal(WithModeration(context.Background(), store), safety.Principal{Wallet: wallet, SessionID: "moderation-decision", Role: role, Pools: []string{pool}, Expires: time.Now().Add(time.Hour), StepUp: time.Now()})
	}
	actor := principal(target, "participant")
	if banned, err := handlers.checkedBan(actor, target, pool); err != nil || banned {
		t.Fatalf("unrestricted actor = banned %t: %v", banned, err)
	}
	if banned, err := handlers.checkedBan(actor, issuer, pool); banned || !errors.Is(err, safety.ErrForbidden) {
		t.Fatalf("different actor = banned %t: %v", banned, err)
	}
	if _, err := handlers.checkedBan(WithModeration(context.Background(), store), target, pool); !errors.Is(err, safety.ErrForbidden) {
		t.Fatalf("missing principal = %v", err)
	}
	banID, err := store.Ban(principal(issuer, "room_moderator"), safety.BanCommand{Wallet: target, Scope: "pool", Resource: pool, Reason: "reviewed conduct", PolicyVersion: "pool-v2"})
	if err != nil || banID == "" {
		t.Fatalf("committed pool ban = %q: %v", banID, err)
	}
	if banned, err := handlers.checkedBan(actor, target, pool); err != nil || !banned || !handlers.isBanned(actor, target, pool) {
		t.Fatalf("restricted pool actor = banned %t: %v", banned, err)
	}
	if banned, err := handlers.checkedBan(actor, target, ""); err != nil || banned {
		t.Fatalf("unrestricted global scope = banned %t: %v", banned, err)
	}
	if err := store.Revoke(principal(issuer, "room_moderator"), banID, 1, "restriction reviewed"); err != nil {
		t.Fatal(err)
	}
	if banned, err := handlers.checkedBan(actor, target, pool); err != nil || banned {
		t.Fatalf("revoked pool ban = banned %t: %v", banned, err)
	}
}
