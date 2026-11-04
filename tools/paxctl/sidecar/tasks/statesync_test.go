package tasks

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type mockHTTPDoer struct {
	responses map[string]*http.Response
}

func (m *mockHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	resp, ok := m.responses[req.URL.String()]
	if !ok {
		return nil, fmt.Errorf("unexpected request: %s", req.URL.String())
	}
	// Read and restore the body so repeated requests to the same URL (the
	// witness reachability probe, then the trust-point query) both succeed.
	body, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewReader(body))
	return &http.Response{StatusCode: resp.StatusCode, Body: io.NopCloser(bytes.NewReader(body))}, nil
}

func generateBlockHash() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%X", b)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// wrapResult wraps inner JSON in the CometBFT JSON-RPC envelope.
func wrapResult(inner string) string {
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":-1,"result":%s}`, inner)
}

func setupPeersInConfig(t *testing.T, homeDir string, peers []string) {
	t.Helper()
	configDir := filepath.Join(homeDir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	content := fmt.Sprintf("[p2p]\npersistent-peers = %q\n", strings.Join(peers, ","))
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(content), 0o644); err != nil {
		t.Fatalf("writing config.toml: %v", err)
	}
}

func TestStateSyncConfigurer_Success(t *testing.T) {
	home := t.TempDir()
	eps, hash, _ := stateSyncFixture(t, home, 10000)
	c := NewStateSyncConfigurer(home, nil)
	if err := c.Configure(context.Background(), StateSyncRequest{RpcServers: eps}); err != nil {
		t.Fatal(err)
	}
	if !markerExists(home, stateSyncMarkerFile) {
		t.Fatal("completion missing")
	}
	ss := readTOML(t, filepath.Join(home, "config", "config.toml"))["statesync"].(map[string]any)
	if ss["enable"] != true || ss["trust-height"] != int64(8000) || ss["trust-hash"] != hash || ss["rpc-servers"] != strings.Join(eps, ",") || ss["use-local-snapshot"] != false {
		t.Fatalf("wrong quorum config: %+v", ss)
	}
}

func TestStateSyncConfigurer_MarkerSkips(t *testing.T) {
	home := t.TempDir()
	if err := writeMarker(home, stateSyncMarkerFile); err != nil {
		t.Fatal(err)
	}
	if err := NewStateSyncConfigurer(home, nil).Configure(context.Background(), StateSyncRequest{}); err == nil {
		t.Fatal("marker bypassed witness verification")
	}
	if exists(filepath.Join(home, "config", "config.toml")) {
		t.Fatal("invalid request wrote configuration")
	}
}

func TestStateSyncConfigurer_NoPeers(t *testing.T) {
	homeDir := t.TempDir()
	setupPeersInConfig(t, homeDir, nil)

	configurer := NewStateSyncConfigurer(homeDir, &mockHTTPDoer{})
	err := configurer.Configure(context.Background(), StateSyncRequest{})
	if err == nil {
		t.Fatal("expected error when no peers in config")
	}
}

func TestStateSyncConfigurer_LowHeight(t *testing.T) {
	home := t.TempDir()
	eps, hash, _ := stateSyncFixture(t, home, 500)
	if err := NewStateSyncConfigurer(home, nil).Configure(context.Background(), StateSyncRequest{RpcServers: eps}); err != nil {
		t.Fatal(err)
	}
	ss := readTOML(t, filepath.Join(home, "config", "config.toml"))["statesync"].(map[string]any)
	if ss["trust-height"] != int64(1) || ss["trust-hash"] != hash || ss["rpc-servers"] != strings.Join(eps, ",") {
		t.Fatalf("low-height quorum: %+v", ss)
	}
}

func TestStateSyncConfigurer_InvalidBlockHash(t *testing.T) {
	for _, tc := range []struct{ name, hash, want string }{{"empty", "", "empty block hash"}, {"short", "ABCDEF", "unexpected block hash length"}, {"long", strings.Repeat("A", 66), "unexpected block hash length"}} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("SEI_CHAIN_ID", repairChainID)
			setupConfigFile(t, home, "mode = \"full\"\n")
			first := repairWitness(t, repairChainID, tc.hash, 10000)
			second := repairWitness(t, repairChainID, tc.hash, 10000)
			err := NewStateSyncConfigurer(home, nil).Configure(context.Background(), StateSyncRequest{RpcServers: []string{strings.TrimPrefix(first.URL, "http://"), strings.TrimPrefix(second.URL, "http://")}})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %s: %v", tc.want, err)
			}
		})
	}
}

