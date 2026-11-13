package l17

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestPortfolioHTTPRequiresIdentityAndValidResources(t *testing.T) {
	r := chi.NewRouter()
	service := Service{}
	service.RegisterHTTP(r)
	routes := []struct{ method, path string }{
		{"GET", "/v2/me/portfolio"}, {"GET", "/v2/me/portfolio/history"},
		{"POST", "/v2/me/portfolio/exports"}, {"GET", "/v2/me/portfolio/exports/id"}, {"GET", "/v2/me/portfolio/exports/id/download"}, {"DELETE", "/v2/me/portfolio/exports/id"},
		{"POST", "/v2/me/portfolio/cards"}, {"GET", "/v2/me/portfolio/cards/id"}, {"DELETE", "/v2/me/portfolio/cards/id"},
	}
	for _, route := range routes {
		out := httptest.NewRecorder()
		r.ServeHTTP(out, httptest.NewRequest(route.method, route.path, nil))
		if out.Code != 503 || !strings.Contains(out.Body.String(), "DEPENDENCY_UNAVAILABLE") {
			t.Fatal(route, out.Code, out.Body.String())
		}
	}
	repository := phase2Repository(t)
	authority, _ := portfolioIdentity(t)
	service = Service{Repository: repository, Authority: authority, CursorKey: []byte(strings.Repeat("k", 32)), ExportsDir: t.TempDir()}
	if err := os.Chmod(service.ExportsDir, 0700); err != nil {
		t.Fatal(err)
	}
	r = chi.NewRouter()
	service.RegisterHTTP(r)
	for _, route := range []struct {
		method, path string
		status       int
	}{
		{"GET", "/v2/wallets/invalid/portfolio", 400},
		{"GET", "/v2/me/portfolio/history?from=2026-01-01T00:00:00.000Z&to=invalid", 400},
		{"GET", "/v2/me/portfolio/history?from=2026-01-01T00:00:00.000Z&to=2026-01-02T00:00:00.000Z&limit=bad", 400},
		{"GET", "/v2/me/portfolio/history?from=2026-01-01T00:00:00.000Z&to=2026-01-02T00:00:00.000Z&interval=bad", 400},
		{"GET", "/v2/me/portfolio/history?from=2026-01-01T00:00:00.000Z&to=2026-01-02T00:00:00.000Z&interval=1h", 200},
		{"GET", "/v2/me/portfolio/history?bad=%zz", 400},
		{"GET", "/v2/me/portfolio?bad=%zz", 400},
		{"GET", "/v2/me/portfolio/exports/" + uuid.NewString() + "/download", 404},
		{"DELETE", "/v2/me/portfolio/exports/" + uuid.NewString(), 404},
		{"DELETE", "/v2/me/portfolio/cards/" + uuid.NewString(), 404},
	} {
		portfolioRequest(t, r, route.method, route.path, nil, route.status)
	}
	missing := uuid.NewString()
	portfolioRequest(t, r, "POST", "/v2/me/portfolio/exports", map[string]any{"snapshotId": missing, "format": "jsonl"}, 404)
	portfolioRequest(t, r, "POST", "/v2/me/portfolio/cards", map[string]any{"snapshotId": missing}, 409)
	// Identity rejection is translated to the same public error on every owner route.
	for _, route := range routes {
		request := httptest.NewRequest(route.method, route.path, nil)
		request.Header.Set("Authorization", "Bearer invalid-owner")
		out := httptest.NewRecorder()
		r.ServeHTTP(out, request)
		if out.Code != 403 || !strings.Contains(out.Body.String(), "SCOPE_DENIED") {
			t.Fatal(route, out.Code, out.Body.String())
		}
	}
}

func TestSelectedShareHTTPRejectsAmbiguousIdentityAndBody(t *testing.T) {
	s, _, summary := sharingV3Fixture(t)
	authority, _ := portfolioIdentity(t)
	s.authority = authority
	r := chi.NewRouter()
	s.RegisterHTTP(r)
	routes := []struct{ method, path string }{{"POST", "/v3/me/portfolio/shares/preview"}, {"POST", "/v3/me/portfolio/shares"}, {"GET", "/v3/me/portfolio/shares/id"}, {"DELETE", "/v3/me/portfolio/shares/id"}}
	for _, route := range routes {
		for _, mode := range []string{"query", "authorization", "origin", "csrf", "idempotency", "cookies", "authority"} {
			request := httptest.NewRequest(route.method, route.path, nil)
			request.Header.Set("Authorization", "Bearer portfolio-owner")
			want := 403
			switch mode {
			case "query":
				request.URL.RawQuery = "x=1"
				want = 400
			case "authorization":
				request.Header.Add("Authorization", "Bearer duplicate")
			case "origin":
				request.Header.Add("Origin", "https://sidiora.fun")
				request.Header.Add("Origin", "https://sidiora.fun")
			case "csrf":
				request.Header.Add("X-CSRF-Token", "a")
				request.Header.Add("X-CSRF-Token", "b")
			case "idempotency":
				request.Header.Add("Idempotency-Key", "a")
				request.Header.Add("Idempotency-Key", "b")
			case "cookies":
				request.Header.Set("Cookie", "__Host-kindle_access=a; __Host-kindle_access=b")
			case "authority":
				s.authority = nil
				want = 503
			}
			out := httptest.NewRecorder()
			r.ServeHTTP(out, request)
			s.authority = authority
			if out.Code != want {
				t.Fatal(route, mode, out.Code, out.Body.String())
			}
		}
	}
	expiry := time.Now().Add(time.Hour)
	for _, tc := range []struct {
		path   string
		body   any
		status int
	}{
		{"/v3/me/portfolio/shares/preview", map[string]any{"snapshotId": summary.SnapshotID, "fields": []string{"wallet"}, "expiresAt": expiry}, 400},
		{"/v3/me/portfolio/shares/preview", map[string]any{"unknown": true}, 400},
		{"/v3/me/portfolio/shares", map[string]any{"unknown": true}, 400},
		{"/v3/me/portfolio/shares", SharePublishV3{ID: uuid.NewString(), Intent: strings.Repeat("s", 43)}, 404},
	} {
		portfolioRequest(t, r, "POST", tc.path, tc.body, tc.status)
	}
	for _, path := range []string{"/v3/me/portfolio/shares/" + uuid.NewString(), "/v3/portfolio/shares/" + uuid.NewString() + "/image.png"} {
		portfolioRequest(t, r, "GET", path, nil, 404)
	}
	portfolioRequest(t, r, "DELETE", "/v3/me/portfolio/shares/"+uuid.NewString(), nil, 404)
}

func TestPortfolioJSONRejectsMalformedNestedValues(t *testing.T) {
	for _, raw := range []string{"", `{"a":`, `{"a":{"b":}}`, `{"a":[}`, `{"a":1`, `{"a":[1}`, `]`, `{]`, `{} {}`, string([]byte{0xff}), strings.Repeat("x", 4097)} {
		var target map[string]any
		if err := decodeBody(httptest.NewRecorder(), httptest.NewRequest("POST", "/", strings.NewReader(raw)), &target); !errors.Is(err, ErrInvalid) {
			t.Fatal(raw, err)
		}
	}
	valid := []byte(`{"a":[1,{"b":true}],"c":null}`)
	if err := uniqueJSON(valid); err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(valid, &parsed); err != nil || len(parsed) != 2 {
		t.Fatal(parsed, err)
	}
	out := httptest.NewRecorder()
	writeFailure(out, context.DeadlineExceeded)
	if out.Code != 503 || out.Header().Get("Retry-After") != "1" {
		t.Fatal(out.Code, out.Header())
	}
}
