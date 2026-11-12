package storage

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvelopeSlowUploadChecksTerminalResponses(t *testing.T) {
	body := make([]byte, 6<<20)
	for index := range body {
		body[index] = byte(index)
	}
	fixture := envelopeTestFile(t, "upload.bin", body)
	for _, tc := range []struct {
		name     string
		status   int
		expected int
		retry    string
		want     string
	}{
		{"acknowledged", http.StatusOK, http.StatusOK, "", "slow_upload_was_acknowledged"},
		{"missing backpressure interval", http.StatusTooManyRequests, http.StatusTooManyRequests, "", "upload_backpressure_retry_missing"},
		{"unexpected terminal status", http.StatusServiceUnavailable, http.StatusTooManyRequests, "1", "slow_upload_unexpected_status"},
		{"bounded timeout", http.StatusRequestTimeout, http.StatusRequestTimeout, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				prefix := make([]byte, 32768)
				if _, err := io.ReadFull(r.Body, prefix); err != nil || r.Method != http.MethodPut || r.ContentLength != int64(len(body)) {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if tc.retry != "" {
					w.Header().Set("Retry-After", tc.retry)
				}
				w.WriteHeader(tc.status)
			}))
			defer server.Close()
			config := EnvelopeConfig{Release: MediaReleaseInputs{FixtureRoot: filepath.Dir(fixture.Path), Fixtures: []LicensedFixture{{Path: "upload.bin", SHA256: fixture.SHA256, Ownership: "owned"}}}}
			runner := envelopeTestRunner(t, server, config)
			ready := make(chan struct{}, 1)
			target := EnvelopeTarget{ID: "upload", Class: uploadKind, URL: server.URL + "/upload", FixturePath: "upload.bin", ExpectedStatus: tc.expected}
			err := runner.slowUpload(context.Background(), target, ready)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("terminal upload response: %v", err)
				}
			} else if err == nil || err.Error() != tc.want {
				t.Fatalf("terminal upload response: %v, want %q", err, tc.want)
			}
			select {
			case <-ready:
			default:
				t.Fatal("upload request did not send its licensed prefix")
			}
			if distribution := runner.aggregate.SnapshotDistributions()[uploadKind]; distribution.Count != 1 || distribution.Statuses[tc.status] != 1 {
				t.Fatalf("terminal upload sample: %+v", distribution)
			}
		})
	}
}

func TestEnvelopeScenarioChecksStructuredResponses(t *testing.T) {
	for _, tc := range []struct {
		name, body, header, want string
	}{
		{"invalid JSON", "not-json", "no-store", "scenario_json_invalid"},
		{"changed JSON", `{"state":"pending"}`, "no-store", "scenario_json_assertion_failed"},
		{"missing header", `{"state":"accepted"}`, "public", "scenario_header_assertion_failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Cache-Control", tc.header)
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			runner := envelopeTestRunner(t, server, EnvelopeConfig{})
			probe := EnvelopeProbe{Target: EnvelopeTarget{ID: "state", Class: scenarioField, URL: server.URL, ExpectedStatus: http.StatusOK}, Method: http.MethodGet, RequiredHeaders: map[string]string{"Cache-Control": "no-store"}, JSONEquals: map[string]json.RawMessage{"state": json.RawMessage(`"accepted"`)}}
			if err := runner.runCase(context.Background(), EnvelopeCase{ID: 2, Probes: []EnvelopeProbe{probe}}); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("scenario response: error=%v, want %q", err, tc.want)
			}
		})
	}
}

