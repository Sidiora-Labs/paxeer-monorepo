package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func repairM807HTTP(t *testing.T, s *server) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/register", s.handleRegister)
	mux.HandleFunc("/api/register/challenge", s.handleRegistrationChallenge)
	mux.HandleFunc("/api/peers", s.handlePeers)
	mux.HandleFunc("/api/peers.txt", s.handlePeersTxt)
	mux.HandleFunc("/api/nodes", s.handleNodes)
	transport := httptest.NewServer(mux)
	t.Cleanup(transport.Close)
	return transport
}

func repairM807Request(t *testing.T, transport *httptest.Server, method, path, source string, body []byte) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, transport.URL+path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Real-IP", source)
	req.Header.Set("X-HPX-Token", "operator-token")
	response, err := transport.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, data
}

func repairM807Challenge(t *testing.T, transport *httptest.Server, source string) registrationChallenge {
	t.Helper()
	code, data := repairM807Request(t, transport, http.MethodGet, "/api/register/challenge", source, nil)
	if code != http.StatusOK {
		t.Fatalf("challenge: %d %s", code, data)
	}
	var challenge registrationChallenge
	if err := json.Unmarshal(data, &challenge); err != nil {
		t.Fatal(err)
	}
	if len(challenge.Nonce) != 64 || challenge.Source != source || challenge.ChainID != "hyperpax_125-1" || !challenge.Expires.After(time.Now()) {
		t.Fatalf("invalid challenge: %+v", challenge)
	}
	return challenge
}

