package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/rest"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
)

func TestRegisterDiscoveryFallbackMountsFrozenAlertRoutes(t *testing.T) {
	a := &App{}
	router := chi.NewRouter()
	admission, err := rest.NewAdmission(10, 1, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.registerDiscovery(context.Background(), router, nil, admission, map[string]sharedhttp.Check{}); err != nil {
		t.Fatalf("registerDiscovery: %v", err)
	}

	mounted := map[string]bool{}
	if err := chi.Walk(router, func(method, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		mounted[route] = true
		return nil
	}); err != nil {
		t.Fatalf("walk discovery routes: %v", err)
	}
	for _, pattern := range []string{"/v2/me/alerts", "/v2/me/alerts/*"} {
		t.Run(pattern, func(t *testing.T) {
			if !mounted[pattern] {
				t.Errorf("fallback discovery route %q is not mounted", pattern)
			}
		})
	}

	for _, route := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/v2/me/alerts"},
		{http.MethodDelete, "/v2/me/alerts/00000000-0000-4000-8000-000000000001"},
	} {
		t.Run(route.method, func(t *testing.T) {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(route.method, route.path, nil))
			if response.Code != http.StatusUnauthorized {
				t.Errorf("unauthenticated %s %s: status = %d, want %d; body = %s", route.method, route.path, response.Code, http.StatusUnauthorized, response.Body.String())
			}
		})
	}
}
