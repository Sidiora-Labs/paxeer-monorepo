package fanout_test

import (
	"context"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/internaltest"
	goredis "github.com/redis/go-redis/v9"
)

func TestDurableChatReportsClosedCommittedStorage(t *testing.T) {
	ctx := context.Background()
	store := durableFixture(t)
	if _, err := store.Pool.Exec(ctx, fanout.ConnectedSchemaSQL); err != nil {
		t.Fatal(err)
	}
	message := sendDM(t, store, "closed-chat-store")
	options, err := goredis.ParseURL(internaltest.NewRedisURL(t))
	if err != nil {
		t.Fatal(err)
	}
	redis := goredis.NewClient(options)
	t.Cleanup(func() { _ = redis.Close() })
	store.Pool.Close()
	for _, tc := range []struct {
		name string
		run  func() error
	}{
		{"send", func() error {
			_, err := store.Send(ctx, fanout.SendInput{Actor: addr(0xaa), Target: addr(0xbb), Kind: "dm", ClientID: "closed-send", Content: "message"})
			return err
		}},
		{"authorization", func() error { return store.Authorized(ctx, addr(0xaa), message.ConversationID) }},
		{"conversations", func() error { _, err := store.List(ctx, addr(0xaa), 10); return err }},
		{"history", func() error { _, err := store.History(ctx, addr(0xaa), message.ConversationID, "", 10); return err }},
		{"outbox", func() error {
			_, err := store.DrainOutbox(ctx, 1, func(context.Context, string, []string, []byte) error { return nil })
			return err
		}},
		{"join", func() error { return store.JoinPool(ctx, addr(0xaa), addr(1)) }},
		{"revision", func() error {
			_, err := store.Revise(ctx, addr(0xaa), message.ID, message.Revision, "updated", false)
			return err
		}},
		{"read", func() error { return store.MarkRead(ctx, addr(0xbb), message.ConversationID, message.Sequence) }},
		{"connected read", func() error {
			return store.MarkReadConnected(ctx, addr(0xbb), message.ConversationID, message.Sequence)
		}},
		{"control outbox", func() error {
			_, err := store.DrainControlOutbox(ctx, 1, func(context.Context, string, string, []byte) error { return nil })
			return err
		}},
		{"notification outbox", func() error {
			_, err := store.DrainNotificationIntents(ctx, 1, func(context.Context, fanout.NotificationIntent) error { return nil })
			return err
		}},
		{"live outbox", func() error { _, err := store.DrainLiveOutbox(ctx, 1, redis); return err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); err == nil {
				t.Fatal("closed committed chat storage accepted an action")
			}
		})
	}
}
