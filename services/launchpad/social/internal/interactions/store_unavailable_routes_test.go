package interactions

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestCommittedSocialRoutesReportUnavailableStore(t *testing.T) {
	f := connected(t)
	actor, peer := f.wallets[0], f.wallets[1]
	routes := f.http.Routes()
	f.db.Close()
	for _, tc := range []struct {
		method, path, body, key string
		public                  bool
	}{
		{http.MethodGet, "/me/social/privacy", "", "", false},
		{http.MethodGet, "/users/" + actor + "/followers", "", "", true},
		{http.MethodGet, "/users/" + actor + "/following", "", "", true},
		{http.MethodGet, "/users/" + actor + "/follow-stats", "", "", true},
		{http.MethodGet, "/pool/" + f.pool + "/comments", "", "", true},
		{http.MethodPut, "/follow/" + peer, "", "follow-unavailable", false},
		{http.MethodPost, "/pool/" + f.pool + "/comments", `{"body":"community post","clientCommentId":"` + uuid.NewString() + `"}`, "comment-unavailable", false},
		{http.MethodPut, "/me/social/blocks/" + peer, "", "", false},
		{http.MethodPatch, "/me/social/privacy", `{"inbox":"nobody","expectedRevision":"1"}`, "", false},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
			if !tc.public {
				r = r.WithContext(f.ctx[actor])
			}
			if tc.key != "" {
				r.Header.Set("Idempotency-Key", tc.key)
			}
			if tc.body != "" {
				r.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()
			routes.ServeHTTP(w, r)
			if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "DEPENDENCY_UNAVAILABLE") {
				t.Fatalf("unavailable social route = %d %s", w.Code, w.Body.String())
			}
		})
	}
}
