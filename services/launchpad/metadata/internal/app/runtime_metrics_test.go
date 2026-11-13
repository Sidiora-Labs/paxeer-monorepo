package app

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/process"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/telemetry"
)

func TestRuntimeReadMetricsReflectResponses(t *testing.T) {
	tracker := process.NewTracker()
	if err := tracker.SetReady(); err != nil {
		t.Fatal(err)
	}
	metrics, err := telemetry.New("metadatad", "qualification", "zone-a")
	if err != nil {
		t.Fatal(err)
	}
	served := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served++
		switch r.URL.Path {
		case "/fail":
			w.WriteHeader(http.StatusServiceUnavailable)
		case "/invalid":
			w.WriteHeader(http.StatusUnprocessableEntity)
		case "/health/ready":
			w.WriteHeader(http.StatusOK)
		}
	})
	handler := runtimeHandler(tracker, next, metrics)
	for _, tc := range []struct {
		method, path string
		cancel       bool
		want         int
	}{
		{http.MethodGet, "/quiet", false, http.StatusOK},
		{http.MethodGet, "/fail", false, http.StatusServiceUnavailable},
		{http.MethodGet, "/invalid", false, http.StatusUnprocessableEntity},
		{http.MethodGet, "/quiet", true, http.StatusOK},
		{http.MethodPost, "/quiet", false, http.StatusOK},
		{http.MethodGet, "/health/ready", false, http.StatusOK},
		{http.MethodGet, "/metrics", false, http.StatusOK},
	} {
		request := httptest.NewRequest(tc.method, tc.path, nil)
		if tc.cancel {
			ctx, cancel := context.WithCancel(request.Context())
			cancel()
			request = request.WithContext(ctx)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != tc.want {
			t.Fatalf("%s %s response = %d, want %d", tc.method, tc.path, response.Code, tc.want)
		}
	}
	if served != 7 {
		t.Fatalf("served requests = %d", served)
	}
	var output bytes.Buffer
	if err := metrics.WritePrometheus(&output); err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{
		`sli_read_eligible_requests_total{service="metadatad",environment="qualification",zone="zone-a",route="other"} 2`,
		`sli_read_good_requests_total{service="metadatad",environment="qualification",zone="zone-a",route="other"} 1`,
	} {
		if !strings.Contains(output.String(), line) {
			t.Fatalf("read metric %q absent from %s", line, output.String())
		}
	}
}
