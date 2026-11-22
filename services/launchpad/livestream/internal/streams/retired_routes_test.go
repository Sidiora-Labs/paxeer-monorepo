package streams

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/go-chi/chi/v5"
)

func TestRetiredRoutesNameVersionedSuccessor(t *testing.T) {
	router := chi.NewRouter()
	var logs bytes.Buffer
	New(Deps{Logger: slog.New(slog.NewJSONHandler(&logs, nil))}).RegisterRoutes(router)
	for _, route := range []struct{ method, path string }{
		{http.MethodPost, "/streams"},
		{http.MethodPost, "/streams/11111111-1111-4111-8111-111111111111/go-live"},
		{http.MethodPost, "/streams/11111111-1111-4111-8111-111111111111/end"},
		{http.MethodPost, "/streams/11111111-1111-4111-8111-111111111111/heartbeat"},
		{http.MethodGet, "/streams/live"},
		{http.MethodGet, "/streams/pool/0x2222222222222222222222222222222222222222"},
		{http.MethodGet, "/streams/11111111-1111-4111-8111-111111111111"},
	} {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			logs.Reset()
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(route.method, route.path, nil))
			var body v2.Error
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if response.Code != http.StatusServiceUnavailable || body.Code != "DEPENDENCY_UNAVAILABLE" || !body.Retryable || body.Details == nil || body.Details.Dependency.Value != "livestream_store" || v2.ValidateJSON("Error", response.Body.Bytes()) != nil {
				t.Fatalf("retired route: %d %s", response.Code, response.Body.String())
			}
			if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Link") != "</v2"+route.path+">; rel=\"successor-version\"" {
				t.Fatalf("successor headers: %v", response.Header())
			}
			if !strings.Contains(logs.String(), "unversioned livestream API unavailable; use /v2/streams") {
				t.Fatalf("unversioned route diagnostics: %s", logs.String())
			}
		})
	}
}
