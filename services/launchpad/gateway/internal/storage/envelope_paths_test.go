package storage

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func envelopeTestFile(t *testing.T, name string, data []byte) EnvelopeFile {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	return EnvelopeFile{Path: path, SHA256: hex.EncodeToString(digest[:])}
}

func envelopeTestRunner(t *testing.T, server *httptest.Server, config EnvelopeConfig) *envelopeRunner {
	t.Helper()
	root := x509.NewCertPool()
	root.AddCert(server.Certificate())
	file, err := os.Create(filepath.Join(t.TempDir(), "requests.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	resources, err := os.Create(filepath.Join(t.TempDir(), "resources.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resources.Close() })
	events, err := os.Create(filepath.Join(t.TempDir(), "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = events.Close() })
	client := server.Client()
	client.Timeout = 3 * time.Second
	return &envelopeRunner{config: config, client: client, tlsConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: root}, aggregate: NewEnvelopeAccumulator(), raw: file, resources: resources, events: events, result: EnvelopeResult{ScenarioResults: map[int]string{}}}
}

func TestEnvelopeOwnedFixtureAndBoundedFile(t *testing.T) {
	file := envelopeTestFile(t, "owned.bin", []byte("owned media bytes"))
	raw, err := readEnvelopeFile(file, 64)
	if err != nil || string(raw) != "owned media bytes" {
		t.Fatalf("read owned file: %q %v", raw, err)
	}
	if _, err := readEnvelopeFile(file, 4); err == nil {
		t.Fatal("oversized evidence admitted")
	}
	changed := file
	changed.SHA256 = strings.Repeat("0", 64)
	if _, err := readEnvelopeFile(changed, 64); err == nil {
		t.Fatal("changed evidence digest admitted")
	}
	if _, err := readEnvelopeFile(EnvelopeFile{Path: file.Path}, 64); err == nil {
		t.Fatal("evidence without digest admitted")
	}
	if _, err := readEnvelopeFile(EnvelopeFile{Path: filepath.Join(t.TempDir(), "absent"), SHA256: file.SHA256}, 64); err == nil {
		t.Fatal("absent evidence admitted")
	}
	var decoded struct{ State string }
	if err := strictEnvelopeJSON([]byte(`{"State":"accepted"}`), &decoded); err != nil || decoded.State != "accepted" {
		t.Fatalf("strict JSON: %q %v", decoded.State, err)
	}
	for _, input := range []string{`{"Other":true}`, `{"State":"accepted"} {"State":"other"}`, `{`} {
		if err := strictEnvelopeJSON([]byte(input), &decoded); err == nil {
			t.Fatalf("invalid JSON admitted: %q", input)
		}
	}
	root := filepath.Dir(file.Path)
	config := EnvelopeConfig{Release: MediaReleaseInputs{FixtureRoot: root, Fixtures: []LicensedFixture{{Path: "owned.bin", SHA256: file.SHA256, Ownership: "owned"}}}}
	if raw, err := licensedEnvelopeBytes(config, "owned.bin"); err != nil || string(raw) != "owned media bytes" {
		t.Fatalf("licensed bytes: %q %v", raw, err)
	}
	if _, err := licensedEnvelopeBytes(config, "other.bin"); err == nil {
		t.Fatal("unlisted fixture admitted")
	}
	config.Release.Fixtures[0].SHA256 = strings.Repeat("0", 64)
	if _, err := licensedEnvelopeBytes(config, "owned.bin"); err == nil {
		t.Fatal("changed licensed fixture admitted")
	}
}

func TestEnvelopeTargetsAndScenarioRules(t *testing.T) {
	file := envelopeTestFile(t, "media.bin", []byte("licensed response"))
	config := EnvelopeConfig{Release: MediaReleaseInputs{FixtureRoot: filepath.Dir(file.Path), Fixtures: []LicensedFixture{{Path: "media.bin", SHA256: file.SHA256, Ownership: "owned"}}}}
	valid := EnvelopeTarget{ID: "licensed", Class: originServiceName, URL: "https://media.example/approved", ExpectedStatus: 200, ExpectedSHA256: file.SHA256}
	if err := validateEnvelopeTarget(config, valid); err != nil {
		t.Fatal(err)
	}
	for _, changed := range []EnvelopeTarget{
		{ID: "bad id", URL: valid.URL, ExpectedStatus: 200},
		{ID: valid.ID, URL: "http://media.example/approved", ExpectedStatus: 200},
		{ID: valid.ID, URL: valid.URL, ExpectedStatus: 199},
		{ID: valid.ID, URL: valid.URL, ExpectedStatus: 200, Headers: map[string]string{"Authorization": "inline"}},
		{ID: valid.ID, URL: valid.URL, ExpectedStatus: 200, HeaderEnvironment: map[string]string{"X-Identity": "ABSENT_ENVELOPE_IDENTITY"}},
		{ID: valid.ID, URL: valid.URL, ExpectedStatus: 200, Class: originServiceName, ExpectedSHA256: strings.Repeat("0", 64)},
	} {
		if err := validateEnvelopeTarget(config, changed); err == nil {
			t.Fatalf("invalid target admitted: %+v", changed)
		}
	}
	t.Setenv("ENVELOPE_TEST_IDENTITY", "fixture-token")
	headers, err := envelopeHeaders(EnvelopeTarget{Headers: map[string]string{"X-Static": "fixture"}, HeaderEnvironment: map[string]string{"X-Identity": "ENVELOPE_TEST_IDENTITY"}})
	if err != nil || headers.Get("X-Identity") != "fixture-token" || headers.Get("X-Static") != "fixture" || headers.Get("X-Request-ID") == "" {
		t.Fatalf("envelope headers: %v %v", headers, err)
	}
	if regexpWallet("0x"+strings.Repeat("a", 40)) == false || regexpWallet("0x"+strings.Repeat("g", 40)) {
		t.Fatal("wallet syntax not enforced")
	}
	if safeEnvelopeState([]byte(`{"state":"accepted","revision":1}`)) == false || safeEnvelopeState([]byte(`{"secret":"private"}`)) {
		t.Fatal("state allowlist not enforced")
	}
	if !sameEnvelopeJSON([]byte(`{"revision":1,"state":"accepted"}`), []byte(`{"state":"accepted","revision":1}`)) {
		t.Fatal("equivalent JSON state disagreed")
	}
	for _, test := range []EnvelopeCase{
		{ID: 5},
		{ID: 1},
		{ID: 2, Probes: []EnvelopeProbe{{Target: EnvelopeTarget{Class: scenarioField, ExpectedStatus: 200}}}},
		{ID: 3, SQL: []EnvelopeSQLCheck{{ExpectedJSON: json.RawMessage(`{"state":"accepted"}`)}}},
		{ID: 4, SQL: []EnvelopeSQLCheck{{ExpectedJSON: json.RawMessage(`{"state":"accepted"}`)}}},
		{ID: 6, SQL: []EnvelopeSQLCheck{{ExpectedJSON: json.RawMessage(`{"state":"accepted"}`)}}},
		{ID: 7, SQL: []EnvelopeSQLCheck{{ExpectedJSON: json.RawMessage(`{"scanAlert":false}`)}}},
	} {
		if err := validateEnvelopeCase(test); err == nil {
			t.Fatalf("incomplete scenario admitted: %+v", test)
		}
	}
	if err := validateEnvelopeCase(EnvelopeCase{ID: 2, Probes: []EnvelopeProbe{{Target: EnvelopeTarget{Class: scenarioField, ExpectedStatus: 404}, RequiredHeaders: map[string]string{"Cache-Control": "no-store"}}}}); err != nil {
		t.Fatalf("bounded traversal denial rejected: %v", err)
	}
}

func TestEnvelopeTLSRequestsAndResourceMetrics(t *testing.T) {
	body := []byte(`{"state":"accepted"}`)
	digest := sha256.Sum256(body)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/approved":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(body)
		case "/denied":
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusNotFound)
		case "/metrics":
			_, _ = io.WriteString(w, "process_resident_memory_bytes 1048576\ngo_goroutines 8\n")
		}
	}))
	defer server.Close()
	runner := envelopeTestRunner(t, server, EnvelopeConfig{ResourceURL: server.URL + "/metrics", MaxResidentBytes: 2 << 20, MaxGoroutines: 16})
	target := EnvelopeTarget{ID: "approved", Class: originServiceName, URL: server.URL + "/approved", ExpectedStatus: 200, ExpectedSHA256: hex.EncodeToString(digest[:])}
	_, _, sample, err := runner.request(context.Background(), target, http.MethodGet)
	if err != nil || sample.Outcome != acceptedState || sample.Bytes != int64(len(body)) {
		t.Fatalf("approved request: %+v %v", sample, err)
	}
	target.ExpectedSHA256 = strings.Repeat("0", 64)
	if _, _, _, err := runner.request(context.Background(), target, http.MethodGet); err == nil {
		t.Fatal("changed response accepted")
	}
	denied := EnvelopeTarget{ID: "denied", Class: scenarioField, URL: server.URL + "/denied", ExpectedStatus: 404}
	caseResult := EnvelopeCase{ID: 2, Probes: []EnvelopeProbe{{Target: denied, Method: http.MethodGet, RequiredHeaders: map[string]string{"Cache-Control": "no-store"}}}}
	if err := runner.runCase(context.Background(), caseResult); err != nil {
		t.Fatalf("denial scenario: %v", err)
	}
	if err := runner.sampleResources(context.Background()); err != nil || runner.result.MaximumResidentBytes != 1048576 || runner.result.MaximumGoroutines != 8 {
		t.Fatalf("resource sample: memory=%d goroutines=%d err=%v", runner.result.MaximumResidentBytes, runner.result.MaximumGoroutines, err)
	}
	if runner.writeError != nil {
		t.Fatalf("sample recording: %v", runner.writeError)
	}
}

