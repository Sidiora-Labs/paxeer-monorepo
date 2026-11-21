package app

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	"github.com/google/uuid"
)

func TestCreationSecondMessageVisibilityEvent(t *testing.T) {
	h := newDevHarness(t)
	ctx := context.Background()
	for _, fraction := range []time.Duration{700 * time.Millisecond, 200 * time.Millisecond} {
		for _, kind := range []string{"dm", "pool"} {
			t.Run(kind+"/"+fraction.String(), func(t *testing.T) {
				store := *h.app.dev.Chat
				now := time.Now().UTC().Truncate(time.Second).Add(fraction)
				store.Clock = func() time.Time { return now }
				in := fanout.SendInput{Actor: addr(0xa11ce), Target: addr(0xb0b), Kind: kind, ClientID: uuid.NewString(), Content: "visible body"}
				if kind == "pool" {
					in.Target = h.pool
					if err := store.JoinPool(ctx, in.Actor, in.Target); err != nil {
						t.Fatal(err)
					}
				}
				message, err := store.Send(ctx, in)
				if err != nil {
					t.Fatal(err)
				}
				if message.CreatedAt != now.Unix() {
					t.Fatalf("Send created at %d, want %d", message.CreatedAt, now.Unix())
				}
				runtime := &socialConnectedRuntime{pool: h.app.pool, chat: &store}
				for i, status := range []string{"removed", "restored"} {
					revision := int64(i + 1)
					tx, err := h.app.pool.Begin(ctx)
					if err != nil {
						t.Fatal(err)
					}
					defer tx.Rollback(ctx)
					actor, err := runtime.messageVisibilityEvent(ctx, tx, safety.Visibility{TargetType: messageAction, TargetID: message.ID, Revision: revision, Status: status}, revision-1)
					if err != nil {
						t.Fatal(err)
					}
					if actor != message.Sender {
						t.Fatalf("visibility actor = %s, want %s", actor, message.Sender)
					}
					if err := tx.Commit(ctx); err != nil {
						t.Fatal(err)
					}
					var payload []byte
					if err := h.dbConn.QueryRow(ctx, `SELECT payload FROM chat.chat_outbox WHERE event_id=$1`, fmt.Sprintf("safety_message_%s_%d", message.ID, revision)).Scan(&payload); err != nil {
						t.Fatal(err)
					}
					var event fanout.DurableMessage
					if err := json.Unmarshal(payload, &event); err != nil {
						t.Fatal(err)
					}
					if event.ID != message.ID || event.CreatedAt != now.Unix() || event.CreatedAt != message.CreatedAt || event.Revision != revision+1 || event.Deleted != (status == "removed") {
						t.Fatalf("%s visibility event = %+v, want message %s created at %d", status, event, message.ID, now.Unix())
					}
					if status == "removed" && event.Content != "" || status == "restored" && event.Content != message.Content {
						t.Fatalf("%s visibility content = %q", status, event.Content)
					}
				}
			})
		}
	}
}
