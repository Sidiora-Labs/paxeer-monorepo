package streams

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/livestream/internal/livepeer"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
)

func measurementRunConfig(t *testing.T) MediaMeasurementConfig {
	t.Helper()
	root, err := filepath.Abs("../../../../../")
	if err != nil {
		t.Fatal(err)
	}
	artifactRoot := t.TempDir()
	write := func(name string, value any) MeasurementArtifact {
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
	binding := measurementSHA([]byte("measurement candidate"))
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
		raw, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		sources[name] = measurementSHA(raw)
	}
	expires := time.Now().Add(2 * time.Hour)
	authority := MediaMeasurementAuthority{RunID: "run_coverage", Candidate: binding, Interface: binding, Topology: binding, Generator: binding, WorkloadVersion: "kindle-500k-dau-v1", RootMixSHA256: binding, RootLedgerSHA256: binding, Lane: "L40", LaneRPS: 668, ApprovalReference: "approved", ExpiresAt: expires}
	quota := MediaQuotaAuthority{AccountID: "account_1", ConcurrentViewers: 20000, ConcurrentBroadcasters: 1, ApprovalReference: "approved", ExpiresAt: expires}
	license := MediaLicenseAuthority{ContentID: "content_1", StreamID: streamID, ProviderID: "provider_1", PlaybackID: "playback_1", RightsReference: "approved", Regions: []string{"region_1"}, SegmentURLs: []string{segment}, SegmentHashes: map[string]string{segment: measurementSHA([]byte("segment-bytes"))}, ValidUntil: expires}
	budget := repairM1001Budget()
	budget.VendorUserID = quota.AccountID
	budget.PeakDurationSeconds = 600
	profile := MediaNetworkProfile{Region: "region_1", AuthorityReference: "approved", DownstreamBitsPerSecond: 40000000000, LatencyMS: 10}
	return MediaMeasurementConfig{
		Version: 1, RunID: authority.RunID, Candidate: binding, Interface: binding, Topology: binding, Generator: binding,
		SourceRoot: root, SourceFiles: sources, Authority: write("authority.json", authority), OutputRoot: t.TempDir(), Origin: "https://livepeer.studio", StreamID: streamID,
		ProviderURL: "https://livepeer.studio/api", ProviderKeyEnvironment: "L40_TEST_LIVEPEER_KEY", Pool: "0x2222222222222222222222222222222222222222", Region: "region_1",
		DurationSeconds: 600, OriginRPS: 1, HeartbeatRPS: 667, MaxFlights: 128, ReadWeights: map[string]int{statusLive: 1, poolCategory: 1, detailCategory: 1},
		PlaybackSessions: 20000, SegmentIntervalSeconds: 30, SegmentURLs: []string{segment}, Quota: write("quota.json", quota), License: write("license.json", license), Budget: write("budget.json", budget), NetworkProfile: write("network.json", profile),
	}
}

func TestMeasurementRunCapturesRealTLSArrivalAndCancellation(t *testing.T) {
	c := measurementRunConfig(t)
	pki := testpki.New(t)
	providerCert := pki.Leaf("provider", "livepeer.studio")
	cdnCert := pki.Leaf("cdn", "livepeercdn.studio")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	var issued atomic.Int64
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/data/usage/query":
			_, _ = w.Write([]byte(`{"UserID":"account_1","DeliveryUsageMins":1,"TotalUsageMins":1,"StorageUsageMins":1}`))
		case r.URL.Path == "/api/user/me":
			_, _ = w.Write([]byte(`{"id":"account_1"}`))
		case r.URL.Path == "/api/stream/provider_1":
			_, _ = fmt.Fprintf(w, `{"id":"provider_1","name":%q,"playbackId":"playback_1","isActive":true,"playbackPolicy":{"type":"webhook","webhookContext":{"operationId":%q},"refreshInterval":1},"profiles":[{"name":"720p","bitrate":2000000},{"name":"480p","bitrate":1000000},{"name":"360p","bitrate":500000}]}`, livepeer.CorrelationName(c.StreamID), c.StreamID)
		case r.Method == http.MethodPost && r.URL.Path == "/v2/streams/"+c.StreamID+"/viewer-session":
			n := issued.Add(1)
			_, _ = fmt.Fprintf(w, `{"streamId":%q,"sessionId":%q,"token":%q,"expiresAt":%q}`, c.StreamID, fmt.Sprintf("00000000-0000-4000-8000-%012x", n), fmt.Sprintf("viewer_%d", n), time.Now().Add(2*time.Hour).UTC().Format(time.RFC3339Nano))
			if n == int64(c.PlaybackSessions) {
				time.AfterFunc(2*time.Second, cancel)
			}
		case strings.HasPrefix(r.URL.Path, "/v2/streams/"):
			_, _ = w.Write([]byte(`{"streams":[]}`))
		case r.URL.Path == "/hls/playback_1/segment.ts":
			w.Header().Set("Content-Type", "video/mp2t")
			_, _ = w.Write([]byte("segment-bytes"))
		default:
			t.Errorf("unexpected measurement path: %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{providerCert.Pair, cdnCert.Pair}}
	server.StartTLS()
	defer server.Close()
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: pki.Roots()},
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
		},
	}
	previous := http.DefaultTransport
	http.DefaultTransport = transport
	defer func() { http.DefaultTransport = previous; transport.CloseIdleConnections() }()
	t.Setenv(c.ProviderKeyEnvironment, "fixture-provider-key")
	summary, err := RunMediaMeasurement(ctx, c)
	if err == nil || summary.PlaybackSessions != c.PlaybackSessions || issued.Load() != int64(c.PlaybackSessions) || summary.Requests <= c.PlaybackSessions || summary.Artifacts["http-arrivals.ndjson"] == "" {
		t.Fatalf("captured canceled measurement = sessions:%d issued:%d requests:%d artifacts:%v error:%v", summary.PlaybackSessions, issued.Load(), summary.Requests, summary.Artifacts, err)
	}
}

