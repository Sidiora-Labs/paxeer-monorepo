package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/internaltest"
)

func TestDevelopmentAppServesRankingsWithFixtureStores(t *testing.T) {
	dsn, _ := internaltest.NewPostgresWithDSN(t)
	redisURL := internaltest.NewRedisURL(t)
	cfg := config.Config{}
	cfg.NodeEnv = "test"
	cfg.DatabaseURL = dsn
	cfg.RedisURL = redisURL
	cfg.RankingMaxEntries = 200
	instance, err := New(context.Background(), cfg, quietLogger())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(instance.Close)
	if err := instance.Service.RunAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := instance.Service.RunNew(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path string
		code int
	}{
		{"/health/live", http.StatusOK},
		{"/health/ready", http.StatusOK},
		{"/rankings/not-a-category", http.StatusBadRequest},
	} {
		response := httptest.NewRecorder()
		instance.Router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if response.Code != tc.code {
			t.Fatalf("%s returned %d, want %d: %s", tc.path, response.Code, tc.code, response.Body.String())
		}
	}
}

func TestScheduleLoopStopsAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app := &App{}
	runs := 0
	app.scheduleLoop(ctx, 10*time.Millisecond, func(context.Context) error {
		runs++
		cancel()
		return errors.New("cycle rejected")
	}, "ranking")
	if runs != 1 {
		t.Fatalf("scheduler ran %d cycles after cancellation", runs)
	}
}

func TestRankingBoundDocumentUsesExactFileBytes(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "ranking-document.json")
	raw := []byte(`{"version":2}`)
	digest := writeBound(t, path, raw)
	got, err := rankingBoundBytes(path, digest)
	if err != nil || string(got) != string(raw) {
		t.Fatalf("bound document = %q, %v", got, err)
	}
	if _, err := rankingBoundBytes(path, strings.Repeat("0", 64)); err == nil {
		t.Fatal("changed document digest accepted")
	}
	link := filepath.Join(root, "linked-document.json")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := rankingBoundBytes(link, digest); err == nil {
		t.Fatal("linked document accepted")
	}
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := rankingBoundBytes(path, digest); err == nil {
		t.Fatal("empty document accepted")
	}
}
