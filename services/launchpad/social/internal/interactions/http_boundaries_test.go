package interactions

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestInteractionHTTPErrorAndRequestBounds(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{ErrInvalid, 400, "INVALID_INPUT"},
		{ErrDenied, 403, "SCOPE_DENIED"},
		{ErrRevision, 409, "REVISION_CONFLICT"},
		{ErrDepth, 422, "thread_depth_exceeded"},
		{ErrReactionKind, 422, "unsupported_reaction_kind"},
		{pgx.ErrNoRows, 404, "NOT_FOUND"},
		{ErrDependency, 503, "DEPENDENCY_UNAVAILABLE"},
	} {
		response := httptest.NewRecorder()
		httpError(response, tc.err)
		if response.Code != tc.status || !bytes.Contains(response.Body.Bytes(), []byte(`"code":"`+tc.code+`"`)) {
			t.Fatalf("interaction response %v = %d %s", tc.err, response.Code, response.Body.String())
		}
	}
	for _, key := range []string{"", strings.Repeat("x", 129), "line\nbreak", "zero\x00byte"} {
		request := httptest.NewRequest(http.MethodPost, "/follow/peer", nil)
		if key != "" {
			request.Header.Set("Idempotency-Key", key)
		}
		if _, err := requestKey(request); err != ErrInvalid {
			t.Fatalf("idempotency key %q = %v", key, err)
		}
	}
	request := httptest.NewRequest(http.MethodPost, "/follow/peer", nil)
	request.Header.Set("Idempotency-Key", "committed-action")
	if key, err := requestKey(request); err != nil || key != "committed-action" {
		t.Fatalf("accepted idempotency key = %q: %v", key, err)
	}
	for _, tc := range []struct {
		query   string
		limit   int
		invalid bool
	}{
		{"", 50, false},
		{"?limit=1", 1, false},
		{"?limit=101", 100, false},
		{"?limit=0", 0, true},
		{"?limit=invalid", 0, true},
	} {
		request := httptest.NewRequest(http.MethodGet, "/users/wallet/followers"+tc.query, nil)
		limit, err := limitOf(request)
		if limit != tc.limit || (err != nil) != tc.invalid {
			t.Fatalf("page limit %q = %d: %v", tc.query, limit, err)
		}
	}
	request = httptest.NewRequest(http.MethodGet, "/users/wallet/followers", nil)
	response := httptest.NewRecorder()
	if HandleOwned(response, request, "followers") || response.Body.Len() != 0 {
		t.Fatal("unbound interaction route handled")
	}
}

func TestInteractionRoutesRejectMalformedActorsAndRequests(t *testing.T) {
	f := connected(t)
	actor := f.wallets[0]
	routes := f.http.Routes()
	for _, tc := range []struct {
		method, path, body, key string
		status                  int
	}{
		{http.MethodGet, "/users/invalid/followers", "", "", 400},
		{http.MethodGet, "/users/invalid/following", "", "", 400},
		{http.MethodGet, "/users/invalid/follow-stats", "", "", 400},
		{http.MethodGet, "/users/" + actor + "/followers?cursor=invalid", "", "", 400},
		{http.MethodGet, "/pool/invalid/comments", "", "", 400},
		{http.MethodPost, "/pool/" + f.pool + "/comments", `{"body":"community post","clientCommentId":"invalid"}`, "comment-invalid-id", 400},
		{http.MethodPost, "/pool/" + f.pool + "/comments", `{"body":"community post","clientCommentId":"invalid"}`, "", 400},
		{http.MethodPut, "/follow/" + actor, "", "follow-self", 400},
		{http.MethodPut, "/follow/invalid", "", "follow-invalid", 400},
		{http.MethodPatch, "/me/social/privacy", `{"inbox":"unknown","expectedRevision":"1"}`, "", 400},
		{http.MethodPatch, "/me/social/privacy", `{"inbox":"everyone","expectedRevision":"invalid"}`, "", 400},
		{http.MethodPut, "/me/social/blocks/" + actor, "", "", 400},
		{http.MethodPost, "/callouts/invalid/references", `{"revision":"invalid","mentions":[]}`, "reference-invalid", 400},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)).WithContext(f.ctx[actor])
			if tc.key != "" {
				request.Header.Set("Idempotency-Key", tc.key)
			}
			response := httptest.NewRecorder()
			routes.ServeHTTP(response, request)
			if response.Code != tc.status {
				t.Fatalf("request = %d %s", response.Code, response.Body.String())
			}
		})
	}
}
