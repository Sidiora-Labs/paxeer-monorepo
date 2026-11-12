package fanout_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/devsocial"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/interactions"
)

func TestNotificationIntentFollowsRecipientPreferences(t *testing.T) {
	f := newRepair1027Fixture(t)
	ctx := context.Background()
	redis := f.redis.client(t)
	t.Cleanup(func() { _ = redis.Close() })
	runtime, err := devsocial.New(ctx, devsocial.Deps{Pool: f.database, Redis: redis, CursorKey: []byte("notification-intent-cursor-key-32"), Pools: devsocial.CanonicalPools})
	if err != nil {
		t.Fatal(err)
	}
	targets, err := fanout.NewPostgresNotificationTargets("users")
	if err != nil {
		t.Fatal(err)
	}
	runtime.Chat.NotificationQueue = (fanout.NotificationIntentWriter{Targets: targets, MaximumRecipients: 10}).Queue
	request := httptest.NewRequest("GET", "/v2/chat", nil)
	request.Header.Set("X-Fixture-Session", addr(0xaa))
	principal, err := f.requestPrincipal(request)
	if err != nil {
		t.Fatal(err)
	}
	principal.ExpiresAt = time.Now().Add(time.Hour)
	ctx = interactions.WithIdentity(ctx, principal)
	first, err := runtime.Chat.Send(ctx, fanout.SendInput{Actor: addr(0xaa), Target: addr(0xbb), Kind: "dm", ClientID: "notification-first", Content: "first private message"})
	if err != nil {
		t.Fatal(err)
	}
	var account, message string
	if err := f.database.QueryRow(ctx, `SELECT account_id::text,message_id FROM chat_notification_intents WHERE event_id=$1`, first.ID+":1").Scan(&account, &message); err != nil || account != f.principal(addr(0xbb)).AccountID || message != first.ID {
		t.Fatalf("recipient intent = %q %q: %v", account, message, err)
	}
	refused := errors.New("recipient delivery unavailable")
	if count, err := runtime.Chat.DrainNotificationIntents(ctx, 10, func(_ context.Context, intent fanout.NotificationIntent) error {
		if intent.AccountID != account || intent.MessageID != first.ID {
			t.Fatalf("pending recipient intent = %+v", intent)
		}
		return refused
	}); count != 0 || !errors.Is(err, refused) {
		t.Fatalf("undelivered intent = %d: %v", count, err)
	}
	var received []fanout.NotificationIntent
	count, err := runtime.Chat.DrainNotificationIntents(ctx, 10, func(_ context.Context, intent fanout.NotificationIntent) error {
		received = append(received, intent)
		return nil
	})
	if err != nil || count != 1 || len(received) != 1 || received[0].AccountID != account || received[0].MessageID != first.ID {
		t.Fatalf("drained intents = %d %+v: %v", count, received, err)
	}
	if count, err = runtime.Chat.DrainNotificationIntents(ctx, 10, func(context.Context, fanout.NotificationIntent) error {
		t.Fatal("delivered intent repeated")
		return nil
	}); err != nil || count != 0 {
		t.Fatalf("repeated delivery = %d: %v", count, err)
	}
	if _, err := f.database.Exec(ctx, `UPDATE chat_members SET muted=true WHERE conversation_id=$1 AND wallet=$2`, first.ConversationID, addr(0xbb)); err != nil {
		t.Fatal(err)
	}
	second, err := runtime.Chat.Send(ctx, fanout.SendInput{Actor: addr(0xaa), Target: addr(0xbb), Kind: "dm", ClientID: "notification-second", Content: "muted private message"})
	if err != nil {
		t.Fatal(err)
	}
	var intents int
	if err := f.database.QueryRow(ctx, `SELECT count(*) FROM chat_notification_intents WHERE message_id=$1`, second.ID).Scan(&intents); err != nil || intents != 0 {
		t.Fatalf("muted recipient intents = %d: %v", intents, err)
	}
}
