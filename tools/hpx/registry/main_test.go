package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var testNodeKey = ed25519.NewKeyFromSeed(bytes.Repeat([]byte{1}, ed25519.SeedSize))
var otherNodeKey = ed25519.NewKeyFromSeed(bytes.Repeat([]byte{2}, ed25519.SeedSize))
var testNodeID = nodeKeyID(testNodeKey)
var otherNodeID = nodeKeyID(otherNodeKey)

func nodeKeyID(key ed25519.PrivateKey) string {
	sum := sha256.Sum256(key.Public().(ed25519.PublicKey))
	return hex.EncodeToString(sum[:20])
}

func signedBody(s *server, remote string, headers map[string]string, node Node, key ed25519.PrivateKey) (string, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(http.MethodGet, "/api/register/challenge", nil)
	req.RemoteAddr = remote
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	s.handleRegistrationChallenge(rec, req)
	if rec.Code != http.StatusOK {
		return "", rec
	}
	var challenge registrationChallenge
	if err := json.Unmarshal(rec.Body.Bytes(), &challenge); err != nil {
		panic(err)
	}
	node.IP = challenge.Source
	if node.P2PPort == 0 {
		node.P2PPort = 26656
	}
	node.Moniker, node.Version = strings.TrimSpace(node.Moniker), strings.TrimSpace(node.Version)
	if node.Type != "validator" {
		node.Type = "fullnode"
	}
	proof := signedRegistration{Node: node, Challenge: challenge.Nonce, PublicKey: key.Public().(ed25519.PublicKey)}
	proof.Signature = ed25519.Sign(key, registrationMessage(challenge.ChainID, challenge.Nonce, node))
	body, err := json.Marshal(proof)
	if err != nil {
		panic(err)
	}
	return string(body), nil
}

func newTestServer(t *testing.T, dir string) *server {
	t.Helper()
	reg, err := openRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	return &server{cfg: config{ChainID: "hyperpax_125-1", RegisterTok: "operator-token"}, reg: reg}
}

func register(s *server, remote string, headers map[string]string, body string) *httptest.ResponseRecorder {
	var node Node
	if json.Unmarshal([]byte(body), &node) == nil && (node.NodeID == testNodeID || node.NodeID == otherNodeID) {
		key := testNodeKey
		if node.NodeID == otherNodeID {
			key = otherNodeKey
		}
		signed, failure := signedBody(s, remote, headers, node, key)
		if failure != nil {
			return failure
		}
		body = signed
	}
	return postRegistration(s, remote, headers, body)
}

func postRegistration(s *server, remote string, headers map[string]string, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/register", strings.NewReader(body))
	req.RemoteAddr = remote
	req.Header.Set("X-HPX-Token", "operator-token")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	s.handleRegister(rec, req)
	return rec
}

func registered(t *testing.T, s *server) []*Node {
	t.Helper()
	return s.reg.list()
}