func TestEnvelopeSlowUploadUsesLicensedBody(t *testing.T) {
	body := make([]byte, 6<<20)
	for i := range body {
		body[i] = byte(i)
	}
	file := envelopeTestFile(t, "upload.bin", body)
	seen := make(chan int64, 1)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		prefix := make([]byte, 32768)
		if _, err := io.ReadFull(r.Body, prefix); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		seen <- r.ContentLength
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	config := EnvelopeConfig{Release: MediaReleaseInputs{FixtureRoot: filepath.Dir(file.Path), Fixtures: []LicensedFixture{{Path: "upload.bin", SHA256: file.SHA256, Ownership: "owned"}}}}
	runner := envelopeTestRunner(t, server, config)
	ready := make(chan struct{}, 1)
	target := EnvelopeTarget{ID: "upload", Class: uploadKind, URL: server.URL + "/upload", FixturePath: "upload.bin", ExpectedStatus: http.StatusTooManyRequests}
	if err := runner.slowUpload(context.Background(), target, ready); err != nil {
		t.Fatalf("bounded upload response: %v", err)
	}
	select {
	case <-ready:
	default:
		t.Fatal("upload headers and body prefix were not sent")
	}
	if got := <-seen; got != int64(len(body)) {
		t.Fatalf("declared body length = %d, want %d", got, len(body))
	}
	if runner.aggregate.SnapshotDistributions()[uploadKind].Count != 1 || runner.writeError != nil {
		t.Fatalf("upload sample: distributions=%v error=%v", runner.aggregate.SnapshotDistributions(), runner.writeError)
	}
}

