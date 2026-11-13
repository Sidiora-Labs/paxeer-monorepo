package storage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvelopeResourceMetricsRejectInvalidValues(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
		status           int
		missingHeader    bool
	}{
		{"response status", "", "resource_endpoint_denied", http.StatusServiceUnavailable, false},
		{"missing metric", "process_resident_memory_bytes 1024\n", "actual_process_resource_metrics_missing", http.StatusOK, false},
		{"duplicate metric", "process_resident_memory_bytes 1024\nprocess_resident_memory_bytes 2048\ngo_goroutines 8\n", "resource_metric_ambiguous", http.StatusOK, false},
		{"negative metric", "process_resident_memory_bytes -1\ngo_goroutines 8\n", "resource_metric_invalid", http.StatusOK, false},
		{"fractional metric", "process_resident_memory_bytes 1024.5\ngo_goroutines 8\n", "resource_metric_invalid", http.StatusOK, false},
		{"identity header", "", "resource_identity_unavailable", http.StatusOK, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/metrics" {
					t.Errorf("resource path=%s", r.URL.Path)
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(server.Close)
			config := EnvelopeConfig{ResourceURL: server.URL + "/metrics", MaxResidentBytes: 1 << 20, MaxGoroutines: 16}
			if tc.missingHeader {
				config.ResourceHeaders = map[string]string{"Authorization": "ENVELOPE_ABSENT_RESOURCE_IDENTITY"}
			}
			runner := envelopeTestRunner(t, server, config)
			if err := runner.sampleResources(context.Background()); err == nil || !strings.Contains(err.Error(), tc.want) || runner.result.MaximumResidentBytes != 0 {
				t.Fatalf("resource sample: error=%v memory=%d, want %q", err, runner.result.MaximumResidentBytes, tc.want)
			}
		})
	}
}

func TestEnvelopeResourceMetricsEnforceObservedBudget(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("process_resident_memory_bytes 2048\ngo_goroutines 8\n"))
	}))
	t.Cleanup(server.Close)
	runner := envelopeTestRunner(t, server, EnvelopeConfig{ResourceURL: server.URL, MaxResidentBytes: 1024, MaxGoroutines: 16})
	if err := runner.sampleResources(context.Background()); err == nil || err.Error() != "process_resource_limit_exceeded" || runner.result.MaximumResidentBytes != 2048 {
		t.Fatalf("resource budget: error=%v resident=%d", err, runner.result.MaximumResidentBytes)
	}
}

func TestEnvelopeRequestRecordsBoundedResponses(t *testing.T) {
	for _, tc := range []struct {
		name, body, outcome, wantError string
		status                         int
		change                         func(*EnvelopeTarget)
		closeServer                    bool
	}{
		{"accepted", "approved", acceptedState, "", http.StatusOK, nil, false},
		{"expected denial", "denied", "expected_rejection", "", http.StatusNotFound, nil, false},
		{"status mismatch", "other", failedState, "target_response_mismatch", http.StatusNotFound, nil, false},
		{"body cap", strings.Repeat("x", (1<<20)+1), failedState, "target_body_failed", http.StatusOK, nil, false},
		{"identity header", "", failedState, "target_identity_unavailable", http.StatusOK, func(target *EnvelopeTarget) {
			target.HeaderEnvironment = map[string]string{"Authorization": "ENVELOPE_ABSENT_TARGET_IDENTITY"}
		}, false},
		{"url", "", failedState, "missing protocol scheme", http.StatusOK, func(target *EnvelopeTarget) { target.URL = ":" }, false},
		{"unlisted fixture", "", failedState, "request_fixture_outside_provenance", http.StatusOK, func(target *EnvelopeTarget) { target.FixturePath = "unlisted.bin" }, false},
		{"transport", "", failedState, "target_transport_failed", http.StatusOK, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(server.Close)
			runner := envelopeTestRunner(t, server, EnvelopeConfig{Release: MediaReleaseInputs{FixtureRoot: filepath.Dir(t.TempDir())}})
			target := EnvelopeTarget{ID: "resource", Class: metadataServiceName, URL: server.URL, ExpectedStatus: http.StatusOK}
			if tc.name == "expected denial" {
				target.ExpectedStatus = http.StatusNotFound
			}
			if tc.change != nil {
				tc.change(&target)
			}
			if tc.closeServer {
				server.Close()
			}
			body, _, sample, err := runner.request(context.Background(), target, http.MethodGet)
			if tc.wantError == "" {
				if err != nil || sample.Outcome != tc.outcome || string(body) != tc.body || sample.Status != tc.status {
					t.Fatalf("request sample=%+v error=%v body=%q", sample, err, body)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.wantError) || sample.Outcome != failedState {
				t.Fatalf("request sample=%+v error=%v, want %q", sample, err, tc.wantError)
			}
		})
	}
}