func TestRegisterRecordsProxyObservedAddress(t *testing.T) {
	s := newTestServer(t, t.TempDir())
	body := `{"node_id":"` + testNodeID + `","ip":"8.8.8.8","p2p_port":26656,"moniker":"hpx-node","type":"fullnode"}`
	rec := register(s, "127.0.0.1:41000", map[string]string{"X-Real-IP": "203.0.113.7", "X-Forwarded-For": "8.8.4.4"}, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	nodes := registered(t, s)
	if len(nodes) != 1 || nodes[0].IP != "203.0.113.7" || nodes[0].Peer() != testNodeID+"@203.0.113.7:26656" {
		t.Fatalf("unexpected registry contents %+v", nodes)
	}
}

func TestSpoofedForwardingHeadersIgnoredOffLoopback(t *testing.T) {
	s := newTestServer(t, t.TempDir())
	body := `{"node_id":"` + testNodeID + `","p2p_port":26656}`
	rec := register(s, "198.51.100.9:41000", map[string]string{"X-Real-IP": "203.0.113.50", "X-Forwarded-For": "203.0.113.51"}, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	nodes := registered(t, s)
	if len(nodes) != 1 || nodes[0].IP != "198.51.100.9" {
		t.Fatalf("spoofed address accepted: %+v", nodes)
	}
}

func TestMalformedRegistrationsRejectedWithoutPersistence(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	public := map[string]string{"X-Real-IP": "203.0.113.7"}
	cases := []struct {
		name    string
		remote  string
		headers map[string]string
		body    string
		want    int
	}{
		{"short node id", "127.0.0.1:1", public, `{"node_id":"abc"}`, http.StatusBadRequest},
		{"non-hex node id", "127.0.0.1:1", public, `{"node_id":"` + strings.Repeat("z", 40) + `"}`, http.StatusBadRequest},
		{"private source", "127.0.0.1:1", map[string]string{"X-Real-IP": "10.1.2.3"}, `{"node_id":"` + testNodeID + `"}`, http.StatusBadRequest},
		{"loopback source", "127.0.0.1:1", nil, `{"node_id":"` + testNodeID + `"}`, http.StatusBadRequest},
		{"malformed source", "127.0.0.1:1", map[string]string{"X-Real-IP": "203.0.113.999"}, `{"node_id":"` + testNodeID + `"}`, http.StatusBadRequest},
		{"port out of range", "127.0.0.1:1", public, `{"node_id":"` + testNodeID + `","p2p_port":70000}`, http.StatusBadRequest},
		{"negative port", "127.0.0.1:1", public, `{"node_id":"` + testNodeID + `","p2p_port":-1}`, http.StatusBadRequest},
		{"control moniker", "127.0.0.1:1", public, `{"node_id":"` + testNodeID + `","moniker":"a\u0007b"}`, http.StatusBadRequest},
		{"oversized version", "127.0.0.1:1", public, `{"node_id":"` + testNodeID + `","version":"` + strings.Repeat("v", 65) + `"}`, http.StatusBadRequest},
		{"bad json", "127.0.0.1:1", public, `{"node_id":`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		if rec := register(s, tc.remote, tc.headers, tc.body); rec.Code != tc.want {
			t.Errorf("%s: status %d want %d", tc.name, rec.Code, tc.want)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/api/register", strings.NewReader(`{"node_id":"`+testNodeID+`"}`))
	req.RemoteAddr = "127.0.0.1:1"
	req.Header.Set("X-Real-IP", "203.0.113.7")
	req.Header.Set("X-HPX-Token", "wrong-token")
	rec := httptest.NewRecorder()
	s.handleRegister(rec, req)
	if rec.Code != http.StatusUnauthorized || strings.Contains(rec.Body.String(), "operator-token") {
		t.Errorf("wrong token: status %d body %q", rec.Code, rec.Body)
	}
	get := httptest.NewRecorder()
	s.handleRegister(get, httptest.NewRequest(http.MethodGet, "/api/register", nil))
	if get.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET register status %d", get.Code)
	}
	if nodes := registered(t, s); len(nodes) != 0 {
		t.Fatalf("rejected registrations persisted: %+v", nodes)
	}
	if _, err := os.Stat(filepath.Join(dir, "registry.json")); !os.IsNotExist(err) {
		t.Fatalf("registry file written for rejected registrations: %v", err)
	}
}

func TestRegistrationRateLimited(t *testing.T) {
	s := newTestServer(t, t.TempDir())
	body := `{"node_id":"` + testNodeID + `"}`
	headers := map[string]string{"X-Real-IP": "203.0.113.7"}
	for i := 0; i < registerBurst; i++ {
		if rec := register(s, "127.0.0.1:1", headers, body); rec.Code != http.StatusOK {
			t.Fatalf("registration %d status %d", i, rec.Code)
		}
	}
	if rec := register(s, "127.0.0.1:1", headers, body); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("over-limit registration status %d", rec.Code)
	}
	if rec := register(s, "127.0.0.1:1", map[string]string{"X-Real-IP": "203.0.113.8"}, body); rec.Code != http.StatusOK {
		t.Fatalf("independent source limited: status %d", rec.Code)
	}
}

func TestRegistryRecoversPeersAfterRestart(t *testing.T) {
	dir := t.TempDir()
	first := newTestServer(t, dir)
	other := otherNodeID
	for ip, id := range map[string]string{"203.0.113.7": testNodeID, "203.0.113.8": other} {
		if rec := register(first, "127.0.0.1:1", map[string]string{"X-Real-IP": ip}, `{"node_id":"`+id+`","p2p_port":26656}`); rec.Code != http.StatusOK {
			t.Fatalf("register %s: %d", id, rec.Code)
		}
	}
	restarted := newTestServer(t, dir)
	rec := httptest.NewRecorder()
	restarted.handlePeers(rec, httptest.NewRequest(http.MethodGet, "/api/peers?self="+other, nil))
	var peers struct {
		ChainID string   `json:"chain_id"`
		Peers   []string `json:"peers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &peers); err != nil {
		t.Fatal(err)
	}
	if peers.ChainID != "hyperpax_125-1" || len(peers.Peers) != 1 || peers.Peers[0] != testNodeID+"@203.0.113.7:26656" {
		t.Fatalf("restart lost peers: %+v", peers)
	}
	if nodes := registered(t, restarted); len(nodes) != 2 {
		t.Fatalf("restart lost nodes: %+v", nodes)
	}
}

func TestArtifactSurfaceServesOnlyDeclaredFiles(t *testing.T) {
	root := t.TempDir()
	for rel, content := range map[string]string{
		"paxd":                     "binary",
		"checksums.txt":            "manifest",
		"lib/libwasmvm.x86_64.so":  "library",
		"config/fullnode/app.toml": "app",
		"undeclared.txt":           "private",
		"lib/undeclared.so":        "private",
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	handler := artifactHandler(root)
	fetch := func(method, target string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
		return rec
	}
	for target, want := range map[string]string{"/paxd": "binary", "/lib/libwasmvm.x86_64.so": "library", "/config/fullnode/app.toml": "app"} {
		rec := fetch(http.MethodGet, target)
		if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), []byte(want)) || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s: status %d body %q", target, rec.Code, rec.Body)
		}
	}
	if rec := fetch(http.MethodGet, "/checksums.txt"); rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("manifest cacheable: %q", rec.Header().Get("Cache-Control"))
	}
	for _, target := range []string{"/", "/lib/", "/lib", "/config/", "/undeclared.txt", "/lib/undeclared.so", "/../undeclared.txt", "/lib/../undeclared.txt", "/genesis.json"} {
		if rec := fetch(http.MethodGet, target); rec.Code != http.StatusNotFound {
			t.Errorf("%s: status %d", target, rec.Code)
		}
	}
	if rec := fetch(http.MethodPost, "/paxd"); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /paxd: status %d", rec.Code)
	}
}

func TestRegistryListensOnlyOnLoopback(t *testing.T) {
	for addr, want := range map[string]bool{
		"127.0.0.1:8099": true, "[::1]:8099": true, ":8099": false, "0.0.0.0:8099": false,
		"[::]:8099": false, "203.0.113.1:8099": false, "localhost:8099": false, "127.0.0.1": false,
	} {
		if got := loopbackListen(addr); got != want {
			t.Errorf("loopbackListen(%q) = %v want %v", addr, got, want)
		}
	}
	t.Setenv("HPX_ADDR", "")
	if cfg := loadConfig(); cfg.Addr != "127.0.0.1:8099" {
		t.Fatalf("default listen address %q", cfg.Addr)
	}
}

// CometBFT and pax-tendermint HTTP wire fixtures, served over loopback.
const stateSyncStatusFixture = `{"node_info":{"network":"hyperpax_125-1"},"sync_info":{"latest_block_height":"3000"}}`
const stateSyncCommitFixture = `{"signed_header":{"header":{"chain_id":"hyperpax_125-1","height":"1000"},"commit":{"height":"1000","block_id":{"hash":"abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"}}},"canonical":true}`
const stateSyncHashFixture = "ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789"

func testStateSyncTransport(t *testing.T) {
	t.Helper()
	cases := []struct {
		name    string
		status  string
		commit  string
		code    int
		wrapped bool
		timeout string
		enabled bool
		height  int64
		offset  int
	}{
		{name: "bare", status: stateSyncStatusFixture, commit: stateSyncCommitFixture, enabled: true, height: 1000, offset: 2000},
		{name: "wrapped", status: stateSyncStatusFixture, commit: stateSyncCommitFixture, wrapped: true, enabled: true, height: 1000, offset: 2000},
		{name: "height floor", status: strings.Replace(stateSyncStatusFixture, "3000", "100", 1), commit: strings.ReplaceAll(stateSyncCommitFixture, "1000", "1"), enabled: true, height: 1, offset: 2000},
		{name: "wrong status chain", status: strings.ReplaceAll(stateSyncStatusFixture, "hyperpax_125-1", "wrong-chain"), commit: stateSyncCommitFixture},
		{name: "wrong commit chain", status: stateSyncStatusFixture, commit: strings.ReplaceAll(stateSyncCommitFixture, "hyperpax_125-1", "wrong-chain"), offset: 2000},
		{name: "wrong header height", status: stateSyncStatusFixture, commit: strings.Replace(stateSyncCommitFixture, `"height":"1000"`, `"height":"999"`, 1), offset: 2000},
		{name: "wrong commit height", status: stateSyncStatusFixture, commit: strings.Replace(stateSyncCommitFixture, `"commit":{"height":"1000"`, `"commit":{"height":"999"`, 1), offset: 2000},
		{name: "short hash", status: stateSyncStatusFixture, commit: strings.ReplaceAll(stateSyncCommitFixture, strings.ToLower(stateSyncHashFixture), "ab"), offset: 2000},
		{name: "long hash", status: stateSyncStatusFixture, commit: strings.ReplaceAll(stateSyncCommitFixture, strings.ToLower(stateSyncHashFixture), strings.Repeat("ab", 33)), offset: 2000},
		{name: "nonhex hash", status: stateSyncStatusFixture, commit: strings.ReplaceAll(stateSyncCommitFixture, strings.ToLower(stateSyncHashFixture), strings.Repeat("z", 64)), offset: 2000},
		{name: "missing header", status: stateSyncStatusFixture, commit: `{"signed_header":{"commit":{"height":"1000","block_id":{"hash":"` + strings.ToLower(stateSyncHashFixture) + `"}}}}`, offset: 2000},
		{name: "numeric status height", status: strings.Replace(stateSyncStatusFixture, `"3000"`, `3000`, 1), commit: stateSyncCommitFixture},
		{name: "numeric commit height", status: stateSyncStatusFixture, commit: strings.ReplaceAll(stateSyncCommitFixture, `"1000"`, `1000`), offset: 2000},
		{name: "zero height", status: strings.Replace(stateSyncStatusFixture, `"3000"`, `"0"`, 1)},
		{name: "negative height", status: strings.Replace(stateSyncStatusFixture, `"3000"`, `"-1"`, 1)},
		{name: "overflow height", status: strings.Replace(stateSyncStatusFixture, `"3000"`, `"9223372036854775808"`, 1)},
		{name: "negative offset", status: stateSyncStatusFixture, commit: stateSyncCommitFixture, offset: -1},
		{name: "malformed status", status: `{"sync_info":`},
		{name: "malformed commit", status: stateSyncStatusFixture, commit: `{"signed_header":`, offset: 2000},
		{name: "trailing status", status: stateSyncStatusFixture + ` {}`},
		{name: "trailing commit", status: stateSyncStatusFixture, commit: stateSyncCommitFixture + ` {}`, offset: 2000},
		{name: "null result", status: `{"result":null}`},
		{name: "array result", status: `{"result":[]}`},
		{name: "RPC error", status: `{"error":{"code":-32603},"result":` + stateSyncStatusFixture + `}`},
		{name: "null status", status: `null`},
		{name: "upstream status failure", status: stateSyncStatusFixture, code: 503},
		{name: "upstream commit failure", status: stateSyncStatusFixture, commit: stateSyncCommitFixture, code: 502, offset: 2000, timeout: "commit failure"},
		{name: "oversized status", status: stateSyncStatusFixture + strings.Repeat(" ", rpcResponseLimit)},
		{name: "oversized commit", status: stateSyncStatusFixture, commit: stateSyncCommitFixture + strings.Repeat(" ", rpcResponseLimit), offset: 2000},
		{name: "status timeout", status: stateSyncStatusFixture, timeout: "/status"},
		{name: "commit timeout", status: stateSyncStatusFixture, commit: stateSyncCommitFixture, offset: 2000, timeout: "/commit"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requests := make(chan string, 4)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests <- r.URL.RequestURI()
				if r.URL.Path == tc.timeout {
					<-r.Context().Done()
					return
				}
				value := tc.status
				code := http.StatusOK
				if tc.code != 0 && tc.timeout != "commit failure" {
					code = tc.code
				}
				if r.URL.Path == "/commit" {
					value = tc.commit
					if r.URL.Query().Get("height") != fmt.Sprint(tc.height) && tc.enabled {
						t.Errorf("unexpected requested height: %s", r.URL.RawQuery)
					}
					if tc.timeout == "commit failure" {
						code = tc.code
					}
				}
				w.WriteHeader(code)
				if tc.wrapped {
					fmt.Fprintf(w, `{"jsonrpc":"2.0","id":-1,"result":%s}`, value)
				} else {
					io.WriteString(w, value)
				}
			}))
			defer upstream.Close()
			s := newTestServer(t, t.TempDir())
			s.cfg.StateSyncRPC, s.cfg.StateSyncServers = upstream.URL, "rpc-one:26657,rpc-two:26657"
			s.cfg.TrustOffset, s.cfg.TrustPeriod = tc.offset, "168h0m0s"
			s.rpc = &http.Client{Timeout: 50 * time.Millisecond}
			if tc.timeout != "/status" && tc.timeout != "/commit" {
				s.rpc.Timeout = 2 * time.Second
			}
			transport := httptest.NewServer(http.HandlerFunc(s.handleStateSync))
			defer transport.Close()
			response, err := transport.Client().Get(transport.URL + "/api/statesync")
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			var output struct {
				Enable     bool   `json:"enable"`
				RPCServers string `json:"rpc_servers"`
				Height     int64  `json:"trust_height"`
				Hash       string `json:"trust_hash"`
				Period     string `json:"trust_period"`
				Latest     int64  `json:"latest"`
				Reason     string `json:"reason"`
			}
			if err := json.NewDecoder(response.Body).Decode(&output); err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != http.StatusOK || response.Header.Get("Cache-Control") != "no-store" {
				t.Fatalf("invalid response: %v", response)
			}
			if output.Enable != tc.enabled {
				t.Fatalf("enable=%v want %v: %+v", output.Enable, tc.enabled, output)
			}
			if tc.enabled {
				wantLatest := int64(3000)
				if tc.height == 1 {
					wantLatest = 100
				}
				if output.Height != tc.height || output.Hash != stateSyncHashFixture || output.Latest != wantLatest || output.RPCServers != s.cfg.StateSyncServers || output.Period != s.cfg.TrustPeriod {
					t.Fatalf("incorrect consumer configuration: %+v", output)
				}
				if len(requests) != 2 {
					t.Fatalf("expected status and commit requests, got %d", len(requests))
				}
			} else if output.Reason == "" || output.Hash != "" || output.Height != 0 || output.RPCServers != "" {
				t.Fatalf("rejected anchor leaked config: %+v", output)
			}
		})
	}
	t.Run("unconfigured", func(t *testing.T) {
		s := newTestServer(t, t.TempDir())
		rec := httptest.NewRecorder()
		s.handleStateSync(rec, httptest.NewRequest(http.MethodGet, "/api/statesync", nil))
		var result map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result["enable"] != false || result["reason"] == "" {
			t.Fatalf("unconfigured state sync enabled: %v", result)
		}
	})
}

func TestStateSyncTransport(t *testing.T) { testStateSyncTransport(t) }