func TestExtractRPCHosts(t *testing.T) {
	tests := []struct {
		name     string
		peers    []string
		maxHosts int
		want     []string
	}{
		{
			name:     "standard peers",
			peers:    []string{"nodeId1@1.2.3.4:26656", "nodeId2@5.6.7.8:26656"},
			maxHosts: 2,
			want:     []string{"1.2.3.4", "5.6.7.8"},
		},
		{
			name:     "max hosts limits output",
			peers:    []string{"a@1.1.1.1:26656", "b@2.2.2.2:26656", "c@3.3.3.3:26656"},
			maxHosts: 2,
			want:     []string{"1.1.1.1", "2.2.2.2"},
		},
		{
			name:     "invalid format skipped",
			peers:    []string{"no-at-sign", "valid@10.0.0.1:26656"},
			maxHosts: 2,
			want:     []string{"10.0.0.1"},
		},
		{
			name:     "empty peers",
			peers:    []string{},
			maxHosts: 2,
			want:     nil,
		},
		{
			name:     "host without port",
			peers:    []string{"nodeId@myhost"},
			maxHosts: 1,
			want:     []string{"myhost"},
		},
		{
			name:     "IPv6-style host with port",
			peers:    []string{"nodeId@[::1]:26656"},
			maxHosts: 1,
			want:     []string{"[::1]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractRPCHosts(tt.peers, tt.maxHosts)
			if len(got) != len(tt.want) {
				t.Fatalf("expected %d hosts, got %d: %v", len(tt.want), len(got), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("host[%d]: expected %q, got %q", i, tt.want[i], got[i])
				}
			}
		})
	}
}

func TestStateSyncConfigurer_Handler(t *testing.T) {
	home := t.TempDir()
	eps, _, _ := stateSyncFixture(t, home, 5000)
	if _, err := NewStateSyncConfigurer(home, nil).Handler()(context.Background(), map[string]any{"rpcServers": eps}); err != nil {
		t.Fatal(err)
	}
	ss := readTOML(t, filepath.Join(home, "config", "config.toml"))["statesync"].(map[string]any)
	if ss["trust-height"] != int64(3000) {
		t.Fatal(ss)
	}
}

func TestStateSyncConfigurer_NetworkWithBackfill(t *testing.T) {
	home := t.TempDir()
	eps, hash, _ := stateSyncFixture(t, home, 10000)
	c := NewStateSyncConfigurer(home, nil)
	if err := c.Configure(context.Background(), StateSyncRequest{RpcServers: eps, TrustPeriod: "168h0m0s", BackfillBlocks: 6000}); err != nil {
		t.Fatal(err)
	}
	ss := readTOML(t, filepath.Join(home, "config", "config.toml"))["statesync"].(map[string]any)
	if ss["trust-period"] != "168h0m0s" || ss["backfill-blocks"] != int64(6000) || ss["use-local-snapshot"] != false || ss["trust-hash"] != hash {
		t.Fatal(ss)
	}
}

