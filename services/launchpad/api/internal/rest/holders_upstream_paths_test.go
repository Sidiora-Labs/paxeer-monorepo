package rest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestHoldersProxyClassifiesUpstreamResponses(t *testing.T) {
	const pool = "0x1111111111111111111111111111111111111111"
	for _, check := range []struct {
		name, body, upstreamCode, wantCode string
		upstreamStatus, wantStatus         int
	}{
		{"known error", `{"code":"NOT_FOUND"}`, "NOT_FOUND", "NOT_FOUND", 404, 404},
		{"unknown error", `{"code":"UNRECOGNIZED"}`, "UNRECOGNIZED", "DEPENDENCY_UNAVAILABLE", 404, 503},
		{"status mismatch", `{"code":"NOT_FOUND"}`, "NOT_FOUND", "DEPENDENCY_UNAVAILABLE", 422, 503},
		{"malformed error", `{`, "", "DEPENDENCY_UNAVAILABLE", 404, 503},
		{"invalid success", `{`, "", "INTEGRITY_QUARANTINED", 200, 503},
		{"oversized success", strings.Repeat("x", 1<<20), "", "INTEGRITY_QUARANTINED", 200, 503},
	} {
		t.Run(check.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/pools/"+pool+"/holders" || r.URL.Query().Get("limit") != "7" || r.URL.Query().Get("cursor") != "page" {
					t.Errorf("upstream request: %s", r.URL)
				}
				w.WriteHeader(check.upstreamStatus)
				_, _ = io.WriteString(w, check.body)
			}))
			defer server.Close()
			upstream, err := newHoldersUpstream(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(http.MethodGet, "/v2/pools/"+pool+"/holders?limit=7&cursor=page", nil)
			route := chi.NewRouteContext()
			route.URLParams.Add("poolAddress", pool)
			request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, route))
			response := httptest.NewRecorder()
			holdersProxy(upstream)(response, request)
			if response.Code != check.wantStatus || !strings.Contains(response.Body.String(), `"code":"`+check.wantCode+`"`) {
				t.Fatalf("proxy response: %d %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestHoldersProxyPreservesValidatedPagesAndPrivacy(t *testing.T) {
	const pool = "0x1111111111111111111111111111111111111111"
	const body = `{"requestId":"00000000-0000-4000-8000-000000000001","data":{"items":[],"coverage":"partial","fromPosition":null,"toPosition":null,"exclusionPolicyVersion":"v1","excludedProtocolAddresses":[]},"watermark":null,"revision":null,"quality":"partial","nextCursor":null,"generatedAt":"2026-10-12T00:00:00.000Z"}`
	for _, check := range []struct {
		name, upstreamCache, wantCache string
	}{
		{"public page", "public, max-age=30", "no-store"},
		{"private page", "private, no-store", "private, no-store"},
	} {
		t.Run(check.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/v1/pools/"+pool+"/holders" || r.URL.Query().Get("limit") != "5" {
					t.Errorf("upstream request: %s %s", r.Method, r.URL)
				}
				w.Header().Set("Cache-Control", check.upstreamCache)
				_, _ = io.WriteString(w, body)
			}))
			defer server.Close()
			upstream, err := newHoldersUpstream(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(http.MethodGet, "/v2/pools/"+pool+"/holders?limit=5", nil)
			route := chi.NewRouteContext()
			route.URLParams.Add("poolAddress", pool)
			request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, route))
			response := httptest.NewRecorder()
			holdersProxy(upstream)(response, request)
			var page struct {
				RequestID string `json:"requestId"`
				Data      struct {
					Items []json.RawMessage `json:"items"`
				} `json:"data"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil || response.Code != http.StatusOK || page.RequestID == "" || page.RequestID == "00000000-0000-4000-8000-000000000001" || len(page.Data.Items) != 0 || response.Header().Get("Cache-Control") != check.wantCache {
				t.Fatalf("holders page: status=%d body=%s cache=%q error=%v", response.Code, response.Body.String(), response.Header().Get("Cache-Control"), err)
			}
		})
	}
}
