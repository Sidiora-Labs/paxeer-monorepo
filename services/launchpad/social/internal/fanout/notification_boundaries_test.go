package fanout_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
)

func TestConnectedOutboxesRejectMissingAuthorityAndInvalidPages(t *testing.T) {
	store := durableFixture(t)
	ctx := context.Background()
	if _, err := store.Pool.Exec(ctx, fanout.ConnectedSchemaSQL); err != nil {
		t.Fatal(err)
	}
	message := sendDM(t, store, "outbox-boundary")
	if err := store.MarkReadConnected(ctx, addr(0xbb), message.ConversationID, -1); !errors.Is(err, fanout.ErrInvalid) {
		t.Fatalf("negative connected read marker = %v", err)
	}
	if err := store.MarkReadConnected(ctx, addr(0xcc), message.ConversationID, 1); !errors.Is(err, fanout.ErrNotFound) {
		t.Fatalf("unjoined connected read marker = %v", err)
	}
	for _, limit := range []int{0, 101} {
		if _, err := store.DrainControlOutbox(ctx, limit, func(context.Context, string, string, []byte) error { return nil }); !errors.Is(err, fanout.ErrInvalid) {
			t.Fatalf("control page %d = %v", limit, err)
		}
		if _, err := store.DrainNotificationIntents(ctx, limit, func(context.Context, fanout.NotificationIntent) error { return nil }); !errors.Is(err, fanout.ErrInvalid) {
			t.Fatalf("notification page %d = %v", limit, err)
		}
		if _, err := store.DrainLiveOutbox(ctx, limit, nil); !errors.Is(err, fanout.ErrUnavailable) {
			t.Fatalf("live delivery without Redis = %v", err)
		}
	}
	if _, err := store.DrainControlOutbox(ctx, 1, nil); !errors.Is(err, fanout.ErrUnavailable) {
		t.Fatalf("control delivery without publisher = %v", err)
	}
	if _, err := store.DrainNotificationIntents(ctx, 1, nil); !errors.Is(err, fanout.ErrUnavailable) {
		t.Fatalf("notification delivery without committer = %v", err)
	}
	if err := (fanout.NotificationIntentWriter{MaximumRecipients: 10}).Queue(ctx, nil, message); !errors.Is(err, fanout.ErrUnavailable) {
		t.Fatalf("notification without account authority = %v", err)
	}
	if _, err := fanout.NewPostgresNotificationTargets("not a schema"); err == nil {
		t.Fatal("invalid account schema accepted")
	}
}