func TestStateSyncConfigurer_LocalSnapshot(t *testing.T) {
	home := t.TempDir()
	eps, hash, _ := stateSyncFixture(t, home, 198030000)
	if err := os.MkdirAll(filepath.Join(home, "data", "snapshots", "198030000", "1"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := NewStateSyncConfigurer(home, nil).Configure(context.Background(), StateSyncRequest{RpcServers: eps, UseLocalSnapshot: true, TrustPeriod: "9999h0m0s"}); err != nil {
		t.Fatal(err)
	}
	ss := readTOML(t, filepath.Join(home, "config", "config.toml"))["statesync"].(map[string]any)
	if ss["trust-height"] != int64(198030000) || ss["trust-hash"] != hash || ss["use-local-snapshot"] != true || ss["trust-period"] != "9999h0m0s" || ss["enable"] != true {
		t.Fatal(ss)
	}
}

func TestStateSyncConfigurer_LocalSnapshotNoDir(t *testing.T) {
	home := t.TempDir()
	eps, _, _ := stateSyncFixture(t, home, 10000)
	err := NewStateSyncConfigurer(home, nil).Configure(context.Background(), StateSyncRequest{RpcServers: eps, UseLocalSnapshot: true})
	if err == nil || !strings.Contains(err.Error(), "discovering local snapshot height") {
		t.Fatalf("missing snapshot directory: %v", err)
	}
}

func TestDiscoverLocalSnapshotHeight(t *testing.T) {
	homeDir := t.TempDir()

	snapshotBase := filepath.Join(homeDir, "data", "snapshots")
	for _, dir := range []string{"198030000/1", "198020000/1", "notaheight"} {
		if err := os.MkdirAll(filepath.Join(snapshotBase, dir), 0o755); err != nil {
			t.Fatalf("creating dir: %v", err)
		}
	}

	h, err := discoverLocalSnapshotHeight(homeDir)
	if err != nil {
		t.Fatalf("discoverLocalSnapshotHeight failed: %v", err)
	}
	if h != 198030000 {
		t.Errorf("expected height 198030000, got %d", h)
	}
}

func TestDiscoverLocalSnapshotHeight_Empty(t *testing.T) {
	homeDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(homeDir, "data", "snapshots"), 0o755); err != nil {
		t.Fatalf("creating dir: %v", err)
	}

	_, err := discoverLocalSnapshotHeight(homeDir)
	if err == nil {
		t.Fatal("expected error for empty snapshots directory")
	}
}

func TestReadPeersFromConfig(t *testing.T) {
	homeDir := t.TempDir()
	setupPeersInConfig(t, homeDir, []string{"abc@1.2.3.4:26656", "def@5.6.7.8:26656"})

	peers, err := readPeersFromConfig(homeDir)
	if err != nil {
		t.Fatalf("readPeersFromConfig failed: %v", err)
	}
	if len(peers) != 2 {
		t.Fatalf("expected 2 peers, got %d", len(peers))
	}
	if peers[0] != "abc@1.2.3.4:26656" {
		t.Errorf("peer[0] = %q, want abc@1.2.3.4:26656", peers[0])
	}
}

func TestReadPeersFromConfig_Empty(t *testing.T) {
	homeDir := t.TempDir()
	setupPeersInConfig(t, homeDir, nil)

	peers, err := readPeersFromConfig(homeDir)
	if err != nil {
		t.Fatalf("readPeersFromConfig failed: %v", err)
	}
	if len(peers) != 0 {
		t.Fatalf("expected 0 peers, got %d", len(peers))
	}
}

func TestReadPeersFromConfig_NoConfigFile(t *testing.T) {
	homeDir := t.TempDir()
	peers, err := readPeersFromConfig(homeDir)
	if err != nil {
		t.Fatalf("readPeersFromConfig failed: %v", err)
	}
	if len(peers) != 0 {
		t.Fatalf("expected 0 peers for missing config, got %d", len(peers))
	}
}

// Caller-provided RpcServers are used verbatim, independent of persistent-peers
// (here there are none).
func TestStateSyncConfigurer_ExplicitRpcServers(t *testing.T) {
	home := t.TempDir()
	eps, hash, _ := stateSyncFixture(t, home, 10000)
	c := NewStateSyncConfigurer(home, nil)
	if err := c.Configure(context.Background(), StateSyncRequest{RpcServers: eps}); err != nil {
		t.Fatal(err)
	}
	ss := readTOML(t, filepath.Join(home, "config", "config.toml"))["statesync"].(map[string]any)
	if ss["rpc-servers"] != strings.Join(eps, ",") || ss["trust-height"] != int64(8000) || ss["trust-hash"] != hash {
		t.Fatal(ss)
	}
}

// The regression case: a peer-derived witness whose host serves P2P but not RPC
// (an external NLB hostname). The reachable witness is kept; the dead one is
// dropped instead of being written and crashlooping seid.
func TestStateSyncConfigurer_DropsUnreachableWitness(t *testing.T) {
	home := t.TempDir()
	eps, _, servers := stateSyncFixture(t, home, 10000)
	servers[1].Close()
	if err := NewStateSyncConfigurer(home, nil).Configure(context.Background(), StateSyncRequest{RpcServers: eps}); err == nil {
		t.Fatal("single authority accepted")
	}
	if markerExists(home, stateSyncMarkerFile) {
		t.Fatal("under-witnessed request completed")
	}
	ss := readTOML(t, filepath.Join(home, "config", "config.toml"))
	if _, ok := ss["statesync"]; ok {
		t.Fatal("under-witnessed request wrote config")
	}
}

// The production-regression ordering: the first candidate is unreachable (a
// P2P-only NLB host), so the trust query must fall through to the reachable
// second one — reachable[0] is the post-probe slice, not the raw peer list.
func TestStateSyncConfigurer_PrimaryUnreachableFallsThrough(t *testing.T) {
	home := t.TempDir()
	eps, _, servers := stateSyncFixture(t, home, 10000)
	servers[0].Close()
	if err := NewStateSyncConfigurer(home, nil).Configure(context.Background(), StateSyncRequest{RpcServers: eps}); err == nil {
		t.Fatal("single authority accepted")
	}
	if markerExists(home, stateSyncMarkerFile) {
		t.Fatal("under-witnessed request completed")
	}
	ss := readTOML(t, filepath.Join(home, "config", "config.toml"))
	if _, ok := ss["statesync"]; ok {
		t.Fatal("under-witnessed request wrote config")
	}
}

func TestWitnessScheme(t *testing.T) {
	cases := []struct {
		name, endpoint, want string
	}{
		{"tls gateway on 443", "archive-0-rpc.arctic-1.platform.sei.io:443", "https"},
		{"in-cluster rpc on 26657", "syncer-0-internal.arctic-1.svc.cluster.local:26657", "http"},
		{"bare ip rpc", "1.2.3.4:26657", "http"},
		{"no port defaults to http", "some-host", "http"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := witnessScheme(tc.endpoint); got != tc.want {
				t.Errorf("witnessScheme(%q) = %q, want %q", tc.endpoint, got, tc.want)
			}
		})
	}
}

