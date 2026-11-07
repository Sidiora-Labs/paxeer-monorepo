package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/httpapi"
	canonicalsearch "github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/search"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/go-chi/chi/v5"
)

func TestCanonicalSearchRejectsQueriesOutsideFrozenSearch(t *testing.T) {
	router := chi.NewRouter()
	httpapi.RegisterCanonicalSearch(router, canonicalsearch.Store{Pool: nil})

	for _, test := range []struct {
		name string
		path string
		code string
	}{
		{"missing q", "/v2/search?type=pool", "FIELD_INVALID"},
		{"unsupported type", "/v2/search?q=coin&type=wallet", "FIELD_INVALID"},
		{"repeated q", "/v2/search?q=coin&q=other&type=pool", "QUERY_INVALID"},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			router.ServeHTTP(response, request)

			if response.Code < http.StatusBadRequest || response.Code >= http.StatusInternalServerError {
				t.Fatalf("status = %d, want 4xx rejection before store access; body = %s", response.Code, response.Body.String())
			}
			if err := v2.ValidateJSON("Error", response.Body.Bytes()); err != nil {
				t.Fatalf("response violates frozen Error schema: %v; body = %s", err, response.Body.String())
			}
			var body struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode Error: %v", err)
			}
			if body.Code != test.code {
				t.Fatalf("code = %q, want parser rejection %q before store access", body.Code, test.code)
			}
		})
	}
}
