package devsocial

import (
	"context"
	"errors"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/interactions"
)

func TestIndexedPoolCreatorPolicyUsesCommittedOwner(t *testing.T) {
	h := newDirectRouteHarness(t)
	ctx := context.Background()
	tx, err := h.db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := h.runtime.policy.CheckCreator(ctx, tx, interactions.Request{Actor: h.owner, Pool: h.pool}); err != nil {
		t.Fatalf("indexed creator = %v", err)
	}
	if err := h.runtime.policy.CheckCreator(ctx, tx, interactions.Request{Actor: directAddr(0xb0b), Pool: h.pool}); !errors.Is(err, interactions.ErrDenied) {
		t.Fatalf("noncreator = %v", err)
	}
	if err := h.runtime.policy.CheckCreator(ctx, tx, interactions.Request{Actor: h.owner, Pool: directAddr(0xdead)}); !errors.Is(err, interactions.ErrDenied) {
		t.Fatalf("unknown pool creator = %v", err)
	}
}
