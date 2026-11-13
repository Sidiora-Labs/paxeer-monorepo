package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/rest"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
	"github.com/go-chi/chi/v5"
)

func TestDiscoveryRegistrationRoutesAndReadiness(t *testing.T) {
	admission, err := rest.NewAdmission(1000, 1000, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	app := &App{}
	router := chi.NewRouter()
	checks := map[string]sharedhttp.Check{}
	if err := app.registerProductionOperations(context.Background(), router, nil, admission, checks); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"discovery_sources", "notification_delivery"} {
		check := checks[key]
		if check == nil || check(context.Background()) == nil {
			t.Fatalf("%s has no readiness check", key)
		}
	}
	for _, route := range []string{"/v2/discovery/feed", "/v2/discovery/leaderboards", "/v2/discovery/callouts/pool"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, route, nil))
		if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "DEPENDENCY_UNAVAILABLE") {
			t.Fatalf("%s: %d %s", route, response.Code, response.Body.String())
		}
	}
	for _, route := range []string{"/v2/me/notifications", "/v2/me/alerts", "/v2/squads", "/v2/discovery/notifications"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, route, nil))
		if response.Code == http.StatusOK || response.Code == http.StatusNotFound {
			t.Fatalf("%s unexpectedly served without identity: %d %s", route, response.Code, response.Body.String())
		}
	}
	bound := &App{cfg: config.Config{DiscoveryRuntimeFile: "missing-runtime.json", DiscoveryRuntimeSHA256: strings.Repeat("0", 64)}}
	if err := bound.registerDiscovery(context.Background(), chi.NewRouter(), nil, admission, map[string]sharedhttp.Check{}); err == nil {
		t.Fatal("unreadable discovery runtime accepted")
	}
}
