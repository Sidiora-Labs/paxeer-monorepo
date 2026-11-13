package streams

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMeasurementArtifactReadsBoundedDigestAndSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "authority.json")
	raw := []byte(`{"runId":"run_1","candidate":"candidate_1"}`)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	artifact := MeasurementArtifact{Path: path, SHA256: measurementSHA(raw)}
	var authority struct {
		RunID     string `json:"runId"`
		Candidate string `json:"candidate"`
	}
	if err := measurementRead(artifact, &authority); err != nil || authority.RunID != "run_1" || authority.Candidate != "candidate_1" {
		t.Fatalf("authority = %+v, %v", authority, err)
	}
	artifact.SHA256 = measurementSHA([]byte("other"))
	if err := measurementRead(artifact, &authority); err == nil || err.Error() != "measurement artifact digest mismatch" {
		t.Fatalf("changed digest = %v", err)
	}
	for _, value := range []string{`{"runId":"run_1","runId":"run_2"}`, `{"runId":"run_1","unknown":1}`, `{"runId":"run_1"} {}`} {
		if err := os.WriteFile(path, []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
		artifact.SHA256 = measurementSHA([]byte(value))
		if err := measurementRead(artifact, &authority); err == nil {
			t.Fatalf("ambiguous or unknown artifact accepted: %s", value)
		}
	}
	if _, err := LoadMediaMeasurement("relative.json", strings.Repeat("0", 64)); err == nil {
		t.Fatal("relative measurement authority accepted")
	}
	if measurementHashValid(strings.Repeat("A", 64)) || !measurementHashValid(strings.Repeat("a", 64)) {
		t.Fatal("artifact hash validation changed")
	}
}

