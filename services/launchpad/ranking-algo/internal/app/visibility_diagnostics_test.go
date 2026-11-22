package app

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/ranker"
)

func TestVisibilityFailuresNameTheirCause(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		body    string
		message string
	}{
		{"unavailable", 503, "unavailable", "ranking visibility response unavailable"},
		{"malformed", 200, "not a visibility document", "ranking visibility document invalid or incomplete"},
		{"stale", 200, `{"revision":"` + strings.Repeat("a", 64) + `","observedAt":"2000-01-01T00:00:00Z","complete":true,"pools":{}}`, "ranking visibility observation outside freshness window"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			previous := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
			defer slog.SetDefault(previous)
			peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer peer.Close()
			reader := &rankingVisibilityClient{endpoint: peer.URL, client: peer.Client(), now: time.Now}
			if _, err := reader.ReadRankingVisibility(context.Background()); err != ranker.ErrUnavailable {
				t.Fatalf("visibility error = %v, want ErrUnavailable", err)
			}
			if !strings.Contains(logs.String(), tc.message) {
				t.Fatalf("visibility diagnostic: %s", logs.String())
			}
			if tc.status != http.StatusOK && !strings.Contains(logs.String(), `"status":503`) {
				t.Fatalf("visibility diagnostic has no HTTP status: %s", logs.String())
			}
		})
	}
}

func TestRankingPruneFailureNamesDatabaseCause(t *testing.T) {
	fixture := publicationRecoveryNewFixture(t)
	fixture.app.pool.Close()
	var logs bytes.Buffer
	fixture.app.native.logger = slog.New(slog.NewJSONHandler(&logs, nil))
	fixture.app.native.prune()
	if !strings.Contains(logs.String(), `"msg":"ranking prune failed"`) || !strings.Contains(logs.String(), `"code":"RANKING_PRUNE_UNAVAILABLE"`) || !strings.Contains(logs.String(), `"error":"closed pool"`) {
		t.Fatalf("prune diagnostic = %s", logs.String())
	}
}
