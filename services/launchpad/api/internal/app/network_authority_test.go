package app

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/rest"
	"github.com/go-chi/chi/v5"
)

func TestNetworkAuthorityRoutesFailClosedWithoutCommittedSource(t *testing.T) {
	a, err := newNetworkAuthority(context.Background(), nil, &config.Config{})
	if err != nil || a == nil || a.unavailable == nil {
		t.Fatalf("unconfigured network authority: %+v, %v", a, err)
	}
	if err := a.Ready(context.Background()); err == nil {
		t.Fatal("unconfigured authority reported ready")
	}
	admission, err := rest.NewAdmission(20, 20, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	a.Register(router, admission)
	for _, tc := range []struct {
		method, path string
		status       int
		code         string
	}{
		{http.MethodGet, "/v2/network/authority", 503, "DEPENDENCY_UNAVAILABLE"},
		{http.MethodGet, "/v2/network/authority?x=1", 400, "QUERY_INVALID"},
		{http.MethodPost, "/v2/preflight/create?x=1", 400, "QUERY_INVALID"},
		{http.MethodPost, "/v2/preflight/create", 403, "SCOPE_DENIED"},
	} {
		record := httptest.NewRecorder()
		router.ServeHTTP(record, httptest.NewRequest(tc.method, tc.path, nil))
		if record.Code != tc.status || !strings.Contains(record.Body.String(), `"code":"`+tc.code+`"`) {
			t.Errorf("%s %s: %d %s", tc.method, tc.path, record.Code, record.Body.String())
		}
	}
	request := httptest.NewRequest(http.MethodPost, "/v2/preflight/create", strings.NewReader(`{}`))
	origin := "https://" + request.Host
	a.origins[origin] = true
	request.Header.Set("Origin", origin)
	record := httptest.NewRecorder()
	router.ServeHTTP(record, request)
	if record.Code != 422 || !strings.Contains(record.Body.String(), `"code":"FIELD_INVALID"`) {
		t.Fatalf("invalid preflight body: %d %s", record.Code, record.Body.String())
	}
	if _, err := contractPreflight([]byte(`{`)); err == nil {
		t.Fatal("malformed preflight accepted")
	}
}

func TestNetworkRPCResponseBound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/large" {
			_, _ = io.WriteString(w, strings.Repeat("x", (2<<20)+1))
			return
		}
		_, _ = io.WriteString(w, "ready")
	}))
	defer srv.Close()
	client := &http.Client{Transport: boundedRPCTransport{base: http.DefaultTransport}}
	response, err := client.Get(srv.URL + "/small")
	if err != nil {
		t.Fatal(err)
	}
	ready, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || string(ready) != "ready" {
		t.Fatalf("bounded response: %q %v", ready, err)
	}
	response, err = client.Get(srv.URL + "/large")
	if err == nil {
		_, err = io.ReadAll(response.Body)
		response.Body.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "RPC response exceeds bound") {
		t.Fatalf("oversized response accepted: %v", err)
	}
}

func TestDevelopmentPreflightRoutesValidateRequestBeforeNetworkRead(t *testing.T) {
	config := &config.Config{CORSAllowedOrigins: "https://example.com"}
	authority, err := newDevelopmentAuthority(context.Background(), developmentReadDeps{cfg: config})
	if err != nil {
		t.Fatal(err)
	}
	admission, err := rest.NewAdmission(20, 20, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	authority.Register(router, admission)
	for _, check := range []struct {
		name, path, body, origin, code string
		status                         int
	}{
		{"query", "/v2/preflight/create?extra=1", `{}`, "https://example.com", "QUERY_INVALID", 400},
		{"origin", "/v2/preflight/create", `{}`, "https://other.example", "SCOPE_DENIED", 403},
		{"body size", "/v2/preflight/create", strings.Repeat("x", 16<<10+1), "https://example.com", "FIELD_INVALID", 422},
		{"unknown field", "/v2/preflight/create", `{"family":"creation","extra":1}`, "https://example.com", "FIELD_INVALID", 422},
		{"family mismatch", "/v2/preflight/create", `{"family":"trading"}`, "https://example.com", "FIELD_INVALID", 422},
		{"network unavailable", "/v2/preflight/create", `{"family":"creation"}`, "https://example.com", "DEPENDENCY_UNAVAILABLE", 503},
	} {
		t.Run(check.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, check.path, strings.NewReader(check.body))
			request.Header.Set("Origin", check.origin)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != check.status || !strings.Contains(response.Body.String(), `"code":"`+check.code+`"`) {
				t.Fatalf("preflight response: %d %s", response.Code, response.Body.String())
			}
		})
	}
}
