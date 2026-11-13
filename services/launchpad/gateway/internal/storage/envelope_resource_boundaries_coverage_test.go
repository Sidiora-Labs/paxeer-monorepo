package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestEnvelopeResourceSamplesEnforceMeasuredBounds(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
		status           int
		limit            int64
	}{
		{"denied", "", "resource_endpoint_denied", http.StatusForbidden, 2 << 20},
		{"oversized", strings.Repeat("x", (4<<20)+1), "resource_capture_invalid", http.StatusOK, 2 << 20},
		{"duplicate", "process_resident_memory_bytes 1024\nprocess_resident_memory_bytes 2048\ngo_goroutines 8\n", "resource_metric_ambiguous", http.StatusOK, 2 << 20},
		{"fractional", "process_resident_memory_bytes 1024.5\ngo_goroutines 8\n", "resource_metric_invalid", http.StatusOK, 2 << 20},
		{"missing", "process_resident_memory_bytes 1024\n", "actual_process_resource_metrics_missing", http.StatusOK, 2 << 20},
		{"budget", "# process metrics\nprocess_resident_memory_bytes 1048576\ngo_goroutines 8\n", "process_resource_limit_exceeded", http.StatusOK, 1024},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			runner := envelopeTestRunner(t, server, EnvelopeConfig{ResourceURL: server.URL, MaxResidentBytes: tc.limit, MaxGoroutines: 16})
			if err := runner.sampleResources(context.Background()); err == nil || err.Error() != tc.want {
				t.Fatalf("resource sample error = %v, want %q", err, tc.want)
			}
			if tc.name == "budget" && (runner.result.MaximumResidentBytes != 1048576 || runner.result.MaximumGoroutines != 8) {
				t.Fatalf("measured resource peak = %+v", runner.result)
			}
		})
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "process_resident_memory_bytes 1048576\ngo_goroutines 8\n")
	}))
	defer server.Close()
	runner := envelopeTestRunner(t, server, EnvelopeConfig{ResourceURL: server.URL, MaxResidentBytes: 2 << 20, MaxGoroutines: 16})
	if err := runner.resources.Close(); err != nil {
		t.Fatal(err)
	}
	if err := runner.sampleResources(context.Background()); err == nil || runner.writeError == nil {
		t.Fatalf("resource sample persistence: error=%v write error=%v", err, runner.writeError)
	}
}

func TestEnvelopeScenarioChecksCleanupAndResponseState(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bad-json":
			_, _ = io.WriteString(w, "{")
		case "/changed-state":
			_, _ = io.WriteString(w, `{"state":"changed"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	runner := envelopeTestRunner(t, server, EnvelopeConfig{})
	program, err := os.ReadFile("/usr/bin/false")
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(program)
	cleanup := EnvelopeCommand{Source: EnvelopeFile{Path: "/usr/bin/false", SHA256: hex.EncodeToString(digest[:])}, TimeoutSeconds: 2}
	for _, tc := range []struct {
		name      string
		caseInput EnvelopeCase
		want      string
	}{
		{"cleanup", EnvelopeCase{ID: 1, After: []EnvelopeCommand{cleanup}}, "scenario_cleanup_failed"},
		{"invalid JSON", EnvelopeCase{ID: 2, Probes: []EnvelopeProbe{{Target: EnvelopeTarget{ID: "bad-json", Class: scenarioField, URL: server.URL + "/bad-json", ExpectedStatus: http.StatusOK}, Method: http.MethodGet, JSONEquals: map[string]json.RawMessage{"state": json.RawMessage(`"accepted"`)}}}}, "scenario_json_invalid"},
		{"changed JSON", EnvelopeCase{ID: 2, Probes: []EnvelopeProbe{{Target: EnvelopeTarget{ID: "changed-state", Class: scenarioField, URL: server.URL + "/changed-state", ExpectedStatus: http.StatusOK}, Method: http.MethodGet, JSONEquals: map[string]json.RawMessage{"state": json.RawMessage(`"accepted"`)}}}}, "scenario_json_assertion_failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := runner.runCase(context.Background(), tc.caseInput); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("scenario error = %v, want %q", err, tc.want)
			}
		})
	}
	if runner.writeError != nil {
		t.Fatalf("scenario sample capture: %v", runner.writeError)
	}
}

func TestEnvelopeRequestsRejectUnboundHeadersAndBodies(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/large" {
			_, _ = io.WriteString(w, strings.Repeat("x", (1<<20)+1))
			return
		}
		_, _ = io.WriteString(w, "owned response")
	}))
	defer server.Close()
	runner := envelopeTestRunner(t, server, EnvelopeConfig{})
	for _, tc := range []struct {
		name, url, want string
		headers         map[string]string
		status          int
	}{
		{"unbound header", server.URL + "/response", "target_identity_unavailable", map[string]string{"X-Identity": "ENVELOPE_ABSENT_IDENTITY"}, http.StatusOK},
		{"oversized body", server.URL + "/large", "target_body_failed", nil, http.StatusOK},
		{"unexpected status", server.URL + "/response", "target_response_mismatch", nil, http.StatusCreated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := EnvelopeTarget{ID: tc.name, Class: metadataServiceName, URL: tc.url, HeaderEnvironment: tc.headers, ExpectedStatus: tc.status}
			_, _, sample, err := runner.request(context.Background(), target, http.MethodGet)
			if err == nil || err.Error() != tc.want || sample.Outcome != failedState {
				t.Fatalf("bounded response: sample=%+v err=%v, want %q", sample, err, tc.want)
			}
		})
	}
	if runner.writeError != nil {
		t.Fatalf("request sample capture: %v", runner.writeError)
	}
}
