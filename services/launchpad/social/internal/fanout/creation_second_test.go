package fanout_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/devsocial"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/social"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

func TestCreationSecondDurableReads(t *testing.T) {
	for _, fraction := range []time.Duration{700 * time.Millisecond, 200 * time.Millisecond} {
		for _, kind := range []string{"dm", "pool"} {
			t.Run(kind+"/"+fraction.String(), func(t *testing.T) {
				store := durableFixture(t)
				ctx := context.Background()
				if _, err := store.Pool.Exec(ctx, fanout.ConnectedSchemaSQL); err != nil {
					t.Fatal(err)
				}
				now := time.Now().UTC().Truncate(time.Second).Add(fraction)
				store.Clock = func() time.Time { return now }
				in := fanout.SendInput{Actor: addr(0xaa), Target: addr(0xbb), Kind: kind, ClientID: "creation-second", Content: "original body"}
				policy := store.Policy
				store.Policy = func(ctx context.Context, tx pgx.Tx, input fanout.SendInput) error {
					if input.Actor != in.Actor {
						return fanout.ErrNotFound
					}
					return policy(ctx, tx, input)
				}
				store.ReadPolicy = func(ctx context.Context, tx pgx.Tx, actor, _ string) error {
					if actor != in.Actor {
						return fanout.ErrNotFound
					}
					var allowed bool
					if err := tx.QueryRow(ctx, `SELECT allowed FROM fixture_policy WHERE wallet=$1`, actor).Scan(&allowed); err != nil {
						return err
					}
					if !allowed {
						return fanout.ErrNotFound
					}
					return nil
				}
				if kind == "pool" {
					in.Target = addr(1)
					if err := store.JoinPool(ctx, in.Actor, in.Target); err != nil {
						t.Fatal(err)
					}
				}
				message, err := store.Send(ctx, in)
				if err != nil {
					t.Fatal(err)
				}
				check := func(label string, got fanout.DurableMessage) {
					t.Helper()
					if got.ID != message.ID || got.CreatedAt != now.Unix() {
						t.Fatalf("%s = %+v, want message %s created at %d", label, got, message.ID, now.Unix())
					}
				}
				check("Send", message)
				var stored time.Time
				if err := store.Pool.QueryRow(ctx, `SELECT created_at FROM chat_messages WHERE id=$1`, message.ID).Scan(&stored); err != nil {
					t.Fatal(err)
				}
				if !stored.Equal(now) {
					t.Fatalf("stored creation time = %s, want %s", stored, now)
				}
				retry, err := store.Send(ctx, in)
				if err != nil {
					t.Fatal(err)
				}
				check("idempotent Send", retry)
				checkHistory := func(deleted bool) {
					t.Helper()
					page, err := store.History(ctx, in.Actor, message.ConversationID, "", 10)
					if err != nil || len(page.Messages) != 1 {
						t.Fatalf("history = %+v: %v", page, err)
					}
					check("history", page.Messages[0])
					if page.Messages[0].Deleted != deleted {
						t.Fatalf("history deleted = %t, want %t", page.Messages[0].Deleted, deleted)
					}
				}
				checkHistory(false)
				var payload []byte
				if err := store.Pool.QueryRow(ctx, `SELECT payload FROM chat_outbox WHERE message_id=$1 AND revision=1`, message.ID).Scan(&payload); err != nil {
					t.Fatal(err)
				}
				current, err := store.CurrentEvent(ctx, payload)
				if err != nil {
					t.Fatal(err)
				}
				var event fanout.DurableMessage
				if err := json.Unmarshal(current, &event); err != nil {
					t.Fatal(err)
				}
				check("CurrentEvent", event)
				wire, err := json.Marshal(message)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(current, wire) {
					t.Fatalf("CurrentEvent = %s, want %s", current, wire)
				}
				if kind == "dm" {
					router := chi.NewRouter()
					if err := fanout.RegisterChatRoutes(router, store, devsocial.Authenticate); err != nil {
						t.Fatal(err)
					}
					social.New(social.Deps{Durable: store, AuthenticateChat: devsocial.Authenticate}).RegisterRoutes(router)
					for _, path := range []string{"/v2/chat/conversations", "/dm/conversations"} {
						request := httptest.NewRequest(http.MethodGet, path, nil)
						request.Header.Set("X-Actor-Wallet", in.Actor)
						response := httptest.NewRecorder()
						router.ServeHTTP(response, request)
						if response.Code != http.StatusOK {
							t.Fatalf("%s = %d: %s", path, response.Code, response.Body.String())
						}
						var result struct {
							Conversations []struct {
								ID            string `json:"id"`
								LastMessageAt int64  `json:"last_message_at"`
							} `json:"conversations"`
						}
						if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
							t.Fatal(err)
						}
						if len(result.Conversations) != 1 || result.Conversations[0].ID != message.ConversationID || result.Conversations[0].LastMessageAt != now.Unix() {
							t.Fatalf("%s = %+v, want conversation %s at %d", path, result, message.ConversationID, now.Unix())
						}
					}
				}
				edited, err := store.Revise(ctx, in.Actor, message.ID, 1, "edited body", false)
				if err != nil {
					t.Fatal(err)
				}
				check("Revise", edited)
				checkHistory(false)
				tx, err := store.Pool.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback(ctx)
				if err := store.ModerateTx(ctx, tx, message.ID); err != nil {
					t.Fatal(err)
				}
				if err := tx.Commit(ctx); err != nil {
					t.Fatal(err)
				}
				checkHistory(true)
				if err := store.Pool.QueryRow(ctx, `SELECT payload FROM chat_outbox WHERE message_id=$1 AND revision=3`, message.ID).Scan(&payload); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(payload, &event); err != nil {
					t.Fatal(err)
				}
				check("ModerateTx event", event)
				current, err = store.CurrentEvent(ctx, wire)
				if err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(current, &event); err != nil {
					t.Fatal(err)
				}
				check("CurrentEvent after moderation", event)
				if !event.Deleted || event.Content != "" || event.Revision != 3 {
					t.Fatalf("moderated event = %+v", event)
				}
			})
		}
	}
}

func TestCreationSecondEditWindow(t *testing.T) {
	for _, fraction := range []time.Duration{700 * time.Millisecond, 200 * time.Millisecond} {
		t.Run(fraction.String(), func(t *testing.T) {
			store := durableFixture(t)
			ctx := context.Background()
			created := time.Now().UTC().Truncate(time.Second).Add(fraction)
			now := created
			store.Clock = func() time.Time { return now }
			message := sendDM(t, store, "edit-window")
			now = created.Add(900 * time.Second)
			edited, err := store.Revise(ctx, message.Sender, message.ID, 1, "within window", false)
			if err != nil || edited.CreatedAt != created.Unix() {
				t.Fatalf("edit at 900 seconds = %+v: %v", edited, err)
			}
			now = created.Add(901 * time.Second)
			if _, err := store.Revise(ctx, message.Sender, message.ID, 2, "after window", false); !errors.Is(err, fanout.ErrInvalid) {
				t.Fatalf("edit at 901 seconds = %v", err)
			}
		})
	}
}