func TestEnvelopeScenarioChecksRealCommandAndSQL(t *testing.T) {
	dsn := os.Getenv("L41_POSTGRES_URL")
	if dsn == "" {
		t.Fatal("L41_POSTGRES_URL is required for the gateway storage fixture")
	}
	t.Setenv("ENVELOPE_TEST_DSN", dsn)
	program, err := os.ReadFile("/usr/bin/true")
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(program)
	command := EnvelopeCommand{Source: EnvelopeFile{Path: "/usr/bin/true", SHA256: hex.EncodeToString(hash[:])}, TimeoutSeconds: 2}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Result", "accepted")
		_, _ = io.WriteString(w, `{"state":"accepted"}`)
	}))
	defer server.Close()
	runner := envelopeTestRunner(t, server, EnvelopeConfig{})
	check := EnvelopeSQLCheck{DSNEnvironment: "ENVELOPE_TEST_DSN", Query: `SELECT '{"state":"accepted"}'::jsonb`, ExpectedJSON: json.RawMessage(`{"state":"accepted"}`)}
	test := EnvelopeCase{
		ID:     4,
		Before: []EnvelopeCommand{command}, After: []EnvelopeCommand{command},
		Probes: []EnvelopeProbe{{Target: EnvelopeTarget{ID: "scenario", Class: scenarioField, URL: server.URL, ExpectedStatus: 200}, Method: http.MethodGet, JSONEquals: map[string]json.RawMessage{"state": json.RawMessage(`"accepted"`)}, RequiredHeaders: map[string]string{"X-Result": "accepted"}}},
		SQL:    []EnvelopeSQLCheck{check},
	}
	if err := validateEnvelopeCase(test); err != nil {
		t.Fatalf("actual scenario inputs: %v", err)
	}
	if err := runner.runCase(context.Background(), test); err != nil {
		t.Fatalf("command, HTTPS probe and SQL observation: %v", err)
	}
	if runner.writeError != nil {
		t.Fatalf("scenario event recording: %v", runner.writeError)
	}
	for _, tc := range []struct {
		name   string
		change func(*EnvelopeCase)
		want   string
	}{
		{"response header", func(c *EnvelopeCase) { c.Probes[0].RequiredHeaders["X-Result"] = "different" }, "scenario_header_assertion_failed"},
		{"response state", func(c *EnvelopeCase) { c.Probes[0].JSONEquals["state"] = json.RawMessage(`"different"`) }, "scenario_json_assertion_failed"},
		{"database state", func(c *EnvelopeCase) { c.SQL[0].ExpectedJSON = json.RawMessage(`{"state":"requested"}`) }, "persisted_state_assertion_failed"},
		{"database query", func(c *EnvelopeCase) { c.SQL[0].Query = `SELECT state FROM missing_media_state` }, "sql_observation_unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := test
			candidate.Probes = append([]EnvelopeProbe(nil), test.Probes...)
			candidate.Probes[0].RequiredHeaders = map[string]string{"X-Result": "accepted"}
			candidate.Probes[0].JSONEquals = map[string]json.RawMessage{"state": json.RawMessage(`"accepted"`)}
			candidate.SQL = append([]EnvelopeSQLCheck(nil), test.SQL...)
			tc.change(&candidate)
			if err := runner.runCase(context.Background(), candidate); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("scenario assertion: error=%v, want %q", err, tc.want)
			}
		})
	}
}

