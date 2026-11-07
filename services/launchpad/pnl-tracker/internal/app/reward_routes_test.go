package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l18"
	"github.com/go-chi/chi/v5"
)

func TestRegisterRewardRoutesMountsFrozenRewardOperations(t *testing.T) {
	router := chi.NewRouter()
	registerRewardRoutes(router, l18.HTTP{})

	want := map[string]int{
		"GET /v2/me/rewards":                1,
		"POST /v2/me/rewards:prepare-claim": 1,
	}
	got := make(map[string]int)
	if err := chi.Walk(router, func(method, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		got[method+" "+route]++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("reward routes = %v, want frozen operations %v", got, want)
	}

	for _, operation := range []struct {
		name   string
		method string
		path   string
	}{
		{"IF_REWARDS", http.MethodGet, "/v2/me/rewards"},
		{"IF_REWARD_CLAIM", http.MethodPost, "/v2/me/rewards:prepare-claim"},
	} {
		t.Run(operation.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(operation.method, operation.path, nil))
			if response.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want %d from the unavailable l18 handler; body = %s", response.Code, http.StatusServiceUnavailable, response.Body.String())
			}
			var body struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Code != "DEPENDENCY_UNAVAILABLE" {
				t.Fatalf("error code = %q, want DEPENDENCY_UNAVAILABLE", body.Code)
			}
		})
	}

	for _, request := range []struct {
		method string
		path   string
		status int
	}{
		{http.MethodPost, "/v2/me/rewards", http.StatusMethodNotAllowed},
		{http.MethodGet, "/v2/me/rewards:prepare-claim", http.StatusMethodNotAllowed},
		{http.MethodPost, "/v2/me/rewards/claim", http.StatusNotFound},
	} {
		t.Run(request.method+" "+request.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(request.method, request.path, nil))
			if response.Code != request.status {
				t.Fatalf("status = %d, want %d", response.Code, request.status)
			}
		})
	}
}
