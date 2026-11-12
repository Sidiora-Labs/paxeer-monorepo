package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sharedprocess "github.com/Sidiora-Technologies/KindleLaunch/shared/process"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/telemetry"
)

func TestRuntimeHandlerRecordsEligibleReadsAndExcludesInvalidRequests(t *testing.T) {
	metrics, err := telemetry.New("chartsd", "qualification", "zone-a")
	if err != nil {
		t.Fatal(err)
	}
	tracker := sharedprocess.NewTracker()
	if err := tracker.SetReady(); err != nil {
		t.Fatal(err)
	}
	underlying := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/empty" {
			return
		}
		if r.URL.Path == "/bad" {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	handler := runtimeHandler(tracker, underlying, metrics)
	for _, request := range []struct {
		method, path string
		status       int
	}{
		{http.MethodGet, "/v2/udf/config", 200},
		{http.MethodGet, "/bad", 422},
		{http.MethodPost, "/v2/udf/config", 200},
		{http.MethodGet, "/health/ready", 200},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(request.method, request.path, nil))
		if response.Code != request.status {
			t.Fatalf("%s %s: %d", request.method, request.path, response.Code)
		}
	}
	response := httptest.NewRecorder()
	metrics.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `sli_read_eligible_requests_total{service="chartsd",environment="qualification",zone="zone-a",route="other"} 1`) {
		t.Fatalf("eligible read metric: %d %s", response.Code, response.Body.String())
	}
	empty := httptest.NewRecorder()
	handler.ServeHTTP(empty, httptest.NewRequest(http.MethodGet, "/empty", nil))
	if empty.Code != http.StatusOK || empty.Body.Len() != 0 {
		t.Fatalf("implicit successful read: %d %s", empty.Code, empty.Body.String())
	}
	cancelled, stop := context.WithCancel(context.Background())
	stop()
	aborted := httptest.NewRecorder()
	handler.ServeHTTP(aborted, httptest.NewRequest(http.MethodGet, "/v2/udf/config", nil).WithContext(cancelled))
	if aborted.Code != http.StatusOK {
		t.Fatalf("cancelled read response: %d %s", aborted.Code, aborted.Body.String())
	}
}
