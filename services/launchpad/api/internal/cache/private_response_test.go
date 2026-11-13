package cache

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPrivateRepresentationUsesScopeAndRevision(t *testing.T) {
	body := []byte(`{"balance":"100"}`)
	request := httptest.NewRequest(http.MethodGet, "/v2/wallet", nil)
	first := httptest.NewRecorder()
	ServePrivateJSON(first, request, body, "wallet-a", "revision-1")
	if first.Code != 200 || first.Body.String() != string(body) || first.Header().Get("Cache-Control") != "private, no-store" || first.Header().Get("ETag") == "" {
		t.Fatalf("private response: %d %s %v", first.Code, first.Body.String(), first.Header())
	}
	conditional := httptest.NewRequest(http.MethodGet, "/v2/wallet", nil)
	conditional.Header.Set("If-None-Match", first.Header().Get("ETag"))
	unchanged := httptest.NewRecorder()
	ServePrivateJSON(unchanged, conditional, body, "wallet-a", "revision-1")
	if unchanged.Code != http.StatusNotModified || unchanged.Body.Len() != 0 {
		t.Fatalf("same private revision: %d %s", unchanged.Code, unchanged.Body.String())
	}
	for _, tc := range []struct{ scope, revision string }{{"wallet-b", "revision-1"}, {"wallet-a", "revision-2"}, {"", "revision-1"}} {
		response := httptest.NewRecorder()
		ServePrivateJSON(response, conditional, body, tc.scope, tc.revision)
		if response.Code != 200 || response.Body.String() != string(body) {
			t.Fatalf("changed private representation %q %q: %d %s", tc.scope, tc.revision, response.Code, response.Body.String())
		}
		if tc.scope == "" && response.Header().Get("ETag") != "" {
			t.Fatalf("unscoped response has validator: %v", response.Header())
		}
	}
}
