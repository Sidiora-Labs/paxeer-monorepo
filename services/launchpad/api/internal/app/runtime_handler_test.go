package app

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sharedprocess "github.com/Sidiora-Technologies/KindleLaunch/shared/process"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/telemetry"
)

func TestRuntimeHandlerTracksEligibleReads(t *testing.T) {
	tracker := sharedprocess.NewTracker()
	if err := tracker.SetReady(); err != nil {
		t.Fatal(err)
	}
	metrics, err := telemetry.New("apid_rest", "qualification", "zone-a")
	if err != nil {
		t.Fatal(err)
	}
	routes := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/data" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.URL.Path == "/v2/invalid" {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	server := httptest.NewServer(runtimeHandler(tracker, routes, metrics))
	defer server.Close()
	for _, path := range []string{"/v2/data", "/v2/invalid", "/health"} {
		response, err := server.Client().Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		if err := response.Body.Close(); err != nil {
			t.Fatal(err)
		}
		want := http.StatusOK
		if path == "/v2/data" {
			want = http.StatusNoContent
		}
		if path == "/v2/invalid" {
			want = http.StatusUnprocessableEntity
		}
		if response.StatusCode != want {
			t.Fatalf("%s status %d, want %d", path, response.StatusCode, want)
		}
	}
	var body bytes.Buffer
	if err := metrics.WritePrometheus(&body); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body.String(), `sli_read_eligible_requests_total{service="apid_rest",environment="qualification",zone="zone-a",route="other"} 1`) {
		t.Fatalf("eligible read count: %s", body.String())
	}
}