// Regression guard for the state-sync witness scheme/port mismatch that blocked
// every new K8s state-syncing node: a canonical syncer resolved to the public
// Istio HTTPRoute hostname on :443 must be probed and queried over https, not
// the previously-hardcoded http (which EOFed against the TLS listener). The mock
// only answers the https URL — an http probe would fall through as an
// "unexpected request" and the configure would fail.
func TestStateSyncConfigurer_TLSWitnessUsesHTTPS(t *testing.T) {
	home := t.TempDir()
	eps, hash, servers := stateSyncFixture(t, home, 10000)
	tlsServer := httptest.NewUnstartedServer(servers[0].Config.Handler)
	tlsServer.Listener.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:443")
	if err != nil {
		t.Fatal(err)
	}
	tlsServer.Listener = listener
	tlsServer.StartTLS()
	defer tlsServer.Close()
	eps[0] = "127.0.0.1:443"
	if err := NewStateSyncConfigurer(home, tlsServer.Client()).Configure(context.Background(), StateSyncRequest{RpcServers: eps}); err != nil {
		t.Fatal(err)
	}
	ss := readTOML(t, filepath.Join(home, "config", "config.toml"))["statesync"].(map[string]any)
	if ss["rpc-servers"] != strings.Join(eps, ",") || ss["trust-height"] != int64(8000) || ss["trust-hash"] != hash {
		t.Fatal(ss)
	}
}

// The in-cluster plaintext path (a syncer's internal Service on :26657) must
// stay on http — the forward-compatible counterpart to the TLS guard above.
func TestStateSyncConfigurer_InternalWitnessUsesHTTP(t *testing.T) {
	home := t.TempDir()
	eps, hash, _ := stateSyncFixture(t, home, 10000)
	c := NewStateSyncConfigurer(home, nil)
	if err := c.Configure(context.Background(), StateSyncRequest{RpcServers: eps}); err != nil {
		t.Fatal(err)
	}
	ss := readTOML(t, filepath.Join(home, "config", "config.toml"))["statesync"].(map[string]any)
	if ss["trust-height"] != int64(8000) || ss["trust-hash"] != hash {
		t.Fatal(ss)
	}
}

// When no candidate witness is reachable, fail at configure time with a clear
// error rather than writing a config that makes seid exit on "no witnesses".
func TestStateSyncConfigurer_NoReachableWitness(t *testing.T) {
	home := t.TempDir()
	eps, _, servers := stateSyncFixture(t, home, 10000)
	for _, server := range servers {
		server.Close()
	}
	err := NewStateSyncConfigurer(home, nil).Configure(context.Background(), StateSyncRequest{RpcServers: eps})
	if err == nil || !strings.Contains(err.Error(), "two distinct reachable RPC witnesses") {
		t.Fatalf("unreachable quorum: %v", err)
	}
}

func stateSyncFixture(t *testing.T, home string, height int64) ([]string, string, []*httptest.Server) {
	t.Helper()
	t.Setenv("SEI_CHAIN_ID", repairChainID)
	setupConfigFile(t, home, "mode = \"full\"\n")
	hash := generateBlockHash()
	first := repairWitness(t, repairChainID, hash, height)
	second := repairWitness(t, repairChainID, hash, height)
	return []string{strings.TrimPrefix(first.URL, "http://"), strings.TrimPrefix(second.URL, "http://")}, hash, []*httptest.Server{first, second}
}
