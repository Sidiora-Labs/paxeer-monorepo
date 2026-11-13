package app

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/interactions"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestFrozenPublicReadsAndErrorsPreserveContract(t *testing.T) {
	pool := addr(0xabcd)
	id := uuid.NewString()
	for _, tc := range []struct {
		method, path string
		public       bool
	}{
		{http.MethodGet, "/v2/pools/" + pool + "/comments", true},
		{http.MethodGet, "/v2/pools/" + pool + "/callouts", true},
		{http.MethodGet, "/v2/bounties", true},
		{http.MethodGet, "/v2/bounties/stats", true},
		{http.MethodGet, "/v2/bounties/" + id, true},
		{http.MethodGet, "/v2/bounties/" + id + "/submissions", true},
		{http.MethodGet, "/v2/social/pool/" + pool + "/comments", true},
		{http.MethodPost, "/v2/bounties", false},
		{http.MethodGet, "/v2/pools/invalid/comments", false},
		{http.MethodGet, "/v2/bounties/invalid", false},
		{http.MethodGet, "/v2/bounties/invalid/submissions", false},
		{http.MethodGet, "/v2/social/pool/invalid/comments", false},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		if actual := frozenPublicRead(req); actual != tc.public {
			t.Fatalf("public read %s %s = %t", tc.method, tc.path, actual)
		}
	}
	if frozenPublicRead(nil) {
		t.Fatal("nil request is public")
	}
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{interactions.ErrInvalid, 422, "FIELD_INVALID"},
		{safety.ErrInvalid, 422, "FIELD_INVALID"},
		{interactions.ErrDenied, 403, "SCOPE_DENIED"},
		{safety.ErrForbidden, 403, "SCOPE_DENIED"},
		{interactions.ErrRevision, 409, "REVISION_CONFLICT"},
		{safety.ErrConflict, 409, "REVISION_CONFLICT"},
		{interactions.ErrDepth, 422, "THREAD_DEPTH_EXCEEDED"},
		{pgx.ErrNoRows, 404, "NOT_FOUND"},
		{interactions.ErrDependency, 503, "DEPENDENCY_UNAVAILABLE"},
	} {
		req := httptest.NewRequest(http.MethodGet, "/v2/bounties", nil)
		req.Header.Set("X-Request-ID", id)
		rw := httptest.NewRecorder()
		frozenError(rw, req, tc.err)
		if rw.Code != tc.status || !strings.Contains(rw.Body.String(), `"code":"`+tc.code+`"`) || !strings.Contains(rw.Body.String(), id) {
			t.Fatalf("frozen error %v = %d %s", tc.err, rw.Code, rw.Body.String())
		}
	}
	for _, key := range []string{strings.Repeat("x", 129), "invalid\nkey", "invalid\x7fkey"} {
		req := httptest.NewRequest(http.MethodPost, "/v2/callouts", nil)
		req.Header.Set("Idempotency-Key", key)
		if _, err := frozenKey(req); !errors.Is(err, interactions.ErrInvalid) {
			t.Fatalf("idempotency key %q = %v", key, err)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/v2/callouts", nil)
	req.Header.Set("X-Request-ID", id)
	if key, err := frozenKey(req); err != nil || key != "request:"+id {
		t.Fatalf("request key = %q: %v", key, err)
	}
}
