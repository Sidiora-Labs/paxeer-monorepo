package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/process"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/telemetry"
)

func TestRuntimeHandlerCountsEligibleReadsAndExcludesHealth(t *testing.T) {
	metrics, err := telemetry.New("userd", "qualification", "zone-a")
	if err != nil {
		t.Fatal(err)
	}
	tracker := process.NewTracker()
	if err := tracker.SetReady(); err != nil {
		t.Fatal(err)
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/invalid" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})
	handler := runtimeHandler(tracker, next, metrics)
	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{http.MethodGet, "/health", http.StatusOK},
		{http.MethodGet, "/metrics", http.StatusOK},
		{http.MethodPost, "/users", http.StatusOK},
		{http.MethodGet, "/users", http.StatusOK},
		{http.MethodGet, "/invalid", http.StatusBadRequest},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(tc.method, tc.path, nil))
		if response.Code != tc.want {
			t.Fatalf("%s %s = %d, want %d", tc.method, tc.path, response.Code, tc.want)
		}
	}
	output := httptest.NewRecorder()
	metrics.ServeHTTP(output, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if output.Code != http.StatusOK || !strings.Contains(output.Body.String(), `sli_read_eligible_requests_total{service="userd",environment="qualification",zone="zone-a",route="other"} 1`) {
		t.Fatalf("eligible read metric = %d %s", output.Code, output.Body.String())
	}
	tracker.BeginDrain()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/users", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("request during drain = %d", response.Code)
	}
}