func TestMeasurementArtifactAndFetchEnforceInputBounds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing-authority.json")
	var authority struct {
		RunID string `json:"runId"`
	}
	if err := measurementRead(MeasurementArtifact{Path: path, SHA256: measurementSHA([]byte(`{}`))}, &authority); err == nil || err.Error() != "measurement artifact unavailable" {
		t.Fatalf("missing measurement authority = %v", err)
	}
	large := bytes.Repeat([]byte("x"), 1048577)
	if err := os.WriteFile(path, large, 0600); err != nil {
		t.Fatal(err)
	}
	if err := measurementRead(MeasurementArtifact{Path: path, SHA256: measurementSHA(large)}, &authority); err == nil || err.Error() != "measurement artifact digest mismatch" {
		t.Fatalf("oversized measurement authority = %v", err)
	}
	if mediaPercentile(nil, .95) != 0 {
		t.Fatal("empty latency sample had a percentile")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/content-type":
			_, _ = w.Write([]byte(`{}`))
		case "/heartbeat":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"unexpected":true}`))
		case "/large":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(bytes.Repeat([]byte("x"), 1048577))
		default:
			t.Errorf("unexpected measurement request %s", r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	for _, tc := range []struct{ path, category string }{
		{"/content-type", statusLive},
		{"/heartbeat", "viewer_heartbeat"},
		{"/large", detailCategory},
	} {
		sample, _ := mediaFetch(t.Context(), server.Client(), "run_1", tc.category, server.URL+tc.path, http.MethodGet, "", nil, time.Now(), nil)
		if sample.Status != http.StatusOK || sample.Reason != "transport_or_payload_failure" {
			t.Fatalf("unbound measurement payload = %+v", sample)
		}
	}
}

func TestMeasurementFetchCapturesOriginAndCDNPayloads(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Request-ID") == "" {
			t.Error("request correlation missing")
		}
		switch r.URL.Path {
		case "/origin":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"streams":[]}`))
		case "/private":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"streamKey":"private"}`))
		case "/segment":
			if r.Header.Get("Livepeer-Access-Key") != "viewer-token" {
				t.Error("viewer access key missing")
			}
			w.Header().Set("Content-Type", "video/mp2t")
			_, _ = w.Write([]byte("segment-bytes"))
		case "/unavailable":
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			t.Error("unexpected request", r.URL.Path)
		}
	}))
	defer server.Close()
	ctx := context.Background()
	start := time.Now()
	good, body := mediaFetch(ctx, server.Client(), "run_1", statusLive, server.URL+"/origin", http.MethodGet, "", nil, start, nil)
	if good.Reason != "ok" || good.Status != 200 || string(body) != `{"streams":[]}` || good.BodySHA256 != measurementSHA(body) || good.LatencyNS < 0 {
		t.Fatalf("origin sample = %+v, %q", good, body)
	}
	private, _ := mediaFetch(ctx, server.Client(), "run_1", detailCategory, server.URL+"/private", http.MethodGet, "", nil, start, nil)
	if private.Reason != "transport_or_payload_failure" {
		t.Fatalf("credential exposure was accepted: %+v", private)
	}
	cdn, body := mediaFetch(ctx, server.Client(), "run_1", cdnSegmentCategory, server.URL+"/segment", http.MethodGet, "session_1", nil, start, map[string]string{"Livepeer-Access-Key": "viewer-token"})
	if cdn.Reason != "ok" || cdn.Bytes != int64(len("segment-bytes")) || len(body) != 0 || cdn.BodySHA256 != measurementSHA([]byte("segment-bytes")) {
		t.Fatalf("CDN sample = %+v, %q", cdn, body)
	}
	unavailable, _ := mediaFetch(ctx, server.Client(), "run_1", poolCategory, server.URL+"/unavailable", http.MethodGet, "", nil, start, nil)
	if unavailable.Reason != "http_failure" || unavailable.Status != 503 {
		t.Fatalf("unavailable origin = %+v", unavailable)
	}
	var output bytes.Buffer
	capture := &mediaCapture{encoder: json.NewEncoder(&output)}
	for _, sample := range []MediaRequestSample{good, private, cdn, unavailable} {
		capture.append(sample)
	}
	if capture.requests != 4 || capture.failures != 2 || capture.cdnBytes != cdn.Bytes || len(capture.latencies) != 3 || strings.Count(output.String(), "\n") != 4 {
		t.Fatalf("capture = %+v, %q", capture, output.String())
	}
}

func TestMeasurementConfigurationBindsSourcesAndAuthorities(t *testing.T) {
	root, err := filepath.Abs("../../../../../")
	if err != nil {
		t.Fatal(err)
	}
	artifactRoot := t.TempDir()
	writeArtifact := func(name string, value any) MeasurementArtifact {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(artifactRoot, name)
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		return MeasurementArtifact{Path: path, SHA256: measurementSHA(raw)}
	}
	const streamID = "11111111-1111-4111-8111-111111111111"
	const segment = "https://livepeercdn.studio/hls/playback_1/segment.ts"
	const region = "region_1"
	binding := measurementSHA([]byte("candidate"))
	sources := map[string]string{}
	for _, name := range []string{
		"services/launchpad/livestream/internal/streams/production_measurement.go",
		"services/launchpad/livestream/internal/streams/lifecycle.go",
		"services/launchpad/livestream/internal/streams/credentials.go",
		"services/launchpad/livestream/internal/streams/provision_worker.go",
		"services/launchpad/livestream/internal/streams/reconcile_worker.go",
		"services/launchpad/livestream/internal/streams/handlers.go",
		"services/launchpad/livestream/internal/livepeer/client.go",
		"services/launchpad/livestream/internal/config/config.go",
		"apps/launchpad/web/src/widgets/trading/live-stream.tsx",
		"apps/launchpad/web/src/core/clients/livestream-api.ts",
	} {
		path := filepath.Join(root, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		sources[name] = measurementSHA(raw)
	}
	expires := time.Now().Add(2 * time.Hour)
	authority := MediaMeasurementAuthority{RunID: "run_1", Candidate: binding, Interface: binding, Topology: binding, Generator: binding, WorkloadVersion: "kindle-500k-dau-v1", RootMixSHA256: binding, RootLedgerSHA256: binding, Lane: "L40", LaneRPS: 668, ApprovalReference: "approved", ExpiresAt: expires}
	quota := MediaQuotaAuthority{AccountID: "account_1", ConcurrentViewers: 20000, ConcurrentBroadcasters: 1, ApprovalReference: "approved", ExpiresAt: expires}
	license := MediaLicenseAuthority{ContentID: "content_1", StreamID: streamID, ProviderID: "provider_1", PlaybackID: "playback_1", RightsReference: "approved", Regions: []string{region}, SegmentURLs: []string{segment}, SegmentHashes: map[string]string{segment: binding}, ValidUntil: expires}
	budget := repairM1001Budget()
	budget.VendorUserID = quota.AccountID
	budget.PeakDurationSeconds = 600
	profile := MediaNetworkProfile{Region: region, AuthorityReference: "approved", DownstreamBitsPerSecond: 40000000000, LatencyMS: 10}
	c := MediaMeasurementConfig{
		Version: 1, RunID: authority.RunID, Candidate: binding, Interface: binding, Topology: binding, Generator: binding,
		SourceRoot: root, SourceFiles: sources, Authority: writeArtifact("authority.json", authority), OutputRoot: t.TempDir(), Origin: "https://livepeer.studio", StreamID: streamID,
		ProviderURL: "https://livepeer.studio/api", ProviderKeyEnvironment: "L40_TEST_LIVEPEER_KEY", Pool: "0x2222222222222222222222222222222222222222", Region: region,
		DurationSeconds: 600, OriginRPS: 1, HeartbeatRPS: 667, MaxFlights: 1, ReadWeights: map[string]int{statusLive: 1, poolCategory: 1, detailCategory: 1},
		PlaybackSessions: 20000, SegmentIntervalSeconds: 30, SegmentURLs: []string{segment}, Quota: writeArtifact("quota.json", quota), License: writeArtifact("license.json", license), Budget: writeArtifact("budget.json", budget), NetworkProfile: writeArtifact("network.json", profile),
	}
	actualLicense, actualBudget, err := c.validate()
	if err != nil || actualLicense.PlaybackID != license.PlaybackID || actualBudget.VendorUserID != quota.AccountID {
		t.Fatalf("measurement authorities = %+v, %+v, %v", actualLicense, actualBudget, err)
	}
	for name, mutate := range map[string]func(*MediaMeasurementConfig){
		"duration":     func(c *MediaMeasurementConfig) { c.DurationSeconds = 599 },
		"weight bound": func(c *MediaMeasurementConfig) { c.ReadWeights[statusLive] = 10001 },
		"weight total": func(c *MediaMeasurementConfig) {
			c.ReadWeights[statusLive], c.ReadWeights[poolCategory], c.ReadWeights[detailCategory] = 4000, 4000, 4000
		},
		"inventory": func(c *MediaMeasurementConfig) {
			delete(c.SourceFiles, "services/launchpad/livestream/internal/streams/handlers.go")
		},
		"source path": func(c *MediaMeasurementConfig) { c.SourceFiles["../escape.go"] = binding },
		"origin":      func(c *MediaMeasurementConfig) { c.Origin = "http://livepeer.studio" },
		"provider":    func(c *MediaMeasurementConfig) { c.ProviderURL = "https://livepeercdn.studio/api" },
		"weights":     func(c *MediaMeasurementConfig) { c.ReadWeights[statusLive] = 0 },
		"binding":     func(c *MediaMeasurementConfig) { c.Candidate = "invalid" },
		"source": func(c *MediaMeasurementConfig) {
			c.SourceFiles["services/launchpad/livestream/internal/streams/lifecycle.go"] = binding
		},
		"authority": func(c *MediaMeasurementConfig) {
			authority.LaneRPS = 1
			c.Authority = writeArtifact("authority-bad.json", authority)
		},
		"quota": func(c *MediaMeasurementConfig) {
			quota.ConcurrentViewers = 1
			c.Quota = writeArtifact("quota-bad.json", quota)
		},
		"license": func(c *MediaMeasurementConfig) {
			license.Regions = []string{"region_2"}
			c.License = writeArtifact("license-bad.json", license)
		},
		"license binding": func(c *MediaMeasurementConfig) {
			changed := license
			changed.PlaybackID = "other_playback"
			changed.Regions = []string{region}
			c.License = writeArtifact("license-binding-bad.json", changed)
		},
		"cdn binding": func(c *MediaMeasurementConfig) {
			c.SegmentURLs = []string{"https://livepeercdn.studio/hls/playback_1/other.ts"}
		},
		"budget": func(c *MediaMeasurementConfig) {
			budget.VendorUserID = "other_account"
			c.Budget = writeArtifact("budget-bad.json", budget)
		},
		"network": func(c *MediaMeasurementConfig) {
			profile.DownstreamBitsPerSecond = 1
			c.NetworkProfile = writeArtifact("network-bad.json", profile)
		},
	} {
		invalid := c
		invalid.ReadWeights = map[string]int{statusLive: 1, poolCategory: 1, detailCategory: 1}
		invalid.SourceFiles = make(map[string]string, len(c.SourceFiles))
		for path, hash := range c.SourceFiles {
			invalid.SourceFiles[path] = hash
		}
		mutate(&invalid)
		if _, _, err := invalid.validate(); err == nil {
			t.Fatalf("%s mismatch accepted", name)
		}
	}
	t.Setenv(c.ProviderKeyEnvironment, "")
	summary, err := RunMediaMeasurement(context.Background(), c)
	if err == nil || err.Error() != "actual provider credential unavailable" || summary.Scope != "isolated_component" || len(summary.Artifacts) != 5 {
		t.Fatalf("measurement authority without provider credential = %+v, %v", summary, err)
	}
}