func TestEnvelopePreparesLicensedMeasurementTopology(t *testing.T) {
	fixture := make([]byte, 6<<20)
	for i := range fixture {
		fixture[i] = byte(i)
	}
	owned := envelopeTestFile(t, "owned.bin", fixture)
	quota := envelopeTestFile(t, "quota.json", []byte(`{"provider":"owned"}`))
	now := time.Now().UTC()
	providerRaw, err := json.Marshal(EnvelopeProviderSnapshot{
		Account: "local-provider", Provider: "object-storage", RequestID: "local-quota", ObservedAt: now,
		Currency: "USD", CostMicros: 1, BudgetMicros: 2, UploadConcurrency: 200,
		CDNViewers: 20000, OriginalRetentionDays: 30,
	})
	if err != nil {
		t.Fatal(err)
	}
	provider := envelopeTestFile(t, "provider.json", providerRaw)
	allocationRaw, err := json.Marshal(EnvelopeAllocation{GlobalDAU: 500000, GlobalPeakRPS: 10000, Lane: "L41", AssignedRPS: 3, OriginRPS: 1, MetadataRPS: 1, CDNRPS: 1, UploadConnections: 200, CDNViewerTarget: 20000})
	if err != nil {
		t.Fatal(err)
	}
	allocation := envelopeTestFile(t, "allocation.json", allocationRaw)
	manifest := envelopeTestFile(t, "candidate.json", []byte(`{"candidate":"local"}`))
	interfaceManifest := envelopeTestFile(t, "interfaces.json", []byte(`{"interfaces":"local"}`))
	program, err := os.ReadFile("/usr/bin/true")
	if err != nil {
		t.Fatal(err)
	}
	programHash := sha256.Sum256(program)
	command := EnvelopeCommand{Source: EnvelopeFile{Path: "/usr/bin/true", SHA256: hex.EncodeToString(programHash[:])}, TimeoutSeconds: 2}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	ca := filepath.Join(t.TempDir(), "root.pem")
	if err := os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	read := func(id, class string) EnvelopeTarget {
		return EnvelopeTarget{ID: id, Class: class, URL: server.URL + "/" + id, ExpectedStatus: 200, ExpectedSHA256: owned.SHA256}
	}
	scenario := func(id string, status int) EnvelopeProbe {
		return EnvelopeProbe{Target: EnvelopeTarget{ID: id, Class: scenarioField, URL: server.URL + "/" + id, ExpectedStatus: status}, Method: http.MethodGet}
	}
	denied := scenario("denied", 404)
	denied.RequiredHeaders = map[string]string{"Cache-Control": "no-store"}
	allowed := scenario("allowed", 200)
	allowed.Target.ExpectedSHA256 = owned.SHA256
	quarantined := scenario("quarantined", 202)
	check := func(state string) EnvelopeSQLCheck {
		return EnvelopeSQLCheck{DSNEnvironment: "L41_POSTGRES_URL", Query: `SELECT '{"state":"accepted"}'::jsonb`, ExpectedJSON: json.RawMessage(fmt.Sprintf(`{"state":%q}`, state))}
	}
	actorCheck := check("accepted")
	actorCheck.ExpectedJSON = json.RawMessage(fmt.Sprintf(`{"actor":%q}`, "0x"+strings.Repeat("a", 40)))
	forged := scenario("forged", 403)
	forged.Target.Headers = map[string]string{"X-Actor-Wallet": "0x" + strings.Repeat("b", 40)}
	uploads := make([]EnvelopeTarget, 200)
	for i := range uploads {
		uploads[i] = EnvelopeTarget{ID: fmt.Sprintf("upload-%d", i), Class: uploadKind, URL: server.URL + "/upload", ExpectedStatus: 429, FixturePath: "owned.bin", Wallet: fmt.Sprintf("0x%040x", i/2)}
	}
	c := EnvelopeConfig{
		Scope: "lane-measurement", CandidateSHA256: manifest.SHA256, CandidateManifest: manifest,
		PlaybackBinary: command.Source, PlaybackSeconds: 1, PlaybackTargetID: "cdn",
		InterfaceManifest: interfaceManifest, SourceFiles: []EnvelopeFile{manifest}, Allocation: allocation, ProviderSnapshot: provider,
		Release: MediaReleaseInputs{ProviderAccount: "local-provider", QuotaConfirmationPath: quota.Path, QuotaConfirmationSHA256: quota.SHA256,
			QuotaExpiresAt: now.Add(time.Hour), OriginalRetentionDays: 30, MixedWorkloadHours: 24, CDNViewerTarget: 20000,
			FixtureRoot: filepath.Dir(owned.Path), Fixtures: []LicensedFixture{{Path: "owned.bin", SHA256: owned.SHA256, Ownership: "owned"}}},
		TLSRootCA: ca, DurationSeconds: 30, ReadWorkers: 3,
		Targets:     []EnvelopeTarget{read("origin", originServiceName), {ID: "metadata", Class: metadataServiceName, URL: server.URL + "/metadata", ExpectedStatus: 200}, read("cdn", cdnServiceName)},
		SlowUploads: uploads, ResourceURL: server.URL + "/metrics", MaxResidentBytes: 1 << 30, MaxGoroutines: 10000,
		Cases: []EnvelopeCase{
			{ID: 1, Probes: []EnvelopeProbe{forged}, SQL: []EnvelopeSQLCheck{actorCheck}},
			{ID: 2, Probes: []EnvelopeProbe{denied}},
			{ID: 3, Probes: []EnvelopeProbe{allowed, denied}, SQL: []EnvelopeSQLCheck{check("accepted")}},
			{ID: 4, Before: []EnvelopeCommand{command}, After: []EnvelopeCommand{command}, Probes: []EnvelopeProbe{allowed}, SQL: []EnvelopeSQLCheck{check("requested")}},
			{ID: 6, Before: []EnvelopeCommand{command}, After: []EnvelopeCommand{command}, Probes: []EnvelopeProbe{denied}, SQL: []EnvelopeSQLCheck{check("deleted")}},
			{ID: 7, Before: []EnvelopeCommand{command}, After: []EnvelopeCommand{command}, Probes: []EnvelopeProbe{quarantined, allowed}, SQL: []EnvelopeSQLCheck{{DSNEnvironment: "L41_POSTGRES_URL", Query: `SELECT '{"scanAlert":true}'::jsonb`, ExpectedJSON: json.RawMessage(`{"scanAlert":true}`)}}},
		},
		OutputDirectory: filepath.Join(t.TempDir(), "measurement"),
	}
	if os.Getenv("L41_POSTGRES_URL") == "" {
		t.Fatal("L41_POSTGRES_URL is required for the gateway storage fixture")
	}
	for _, tc := range []struct {
		name   string
		change func(*EnvelopeConfig)
		want   string
	}{
		{"scope", func(c *EnvelopeConfig) { c.Scope = "unbound" }, "measurement_scope_required"},
		{"candidate", func(c *EnvelopeConfig) { c.CandidateSHA256 = "invalid" }, "measurement_budget_or_candidate_missing"},
		{"shared duration", func(c *EnvelopeConfig) { c.Scope = sharedReleaseClass }, "shared_profile_requires_24h"},
		{"ownership", func(c *EnvelopeConfig) { c.Release.Fixtures[0].Ownership = "" }, "fixture"},
		{"source digest", func(c *EnvelopeConfig) { c.SourceFiles[0].SHA256 = strings.Repeat("0", 64) }, "evidence"},
		{"provider account", func(c *EnvelopeConfig) { c.Release.ProviderAccount = "other" }, "actual_provider_quota_cost_retention_snapshot_missing_or_outside_budget"},
		{"provider document", func(c *EnvelopeConfig) {
			c.ProviderSnapshot = envelopeTestFile(t, "invalid-provider.json", []byte(`{"provider":`))
		}, "invalid"},
		{"allocation", func(c *EnvelopeConfig) {
			c.Allocation = envelopeTestFile(t, "invalid-allocation.json", []byte(`{"lane":"L42"}`))
		}, "workload_allocation_invalid"},
		{"allocation document", func(c *EnvelopeConfig) {
			c.Allocation = envelopeTestFile(t, "malformed-allocation.json", []byte(`{"lane":`))
		}, "invalid"},
		{"playback binary", func(c *EnvelopeConfig) { c.PlaybackBinary.SHA256 = strings.Repeat("0", 64) }, "evidence"},
		{"root authority", func(c *EnvelopeConfig) { c.TLSRootCA = filepath.Join(t.TempDir(), "absent.pem") }, "tls_authority_missing"},
		{"invalid authority", func(c *EnvelopeConfig) {
			c.TLSRootCA = envelopeTestFile(t, "invalid.pem", []byte("not a certificate")).Path
		}, "tls_authority_invalid"},
		{"client identity", func(c *EnvelopeConfig) { c.TLSCertificate = filepath.Join(t.TempDir(), "absent.pem") }, "tls_client_identity_unavailable"},
		{"read status", func(c *EnvelopeConfig) { c.Targets[0].ExpectedStatus = 404 }, "load_reads_must_expect_actual_success"},
		{"read class", func(c *EnvelopeConfig) { c.Targets[0].Class = "unknown" }, "read_target_class_invalid"},
		{"read duplicate", func(c *EnvelopeConfig) { c.Targets[1].ID = c.Targets[0].ID }, "conflict"},
		{"read target authority", func(c *EnvelopeConfig) { c.Targets[0].URL = "http://insecure.example/approved" }, "explicit_https_target_required"},
		{"cdn playback", func(c *EnvelopeConfig) { c.PlaybackTargetID = "absent" }, "licensed_public_cdn_playback_target_required"},
		{"missing class", func(c *EnvelopeConfig) { c.Targets = []EnvelopeTarget{c.Targets[0], c.Targets[2]} }, "complete_media_target_topology_required"},
		{"upload wallet", func(c *EnvelopeConfig) { c.SlowUploads[0].Wallet = "invalid" }, "authenticated_upload_wallet_required"},
		{"upload duplicate", func(c *EnvelopeConfig) { c.SlowUploads[1].ID = c.SlowUploads[0].ID }, "conflict"},
		{"upload wallet capacity", func(c *EnvelopeConfig) { c.SlowUploads[2].Wallet = c.SlowUploads[0].Wallet }, "upload_wallet_concurrency_exceeds_two"},
		{"upload fixture", func(c *EnvelopeConfig) {
			path := filepath.Join(c.Release.FixtureRoot, "small.bin")
			if err := os.WriteFile(path, []byte("short"), 0600); err != nil {
				t.Fatal(err)
			}
			c.Release.Fixtures = append(c.Release.Fixtures, LicensedFixture{Path: "small.bin", SHA256: measurementHashForEnvelope([]byte("short")), Ownership: "owned"})
			c.SlowUploads[0].FixturePath = "small.bin"
		}, "licensed_6mib_upload_required"},
		{"resource authority", func(c *EnvelopeConfig) { c.ResourceURL = "http://insecure.example/metrics" }, "explicit_https_target_required"},
		{"resource credential", func(c *EnvelopeConfig) {
			c.ResourceHeaders = map[string]string{"Authorization": "ENVELOPE_ABSENT_RESOURCE_IDENTITY"}
		}, "resource_identity_missing"},
		{"duplicate case", func(c *EnvelopeConfig) {
			c.Cases[1] = c.Cases[0]
		}, "adversarial_case_invalid"},
		{"case target authority", func(c *EnvelopeConfig) {
			probes := append([]EnvelopeProbe(nil), c.Cases[1].Probes...)
			probes[0].Target.URL = "http://insecure.example/denied"
			c.Cases[1].Probes = probes
		}, "explicit_https_target_required"},
		{"case command source", func(c *EnvelopeConfig) {
			commands := append([]EnvelopeCommand(nil), c.Cases[3].Before...)
			commands[0].Source.SHA256 = strings.Repeat("0", 64)
			c.Cases[3].Before = commands
		}, "evidence"},
		{"reserved case", func(c *EnvelopeConfig) { c.Cases[0].ID = 5 }, "conflict"},
		{"case command timeout", func(c *EnvelopeConfig) {
			commands := append([]EnvelopeCommand(nil), c.Cases[3].Before...)
			commands[0].TimeoutSeconds = 0
			c.Cases[3].Before = commands
		}, "conflict"},
		{"case probe method", func(c *EnvelopeConfig) {
			probes := append([]EnvelopeProbe(nil), c.Cases[1].Probes...)
			probes[0].Method = http.MethodTrace
			c.Cases[1].Probes = probes
		}, "conflict"},
		{"case database authority", func(c *EnvelopeConfig) {
			checks := append([]EnvelopeSQLCheck(nil), c.Cases[0].SQL...)
			checks[0].DSNEnvironment = "ENVELOPE_ABSENT_DATABASE"
			c.Cases[0].SQL = checks
		}, "sql_observation_authority_missing"},
		{"missing case", func(c *EnvelopeConfig) { c.Cases = c.Cases[:len(c.Cases)-1] }, "adversarial_case_missing"},
		{"existing output directory", func(c *EnvelopeConfig) {
			if err := os.Mkdir(c.OutputDirectory, 0700); err != nil {
				t.Fatal(err)
			}
		}, "new_private_output_directory_required"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := c
			candidate.SourceFiles = append([]EnvelopeFile(nil), c.SourceFiles...)
			candidate.Targets = append([]EnvelopeTarget(nil), c.Targets...)
			candidate.SlowUploads = append([]EnvelopeTarget(nil), c.SlowUploads...)
			candidate.Cases = append([]EnvelopeCase(nil), c.Cases...)
			candidate.Release.Fixtures = append([]LicensedFixture(nil), c.Release.Fixtures...)
			candidate.OutputDirectory = filepath.Join(t.TempDir(), "measurement")
			tc.change(&candidate)
			runner, err := prepareMediaEnvelope(candidate)
			if runner != nil || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("%s: runner=%v error=%v, want %q", tc.name, runner, err, tc.want)
			}
		})
	}
	runner, err := prepareMediaEnvelope(c)
	if err != nil {
		t.Fatalf("licensed measurement topology: %v", err)
	}
	defer func() {
		_ = runner.root.Close()
		_ = runner.raw.Close()
		_ = runner.resources.Close()
		_ = runner.events.Close()
		runner.client.CloseIdleConnections()
	}()
	if runner.result.ProviderAccount != "local-provider" || runner.result.CandidateSHA256 != manifest.SHA256 || runner.allocation.UploadConnections != 200 || len(runner.config.SlowUploads) != 200 {
		t.Fatalf("prepared topology: provider=%q candidate=%q uploads=%d", runner.result.ProviderAccount, runner.result.CandidateSHA256, len(runner.config.SlowUploads))
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c.OutputDirectory = filepath.Join(t.TempDir(), "cancelled-measurement")
	result, err := RunMediaEnvelope(ctx, c)
	if err == nil || result.Scope != "lane-measurement" || result.HeldUploadConnections != 0 || len(result.Blockers) == 0 || result.SharedReleaseQualified {
		t.Fatalf("cancelled measurement: scope=%q held=%d blockers=%v qualified=%v err=%v", result.Scope, result.HeldUploadConnections, result.Blockers, result.SharedReleaseQualified, err)
	}
	for _, name := range []string{"requests.jsonl", "resources.jsonl", "events.jsonl"} {
		raw, err := os.ReadFile(filepath.Join(c.OutputDirectory, name))
		if err != nil || result.Artifacts[name] != measurementHashForEnvelope(raw) {
			t.Fatalf("measurement artifact %q: digest=%q err=%v", name, result.Artifacts[name], err)
		}
	}
	if raw, err := os.ReadFile(filepath.Join(c.OutputDirectory, "result.json")); err != nil || !json.Valid(raw) {
		t.Fatalf("measurement result: valid=%v err=%v", json.Valid(raw), err)
	}
	videoPath := filepath.Join(c.Release.FixtureRoot, "owned-playback.mp4")
	generate := exec.Command("ffmpeg", "-nostdin", "-loglevel", "error", "-f", "lavfi", "-i", "color=c=red:s=16x16:r=10", "-t", "2", "-c:v", "mpeg4", "-movflags", "+faststart", "-y", videoPath)
	if output, err := generate.CombinedOutput(); err != nil {
		t.Fatalf("generate owned playback bytes: %v: %s", err, output)
	}
	video, err := os.ReadFile(videoPath)
	if err != nil {
		t.Fatal(err)
	}
	videoSHA := measurementHashForEnvelope(video)
	c.Release.Fixtures = append(c.Release.Fixtures, LicensedFixture{Path: "owned-playback.mp4", SHA256: videoSHA, Ownership: "owned"})
	var receivedUploads, reads, resources atomic.Int64
	load := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/upload":
			prefix := make([]byte, 32768)
			if _, err := io.ReadFull(r.Body, prefix); err != nil || r.ContentLength != int64(len(fixture)) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			receivedUploads.Add(1)
			time.Sleep(4 * time.Second)
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
		case "/metrics":
			resources.Add(1)
			_, _ = io.WriteString(w, "process_resident_memory_bytes 1048576\ngo_goroutines 8\n")
		default:
			reads.Add(1)
			_, _ = w.Write(video)
		}
	}))
	defer load.Close()
	caPath := filepath.Join(t.TempDir(), "load-root.pem")
	if err := os.WriteFile(caPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: load.Certificate().Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	player, err := os.ReadFile("/usr/bin/ffmpeg")
	if err != nil {
		t.Fatal(err)
	}
	c.PlaybackBinary = EnvelopeFile{Path: "/usr/bin/ffmpeg", SHA256: measurementHashForEnvelope(player)}
	c.TLSRootCA = caPath
	c.ResourceURL = load.URL + "/metrics"
	c.OutputDirectory = filepath.Join(t.TempDir(), "bounded-measurement")
	for index := range c.Targets {
		c.Targets[index].URL = load.URL + "/" + c.Targets[index].ID
		c.Targets[index].ExpectedSHA256 = videoSHA
	}
	for index := range c.SlowUploads {
		c.SlowUploads[index].URL = load.URL + "/upload"
	}
	for index := range c.Cases {
		for probe := range c.Cases[index].Probes {
			p := &c.Cases[index].Probes[probe]
			p.Target.URL = load.URL + "/" + p.Target.ID
			if p.Target.ExpectedStatus == http.StatusOK {
				p.Target.ExpectedSHA256 = videoSHA
			}
		}
	}
	result, err = RunMediaEnvelope(context.Background(), c)
	if err == nil || len(result.Blockers) == 0 || result.HeldUploadConnections != 200 || len(result.Artifacts) != 3 || result.Distributions[originServiceName].Succeeded == 0 || result.Distributions[metadataServiceName].Succeeded == 0 || result.Distributions[cdnServiceName].Succeeded == 0 || receivedUploads.Load() != 200 || reads.Load() < 3 || resources.Load() == 0 {
		t.Fatalf("bounded measurement: held=%d blockers=%v artifacts=%v distributions=%v uploads=%d reads=%d resources=%d err=%v", result.HeldUploadConnections, result.Blockers, result.Artifacts, result.Distributions, receivedUploads.Load(), reads.Load(), resources.Load(), err)
	}
	if raw, err := os.ReadFile(filepath.Join(c.OutputDirectory, "result.json")); err != nil || !json.Valid(raw) || !strings.Contains(string(raw), "scenarioResults") {
		t.Fatalf("bounded measurement result: valid=%v err=%v", json.Valid(raw), err)
	}
}

func measurementHashForEnvelope(raw []byte) string {
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}
