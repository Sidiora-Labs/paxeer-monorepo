package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/rest"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func TestProductionUnservedOperationsAnswerNotConfigured(t *testing.T) {
	if len(productionUnservedOperations) == 0 {
		t.Fatal("production unserved operations list is empty")
	}
	router := chi.NewRouter()
	registerUnservedOperations(router, nil, productionUnservedOperations)
	parameter := regexp.MustCompile(`\{[^}]+\}`)
	for _, op := range productionUnservedOperations {
		t.Run(op.method+" "+op.path, func(t *testing.T) {
			path := parameter.ReplaceAllString(op.path, "11111111-1111-4111-8111-111111111111")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(op.method, path, nil))
			if response.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want 503; body = %s", response.Code, response.Body.String())
			}
			body := response.Body.Bytes()
			if err := interfaces.ValidateJSON("Error", body); err != nil {
				t.Fatalf("invalid frozen Error: %v; body = %s", err, body)
			}
			var failure rest.APIError
			if err := json.Unmarshal(body, &failure); err != nil {
				t.Fatal(err)
			}
			if failure.Code != "DEPENDENCY_UNAVAILABLE" {
				t.Errorf("code = %q, want DEPENDENCY_UNAVAILABLE", failure.Code)
			}
			if failure.Retryable {
				t.Error("not configured error must not be retryable")
			}
			if failure.RetryAfterMS != nil {
				t.Errorf("retryAfterMs = %d, want null", *failure.RetryAfterMS)
			}
			if values, ok := response.Header()["Retry-After"]; ok {
				t.Errorf("unexpected Retry-After header: %q", values)
			}
		})
	}
}
