package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/store"
	"github.com/go-chi/chi/v5"
	goredis "github.com/redis/go-redis/v9"
)

func TestSnapshotRoutesRejectInvalidIdentifiers(t *testing.T) {
	router := chi.NewRouter()
	RegisterSnapshots(router, &Snapshots{admission: make(chan struct{}, 1)})
	for _, tc := range []struct {
		path string
		code int
	}{
		{"/rankings/unknown", http.StatusBadRequest},
		{"/rankings/trending?limit=101", http.StatusBadRequest},
		{"/rankings/runs/not-a-run/trending", http.StatusBadRequest},
		{"/rankings/runs/not-a-run/unknown", http.StatusBadRequest},
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if response.Code != tc.code {
			t.Fatalf("%s returned %d, want %d", tc.path, response.Code, tc.code)
		}
	}
}

func TestRankingPaginationDefaultsAndBounds(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		def  int
		want int
	}{{"", 25, 25}, {"7", 25, 7}, {"0", 25, 0}, {"bad", 25, 25}} {
		if got := parseIntDefault(tc.raw, tc.def); got != tc.want {
			t.Fatalf("parseIntDefault(%q, %d) = %d, want %d", tc.raw, tc.def, got, tc.want)
		}
	}
}

func TestSnapshotInspectionUsesRetainedRevision(t *testing.T) {
	repository, visibility := repair33MemoryStore(t)
	server, _ := repair33Server(t, repository, visibility)
	runID := repository.headers["trending"].RunID
	response, err := server.Client().Get(server.URL + "/rankings/runs/" + runID + "/trending")
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		RunID    string `json:"runId"`
		Category string `json:"category"`
	}
	decodeErr := json.NewDecoder(response.Body).Decode(&body)
	closeErr := response.Body.Close()
	if decodeErr != nil || closeErr != nil || response.StatusCode != http.StatusOK || body.RunID != runID || body.Category != "trending" {
		t.Fatalf("retained snapshot inspection = %+v, status %d, decode %v, close %v", body, response.StatusCode, decodeErr, closeErr)
	}
	server, _ = repair33Server(t, repository, repair33Visibility{observed: time.Now().Add(-2 * time.Minute)})
	response, err = server.Client().Get(server.URL + "/rankings/runs/" + runID + "/trending")
	if err != nil {
		t.Fatal(err)
	}
	_, readErr := io.Copy(io.Discard, response.Body)
	closeErr = response.Body.Close()
	if readErr != nil || closeErr != nil || response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("stale inspection status %d, read %v, close %v", response.StatusCode, readErr, closeErr)
	}
}

func TestSnapshotReadUsesCacheAndValidatesContinuation(t *testing.T) {
	repository, visibility := repair33MemoryStore(t)
	server, handler := repair33Server(t, repository, visibility)
	rdb := internaltest.NewRedis(t)
	cache, err := store.NewSnapshotCache(rdb, "ranking-http")
	if err != nil {
		t.Fatal(err)
	}
	handler.cache = cache
	endpoint := server.URL + "/rankings/trending?limit=1"
	response, err := server.Client().Get(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	var first struct {
		NextCursor *string `json:"nextCursor"`
		Quality    string  `json:"quality"`
	}
	decodeErr := json.NewDecoder(response.Body).Decode(&first)
	closeErr := response.Body.Close()
	etag := response.Header.Get("ETag")
	if decodeErr != nil || closeErr != nil || response.StatusCode != http.StatusOK || first.NextCursor == nil || etag == "" {
		t.Fatalf("first ranking page: status %d, body %+v, decode %v, close %v", response.StatusCode, first, decodeErr, closeErr)
	}
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("If-None-Match", etag)
	response, err = server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_, readErr := io.Copy(io.Discard, response.Body)
	closeErr = response.Body.Close()
	if readErr != nil || closeErr != nil || response.StatusCode != http.StatusNotModified {
		t.Fatalf("conditional ranking response: status %d, read %v, close %v", response.StatusCode, readErr, closeErr)
	}
	repository.items[0].Score = "0.5"
	continuation := endpoint + "&cursor=" + url.QueryEscape(*first.NextCursor)
	response, err = server.Client().Get(continuation)
	if err != nil {
		t.Fatal(err)
	}
	_, readErr = io.Copy(io.Discard, response.Body)
	closeErr = response.Body.Close()
	if readErr != nil || closeErr != nil || response.StatusCode != http.StatusBadRequest {
		t.Fatalf("changed continuation: status %d, read %v, close %v", response.StatusCode, readErr, closeErr)
	}
	delete(repository.headers, "trending")
	response, err = server.Client().Get(continuation)
	if err != nil {
		t.Fatal(err)
	}
	_, readErr = io.Copy(io.Discard, response.Body)
	closeErr = response.Body.Close()
	if readErr != nil || closeErr != nil || response.StatusCode != http.StatusGone {
		t.Fatalf("expired continuation: status %d, read %v, close %v", response.StatusCode, readErr, closeErr)
	}
}

func TestRankingRoutesReportUnavailableRedis(t *testing.T) {
	redisURL := internaltest.NewRedisURL(t)
	options, err := goredis.ParseURL(redisURL)
	if err != nil {
		t.Fatal(err)
	}
	rdb := goredis.NewClient(options)
	if err := rdb.Close(); err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	RegisterRankings(router, rdb)
	address := fmt.Sprintf("0x%040x", 1)
	for _, path := range []string{"/rankings/trending", "/rankings/pool/" + address} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusInternalServerError {
			t.Fatalf("%s returned %d after Redis closed", path, response.Code)
		}
	}
	items := []map[string]any{{"poolAddress": address}}
	if err := enrichWithStats(context.Background(), rdb, items); err == nil {
		t.Fatal("stats enrichment accepted a closed Redis connection")
	}
}
