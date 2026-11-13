package auth

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func requestMeasurementRecorder(t *testing.T) (*IdentityMeasurementRecorder, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "requests.jsonl")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	return &IdentityMeasurementRecorder{
		file: file, digest: sha256.New(),
		summary:         IdentityMeasurementSummary{PhaseWindows: map[string]*IdentityPhaseWindow{}},
		phaseRequests:   map[string]int64{},
		phaseSuccessful: map[string]int64{},
		phaseFailures:   map[string]int64{},
	}, path
}

func TestIdentityMeasuredRequestsClassifyActualResponses(t *testing.T) {
	for _, tc := range []struct {
		name, behavior, failure string
		status                  int
	}{
		{"acknowledged", "echo", "", http.StatusCreated},
		{"unexpected status", "status", "unexpected_status", http.StatusServiceUnavailable},
		{"wrong correlation", "correlation", "request_correlation_mismatch", http.StatusCreated},
		{"bounded body", "oversized", "transport_or_response_unavailable", http.StatusCreated},
		{"redirect refused", "redirect", "unexpected_status", http.StatusFound},
		{"transport unavailable", "closed", "transport_or_response_unavailable", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			received := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				received++
				if r.Method != http.MethodPost || r.Header.Get("Origin") != "https://kindle.test" || r.Header.Get("Content-Type") != "application/json" || r.Header.Get("Idempotency-Key") == "" {
					t.Errorf("measured request headers: %v", r.Header)
				}
				if tc.behavior == "correlation" {
					w.Header().Set("X-Request-Id", "different")
				} else {
					w.Header().Set("X-Request-Id", r.Header.Get("X-Request-Id"))
				}
				if tc.behavior == "redirect" {
					w.Header().Set("Location", "/other")
				}
				w.WriteHeader(tc.status)
				if tc.behavior == "oversized" {
					_, _ = io.WriteString(w, strings.Repeat("x", 65537))
				} else {
					_, _ = io.WriteString(w, `{"accepted":true}`)
				}
			}))
			t.Cleanup(server.Close)
			if tc.behavior == "closed" {
				server.Close()
			}
			recorder, path := requestMeasurementRecorder(t)
			client := IdentityMeasurementClient{BaseURL: server.URL, Origin: "https://kindle.test", Client: server.Client()}
			body, headers, err := recorder.request(context.Background(), client, IdentityMeasurementPhase{Name: "observed"}, "challenge", "/challenge", "", "", "", map[string]string{"request": "owned"}, http.StatusCreated, time.Now(), time.Second)
			if tc.failure == "" {
				if err != nil || string(body) != `{"accepted":true}` || headers.Get("X-Request-Id") == "" || recorder.summary.SuccessfulRequests != 1 {
					t.Fatalf("acknowledged request: body=%q headers=%v summary=%+v err=%v", body, headers, recorder.summary, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.failure) || body != nil || recorder.summary.SuccessfulRequests != 0 {
				t.Fatalf("measured response: body=%q summary=%+v err=%v, want %s", body, recorder.summary, err, tc.failure)
			}
			if tc.behavior != "closed" && received != 1 {
				t.Fatalf("actual request count=%d", received)
			}
			if recorder.summary.Requests != 1 || recorder.phaseRequests["observed"] != 1 {
				t.Fatalf("request accounting: %+v", recorder.summary)
			}
			raw, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			var sample IdentityRequestMeasurement
			if json.Unmarshal(raw, &sample) != nil || sample.Failure != tc.failure || sample.Status != tc.status || sample.RequestID == "" || sample.RequestSHA256 != measurementHash([]byte(`{"request":"owned"}`)) {
				t.Fatalf("recorded request: sample=%+v raw=%s", sample, raw)
			}
		})
	}
}

func TestIdentityMeasuredRequestsRejectInvalidInputsAndCaptureWrites(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(server.Close)
	for _, tc := range []struct {
		name, baseURL, want string
		payload             any
	}{
		{"request encoding", server.URL, "request encoding failed", make(chan int)},
		{"request construction", "://invalid", "request construction failed", map[string]string{"owned": "value"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder, path := requestMeasurementRecorder(t)
			client := IdentityMeasurementClient{BaseURL: tc.baseURL, Origin: "https://kindle.test", Client: server.Client()}
			body, headers, err := recorder.request(context.Background(), client, IdentityMeasurementPhase{Name: "observed"}, "challenge", "/challenge", "", "", "", tc.payload, http.StatusCreated, time.Now(), time.Second)
			raw, readErr := os.ReadFile(path)
			if err == nil || !strings.Contains(err.Error(), tc.want) || body != nil || headers != nil || recorder.summary.Requests != 0 || readErr != nil || len(raw) != 0 {
				t.Fatalf("invalid measured request: body=%q headers=%v summary=%+v capture=%q read=%v err=%v", body, headers, recorder.summary, raw, readErr, err)
			}
		})
	}
	recorder, path := requestMeasurementRecorder(t)
	if err := recorder.file.Close(); err != nil {
		t.Fatal(err)
	}
	recorder.append(IdentityRequestMeasurement{Phase: "observed", Status: 201, ExpectedStatus: 201})
	raw, err := os.ReadFile(path)
	if err != nil || recorder.failure == nil || !strings.Contains(recorder.failure.Error(), "raw capture write failed") || recorder.summary.Requests != 0 || len(raw) != 0 {
		t.Fatalf("closed capture: summary=%+v failure=%v data=%q read=%v", recorder.summary, recorder.failure, raw, err)
	}
}

func TestIdentityMeasuredJourneyRejectsInvalidChallengeWire(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", r.Header.Get("X-Request-Id"))
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"data":{"challenge":{"chainId":"125","purpose":"login"}}}`)
	}))
	t.Cleanup(server.Close)
	recorder, path := requestMeasurementRecorder(t)
	recorder.journey(context.Background(), IdentityMeasurementClient{BaseURL: server.URL, Origin: "https://kindle.test", Client: server.Client()}, IdentityMeasurementPhase{Name: "observed"}, time.Now(), time.Second)
	raw, err := os.ReadFile(path)
	if err != nil || recorder.summary.Requests != 1 || recorder.summary.SuccessfulRequests != 1 || recorder.summary.UnexpectedFailures != 1 || len(recorder.summary.Failures) != 1 || recorder.summary.Failures[0] != "challenge_wire_invalid" || !strings.Contains(string(raw), `"operation":"challenge"`) {
		t.Fatalf("challenge wire: summary=%+v capture=%q read=%v", recorder.summary, raw, err)
	}
}