func repairM807Proof(t *testing.T, challenge registrationChallenge, node Node, key ed25519.PrivateKey) []byte {
	t.Helper()
	proof := signedRegistration{Node: node, Challenge: challenge.Nonce, PublicKey: key.Public().(ed25519.PublicKey)}
	proof.Signature = ed25519.Sign(key, registrationMessage(challenge.ChainID, challenge.Nonce, node))
	body, err := json.Marshal(proof)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func repairM807Node(key ed25519.PrivateKey, source string) Node {
	return Node{NodeID: nodeKeyID(key), IP: source, P2PPort: 26656, Moniker: "repair-node", Type: "fullnode", Version: "v1"}
}

func repairM807Register(t *testing.T, transport *httptest.Server, key ed25519.PrivateKey, source string) Node {
	t.Helper()
	challenge := repairM807Challenge(t, transport, source)
	node := repairM807Node(key, source)
	body := repairM807Proof(t, challenge, node, key)
	code, data := repairM807Request(t, transport, http.MethodPost, "/api/register", source, body)
	if code != http.StatusOK {
		t.Fatalf("register: %d %s", code, data)
	}
	var output struct {
		OK   bool `json:"ok"`
		Node Node `json:"node"`
	}
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if !output.OK || output.Node.NodeID != node.NodeID || output.Node.IP != source || output.Node.FirstSeen == "" || output.Node.LastSeen == "" {
		t.Fatalf("invalid registration: %s", data)
	}
	return output.Node
}

func TestRepairM_807_39RetentionRefreshCapacityAndBoundedPeers(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	now := time.Now().UTC()
	firstSeen := now.Add(-time.Hour).Format(time.RFC3339Nano)
	nodes := make(map[string]*Node, registryCapacity)
	for i := 0; i < registryCapacity; i++ {
		id := fmt.Sprintf("%040x", i+1)
		nodes[id] = &Node{NodeID: id, IP: "203.0.113.10", P2PPort: 26656, FirstSeen: firstSeen, LastSeen: now.Format(time.RFC3339Nano)}
	}
	if err := s.reg.saveLocked(nodes); err != nil {
		t.Fatal(err)
	}
	reopened, err := openRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	s.reg = reopened
	seed := strings.Repeat("f", 40) + "@203.0.113.20:26656"
	s.cfg.SeedPeers = []string{seed, seed}
	transport := repairM807HTTP(t, s)
	challenge := repairM807Challenge(t, transport, "203.0.113.7")
	rejected := repairM807Node(testNodeKey, "203.0.113.7")
	code, data := repairM807Request(t, transport, http.MethodPost, "/api/register", rejected.IP, repairM807Proof(t, challenge, rejected, testNodeKey))
	if code != http.StatusServiceUnavailable {
		t.Fatalf("full registry accepted registration: %d %s", code, data)
	}
	if _, ok := s.reg.Nodes[rejected.NodeID]; ok {
		t.Fatal("capacity rejection became live")
	}
	id := fmt.Sprintf("%040x", 1)
	refreshed := *s.reg.Nodes[id]
	oldSeen := refreshed.LastSeen
	if err := s.reg.upsert(&refreshed); err != nil {
		t.Fatalf("refresh at capacity: %v", err)
	}
	if refreshed.FirstSeen != firstSeen || refreshed.LastSeen == oldSeen || len(s.reg.Nodes) != registryCapacity {
		t.Fatal("refresh changed stable identity or capacity")
	}
	expiredID := fmt.Sprintf("%040x", 2)
	s.reg.Nodes[expiredID].LastSeen = now.Add(-registryTTL - time.Second).Format(time.RFC3339Nano)
	s.reg.Nodes[fmt.Sprintf("%040x", 3)].LastSeen = "invalid-time"
	s.reg.Nodes[fmt.Sprintf("%040x", 4)].LastSeen = now.Add(registryTTL).Format(time.RFC3339Nano)
	for _, path := range []string{"/api/peers", "/api/peers.txt", "/api/nodes"} {
		code, data := repairM807Request(t, transport, http.MethodGet, path, "203.0.113.7", nil)
		if code != http.StatusOK {
			t.Fatalf("%s: %d %s", path, code, data)
		}
		if strings.Contains(string(data), expiredID+"@") || (path == "/api/nodes" && strings.Contains(string(data), `"node_id":"`+expiredID+`"`)) {
			t.Fatalf("expired peer returned by %s", path)
		}
		if path == "/api/peers.txt" {
			peers := strings.Split(string(data), ",")
			if len(peers) != peerResponseLimit || peers[0] != seed || bytes.Count(data, []byte(seed)) != 1 {
				t.Fatalf("unbounded or unstable text peers: %d", len(peers))
			}
		} else if path == "/api/peers" {
			var output struct {
				Count      int      `json:"count"`
				Peers      []string `json:"peers"`
				Persistent string   `json:"persistent_peers"`
			}
			if err := json.Unmarshal(data, &output); err != nil {
				t.Fatal(err)
			}
			if output.Count != peerResponseLimit || len(output.Peers) != peerResponseLimit || output.Peers[0] != seed || output.Persistent != strings.Join(output.Peers, ",") {
				t.Fatalf("unbounded or inconsistent peer JSON: %+v", output)
			}
		} else {
			var output struct {
				Count int    `json:"count"`
				Nodes []Node `json:"nodes"`
			}
			if err := json.Unmarshal(data, &output); err != nil {
				t.Fatal(err)
			}
			if output.Count != registryCapacity-3 || len(output.Nodes) != registryCapacity-3 {
				t.Fatalf("invalid active node count: %d", output.Count)
			}
		}
	}
	accepted := repairM807Register(t, transport, testNodeKey, "203.0.113.7")
	if accepted.NodeID != rejected.NodeID || len(s.reg.Nodes) != registryCapacity-2 {
		t.Fatal("expired slots were not reclaimed")
	}
	if _, exists := s.reg.Nodes[expiredID]; exists {
		t.Fatal("expired record retained after durable update")
	}
	snapshot := s.reg.list()
	snapshot[0].IP = "203.0.113.99"
	if s.reg.Nodes[snapshot[0].NodeID].IP == snapshot[0].IP {
		t.Fatal("list exposed mutable registry storage")
	}
	transport.Close()
	restarted := newTestServer(t, dir)
	if len(restarted.reg.Nodes) != registryCapacity-2 || restarted.reg.Nodes[accepted.NodeID].IP != accepted.IP {
		t.Fatal("bounded update did not recover")
	}
	if activeNode(&Node{LastSeen: now.Add(-registryTTL).Format(time.RFC3339Nano)}, now) {
		t.Fatal("exact expiry boundary retained")
	}
}

func TestRepairM_807_36DurableRegistrationAndInterruptedRecovery(t *testing.T) {
	for _, interrupted := range []string{"partial temporary image", "complete temporary image", "renamed image"} {
		t.Run(interrupted, func(t *testing.T) {
			dir := t.TempDir()
			s := newTestServer(t, dir)
			transport := repairM807HTTP(t, s)
			acknowledged := repairM807Register(t, transport, testNodeKey, "203.0.113.7")
			transport.Close()
			original, err := os.ReadFile(s.reg.path)
			if err != nil {
				t.Fatal(err)
			}
			next := acknowledged
			next.IP = "203.0.113.8"
			candidate, err := json.Marshal(struct {
				Nodes map[string]*Node `json:"nodes"`
			}{map[string]*Node{next.NodeID: &next}})
			if err != nil {
				t.Fatal(err)
			}
			pending, err := os.CreateTemp(dir, ".registry-*.tmp")
			if err != nil {
				t.Fatal(err)
			}
			content := candidate
			if interrupted == "partial temporary image" {
				content = candidate[:len(candidate)/2]
			}
			if _, err := pending.Write(content); err != nil {
				t.Fatal(err)
			}
			if interrupted != "partial temporary image" {
				if err := pending.Sync(); err != nil {
					t.Fatal(err)
				}
			}
			if err := pending.Close(); err != nil {
				t.Fatal(err)
			}
			if interrupted == "renamed image" {
				if err := os.Rename(pending.Name(), s.reg.path); err != nil {
					t.Fatal(err)
				}
			}
			recovered := newTestServer(t, dir)
			want := acknowledged
			if interrupted == "renamed image" {
				want = next
			}
			got := recovered.reg.list()
			if len(got) != 1 || !reflect.DeepEqual(*got[0], want) {
				t.Fatalf("incoherent recovery after %s: %+v want %+v", interrupted, got, want)
			}
			committed, err := os.ReadFile(recovered.reg.path)
			if err != nil {
				t.Fatal(err)
			}
			if interrupted != "renamed image" && !bytes.Equal(original, committed) {
				t.Fatal("uncommitted temp replaced acknowledged image")
			}
			live := repairM807HTTP(t, recovered)
			code, data := repairM807Request(t, live, http.MethodGet, "/api/peers.txt", "203.0.113.7", nil)
			if code != http.StatusOK || string(data) != want.Peer() {
				t.Fatalf("recovered consumer peer state: %d %s", code, data)
			}
			repairM807Register(t, live, otherNodeKey, "203.0.113.9")
			live.Close()
			again := newTestServer(t, dir)
			if len(again.reg.list()) != 2 {
				t.Fatal("interrupted file prevented subsequent durable registration")
			}
		})
	}
}

func TestRepairM_807_37FailedPersistenceLeavesExactPriorState(t *testing.T) {
	for _, failure := range []string{"create temporary file", "rename image"} {
		t.Run(failure, func(t *testing.T) {
			parent := t.TempDir()
			dir := filepath.Join(parent, "live")
			s := newTestServer(t, dir)
			transport := repairM807HTTP(t, s)
			acknowledged := repairM807Register(t, transport, testNodeKey, "203.0.113.7")
			original, err := os.ReadFile(s.reg.path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(dir, dir+"-offline"); err != nil {
				t.Fatal(err)
			}
			if failure == "rename image" {
				if err := os.MkdirAll(s.reg.path, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(s.reg.path, "obstruction"), []byte("occupied"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			for i, key := range []ed25519.PrivateKey{testNodeKey, otherNodeKey} {
				source := fmt.Sprintf("203.0.113.%d", 8+i)
				challenge := repairM807Challenge(t, transport, source)
				replacement := repairM807Node(key, source)
				code, data := repairM807Request(t, transport, http.MethodPost, "/api/register", source, repairM807Proof(t, challenge, replacement, key))
				if code != http.StatusInternalServerError {
					t.Fatalf("expected failed persistence, got %d %s", code, data)
				}
				if got := s.reg.list(); len(got) != 1 || !reflect.DeepEqual(*got[0], acknowledged) {
					t.Fatalf("failed mutation became visible: %+v", got)
				}
			}
			candidate := repairM807Node(testNodeKey, "203.0.113.99")
			before := candidate
			if err := s.reg.upsert(&candidate); err == nil || !reflect.DeepEqual(candidate, before) {
				t.Fatal("failed save mutated caller or did not fail")
			}
			code, data := repairM807Request(t, transport, http.MethodGet, "/api/peers.txt", "203.0.113.7", nil)
			if code != http.StatusOK || string(data) != acknowledged.Peer() {
				t.Fatalf("failed registration reached peer consumer: %d %s", code, data)
			}
			transport.Close()
			if failure == "rename image" {
				if err := os.RemoveAll(dir); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Rename(dir+"-offline", dir); err != nil {
				t.Fatal(err)
			}
			recovered := newTestServer(t, dir)
			after, err := os.ReadFile(recovered.reg.path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(after, original) || len(recovered.reg.list()) != 1 || !reflect.DeepEqual(*recovered.reg.list()[0], acknowledged) {
				t.Fatal("rejected state changed durable image or restart state")
			}
		})
	}
}

func TestRepairM_807_38OwnershipTamperReplayAndRestart(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	transport := repairM807HTTP(t, s)
	owner := repairM807Register(t, transport, testNodeKey, "203.0.113.7")
	for _, attack := range []string{"unsigned copied identity", "different key", "modified port", "modified source", "modified moniker", "modified type", "modified version", "different chain", "expired", "unknown challenge"} {
		t.Run(attack, func(t *testing.T) {
			source := "203.0.113.8"
			challenge := repairM807Challenge(t, transport, source)
			node := repairM807Node(testNodeKey, source)
			key := testNodeKey
			if attack == "different key" {
				key = otherNodeKey
			}
			if attack == "different chain" {
				challenge.ChainID = "another-chain"
			}
			body := repairM807Proof(t, challenge, node, key)
			var proof signedRegistration
			if err := json.Unmarshal(body, &proof); err != nil {
				t.Fatal(err)
			}
			switch attack {
			case "unsigned copied identity":
				proof.PublicKey, proof.Signature = nil, nil
			case "modified port":
				proof.P2PPort++
			case "modified source":
				source = "203.0.113.9"
			case "modified moniker":
				proof.Moniker = "hijacked"
			case "modified type":
				proof.Type = "validator"
			case "modified version":
				proof.Version = "v2"
			case "expired":
				s.challengeMu.Lock()
				expired := s.challenges[challenge.Nonce]
				expired.Expires = time.Now().Add(-time.Second)
				s.challenges[challenge.Nonce] = expired
				s.challengeMu.Unlock()
			case "unknown challenge":
				proof.Challenge = strings.Repeat("0", 64)
			}
			body, err := json.Marshal(proof)
			if err != nil {
				t.Fatal(err)
			}
			code, data := repairM807Request(t, transport, http.MethodPost, "/api/register", source, body)
			if code != http.StatusUnauthorized {
				t.Fatalf("%s accepted or bypassed proof: %d %s", attack, code, data)
			}
			if got := s.reg.Nodes[owner.NodeID]; !reflect.DeepEqual(*got, owner) {
				t.Fatalf("attack changed address: %+v", got)
			}
		})
	}
	// Move to a fresh source after exhausting a source's admission window.
	source := "203.0.113.10"
	challenge := repairM807Challenge(t, transport, source)
	body := repairM807Proof(t, challenge, repairM807Node(testNodeKey, source), testNodeKey)
	code, data := repairM807Request(t, transport, http.MethodPost, "/api/register", source, body)
	if code != http.StatusOK {
		t.Fatalf("owner update refused: %d %s", code, data)
	}
	updated := *s.reg.Nodes[owner.NodeID]
	if updated.IP != source || updated.FirstSeen != owner.FirstSeen {
		t.Fatal("owner refresh lost stable first_seen")
	}
	code, data = repairM807Request(t, transport, http.MethodPost, "/api/register", source, body)
	if code != http.StatusUnauthorized {
		t.Fatalf("replay accepted: %d %s", code, data)
	}
	outstanding := repairM807Challenge(t, transport, source)
	pending := repairM807Proof(t, outstanding, repairM807Node(testNodeKey, source), testNodeKey)
	transport.Close()
	restarted := newTestServer(t, dir)
	recovered := repairM807HTTP(t, restarted)
	for _, replay := range [][]byte{body, pending} {
		code, data = repairM807Request(t, recovered, http.MethodPost, "/api/register", source, replay)
		if code != http.StatusUnauthorized {
			t.Fatalf("pre-restart proof accepted: %d %s", code, data)
		}
	}
	if got := restarted.reg.Nodes[owner.NodeID]; !reflect.DeepEqual(*got, updated) {
		t.Fatalf("proof rejection changed recovered state: %+v", got)
	}
	repairM807Register(t, recovered, testNodeKey, "203.0.113.11")
}

func TestRepairM_807_38SourceAndAdmissionBounds(t *testing.T) {
	s := newTestServer(t, t.TempDir())
	transport := repairM807HTTP(t, s)
	for i := 0; i < registerBurst; i++ {
		code, _ := repairM807Request(t, transport, http.MethodPost, "/api/register", "203.0.113.7", []byte(`{"node_id":"`+testNodeID+`"}`))
		if code != http.StatusUnauthorized {
			t.Fatalf("unsigned registration %d returned %d", i, code)
		}
	}
	code, _ := repairM807Request(t, transport, http.MethodPost, "/api/register", "203.0.113.7", []byte(`{}`))
	if code != http.StatusTooManyRequests {
		t.Fatalf("registration limit lost: %d", code)
	}
	code, _ = repairM807Request(t, transport, http.MethodGet, "/api/register/challenge", "10.1.2.3", nil)
	if code != http.StatusBadRequest {
		t.Fatalf("private source admitted: %d", code)
	}
	for i := 0; i < registerBurst; i++ {
		repairM807Challenge(t, transport, "203.0.113.8")
	}
	code, _ = repairM807Request(t, transport, http.MethodGet, "/api/register/challenge", "203.0.113.8", nil)
	if code != http.StatusTooManyRequests {
		t.Fatalf("challenge rate limit lost: %d", code)
	}
	s.challengeMu.Lock()
	for i := len(s.challenges); i < challengeCapacity; i++ {
		nonce := fmt.Sprintf("fixture-%d", i)
		s.challenges[nonce] = registrationChallenge{Nonce: nonce, Expires: time.Now().Add(time.Minute)}
	}
	s.challengeMu.Unlock()
	code, _ = repairM807Request(t, transport, http.MethodGet, "/api/register/challenge", "203.0.113.9", nil)
	if code != http.StatusServiceUnavailable || len(s.challenges) != challengeCapacity {
		t.Fatalf("unbounded challenge cache: %d %d", code, len(s.challenges))
	}
	s.challengeMu.Lock()
	for nonce, challenge := range s.challenges {
		challenge.Expires = time.Now().Add(-time.Second)
		s.challenges[nonce] = challenge
	}
	s.challengeMu.Unlock()
	repairM807Challenge(t, transport, "203.0.113.9")
	if len(s.challenges) != 1 {
		t.Fatalf("expired challenge retention: %d", len(s.challenges))
	}
	req := httptest.NewRequest(http.MethodGet, "/api/register/challenge", nil)
	req.RemoteAddr = "198.51.100.9:41000"
	req.Header.Set("X-Real-IP", "203.0.113.50")
	rec := httptest.NewRecorder()
	s.handleRegistrationChallenge(rec, req)
	var observed registrationChallenge
	if err := json.Unmarshal(rec.Body.Bytes(), &observed); err != nil {
		t.Fatal(err)
	}
	if observed.Source != "198.51.100.9" {
		t.Fatal("spoofed forwarding header affected proof source")
	}
}

func TestRepairM_807_40And41StateSyncTransport(t *testing.T) {
	testStateSyncTransport(t)
}

func TestRepairM_807_39RejectOversizedPersistedImage(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "registry.json"), bytes.Repeat([]byte(" "), registryMaxBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := openRegistry(dir); err == nil {
		t.Fatal("oversized persisted image accepted")
	}
	nodes := make(map[string]*Node, registryCapacity+1)
	for i := 0; i <= registryCapacity; i++ {
		nodes[fmt.Sprint(i)] = &Node{}
	}
	body, err := json.Marshal(struct {
		Nodes map[string]*Node `json:"nodes"`
	}{nodes})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "registry.json"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := openRegistry(dir); !errors.Is(err, errRegistryFull) {
		t.Fatalf("over-capacity image accepted: %v", err)
	}
}
