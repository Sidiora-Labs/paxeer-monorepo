package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
)

const notificationActorWallet = "0x0123456789aBcDEF0123456789aBcDEF01234567"

func TestPostgresV2NotificationsListAndRead(t *testing.T) {
	s, account := notificationDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	other := account + "-other"
	t.Cleanup(func() {
		for _, actor := range []string{account, other} {
			if err := s.DeleteAccount(context.Background(), actor); err != nil {
				t.Error("delete fixture:", err)
			}
		}
	})
	now := time.Now().UTC().Truncate(time.Microsecond)
	var ids []string
	for i := 0; i < 3; i++ {
		item, created, err := s.CommitTrigger(ctx, InboxItem{
			Identity: Identity{account, fmt.Sprintf("http-%d", i), "v2"},
			Category: "watchlist", Summary: fmt.Sprintf("Notification %d", i),
			ResourceID: fmt.Sprintf("pool-%d", i), CreatedAt: now.Add(-time.Duration(i) * time.Second),
		}, nil)
		if err != nil || !created {
			t.Fatalf("insert notification %d: created=%v err=%v", i, created, err)
		}
		var id string
		if err = s.DB.QueryRowContext(ctx, `SELECT v2_id::text FROM discovery_notifications.inbox WHERE id=$1`, item.ID).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	foreign, created, err := s.CommitTrigger(ctx, InboxItem{
		Identity: Identity{other, "foreign", "v2"}, Category: "watchlist",
		Summary: "Other account", ResourceID: "other-pool", CreatedAt: now.Add(time.Hour),
	}, nil)
	if err != nil || !created {
		t.Fatalf("insert other account: created=%v err=%v", created, err)
	}
	var foreignID string
	if err = s.DB.QueryRowContext(ctx, `SELECT v2_id::text FROM discovery_notifications.inbox WHERE id=$1`, foreign.ID).Scan(&foreignID); err != nil {
		t.Fatal(err)
	}
	h := HTTP{Store: s, Authenticate: func(*http.Request) (string, error) { return account, nil }}
	handler := h.Handler()
	request := func(handler http.Handler, method, path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, nil).WithContext(ctx)
		r.Header.Set("X-Request-Id", notificationErrorRequestID)
		r.Header.Set("X-Actor-Wallet", notificationActorWallet)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	decodeList := func(t *testing.T, w *httptest.ResponseRecorder) v2.NotificationsResponse {
		t.Helper()
		if w.Code != http.StatusOK {
			t.Fatalf("list status=%d, want 200: %s", w.Code, w.Body.String())
		}
		if err := v2.ValidateJSON("NotificationsResponse", w.Body.Bytes()); err != nil {
			t.Fatalf("invalid frozen list: %v: %s", err, w.Body.String())
		}
		var got v2.NotificationsResponse
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		pattern := regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-7[0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
		for _, item := range got.Data.Items {
			if !pattern.MatchString(item.NotificationId) || item.Wallet != notificationActorWallet {
				t.Fatalf("notification identity does not match frozen contract and actor wallet: %+v", item)
			}
			millis, err := strconv.ParseInt(strings.ReplaceAll(item.NotificationId[:13], "-", ""), 16, 64)
			createdAt, timeErr := time.Parse(time.RFC3339Nano, item.CreatedAt)
			if err != nil || timeErr != nil || millis != createdAt.UnixMilli() {
				t.Fatalf("UUID timestamp does not match createdAt: %+v, millis=%d err=%v timeErr=%v", item, millis, err, timeErr)
			}
		}
		if got.RequestId != notificationErrorRequestID || got.Quality != "complete" || got.GeneratedAt == "" || got.Watermark != nil || got.Revision != nil {
			t.Fatalf("unexpected envelope: %+v", got)
		}
		if w.Header().Get("Cache-Control") != "private, no-store" || w.Header().Get("Content-Type") != "application/json" {
			t.Fatalf("unexpected headers: %v", w.Header())
		}
		return got
	}
	t.Run("pages", func(t *testing.T) {
		first := decodeList(t, request(handler, http.MethodGet, "/v2/me/notifications?limit=2&unreadOnly=false"))
		if len(first.Data.Items) != 2 || first.NextCursor == nil || len(*first.NextCursor) < 1 || len(*first.NextCursor) > 256 {
			t.Fatalf("first page: %+v", first)
		}
		second := decodeList(t, request(handler, http.MethodGet, "/v2/me/notifications?limit=2&cursor="+url.QueryEscape(*first.NextCursor)))
		if len(second.Data.Items) != 1 || second.NextCursor != nil {
			t.Fatalf("second page: %+v", second)
		}
		items := append(first.Data.Items, second.Data.Items...)
		for i, item := range items {
			if item.NotificationId != ids[i] || item.Kind != "price_alert" || item.EntityId != fmt.Sprintf("pool-%d", i) || item.ReadAt != nil || item.DeliveryState != "sent" {
				t.Errorf("item %d: %+v", i, item)
			}
		}
	})
	t.Run("read", func(t *testing.T) {
		var originalReadAt string
		for attempt := 0; attempt < 2; attempt++ {
			w := request(handler, http.MethodPut, "/v2/me/notifications/"+strings.ToUpper(ids[0])+"/read")
			if w.Code != http.StatusOK {
				t.Fatalf("read status=%d, want 200: %s", w.Code, w.Body.String())
			}
			if err := v2.ValidateJSON("NotificationReadResponse", w.Body.Bytes()); err != nil {
				t.Fatalf("invalid frozen read: %v: %s", err, w.Body.String())
			}
			var got v2.NotificationReadResponse
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			var persisted time.Time
			var marked bool
			if err := s.DB.QueryRowContext(ctx, `SELECT read_at,(document->>'Read')::boolean FROM discovery_notifications.inbox WHERE account_id=$1 AND v2_id=$2::uuid`, account, ids[0]).Scan(&persisted, &marked); err != nil {
				t.Fatal(err)
			}
			if !marked || got.Data.ReadAt != persisted.UTC().Format("2006-01-02T15:04:05.000Z") || got.RequestId != notificationErrorRequestID || got.Quality != "complete" || got.NextCursor != nil {
				t.Fatalf("read response does not match persisted state: %+v, persisted=%v marked=%v", got, persisted, marked)
			}
			if attempt == 0 {
				originalReadAt = got.Data.ReadAt
			} else if got.Data.ReadAt != originalReadAt {
				t.Fatalf("repeat read changed timestamp: %s -> %s", originalReadAt, got.Data.ReadAt)
			}
		}
		unread := decodeList(t, request(handler, http.MethodGet, "/v2/me/notifications?unreadOnly=true&limit=100"))
		if len(unread.Data.Items) != 2 || unread.NextCursor != nil {
			t.Fatalf("unread page: %+v", unread)
		}
		for _, item := range unread.Data.Items {
			if item.NotificationId == ids[0] || item.ReadAt != nil {
				t.Fatalf("read item in unread results: %+v", item)
			}
		}
		all := decodeList(t, request(handler, http.MethodGet, "/v2/me/notifications"))
		if len(all.Data.Items) != 3 || all.NextCursor != nil || all.Data.Items[0].ReadAt == nil || *all.Data.Items[0].ReadAt != originalReadAt {
			t.Fatalf("default list lost read state: %+v", all)
		}
	})
	t.Run("unknown and foreign IDs", func(t *testing.T) {
		for _, id := range []string{uuid.NewString(), foreignID} {
			w := request(handler, http.MethodPut, "/v2/me/notifications/"+id+"/read")
			assertNotificationFrozenError(t, w, http.StatusNotFound, "NOT_FOUND", notificationErrorRequestID)
		}
	})
	t.Run("runtime mounts", func(t *testing.T) {
		runtime := (&HTTPRuntime{Notifications: h}).Handler()
		decodeList(t, request(runtime, http.MethodGet, "/v2/me/notifications?limit=2"))
		w := request(runtime, http.MethodPut, "/v2/me/notifications/"+foreignID+"/read")
		assertNotificationFrozenError(t, w, http.StatusNotFound, "NOT_FOUND", notificationErrorRequestID)
	})
	t.Run("legacy schema migration", func(t *testing.T) {
		tx, err := s.DB.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		if _, err = tx.ExecContext(ctx, `ALTER TABLE discovery_notifications.inbox DROP COLUMN v2_id;
			ALTER TABLE discovery_notifications.inbox ADD COLUMN v2_id uuid GENERATED ALWAYS AS (md5(id)::uuid) STORED;
			CREATE UNIQUE INDEX notification_v2_id ON discovery_notifications.inbox(v2_id)`); err != nil {
			t.Fatal(err)
		}
		for attempt := 0; attempt < 2; attempt++ {
			if _, err = tx.ExecContext(ctx, Schema); err != nil {
				t.Fatalf("apply schema attempt %d: %v", attempt, err)
			}
			rows, err := tx.QueryContext(ctx, `SELECT v2_id::text FROM discovery_notifications.inbox WHERE account_id=$1 ORDER BY created_at DESC,id`, account)
			if err != nil {
				t.Fatal(err)
			}
			var migrated []string
			for rows.Next() {
				var id string
				if err = rows.Scan(&id); err != nil {
					rows.Close()
					t.Fatal(err)
				}
				migrated = append(migrated, id)
			}
			err = rows.Err()
			rows.Close()
			if err != nil || strings.Join(migrated, ",") != strings.Join(ids, ",") {
				t.Fatalf("migration changed deterministic IDs: got=%v want=%v err=%v", migrated, ids, err)
			}
			var unique bool
			if err = tx.QueryRowContext(ctx, `SELECT indisunique FROM pg_index WHERE indexrelid='discovery_notifications.notification_v2_id'::regclass`).Scan(&unique); err != nil || !unique {
				t.Fatalf("migration lost unique index: unique=%v err=%v", unique, err)
			}
		}
	})
}

func TestV2NotificationsRejectsInvalidQuery(t *testing.T) {
	for _, tc := range []struct {
		query  string
		unread bool
		cursor string
		limit  int
	}{
		{query: "", limit: 50},
		{query: "unreadOnly=false&limit=1", limit: 1},
		{query: "unreadOnly=true&limit=100&cursor=" + strings.Repeat("a", 256), unread: true, cursor: strings.Repeat("a", 256), limit: 100},
	} {
		unread, cursor, limit, err := notificationListQuery(tc.query)
		if err != nil || unread != tc.unread || cursor != tc.cursor || limit != tc.limit {
			t.Fatalf("valid query %q: unread=%v cursor=%q limit=%d err=%v", tc.query, unread, cursor, limit, err)
		}
	}
	// A zero Store has no database: reaching persistence cannot produce a valid response.
	handler := (HTTP{Authenticate: func(*http.Request) (string, error) { return "actor", nil }}).Handler()
	for _, query := range []string{
		"limit=0", "limit=101", "cursor=" + strings.Repeat("x", 257), "unreadOnly=yes", "unknown=value",
		"limit=", "limit=-1", "limit=1.5", "limit=999999999999999999999", "cursor=", "unreadOnly=", "unreadOnly=1", "unreadOnly=True",
		"limit=1&limit=2", "unreadOnly=true&unreadOnly=false", "cursor=a&cursor=b", "cursor=%FF", "limit=2;unknown=x", "unknown=%ZZ",
	} {
		t.Run(query, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/v2/me/notifications?"+query, nil)
			r.Header.Set("X-Request-Id", notificationErrorRequestID)
			r.Header.Set("X-Actor-Wallet", notificationActorWallet)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			assertNotificationFrozenError(t, w, http.StatusBadRequest, "INVALID_REQUEST", notificationErrorRequestID)
		})
	}
	for _, id := range []string{"not-a-uuid", strings.Repeat("a", 32), "00000000-0000-0000-0000-00000000000z"} {
		t.Run(id, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPut, "/v2/me/notifications/"+id+"/read", nil)
			r.Header.Set("X-Request-Id", notificationErrorRequestID)
			r.Header.Set("X-Actor-Wallet", notificationActorWallet)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			assertNotificationFrozenError(t, w, http.StatusBadRequest, "INVALID_REQUEST", notificationErrorRequestID)
		})
	}
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		for _, wallet := range []string{"", "actor", "0X" + strings.Repeat("a", 40), "0x" + strings.Repeat("a", 39), "0x" + strings.Repeat("a", 41), "0x" + strings.Repeat("g", 40), notificationActorWallet + "\n"} {
			t.Run("wallet "+method+" "+wallet, func(t *testing.T) {
				path := "/v2/me/notifications"
				if method == http.MethodPut {
					path += "/" + uuid.NewString() + "/read"
				}
				r := httptest.NewRequest(method, path, nil)
				r.Header.Set("X-Request-Id", notificationErrorRequestID)
				r.Header.Set("X-Actor-Wallet", wallet)
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, r)
				assertNotificationFrozenError(t, w, http.StatusBadRequest, "INVALID_REQUEST", notificationErrorRequestID)
			})
		}
	}
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		t.Run("authentication "+method, func(t *testing.T) {
			path := "/v2/me/notifications"
			if method == http.MethodPut {
				path += "/" + uuid.NewString() + "/read"
			}
			r := httptest.NewRequest(method, path, nil)
			w := httptest.NewRecorder()
			(HTTP{}).Handler().ServeHTTP(w, r)
			assertNotificationFrozenError(t, w, http.StatusUnauthorized, "AUTH_REQUIRED", "")
		})
	}
}
