package fanout_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
)

func TestDurableChatActionsRequireCommittedStore(t *testing.T) {
	store := &fanout.DurableStore{}
	ctx := context.Background()
	wallet := addr(0xaa)
	for _, tc := range []struct {
		name string
		run  func() error
		want error
	}{
		{"send", func() error {
			_, err := store.Send(ctx, fanout.SendInput{Actor: wallet, Target: addr(0xbb), Kind: "dm", ClientID: "unbound-send", Content: "message"})
			return err
		}, nil},
		{"authorize", func() error { return store.Authorized(ctx, wallet, "conversation") }, nil},
		{"list", func() error { _, err := store.List(ctx, wallet, 10); return err }, nil},
		{"history", func() error { _, err := store.History(ctx, wallet, "conversation", "", 10); return err }, nil},
		{"drain", func() error { _, err := store.DrainOutbox(ctx, 1, nil); return err }, nil},
		{"join pool", func() error { return store.JoinPool(ctx, wallet, addr(0x9001)) }, nil},
		{"revise", func() error { _, err := store.Revise(ctx, wallet, "message", 1, "updated", false); return err }, nil},
		{"read", func() error { return store.MarkRead(ctx, wallet, "conversation", 1) }, nil},
		{"moderate", func() error { return store.ModerateTx(ctx, nil, "message") }, nil},
		{"current event", func() error { _, err := store.CurrentEvent(ctx, []byte(`{}`)); return err }, nil},
		{"connected read", func() error { return store.MarkReadConnected(ctx, wallet, "conversation", 1) }, fanout.ErrInvalid},
		{"control outbox", func() error { _, err := store.DrainControlOutbox(ctx, 1, nil); return err }, nil},
		{"notification outbox", func() error { _, err := store.DrainNotificationIntents(ctx, 1, nil); return err }, nil},
		{"live outbox", func() error { _, err := store.DrainLiveOutbox(ctx, 1, nil); return err }, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := tc.want
			if want == nil {
				want = fanout.ErrUnavailable
			}
			if err := tc.run(); !errors.Is(err, want) {
				t.Fatalf("unbound chat action = %v", err)
			}
		})
	}
	if err := store.ConsumeSocialBroker(ctx, nil, "social", "SOCIAL"); !errors.Is(err, fanout.ErrInvalid) {
		t.Fatalf("unbound broker message = %v", err)
	}
}
