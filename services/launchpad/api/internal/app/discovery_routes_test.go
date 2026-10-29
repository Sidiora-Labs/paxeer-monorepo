package app

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/rest"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
)

func TestRegisterDiscoveryFallbackMountsFrozenSquadAndNotificationRoutes(t *testing.T) {
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
	for _, pattern := range []string{
		"/v2/squads",
		"/v2/squads/*",
		"/v2/me/notifications",
		"/v2/me/notifications/*",
	} {
		t.Run(pattern, func(t *testing.T) {
			if !mounted[pattern] {
				t.Errorf("fallback discovery route %q is not mounted", pattern)
			}
		})
	}
}
