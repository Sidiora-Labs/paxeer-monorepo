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
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvelopeSlowUploadEnforcesLicensedTerminalResponse(t *testing.T) {
	body := make([]byte, 6<<20)
	for index := range body {
		body[index] = byte(index)
	}
	file := envelopeTestFile(t, "licensed.bin", body)
	for _, tc := range []struct {
		name, path, want string
		status, expected int
		retry            bool
	}{
		{"missing fixture", "unlisted.bin", "upload_fixture_provenance_missing", 429, 429, true},
		{"accepted upload", "licensed.bin", "slow_upload_unexpected_status", 200, 429, false},
		{"unexpected denial", "licensed.bin", "slow_upload_was_acknowledged", 401, 401, false},
		{"missing retry", "licensed.bin", "upload_backpressure_retry_missing", 429, 429, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				prefix := make([]byte, 32768)
				if _, err := io.ReadFull(r.Body, prefix); err != nil || r.ContentLength != int64(len(body)) {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if tc.retry {
					w.Header().Set("Retry-After", "1")
				}
				w.WriteHeader(tc.status)
			}))
			defer server.Close()
			cfg := EnvelopeConfig{Release: MediaReleaseInputs{FixtureRoot: filepath.Dir(file.Path), Fixtures: []LicensedFixture{{Path: "licensed.bin", SHA256: file.SHA256, Ownership: "owned"}}}}
			runner := envelopeTestRunner(t, server, cfg)
			target := EnvelopeTarget{ID: "upload", Class: uploadKind, URL: server.URL + "/upload", FixturePath: tc.path, ExpectedStatus: tc.expected}
			err := runner.slowUpload(context.Background(), target, make(chan struct{}, 1))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("terminal response error = %v, want %q", err, tc.want)
			}
			if got := runner.aggregate.SnapshotDistributions()[uploadKind]; got.Count != 1 || got.Failed != 1 {
				t.Fatalf("failed upload distribution = %+v", got)
			}
		})
	}
}

func TestEnvelopeScenarioRejectsFailedExecutableAndUnsafeState(t *testing.T) {
	server := httptest.NewTLSServer(http.NotFoundHandler())
	defer server.Close()
	runner := envelopeTestRunner(t, server, EnvelopeConfig{})
	program, err := os.ReadFile("/usr/bin/false")
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(program)
	command := EnvelopeCommand{Source: EnvelopeFile{Path: "/usr/bin/false", SHA256: hex.EncodeToString(digest[:])}, TimeoutSeconds: 2}
	if err := runner.runCommand(context.Background(), command, 1); err == nil || !strings.Contains(err.Error(), "authorized_fixture_operation_failed") {
		t.Fatalf("failed executable was accepted: %v", err)
	}
	if err := runner.runCase(context.Background(), EnvelopeCase{ID: 1, Before: []EnvelopeCommand{command}}); err == nil || !strings.Contains(err.Error(), "authorized_fixture_operation_failed") {
		t.Fatalf("failed prerequisite was accepted: %v", err)
	}
	dsn := os.Getenv("L41_POSTGRES_URL")
	if dsn == "" {
		t.Fatal("database fixture is required")
	}
	t.Setenv("ENVELOPE_TEST_UNSAFE_STATE_DSN", dsn)
	check := EnvelopeSQLCheck{DSNEnvironment: "ENVELOPE_TEST_UNSAFE_STATE_DSN", Query: `SELECT '{"secret":"private"}'::jsonb`, ExpectedJSON: json.RawMessage(`{"secret":"private"}`)}
	if err := runner.runSQL(context.Background(), check, 2); err == nil || !strings.Contains(err.Error(), "sql_observation_not_bounded_safe_state") {
		t.Fatalf("unsafe persisted state was accepted: %v", err)
	}
	if err := runner.runSQL(context.Background(), EnvelopeSQLCheck{DSNEnvironment: "ENVELOPE_TEST_UNSAFE_STATE_DSN", Query: `SELECT '{"state":"accepted"}'::jsonb`, ExpectedJSON: json.RawMessage(`{"state":"accepted"}`)}, 2); err != nil {
		t.Fatalf("bounded persisted state rejected: %v", err)
	}
	if runner.writeError != nil {
		t.Fatalf("scenario event capture: %v", runner.writeError)
	}
}
