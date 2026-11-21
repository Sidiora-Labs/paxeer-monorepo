package rest

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/broker"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func TestTopicNotStartedSnapshotErrorEnvelope(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelError})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	for _, signedIn := range []bool{false, true} {
		request := httptest.NewRequest(http.MethodGet, "/v2/snapshots", nil)
		if signedIn {
			request.Header.Set("Authorization", "Bearer credential")
		}
		response := httptest.NewRecorder()
		response.Header().Set("Retry-After", "1")
		writeSnapshotError(response, request, fmt.Errorf("snapshot: %w", broker.ErrTopicNotStarted))
		if err := interfaces.ValidateJSON("Error", response.Body.Bytes()); err != nil {
			t.Fatal(err)
		}
		failure, err := interfaces.DecodeError(response.Body.Bytes())
		if err != nil || response.Code != 404 || failure.Code != "TOPIC_NOT_STARTED" || !failure.Retryable || failure.RetryAfterMs == nil || *failure.RetryAfterMs != 5000 || response.Header().Get("Retry-After") != "5" {
			t.Fatalf("snapshot error: %d %s %v %v", response.Code, response.Body, response.Header(), err)
		}
		cache := "no-store"
		if signedIn {
			cache = "private, no-store"
		}
		if response.Header().Get("Cache-Control") != cache {
			t.Fatalf("snapshot cache control: %v", response.Header())
		}
	}
	if logs.Len() != 0 {
		t.Fatalf("snapshot errors logged: %s", logs.String())
	}
}
