package fanout_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/internaltest"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	goredis "github.com/redis/go-redis/v9"
)

func TestConnectedChatRoutesRejectInvalidCommandsAndUnavailableStorage(t *testing.T) {
	f := newRepair1027Fixture(t)
	runtime := f.start(500, 20, 20)
	router := chi.NewRouter()
	if err := fanout.RegisterChatRoutes(router, runtime.store, f.requestPrincipal); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		method, path, body, wallet string
		status                     int
	}{
		{http.MethodPost, "/v2/chat/read", `{}`, "", 401},
		{http.MethodPost, "/v2/chat/revise", `{}`, "", 401},
		{http.MethodPost, "/v2/chat/read", `{`, addr(0xaa), 400},
		{http.MethodPost, "/v2/chat/read", `{"unknown":true}`, addr(0xaa), 400},
		{http.MethodPost, "/v2/chat/read", `{"through_sequence":"1"} {}`, addr(0xaa), 400},
		{http.MethodPost, "/v2/chat/read", `{"through_sequence":"-1"}`, addr(0xaa), 400},
		{http.MethodPost, "/v2/chat/read", `{"conversation_id":"unknown","through_sequence":"1"}`, addr(0xaa), 404},
		{http.MethodPost, "/v2/chat/revise", `{`, addr(0xaa), 400},
		{http.MethodPost, "/v2/chat/revise", `{"expected_revision":"invalid"}`, addr(0xaa), 400},
		{http.MethodPost, "/v2/chat/revise", `{"expected_revision":"0"}`, addr(0xaa), 400},
		{http.MethodPost, "/v2/chat/revise", `{"message_id":"unknown","expected_revision":"1","content":"new content"}`, addr(0xaa), 404},
	} {
		t.Run(tc.method+tc.path+tc.body+tc.wallet, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			request.Header.Set("X-Fixture-Session", tc.wallet)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != tc.status {
				t.Fatalf("chat response = %d %s", response.Code, response.Body.String())
			}
		})
	}
	withoutChatRelation(t, runtime.store, "chat_conversations", func() {
		request := httptest.NewRequest(http.MethodGet, "/v2/chat/conversations", nil)
		request.Header.Set("X-Fixture-Session", addr(0xaa))
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != 503 || !strings.Contains(response.Body.String(), "unavailable") {
			t.Fatalf("conversation dependency = %d %s", response.Code, response.Body.String())
		}
	})
	if err := fanout.RegisterChatRoutes(chi.NewRouter(), runtime.store, nil); !errors.Is(err, fanout.ErrUnavailable) {
		t.Fatalf("unbound chat authentication = %v", err)
	}
}

func TestChatOutboxesKeepPendingEventsUntilTransportSucceeds(t *testing.T) {
	store := durableFixture(t)
	ctx := context.Background()
	if _, err := store.Pool.Exec(ctx, fanout.ConnectedSchemaSQL); err != nil {
		t.Fatal(err)
	}
	message := sendDM(t, store, "pending-transport")
	options, err := goredis.ParseURL(internaltest.NewRedisURL(t))
	if err != nil {
		t.Fatal(err)
	}
	client := goredis.NewClient(options)
	t.Cleanup(func() { _ = client.Close() })
	publish := func(ctx context.Context, id string, channels []string, payload []byte) error {
		for _, channel := range channels {
			if err := client.Publish(ctx, channel, payload).Err(); err != nil {
				return err
			}
		}
		return nil
	}
	control := func(ctx context.Context, id, wallet string, payload []byte) error {
		return client.Publish(ctx, fanout.WalletChannel(store.ChannelNamespace, wallet), payload).Err()
	}
	notify := func(ctx context.Context, intent fanout.NotificationIntent) error {
		return client.Publish(ctx, "notifications:"+intent.AccountID, intent.MessageID).Err()
	}
	for _, tc := range []struct {
		name, table string
		run         func() (int, error)
	}{
		{"message", "chat_outbox", func() (int, error) { return store.DrainOutbox(ctx, 1, publish) }},
		{"control", "chat_control_outbox", func() (int, error) { return store.DrainControlOutbox(ctx, 1, control) }},
		{"notification", "chat_notification_intents", func() (int, error) { return store.DrainNotificationIntents(ctx, 1, notify) }},
		{"live", "chat_live_outbox", func() (int, error) { return store.DrainLiveOutbox(ctx, 1, client) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withoutChatRelation(t, store, tc.table, func() {
				count, err := tc.run()
				var postgres *pgconn.PgError
				if count != 0 || !errors.As(err, &postgres) || postgres.Code != "42P01" {
					t.Fatalf("outbox dependency = %d: %v", count, err)
				}
			})
		})
	}
	if err := store.MarkReadConnected(ctx, addr(0xbb), message.ConversationID, 1); err != nil {
		t.Fatal(err)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		run  func() (int, error)
	}{
		{"message", func() (int, error) { return store.DrainOutbox(ctx, 1, publish) }},
		{"control", func() (int, error) { return store.DrainControlOutbox(ctx, 1, control) }},
	} {
		t.Run(tc.name+" transport", func(t *testing.T) {
			count, err := tc.run()
			if count != 0 || err == nil {
				t.Fatalf("closed transport = %d: %v", count, err)
			}
		})
	}
	var pending, controlPending int
	if err := store.Pool.QueryRow(ctx, `SELECT count(*) FROM chat_outbox WHERE delivered_at IS NULL`).Scan(&pending); err != nil {
		t.Fatal(err)
	}
	if err := store.Pool.QueryRow(ctx, `SELECT count(*) FROM chat_control_outbox WHERE delivered_at IS NULL`).Scan(&controlPending); err != nil {
		t.Fatal(err)
	}
	if pending != 1 || controlPending != 1 {
		t.Fatalf("pending publication intents = %d, %d", pending, controlPending)
	}
}
