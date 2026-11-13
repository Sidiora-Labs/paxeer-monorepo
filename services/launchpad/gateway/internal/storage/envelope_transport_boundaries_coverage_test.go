package storage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEnvelopeRequestAndResourceTransportBoundaries(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			_, _ = w.Write([]byte("process_resident_memory_bytes 1024\ngo_goroutines 3\n"))
			return
		}
		_, _ = w.Write([]byte(strings.Repeat("x", (1<<20)+1)))
	}))
	defer server.Close()
	runner := envelopeTestRunner(t, server, EnvelopeConfig{ResourceURL: server.URL + "/metrics", MaxResidentBytes: 2048, MaxGoroutines: 4})
	for _, tc := range []struct {
		name, url, method, want string
		headers                 map[string]string
	}{
		{"invalid target", "://invalid", http.MethodPost, "missing protocol scheme", nil},
		{"unbound header", server.URL, http.MethodGet, "target_identity_unavailable", map[string]string{"X-Role": "ENVELOPE_ABSENT_ROLE"}},
		{"bounded metadata body", server.URL, http.MethodGet, "target_body_failed", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := EnvelopeTarget{ID: "metadata", Class: metadataServiceName, URL: tc.url, ExpectedStatus: http.StatusOK, HeaderEnvironment: tc.headers}
			_, _, sample, err := runner.request(context.Background(), target, tc.method)
			if err == nil || !strings.Contains(err.Error(), tc.want) || sample.Outcome != failedState || sample.TargetID != "metadata" {
				t.Fatalf("bounded request: sample=%+v error=%v, want %s", sample, err, tc.want)
			}
		})
	}
	t.Setenv("ENVELOPE_TEST_RESOURCE_ROLE", "resource-reader")
	runner.config.ResourceHeaders = map[string]string{"X-Role": "ENVELOPE_TEST_RESOURCE_ROLE"}
	if err := runner.sampleResources(context.Background()); err != nil || runner.result.MaximumResidentBytes != 1024 || runner.result.MaximumGoroutines != 3 {
		t.Fatalf("bounded resource sample: memory=%d goroutines=%d err=%v", runner.result.MaximumResidentBytes, runner.result.MaximumGoroutines, err)
	}
	server.Close()
	runner.config.ResourceURL = server.URL + "/metrics"
	if err := runner.sampleResources(context.Background()); err == nil || !strings.Contains(err.Error(), "resource_endpoint_unavailable") {
		t.Fatalf("unavailable resource endpoint: %v", err)
	}
	t.Setenv("ENVELOPE_TEST_INVALID_DSN", "://invalid")
	if err := runner.runSQL(context.Background(), EnvelopeSQLCheck{DSNEnvironment: "ENVELOPE_TEST_INVALID_DSN", Query: "SELECT 1"}, 1); err == nil || !strings.Contains(err.Error(), "sql_observation_unavailable") {
		t.Fatalf("invalid resource database authority: %v", err)
	}
}
