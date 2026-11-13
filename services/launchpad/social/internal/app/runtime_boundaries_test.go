package app

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/voice"
	"github.com/google/uuid"
)

func TestSocialTargetResolverReadsCommittedChatAndComment(t *testing.T) {
	h := newDevHarness(t)
	ctx := context.Background()
	actor := addr(0xa11ce)
	resolver := socialTargetResolver{pool: h.app.pool}
	if _, _, err := (socialTargetResolver{}).Resolve(ctx, "message", uuid.NewString()); !errors.Is(err, safety.ErrUnavailable) {
		t.Fatalf("missing target store = %v", err)
	}
	if _, _, err := resolver.Resolve(ctx, "unknown", "target"); !errors.Is(err, safety.ErrUnavailable) {
		t.Fatalf("unsupported target = %v", err)
	}
	if err := h.app.dev.Chat.JoinPool(ctx, actor, h.pool); err != nil {
		t.Fatal(err)
	}
	message, err := h.app.dev.Chat.Send(ctx, fanout.SendInput{Actor: actor, Kind: "pool", Target: h.pool, ClientID: uuid.NewString(), Content: "visible community message"})
	if err != nil {
		t.Fatal(err)
	}
	if pool, affected, err := resolver.Resolve(ctx, "message", message.ID); err != nil || pool != h.pool || affected != actor {
		t.Fatalf("committed message target = %q %q: %v", pool, affected, err)
	}
	created := h.expect(h.do(http.MethodPost, "/v2/pools/"+h.pool+"/comments", actor, map[string]string{"body": "visible community comment", "clientCommentId": uuid.NewString()}, nil), http.StatusCreated, "CommentCreateResponse")
	comment := created["data"].(map[string]any)["comment"].(map[string]any)
	id := comment["commentId"].(string)
	if pool, affected, err := resolver.Resolve(ctx, "comment", id); err != nil || pool != h.pool || affected != actor {
		t.Fatalf("committed comment target = %q %q: %v", pool, affected, err)
	}
	if _, _, err := resolver.Resolve(ctx, "message", uuid.NewString()); err == nil {
		t.Fatal("absent message resolved")
	}
}

func TestVisibilityWorkerStopsAfterContextCancellation(t *testing.T) {
	h := newDevHarness(t)
	runtime := &socialConnectedRuntime{pool: h.app.pool}
	if err := runtime.visibilityOnce(context.Background()); err != nil {
		t.Fatalf("empty visibility cycle = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runtime.runVisibility(ctx) }()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("stopped visibility worker = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("visibility worker did not stop")
	}
	if exists, err := (socialPools{}).Exists(context.Background(), h.pool); exists || !errors.Is(err, voice.ErrUnavailable) {
		t.Fatalf("unbound pool lookup = %t: %v", exists, err)
	}
}

func TestPlatformPoolAndSocialTargetsReadCanonicalRecords(t *testing.T) {
	h := newPlatformHarness(t)
	ctx := context.Background()
	exists, err := (platformPools{db: h.app.pool}).Exists(ctx, h.pool)
	if err != nil || !exists {
		t.Fatalf("canonical pool membership = %t: %v", exists, err)
	}
	exists, err = (platformPools{db: h.app.pool}).Exists(ctx, addr(0xdead))
	if err != nil || exists {
		t.Fatalf("absent canonical pool = %t: %v", exists, err)
	}
	resolver := socialTargetResolver{pool: h.app.pool}
	pool, creator, err := resolver.Resolve(ctx, "token", h.token)
	if err != nil || pool != h.pool || creator != h.creator {
		t.Fatalf("canonical token target = %q %q: %v", pool, creator, err)
	}
	account := h.wire.session("Bearer profile-target", addr(0xa11ce))
	pool, wallet, err := resolver.Resolve(ctx, "profile", account.Wallet)
	if err != nil || pool != "" || wallet != account.Wallet {
		t.Fatalf("active profile target = %q %q: %v", pool, wallet, err)
	}
}