func TestEnvelopeUploadPrefixMatchesLicensedObject(t *testing.T) {
	body := make([]byte, 6<<20)
	for index := range body {
		body[index] = byte(index)
	}
	file := envelopeTestFile(t, "owned.bin", body)
	config := EnvelopeConfig{Release: MediaReleaseInputs{FixtureRoot: filepath.Dir(file.Path), Fixtures: []LicensedFixture{{Path: "owned.bin", SHA256: file.SHA256, Ownership: "owned"}}}}
	prefix, size, err := licensedEnvelopePrefix(config, "owned.bin")
	if err != nil || size != int64(len(body)) || len(prefix) != 32768 || string(prefix) != string(body[:32768]) {
		t.Fatalf("licensed upload prefix: size=%d prefix=%d err=%v", size, len(prefix), err)
	}
	for _, tc := range []struct {
		name   string
		change func(*EnvelopeConfig)
		path   string
	}{
		{"missing provenance", func(*EnvelopeConfig) {}, "absent.bin"},
		{"missing root", func(c *EnvelopeConfig) { c.Release.FixtureRoot = filepath.Join(t.TempDir(), "absent") }, "owned.bin"},
		{"missing object", func(c *EnvelopeConfig) { c.Release.Fixtures[0].Path = "absent.bin" }, "absent.bin"},
		{"changed digest", func(c *EnvelopeConfig) { c.Release.Fixtures[0].SHA256 = strings.Repeat("0", 64) }, "owned.bin"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := config
			candidate.Release.Fixtures = append([]LicensedFixture(nil), config.Release.Fixtures...)
			tc.change(&candidate)
			if prefix, size, err := licensedEnvelopePrefix(candidate, tc.path); err == nil || len(prefix) != 0 || size != 0 {
				t.Fatalf("unlicensed prefix: size=%d prefix=%d err=%v", size, len(prefix), err)
			}
		})
	}
}

func TestEnvelopeScenarioRulesRequireObservedStates(t *testing.T) {
	probe := EnvelopeProbe{Target: EnvelopeTarget{ID: "state", Class: scenarioField, ExpectedStatus: http.StatusOK}}
	check := EnvelopeSQLCheck{ExpectedJSON: json.RawMessage(`{"state":"accepted"}`)}
	command := EnvelopeCommand{TimeoutSeconds: 1}
	for _, tc := range []struct {
		name     string
		caseData EnvelopeCase
		want     string
	}{
		{"mixed load probe", EnvelopeCase{ID: 2, Probes: []EnvelopeProbe{{Target: EnvelopeTarget{Class: originServiceName}}}}, "scenario_probes_must_not_mix_with_load_metrics"},
		{"forged actor", EnvelopeCase{ID: 1, Probes: []EnvelopeProbe{probe}, SQL: []EnvelopeSQLCheck{check}}, "forged_actor_and_authenticated_durable_actor_required"},
		{"purge state JSON", EnvelopeCase{ID: 4, Before: []EnvelopeCommand{command}, After: []EnvelopeCommand{command}, Probes: []EnvelopeProbe{probe}, SQL: []EnvelopeSQLCheck{{ExpectedJSON: json.RawMessage(`{`)}}}, "conflict"},
		{"purge state", EnvelopeCase{ID: 4, Before: []EnvelopeCommand{command}, After: []EnvelopeCommand{command}, Probes: []EnvelopeProbe{probe}, SQL: []EnvelopeSQLCheck{{ExpectedJSON: json.RawMessage(`{"state":"deleted"}`)}}}, "purge_timeout_must_preserve_pending_state"},
		{"tombstone state JSON", EnvelopeCase{ID: 6, Before: []EnvelopeCommand{command}, After: []EnvelopeCommand{command}, Probes: []EnvelopeProbe{probe}, SQL: []EnvelopeSQLCheck{{ExpectedJSON: json.RawMessage(`{`)}}}, "conflict"},
		{"tombstone state", EnvelopeCase{ID: 6, Before: []EnvelopeCommand{command}, After: []EnvelopeCommand{command}, Probes: []EnvelopeProbe{probe}, SQL: []EnvelopeSQLCheck{check}}, "restored_tombstone_must_remain_deleted"},
		{"scan state", EnvelopeCase{ID: 7, Before: []EnvelopeCommand{command}, After: []EnvelopeCommand{command}, Probes: []EnvelopeProbe{probe}, SQL: []EnvelopeSQLCheck{{ExpectedJSON: json.RawMessage(`{"scanAlert":false}`)}}}, "scan_outage_alert_quarantine_and_approved_reads_required"},
	} {
		if err := validateEnvelopeCase(tc.caseData); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: error=%v, want %q", tc.name, err, tc.want)
		}
	}
}
