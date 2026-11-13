package l18

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestLiabilitiesReadAndSettlementAuthorization(t *testing.T) {
	ledger := phase2Storage(t)
	ctx := context.Background()
	actor := Actor{Wallet: actorA}
	items, err := ledger.Liabilities(ctx, actor, "", 10)
	if err != nil || len(items) != 0 {
		t.Fatalf("empty liability page = %+v, error = %v", items, err)
	}
	if _, err := ledger.Liabilities(ctx, Actor{Wallet: "invalid"}, "", 10); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("invalid wallet read: %v", err)
	}
	if _, err := ledger.Liabilities(ctx, actor, "", 0); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid page limit: %v", err)
	}
	if err := ledger.ReviewHold(ctx, actor, "request", "award", "review"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("review without role: %v", err)
	}
	if err := ledger.ReviewHold(ctx, Actor{Wallet: actorA, Roles: map[string]bool{"reward-review": true}}, "request", "award", "review"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("review with invalid request identity: %v", err)
	}
	if err := ledger.Correction(ctx, actor, "request", "award", "reason", false); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("correction without role: %v", err)
	}
	if err := ledger.RecordAnalytics(ctx, actor, "request", "campaign", "code", "view", time.Now()); !errors.Is(err, ErrInvalid) {
		t.Fatalf("analytics with invalid request identity: %v", err)
	}
}
