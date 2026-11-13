package rest

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/broker"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/go-chi/chi/v5"
)

func TestProductionBatchRequestRejectsInvalidRepresentations(t *testing.T) {
	pool := "0x1111111111111111111111111111111111111111"
	for _, check := range []struct {
		name, query, contentType, body string
		status                         int
	}{
		{"unexpected query", "?x=1", "application/json", `{"poolAddresses":["` + pool + `"]}`, 400},
		{"wrong media type", "", "text/plain", `{"poolAddresses":["` + pool + `"]}`, 422},
		{"invalid JSON", "", "application/json", `{`, 422},
		{"trailing document", "", "application/json", `{"poolAddresses":["` + pool + `"]}{}`, 400},
		{"invalid address", "", "application/json", `{"poolAddresses":["bad"]}`, 422},
		{"single address", "", "application/json", `{"poolAddresses":["` + pool + `"]}`, 200},
		{"duplicate address", "", "application/json", `{"poolAddresses":["` + pool + `","` + pool + `"]}`, 422},
	} {
		t.Run(check.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/v2/pools/stats:batch"+check.query, strings.NewReader(check.body))
			request.Header.Set("Content-Type", check.contentType)
			response := httptest.NewRecorder()
			addresses, ok := statsBatchRequest(response, request)
			if check.status == 200 {
				if !ok || len(addresses) != 1 || addresses[0] != pool {
					t.Fatalf("normalized pools = %v, accepted=%t", addresses, ok)
				}
				return
			}
			if ok || response.Code != check.status || !strings.Contains(response.Body.String(), `"code":`) {
				t.Fatalf("batch request: accepted=%t status=%d body=%s", ok, response.Code, response.Body.String())
			}
		})
	}
}

func TestProductionRankingRequestBoundsFirstPage(t *testing.T) {
	for _, check := range []struct {
		category, query string
		status          int
		limit           int
	}{
		{"trending", "", 200, 50},
		{"volume", "?limit=7", 200, 7},
		{"unknown", "", 422, 0},
		{"volume", "?limit=0", 422, 0},
		{"volume", "?limit=101", 422, 0},
		{"volume", "?limit=bad", 422, 0},
		{"volume", "?unexpected=1", 422, 0},
		{"volume", "?cursor=abc", 400, 0},
		{"volume", "?cursor=", 422, 0},
		{"volume", "?limit=1&limit=2", 422, 0},
	} {
		t.Run(check.category+check.query, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/v2/rankings/"+check.category+check.query, nil)
			route := chi.NewRouteContext()
			route.URLParams.Add("category", check.category)
			request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, route))
			response := httptest.NewRecorder()
			category, accepted := rankingsRequest(response, request)
			if check.status == 200 {
				if !accepted || category != check.category || rankingsLimit(request) != check.limit {
					t.Fatalf("ranking request: category=%q accepted=%t limit=%d", category, accepted, rankingsLimit(request))
				}
			} else if accepted || response.Code != check.status || !strings.Contains(response.Body.String(), `"code":`) {
				t.Fatalf("ranking request: accepted=%t status=%d body=%s", accepted, response.Code, response.Body.String())
			}
		})
	}
}

func TestProductionSnapshotErrorClassifiesPublicFailures(t *testing.T) {
	for _, check := range []struct {
		cause  error
		status int
		code   string
	}{
		{broker.ErrExpired, 409, "RESNAPSHOT_REQUIRED"},
		{broker.ErrCursor, 400, "CURSOR_INVALID"},
		{broker.ErrSequence, 400, "CURSOR_INVALID"},
		{sharedauth.ErrScopeDenied, 403, "SCOPE_DENIED"},
		{broker.ErrFrame, 503, "INTEGRITY_QUARANTINED"},
		{store.ErrDataIntegrity, 503, "INTEGRITY_QUARANTINED"},
		{errors.New("source unavailable"), 503, "DEPENDENCY_UNAVAILABLE"},
	} {
		response := httptest.NewRecorder()
		writeSnapshotError(response, httptest.NewRequest(http.MethodGet, "/v2/snapshots", nil), check.cause)
		if response.Code != check.status || !strings.Contains(response.Body.String(), `"code":"`+check.code+`"`) {
			t.Fatalf("snapshot error %v: status=%d body=%s", check.cause, response.Code, response.Body.String())
		}
	}
}

func TestProductionSnapshotFiltersBindRequestedTopics(t *testing.T) {
	const pool = "0x1111111111111111111111111111111111111111"
	for _, check := range []struct {
		name, query string
		status      int
		code        string
	}{
		{"unknown field", "topics=market.stats." + pool + "&extra=1", 422, "FIELD_UNKNOWN"},
		{"duplicate topics", "topics=market.stats." + pool + "&topics=market.stats." + pool, 422, "FIELD_UNKNOWN"},
		{"malformed filters", "topics=market.stats." + pool + "&normalizedFilters=%7B", 422, "FIELD_INVALID"},
		{"unread only true", "topics=market.stats." + pool + "&normalizedFilters=" + url.QueryEscape(`{"unreadOnly":true}`), 422, "FIELD_INVALID"},
		{"invalid pool", "topics=market.stats." + pool + "&normalizedFilters=" + url.QueryEscape(`{"poolAddress":"bad"}`), 422, "FIELD_INVALID"},
		{"different pool", "topics=market.stats." + pool + "&normalizedFilters=" + url.QueryEscape(`{"poolAddress":"0x2222222222222222222222222222222222222222"}`), 422, "FIELD_INVALID"},
		{"different category", "topics=market.rankings.volume&normalizedFilters=" + url.QueryEscape(`{"category":"trending"}`), 422, "FIELD_INVALID"},
		{"different interval", "topics=market.candles." + pool + ".1m&normalizedFilters=" + url.QueryEscape(`{"interval":"1h"}`), 422, "FIELD_INVALID"},
		{"missing topics", "normalizedFilters=" + url.QueryEscape(`{"unreadOnly":false}`), 422, "FIELD_INVALID"},
	} {
		t.Run(check.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/v2/snapshots?"+check.query, nil)
			response := httptest.NewRecorder()
			streamSnapshot(ProductionReadOptions{})(response, request)
			if response.Code != check.status || !strings.Contains(response.Body.String(), `"code":"`+check.code+`"`) {
				t.Fatalf("snapshot filter %q: %d %s", check.query, response.Code, response.Body.String())
			}
		})
	}
}
