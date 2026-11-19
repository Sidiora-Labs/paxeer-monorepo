package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/ranker"
	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func TestRankingReadFailuresNameRedisCause(t *testing.T) {
	options, err := redis.ParseURL(internaltest.NewRedisURL(t))
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(options)
	if err = client.Close(); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	defer slog.SetDefault(previous)
	router := chi.NewRouter()
	RegisterRankings(router, client)
	for _, path := range []string{"/rankings/trending", "/rankings/pool/" + strings.Repeat("a", 42)} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusInternalServerError {
			t.Fatalf("ranking response = %d, want 500", response.Code)
		}
		if strings.Contains(response.Body.String(), "client is closed") {
			t.Fatal("ranking response exposes dependency diagnostics")
		}
	}
	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("ranking diagnostics = %s", logs.String())
	}
	for i, message := range []string{"ranking query failed", "ranking rank lookup failed"} {
		var record map[string]any
		if err = json.Unmarshal([]byte(lines[i]), &record); err != nil {
			t.Fatal(err)
		}
		if record["msg"] != message || record["error"] != "redis: client is closed" || record["category"] != "trending" {
			t.Fatalf("ranking diagnostic = %s", lines[i])
		}
	}
}

func TestSnapshotReadFailuresNameDatabaseCause(t *testing.T) {
	database := internaltest.NewRankingRuntimeDatabase(t)
	pool, err := pgxpool.New(context.Background(), database.RuntimeDSN)
	if err != nil {
		t.Fatal(err)
	}
	if err = pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	repository, err := store.NewSnapshotStore(pool, "ranking")
	if err != nil {
		pool.Close()
		t.Fatal(err)
	}
	pool.Close()
	peer := httptest.NewServer(http.NotFoundHandler())
	defer peer.Close()
	visibility, err := store.NewHTTPRankingVisibility(peer.URL, peer.Client())
	if err != nil {
		t.Fatal(err)
	}
	cursor, err := ranker.NewCursorCodec([]byte(strings.Repeat("c", 32)))
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewSnapshots(repository, nil, cursor, visibility, 1)
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	RegisterSnapshots(router, handler)
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	defer slog.SetDefault(previous)
	for _, path := range []string{"/rankings/trending", "/rankings/runs/be0ebdd7-e6e2-4f9a-b856-68e7fb31bf22/trending"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "RANKING_UNAVAILABLE") {
			t.Fatalf("ranking response = %d %s", response.Code, response.Body.String())
		}
	}
	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("ranking diagnostics = %s", logs.String())
	}
	for i, message := range []string{"ranking header read failed", "ranking inspection read failed"} {
		var record map[string]any
		if err = json.Unmarshal([]byte(lines[i]), &record); err != nil {
			t.Fatal(err)
		}
		if record["msg"] != message || record["error"] != "closed pool" {
			t.Fatalf("ranking diagnostic = %s", lines[i])
		}
	}
}
