package social

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/interactions"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

func TestLegacyChatErrorsAndAdminGatePreserveHTTPStatus(t *testing.T) {
	h := New(Deps{AdminAPIKey: "fixture-admin-key"})
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{fanout.ErrNotFound, 404, "conversation not found"},
		{pgx.ErrNoRows, 404, "conversation not found"},
		{interactions.ErrDenied, 403, "chat access denied"},
		{safety.ErrForbidden, 403, "chat access denied"},
		{fanout.ErrConflict, 409, "revision conflict"},
		{fanout.ErrDeleted, 410, "message deleted"},
		{fanout.ErrInvalid, 400, "invalid chat input"},
		{fanout.ErrCursor, 400, "invalid chat input"},
		{fanout.ErrReplayExpired, 410, fanout.ErrReplayExpired.Error()},
		{fanout.ErrSnapshotRequired, 410, fanout.ErrSnapshotRequired.Error()},
		{fanout.ErrUnavailable, 503, "chat_unavailable"},
	} {
		rw := httptest.NewRecorder()
		h.chatError(rw, tc.err, fanout.HistoryPage{})
		if rw.Code != tc.status || !bytes.Contains(rw.Body.Bytes(), []byte(tc.code)) || rw.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatalf("chat error %v = %d %s", tc.err, rw.Code, rw.Body.String())
		}
	}
	for _, tc := range []struct {
		body    string
		status  int
		content string
	}{
		{`{`, 400, ""},
		{`{"content":" "}`, 400, ""},
		{`{"content":"` + strings.Repeat("x", editMaxLen+1) + `"}`, 400, ""},
		{`{"content":"  revised message  "}`, 200, "revised message"},
	} {
		req := httptest.NewRequest(http.MethodPatch, "/pool/x/messages/y", strings.NewReader(tc.body))
		rw := httptest.NewRecorder()
		content, ok := h.decodeEdit(rw, req)
		if content != tc.content || ok != (tc.status == 200) || (tc.status != 200 && rw.Code != tc.status) {
			t.Fatalf("edit body %q = %q, %t, %d", tc.body, content, ok, rw.Code)
		}
	}
	for _, tc := range []struct {
		key    string
		status int
	}{
		{"", 401},
		{"wrong-key", 403},
		{"fixture-admin-key", 204},
	} {
		rw := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/admin/bans", nil)
		if tc.key != "" {
			req.Header.Set("X-API-Key", tc.key)
		}
		h.adminAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })).ServeHTTP(rw, req)
		if rw.Code != tc.status {
			t.Fatalf("admin key %q = %d %s", tc.key, rw.Code, rw.Body.String())
		}
	}
	empty := New(Deps{})
	rw := httptest.NewRecorder()
	empty.adminAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })).ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/admin/bans", nil))
	if rw.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured admin gate = %d", rw.Code)
	}
	rw = httptest.NewRecorder()
	h.fail(rw, "read", errors.New("unavailable"))
	if rw.Code != http.StatusInternalServerError {
		t.Fatalf("internal handler error = %d", rw.Code)
	}
}

func TestLegacyChatRoutesRejectUnboundReadsAndInvalidPool(t *testing.T) {
	h := New(Deps{})
	router := chi.NewRouter()
	h.RegisterRoutes(router)
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{http.MethodGet, "/pool/invalid/messages", 400},
		{http.MethodPatch, "/pool/invalid/messages/id", 400},
		{http.MethodDelete, "/pool/invalid/messages/id", 400},
		{http.MethodGet, "/pool/0x1111111111111111111111111111111111111111/messages", 503},
		{http.MethodGet, "/dm/conversations", 503},
		{http.MethodGet, "/dm/conversations/pool:0x1111111111111111111111111111111111111111/messages", 404},
		{http.MethodGet, "/dm/conversations/other/messages", 503},
		{http.MethodPost, "/report", 503},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rw := httptest.NewRecorder()
		router.ServeHTTP(rw, req)
		if rw.Code != tc.status {
			t.Fatalf("%s %s = %d %s", tc.method, tc.path, rw.Code, rw.Body.String())
		}
	}
}
