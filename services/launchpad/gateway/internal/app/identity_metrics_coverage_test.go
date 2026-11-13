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

func TestIdentityRuntimeMetricsExposeKnownOutcomes(t *testing.T) {
	metrics := newIdentityRuntimeMetrics()
	metrics.observe("issued")
	metrics.observe("issued")
	metrics.observe("unknown-outcome")
	w := httptest.NewRecorder()
	metrics.write(w)
	if !strings.Contains(w.Body.String(), "identity_outcomes_total{outcome=\"issued\"} 2") || strings.Contains(w.Body.String(), "unknown-outcome") {
		t.Fatalf("identity outcome metrics: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "identity_outcomes_total{outcome=\"verified\"} 0") {
		t.Fatalf("registered zero outcome absent: %s", w.Body.String())
	}
}

func TestGatewayRuntimeHandlerRecordsEligibleReads(t *testing.T) {
	metrics, err := telemetry.New("mediagatewayd", "qualification", "zone-a")
	if err != nil {
		t.Fatal(err)
	}
	tracker := process.NewTracker()
	if err := tracker.SetReady(); err != nil {
		t.Fatal(err)
	}
	service := runtimeHandler(tracker, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/invalid":
			w.WriteHeader(http.StatusBadRequest)
		case "/unprocessable":
			w.WriteHeader(http.StatusUnprocessableEntity)
		case "/dependency":
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}), metrics)
	for _, tc := range []struct {
		method, path string
		status       int
		cancel       bool
	}{
		{http.MethodGet, "/health/ready", http.StatusOK, false},
		{http.MethodGet, "/metrics", http.StatusOK, false},
		{http.MethodPost, "/resource", http.StatusOK, false},
		{http.MethodGet, "/resource", http.StatusOK, false},
		{http.MethodGet, "/dependency", http.StatusServiceUnavailable, false},
		{http.MethodGet, "/invalid", http.StatusBadRequest, false},
		{http.MethodGet, "/unprocessable", http.StatusUnprocessableEntity, false},
		{http.MethodGet, "/resource", http.StatusOK, true},
	} {
		request := httptest.NewRequest(tc.method, tc.path, nil)
		if tc.cancel {
			ctx, cancel := context.WithCancel(request.Context())
			cancel()
			request = request.WithContext(ctx)
		}
		response := httptest.NewRecorder()
		service.ServeHTTP(response, request)
		if response.Code != tc.status {
			t.Fatalf("%s %s status=%d, want %d", tc.method, tc.path, response.Code, tc.status)
		}
	}
	var report bytes.Buffer
	if err := metrics.WritePrometheus(&report); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`sli_read_eligible_requests_total{service="mediagatewayd",environment="qualification",zone="zone-a",route="other"} 2`,
		`sli_read_good_requests_total{service="mediagatewayd",environment="qualification",zone="zone-a",route="other"} 1`,
	} {
		if !strings.Contains(report.String(), want) {
			t.Fatalf("runtime read metrics omit %q: %s", want, report.String())
		}
	}
}
