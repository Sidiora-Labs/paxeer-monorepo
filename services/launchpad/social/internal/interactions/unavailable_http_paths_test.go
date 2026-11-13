package interactions

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestCommittedRoutesReportUnavailableStorageForReadsAndWrites(t *testing.T) {
	f := connected(t)
	actor, peer := f.wallets[0], f.wallets[1]
	routes := f.http.Routes()
	f.db.Close()
	comment := uuid.NewString()
	for _, tc := range []struct {
		method, path, key, body string
	}{
		{http.MethodDelete, "/follow/" + peer, "unavailable-unfollow", ""},
		{http.MethodPatch, "/comments/" + comment, "unavailable-edit", `{"body":"updated text","expectedRevision":"1"}`},
		{http.MethodDelete, "/comments/" + comment, "unavailable-delete", ""},
		{http.MethodPut, "/comments/" + comment + "/like", "unavailable-like", ""},
		{http.MethodDelete, "/comments/" + comment + "/like", "unavailable-unlike", ""},
		{http.MethodPost, "/comments/" + comment + "/hide", "unavailable-hide", `{"poolAddress":"` + f.pool + `","reason":"reviewed","expectedRevision":"1"}`},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)).WithContext(f.ctx[actor])
			if tc.key != "" {
				request.Header.Set("Idempotency-Key", tc.key)
			}
			response := httptest.NewRecorder()
			routes.ServeHTTP(response, request)
			if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "DEPENDENCY_UNAVAILABLE") {
				t.Fatalf("unavailable route = %d %s", response.Code, response.Body.String())
			}
		})
	}
}