func TestMeasurementRunRejectsChangedProviderIdentityBeforeLoad(t *testing.T) {
	c := measurementRunConfig(t)
	pki := testpki.New(t)
	providerCert := pki.Leaf("provider", "livepeer.studio")
	var stage atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/data/usage/query":
			_, _ = w.Write([]byte(`{"UserID":"account_1","DeliveryUsageMins":1,"TotalUsageMins":1,"StorageUsageMins":1}`))
		case "/api/user/me":
			if stage.Load() == 1 {
				_, _ = w.Write([]byte(`{"id":"other_account"}`))
			} else {
				_, _ = w.Write([]byte(`{"id":"account_1"}`))
			}
		case "/api/stream/provider_1":
			name := livepeer.CorrelationName(c.StreamID)
			if stage.Load() == 2 {
				name = "other_stream"
			}
			profiles := `[{"name":"720p","bitrate":2000000},{"name":"480p","bitrate":1000000},{"name":"360p","bitrate":500000}]`
			if stage.Load() == 3 {
				profiles = `[{"name":"720p","bitrate":2000000},{"name":"720p","bitrate":1000000},{"name":"360p","bitrate":500000}]`
			}
			if stage.Load() == 4 {
				profiles = `[{"name":"720p","bitrate":1000000},{"name":"480p","bitrate":1000000},{"name":"360p","bitrate":500000}]`
			}
			_, _ = fmt.Fprintf(w, `{"id":"provider_1","name":%q,"playbackId":"playback_1","isActive":true,"playbackPolicy":{"type":"webhook","webhookContext":{"operationId":%q},"refreshInterval":1},"profiles":%s}`, name, c.StreamID, profiles)
		default:
			t.Errorf("unexpected provider preflight request %s", r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{providerCert.Pair}}
	server.StartTLS()
	defer server.Close()
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: pki.Roots()}, DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}}
	previous := http.DefaultTransport
	http.DefaultTransport = transport
	defer func() { http.DefaultTransport = previous; transport.CloseIdleConnections() }()
	t.Setenv(c.ProviderKeyEnvironment, "fixture-provider-key")
	for _, tc := range []struct {
		stage int32
		want  string
	}{
		{1, "actual provider account usage unavailable"},
		{2, "actual licensed provider stream identity mismatch"},
		{3, "provider rendition identity duplicated"},
		{4, "actual provider rendition ladder differs from required source profiles"},
		{5, "exclusive measurement output directory required"},
	} {
		stage.Store(tc.stage)
		if tc.stage == 5 {
			if err := os.Mkdir(filepath.Join(c.OutputRoot, c.RunID), 0700); err != nil {
				t.Fatal(err)
			}
		}
		summary, err := RunMediaMeasurement(t.Context(), c)
		if err == nil || err.Error() != tc.want || summary.RunID != c.RunID || summary.PlaybackSessions != 0 {
			t.Fatalf("provider stage %d = %+v, %v", tc.stage, summary, err)
		}
	}
}
