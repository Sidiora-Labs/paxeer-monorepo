package rest_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/rest"
	"github.com/go-chi/chi/v5"
)

func TestProductionSnapshotFiltersBindToRequestedTopics(t *testing.T) {
	admin := os.Getenv(internaltest.ExternalPostgresEnv)
	if admin == "" {
		t.Fatal("PostgreSQL fixture is required")
	}
	t.Setenv("API_TEST_DATABASE_URL", admin)
	_, model, stream, pool, _ := productionReadsFixture(t)
	admission, err := rest.NewAdmission(100, 100, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	if err := rest.RegisterProductionReads(router, rest.ProductionReadOptions{Model: model, Stream: stream, Admission: admission, RegisterSnapshots: true}); err != nil {
		t.Fatal(err)
	}
	topic := "market.stats." + pool
	for _, tc := range []struct {
		name, query, code string
		status            int
	}{
		{"matching pool", url.Values{"topics": {topic}, "normalizedFilters": {`{"poolAddress":"` + pool + `"}`}}.Encode(), "", 200},
		{"unread filter disabled", url.Values{"topics": {topic}, "normalizedFilters": {`{"unreadOnly":false}`}}.Encode(), "", 200},
		{"unknown parameter", url.Values{"topics": {topic}, "unexpected": {"1"}}.Encode(), "FIELD_UNKNOWN", 422},
		{"duplicate topics", "topics=" + url.QueryEscape(topic) + "&topics=" + url.QueryEscape(topic), "FIELD_UNKNOWN", 422},
		{"invalid filter JSON", url.Values{"topics": {topic}, "normalizedFilters": {`{`}}.Encode(), "FIELD_INVALID", 422},
		{"unread restriction", url.Values{"topics": {topic}, "normalizedFilters": {`{"unreadOnly":true}`}}.Encode(), "FIELD_INVALID", 422},
		{"invalid pool", url.Values{"topics": {topic}, "normalizedFilters": {`{"poolAddress":"invalid"}`}}.Encode(), "FIELD_INVALID", 422},
		{"different pool", url.Values{"topics": {topic}, "normalizedFilters": {`{"poolAddress":"0x` + strings.Repeat("2", 40) + `"}`}}.Encode(), "FIELD_INVALID", 422},
		{"ranking filter on stats", url.Values{"topics": {topic}, "normalizedFilters": {`{"category":"trending"}`}}.Encode(), "FIELD_INVALID", 422},
		{"interval filter on stats", url.Values{"topics": {topic}, "normalizedFilters": {`{"interval":"1m"}`}}.Encode(), "FIELD_INVALID", 422},
		{"ranking category mismatch", url.Values{"topics": {"market.rankings.trending"}, "normalizedFilters": {`{"category":"volume"}`}}.Encode(), "FIELD_INVALID", 422},
		{"candle interval mismatch", url.Values{"topics": {"market.candles." + pool + ".1m"}, "normalizedFilters": {`{"interval":"5m"}`}}.Encode(), "FIELD_INVALID", 422},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := waitForV2Ready(t, func() v2Response {
				response := httptest.NewRecorder()
				router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v2/snapshots?"+tc.query, nil))
				return v2Response{status: response.Code, body: response.Body.Bytes(), header: response.Header()}
			})
			if got.status != tc.status || tc.code != "" && !strings.Contains(string(got.body), tc.code) {
				t.Fatalf("snapshot status=%d body=%s, want %d %s", got.status, got.body, tc.status, tc.code)
			}
			if tc.status == 200 && !strings.Contains(string(got.body), `"snapshot"`) {
				t.Fatalf("snapshot omitted: %s", got.body)
			}
		})
	}
	first := waitForV2Ready(t, func() v2Response {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v2/snapshots?topics="+url.QueryEscape(topic), nil))
		return v2Response{status: response.Code, body: response.Body.Bytes(), header: response.Header()}
	})
	var document struct {
		Data struct {
			Snapshot struct {
				ResumeCursor string `json:"resumeCursor"`
			} `json:"snapshot"`
		} `json:"data"`
	}
	if err := json.Unmarshal(first.body, &document); err != nil || document.Data.Snapshot.ResumeCursor == "" {
		t.Fatalf("public snapshot cursor: %s, %v", first.body, err)
	}
	resumed := waitForV2Ready(t, func() v2Response {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v2/snapshots?"+url.Values{"topics": {topic}, "cursor": {document.Data.Snapshot.ResumeCursor}}.Encode(), nil))
		return v2Response{status: response.Code, body: response.Body.Bytes(), header: response.Header()}
	})
	if resumed.status != http.StatusOK || !strings.Contains(string(resumed.body), `"snapshot"`) {
		t.Fatalf("resumed snapshot: %d %s", resumed.status, resumed.body)
	}
}
