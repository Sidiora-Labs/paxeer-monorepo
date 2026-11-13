package rest_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/rest"
	"github.com/go-chi/chi/v5"
)

func TestProductionSnapshotRejectsUnboundPrivateAndExpiredPublicReads(t *testing.T) {
	_, model, stream, address, _ := productionReadsFixture(t)
	admission, err := rest.NewAdmission(100, 100, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	const origin = "https://web.example"
	if err := rest.RegisterProductionReads(router, rest.ProductionReadOptions{Model: model, Stream: stream, Admission: admission, RegisterSnapshots: true, AllowedOrigins: map[string]bool{origin: true}}); err != nil {
		t.Fatal(err)
	}
	private := "private.notifications.0x1111111111111111111111111111111111111111"
	for _, check := range []struct {
		name, topic, cursor, origin, token, code, cache string
		status                                          int
	}{
		{"private origin", private, "", "https://other.example", "Bearer invalid", "SCOPE_DENIED", "private, no-store", http.StatusForbidden},
		{"private identity", private, "", origin, "Bearer invalid", "AUTH_EXPIRED", "private, no-store", http.StatusUnauthorized},
		{"public cursor", "market.stats." + address, "invalid", "", "", "CURSOR_INVALID", "no-store", http.StatusBadRequest},
	} {
		t.Run(check.name, func(t *testing.T) {
			query := url.Values{"topics": {check.topic}}
			if check.cursor != "" {
				query.Set("cursor", check.cursor)
			}
			request := httptest.NewRequest(http.MethodGet, "/v2/snapshots?"+query.Encode(), nil)
			if check.origin != "" {
				request.Header.Set("Origin", check.origin)
			}
			if check.token != "" {
				request.Header.Set("Authorization", check.token)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			var document rest.APIError
			if err := json.Unmarshal(response.Body.Bytes(), &document); err != nil || response.Code != check.status || document.Code != check.code || response.Header().Get("Cache-Control") != check.cache || strings.Contains(response.Body.String(), check.token) && check.token != "" {
				t.Fatalf("snapshot status=%d body=%s headers=%v error=%v", response.Code, response.Body.String(), response.Header(), err)
			}
		})
	}
}
