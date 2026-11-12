package rest

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestSnapshotRequestRejectsInvalidFiltersBeforeReadingProjection(t *testing.T) {
	pool := "0x" + strings.Repeat("1", 40)
	for _, check := range []struct {
		name, query, code string
		status            int
	}{
		{"unknown parameter", "topics=market.stats." + pool + "&unexpected=1", "FIELD_UNKNOWN", http.StatusUnprocessableEntity},
		{"duplicate parameter", "topics=market.stats." + pool + "&topics=market.stats." + pool, "FIELD_UNKNOWN", http.StatusUnprocessableEntity},
		{"malformed filters", "topics=market.stats." + pool + "&normalizedFilters=" + url.QueryEscape("{"), "FIELD_INVALID", http.StatusUnprocessableEntity},
		{"invalid pool filter", "topics=market.stats." + pool + "&normalizedFilters=" + url.QueryEscape(`{"poolAddress":"invalid"}`), "FIELD_INVALID", http.StatusUnprocessableEntity},
		{"unmatched pool filter", "topics=market.stats." + pool + "&normalizedFilters=" + url.QueryEscape(`{"poolAddress":"0x2222222222222222222222222222222222222222"}`), "FIELD_INVALID", http.StatusUnprocessableEntity},
		{"invalid topic", "topics=market.unknown." + pool, "SCOPE_DENIED", http.StatusForbidden},
	} {
		t.Run(check.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/v2/snapshots?"+check.query, nil)
			response := httptest.NewRecorder()
			streamSnapshot(ProductionReadOptions{})(response, request)
			if response.Code != check.status || !strings.Contains(response.Body.String(), check.code) {
				t.Fatalf("status %d body %s, want %d and %s", response.Code, response.Body.String(), check.status, check.code)
			}
		})
	}
}
