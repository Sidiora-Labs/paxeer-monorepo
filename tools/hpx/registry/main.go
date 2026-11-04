// hpx-registry — the HyperPax node distribution + peer registry service.
//
// Two jobs in one tiny stdlib-only binary:
//
//  1. Serve the install artifacts (paxd binary, libwasmvm .so, genesis,
//     fullnode/validator config variants, the hpx CLI + install scripts).
//     These live under ARTIFACTS_DIR and are served at the URL root, so e.g.
//     GET /get-hpx.sh, /hpx, /paxd, /genesis.json, /lib/libwasmvm.x86_64.so,
//     /config/fullnode/config.toml, /chain-info.json.
//
//  2. Maintain the live peer registry. A node POSTs /api/register with its
//     CometBFT node id once it is up; the server records it and returns the
//     current peer list. Any node (new or updating) can pull /api/peers[.txt]
//     to learn every known peer, so the mesh grows automatically.
//
// State is a single JSON file under DATA_DIR (registry.json), guarded by a
// mutex. No external dependencies, no database.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var sourceRevision = "development"

// ---- config ----------------------------------------------------------------

type config struct {
	Addr         string
	ArtifactsDir string
	DataDir      string
	RegisterTok  string   // optional; if set, POST /api/register requires X-HPX-Token
	SeedPeers    []string // always-present peers (e.g. the validator), id@host:port
	ChainID      string

	// State sync: a fresh node cannot blocksync from genesis because the chain
	// is pruned, so it bootstraps from a recent snapshot. We compute a trusted
	// light-client anchor (height+hash) from a live RPC on demand.
	StateSyncRPC     string // http base, e.g. http://1.2.3.4:26657 (defaults to seed host)
	StateSyncServers string // value for config.toml rpc-servers (>=2 entries)
	TrustOffset      int    // trust-height = latest - TrustOffset
	TrustPeriod      string
}

func loadConfig() config {
	c := config{
		Addr:         env("HPX_ADDR", "127.0.0.1:8099"),
		ArtifactsDir: env("HPX_ARTIFACTS_DIR", "/srv/hpx/artifacts/current"),
		DataDir:      env("HPX_DATA_DIR", "/srv/hpx/data"),
		RegisterTok:  os.Getenv("HPX_REGISTER_TOKEN"),
		ChainID:      env("HPX_CHAIN_ID", "hyperpax_125-1"),
	}
	for _, p := range strings.Split(os.Getenv("HPX_SEED_PEERS"), ",") {
		if p = strings.TrimSpace(p); p != "" {
			c.SeedPeers = append(c.SeedPeers, p)
		}
	}
	// Derive the state-sync RPC host from the first seed peer (id@host:port)
	// unless explicitly overridden.
	seedHost := ""
	if len(c.SeedPeers) > 0 {
		if at := strings.Index(c.SeedPeers[0], "@"); at >= 0 {
			hp := c.SeedPeers[0][at+1:]
			if h, _, err := net.SplitHostPort(hp); err == nil {
				seedHost = h
			}
		}
	}
	c.StateSyncRPC = env("HPX_STATESYNC_RPC", "")
	if c.StateSyncRPC == "" && seedHost != "" {
		c.StateSyncRPC = "http://" + seedHost + ":26657"
	}
	c.StateSyncServers = env("HPX_STATESYNC_RPC_SERVERS", "")
	if c.StateSyncServers == "" && seedHost != "" {
		rpc := seedHost + ":26657"
		c.StateSyncServers = rpc + "," + rpc
	}
	c.TrustOffset = envInt("HPX_STATESYNC_TRUST_OFFSET", 2000)
	c.TrustPeriod = env("HPX_STATESYNC_TRUST_PERIOD", "168h0m0s")
	return c
}

func envInt(k string, def int) int {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func env(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

// ---- registry model --------------------------------------------------------

type Node struct {
	NodeID    string `json:"node_id"`
	Moniker   string `json:"moniker"`
	IP        string `json:"ip"`
	P2PPort   int    `json:"p2p_port"`
	Type      string `json:"type"` // fullnode | validator
	Version   string `json:"version"`
	FirstSeen string `json:"first_seen"`
	LastSeen  string `json:"last_seen"`
}

// Peer renders the CometBFT persistent-peer string id@host:port.
func (n Node) Peer() string {
	port := n.P2PPort
	if port == 0 {
		port = 26656
	}
	return fmt.Sprintf("%s@%s", n.NodeID, net.JoinHostPort(n.IP, strconv.Itoa(port)))
}

const (
	registryTTL       = 24 * time.Hour
	registryCapacity  = 4096
	registryMaxBytes  = 8 << 20
	peerResponseLimit = 256
	seedPeerLimit     = 64
)

var errRegistryFull = errors.New("registry capacity reached")

func activeNode(n *Node, now time.Time) bool {
	if n == nil {
		return false
	}
	last, err := time.Parse(time.RFC3339Nano, n.LastSeen)
	return err == nil && !last.After(now) && now.Sub(last) < registryTTL
}

type registry struct {
	mu    sync.Mutex
	path  string
	Nodes map[string]*Node `json:"nodes"` // keyed by node_id
}

func openRegistry(dir string) (*registry, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	r := &registry{path: filepath.Join(dir, "registry.json"), Nodes: map[string]*Node{}}
	file, err := os.Open(r.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return r, nil
		}
		return nil, err
	}
	defer file.Close()
	b, err := io.ReadAll(io.LimitReader(file, registryMaxBytes+1))
	if len(b) > registryMaxBytes {
		return nil, errors.New("registry image exceeds size limit")
	}
	if err != nil {
		return nil, err
	}
	if len(b) > 0 {
		if err := json.Unmarshal(b, r); err != nil {
			return nil, fmt.Errorf("parse registry.json: %w", err)
		}
		if r.Nodes == nil {
			r.Nodes = map[string]*Node{}
		}
	}
	if len(r.Nodes) > registryCapacity {
		return nil, errRegistryFull
	}
	return r, nil
}

// saveLocked persists the registry; caller must hold r.mu.
func (r *registry) saveLocked(nodes map[string]*Node) error {
	b, err := json.MarshalIndent(struct {
		Nodes map[string]*Node `json:"nodes"`
	}{nodes}, "", "  ")
	if err != nil {
		return err
	}
	if len(b) > registryMaxBytes {
		return errors.New("registry image exceeds size limit")
	}
	dir, err := os.Open(filepath.Dir(r.path))
	if err != nil {
		return err
	}
	defer dir.Close()
	file, err := os.CreateTemp(filepath.Dir(r.path), ".registry-*.tmp")
	if err != nil {
		return err
	}
	tmp := file.Name()
	defer os.Remove(tmp)
	if _, err = file.Write(b); err != nil {
		file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	previous := tmp + ".previous"
	hadPrevious := true
	if err = os.Link(r.path, previous); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		hadPrevious = false
	}
	defer os.Remove(previous)
	if err = os.Rename(tmp, r.path); err != nil {
		return err
	}
	if err = dir.Sync(); err != nil {
		var restoreErr error
		if hadPrevious {
			restoreErr = os.Rename(previous, r.path)
		} else {
			restoreErr = os.Remove(r.path)
		}
		if restoreErr == nil {
			restoreErr = dir.Sync()
		}
		if restoreErr != nil {
			return errors.Join(err, fmt.Errorf("restore previous registry image: %w", restoreErr))
		}
		return err
	}
	return nil
}

func (r *registry) upsert(n *Node) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	candidate := make(map[string]*Node, len(r.Nodes)+1)
	for id, node := range r.Nodes {
		if activeNode(node, now) {
			candidate[id] = node
		}
	}
	value := *n
	if ex, ok := candidate[value.NodeID]; ok {
		value.FirstSeen = ex.FirstSeen
	} else {
		if len(candidate) >= registryCapacity {
			return errRegistryFull
		}
		value.FirstSeen = now.Format(time.RFC3339Nano)
	}
	value.LastSeen = now.Format(time.RFC3339Nano)
	candidate[value.NodeID] = &value
	if err := r.saveLocked(candidate); err != nil {
		return err
	}
	r.Nodes = candidate
	*n = value
	return nil
}

// list returns all nodes sorted by first_seen (stable peer ordering).
func (r *registry) list() []*Node {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*Node, 0, len(r.Nodes))
	now := time.Now().UTC()
	for _, n := range r.Nodes {
		if activeNode(n, now) {
			value := *n
			out = append(out, &value)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].FirstSeen != out[j].FirstSeen {
			return out[i].FirstSeen < out[j].FirstSeen
		}
		return out[i].NodeID < out[j].NodeID
	})
	return out
}

// ---- node ownership ---------------------------------------------------------

const (
	registrationProofDomain = "hpx-registry.registration.v1"
	challengeTTL            = 2 * time.Minute
	challengeCapacity       = 4096
)

type registrationChallenge struct {
	Source  string    `json:"ip"`
	ChainID string    `json:"chain_id"`
	Nonce   string    `json:"challenge"`
	Expires time.Time `json:"expires"`
}

type signedRegistration struct {
	Node
	Challenge string `json:"challenge"`
	PublicKey []byte `json:"public_key"`
	Signature []byte `json:"signature"`
}

func registrationMessage(chain, nonce string, node Node) []byte {
	node.FirstSeen, node.LastSeen = "", ""
	message, _ := json.Marshal(struct {
		Domain    string `json:"domain"`
		Chain     string `json:"chain_id"`
		Challenge string `json:"challenge"`
		Node      Node   `json:"node"`
	}{registrationProofDomain, chain, nonce, node})
	return message
}

func (s *server) handleRegistrationChallenge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	source := clientIP(r)
	if !validPublicIP(source) {
		http.Error(w, "registration requires a public source address", http.StatusBadRequest)
		return
	}
	now := time.Now()
	if !s.challengeLimiter.allow(source, now) {
		http.Error(w, "challenge rate limit exceeded", http.StatusTooManyRequests)
		return
	}
	s.challengeMu.Lock()
	defer s.challengeMu.Unlock()
	if s.challenges == nil {
		s.challenges = map[string]registrationChallenge{}
	}
	for nonce, challenge := range s.challenges {
		if !now.Before(challenge.Expires) {
			delete(s.challenges, nonce)
		}
	}
	if len(s.challenges) >= challengeCapacity {
		http.Error(w, "challenge capacity reached", http.StatusServiceUnavailable)
		return
	}
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		http.Error(w, "challenge unavailable", http.StatusInternalServerError)
		return
	}
	challenge := registrationChallenge{Source: source, ChainID: s.cfg.ChainID, Nonce: hex.EncodeToString(nonce[:]), Expires: now.Add(challengeTTL)}
	s.challenges[challenge.Nonce] = challenge
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, challenge)
}

func (s *server) verifyRegistration(proof signedRegistration, node Node, now time.Time) bool {
	if len(proof.PublicKey) != ed25519.PublicKeySize || len(proof.Signature) != ed25519.SignatureSize {
		return false
	}
	sum := sha256.Sum256(proof.PublicKey)
	if hex.EncodeToString(sum[:20]) != node.NodeID {
		return false
	}
	s.challengeMu.Lock()
	defer s.challengeMu.Unlock()
	challenge, ok := s.challenges[proof.Challenge]
	if !ok || !now.Before(challenge.Expires) || challenge.Source != node.IP || challenge.ChainID != s.cfg.ChainID {
		return false
	}
	if !ed25519.Verify(proof.PublicKey, registrationMessage(s.cfg.ChainID, proof.Challenge, node), proof.Signature) {
		return false
	}
	delete(s.challenges, proof.Challenge)
	return true
}

// ---- server ----------------------------------------------------------------

type server struct {
	cfg              config
	reg              *registry
	limiter          registrationLimiter
	challengeLimiter registrationLimiter
	challengeMu      sync.Mutex
	challenges       map[string]registrationChallenge
	rpc              *http.Client
}

// registrationLimiter admits registerBurst registrations per source address in
// each registerWindow, matching the Nginx hpx_register zone.
type registrationLimiter struct {
	mu      sync.Mutex
	windows map[string]registrationWindow
}

type registrationWindow struct {
	start time.Time
	count int
}

const (
	registerBurst      = 10
	registerWindow     = time.Minute
	registerMaxSources = 65536
)

func (l *registrationLimiter) allow(source string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.windows == nil {
		l.windows = map[string]registrationWindow{}
	}
	if len(l.windows) >= registerMaxSources {
		for key, window := range l.windows {
			if now.Sub(window.start) >= registerWindow {
				delete(l.windows, key)
			}
		}
		if len(l.windows) >= registerMaxSources {
			return false
		}
	}
	window := l.windows[source]
	if now.Sub(window.start) >= registerWindow {
		window = registrationWindow{start: now}
	}
	if window.count >= registerBurst {
		return false
	}
	window.count++
	l.windows[source] = window
	return true
}

func loopbackListen(addr string) bool {
	host, port, err := net.SplitHostPort(addr)
	if err != nil || port == "" {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func main() {
	cfg := loadConfig()
	if !loopbackListen(cfg.Addr) {
		log.Fatalf("registry: HPX_ADDR %q must be a loopback address and port behind the reverse proxy", cfg.Addr)
	}
	if len(cfg.SeedPeers) > seedPeerLimit {
		log.Fatalf("registry: at most %d seed peers are allowed", seedPeerLimit)
	}
	for _, seed := range cfg.SeedPeers {
		id, address, ok := strings.Cut(seed, "@")
		host, port, err := net.SplitHostPort(address)
		p, portErr := strconv.Atoi(port)
		if !ok || !isHexID(id) || err != nil || len(host) > 253 || host == "" || portErr != nil || p < 1 || p > 65535 {
			log.Fatal("registry: invalid seed peer")
		}
	}
	reg, err := openRegistry(cfg.DataDir)
	if err != nil {
		log.Fatalf("registry: %v", err)
	}
	s := &server{cfg: cfg, reg: reg}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/api/register", s.handleRegister)
	mux.HandleFunc("/api/register/challenge", s.handleRegistrationChallenge)
	mux.HandleFunc("/api/peers", s.handlePeers)
	mux.HandleFunc("/api/peers.txt", s.handlePeersTxt)
	mux.HandleFunc("/api/nodes", s.handleNodes)
	mux.HandleFunc("/api/myip", s.handleMyIP)
	mux.HandleFunc("/api/statesync", s.handleStateSync)

	// Everything else is an explicitly declared static artifact. Filesystem
	// traversal and directory indexes are intentionally unavailable.
	mux.Handle("/", artifactHandler(cfg.ArtifactsDir))

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           logreq(mux),
		ReadHeaderTimeout: 15 * time.Second,
	}
	log.Printf("hpx-registry listening on %s (artifacts=%s data=%s chain=%s seeds=%d)",
		cfg.Addr, cfg.ArtifactsDir, cfg.DataDir, cfg.ChainID, len(cfg.SeedPeers))
	log.Fatal(srv.ListenAndServe())
}

func (s *server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "chain_id": s.cfg.ChainID, "source_revision": sourceRevision,
		"evm_chain_id": evmChainID(s.cfg.ChainID), "protocol_version": "hpx-registry.v1",
	})
}

func evmChainID(chainID string) *uint64 {
	separator := strings.LastIndexByte(chainID, '_')
	if separator <= 0 {
		return nil
	}
	parts := strings.Split(chainID[separator+1:], "-")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil
	}
	for _, part := range parts {
		for _, digit := range part {
			if digit < '0' || digit > '9' {
				return nil
			}
		}
	}
	value, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil || value == 0 {
		return nil
	}
	if _, err := strconv.ParseUint(parts[1], 10, 64); err != nil {
		return nil
	}
	return &value
}

// peerStrings returns seeds first, then every registered node, de-duplicated.
// If exclude is non-empty, the node with that id is omitted (so a node never
// lists itself as a peer).
func (s *server) peerStrings(exclude string) []string {
	seen := map[string]bool{}
	var peers []string
	add := func(p string) {
		if p == "" || seen[p] || len(peers) >= peerResponseLimit {
			return
		}
		seen[p] = true
		peers = append(peers, p)
	}
	for _, p := range s.cfg.SeedPeers {
		// allow excluding a seed too, by id prefix
		if exclude != "" && strings.HasPrefix(p, exclude+"@") {
			continue
		}
		add(p)
	}
	for _, n := range s.reg.list() {
		if exclude != "" && n.NodeID == exclude {
			continue
		}
		add(n.Peer())
	}
	return peers
}

func (s *server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.cfg.RegisterTok != "" && subtle.ConstantTimeCompare([]byte(r.Header.Get("X-HPX-Token")), []byte(s.cfg.RegisterTok)) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !s.limiter.allow(clientIP(r), time.Now()) {
		http.Error(w, "registration rate limit exceeded", http.StatusTooManyRequests)
		return
	}
	var registration signedRegistration
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16))
	if err := decoder.Decode(&registration); err != nil {
		http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		http.Error(w, "expected a single registration", http.StatusBadRequest)
		return
	}
	n := registration.Node
	n.FirstSeen, n.LastSeen = "", ""
	n.NodeID = strings.ToLower(strings.TrimSpace(n.NodeID))
	if !isHexID(n.NodeID) {
		http.Error(w, "invalid node_id (expect 40-char hex CometBFT id)", http.StatusBadRequest)
		return
	}
	// The peer address is always the address observed by the trusted local
	// reverse proxy. A caller-provided address must never poison peer discovery.
	n.IP = clientIP(r)
	if !validPublicIP(n.IP) {
		http.Error(w, "registration requires a public source address", http.StatusBadRequest)
		return
	}
	if n.P2PPort == 0 {
		n.P2PPort = 26656
	}
	if n.P2PPort < 1 || n.P2PPort > 65535 {
		http.Error(w, "invalid p2p_port", http.StatusBadRequest)
		return
	}
	n.Moniker = strings.TrimSpace(n.Moniker)
	n.Version = strings.TrimSpace(n.Version)
	if len(n.Moniker) > 64 || strings.IndexFunc(n.Moniker, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
		http.Error(w, "invalid moniker", http.StatusBadRequest)
		return
	}
	if len(n.Version) > 64 || strings.IndexFunc(n.Version, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
		http.Error(w, "invalid version", http.StatusBadRequest)
		return
	}
	switch n.Type {
	case "fullnode", "validator":
	default:
		n.Type = "fullnode"
	}
	if !s.verifyRegistration(registration, n, time.Now()) {
		http.Error(w, "invalid or expired node ownership proof", http.StatusUnauthorized)
		return
	}
	if err := s.reg.upsert(&n); err != nil {
		if errors.Is(err, errRegistryFull) {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		http.Error(w, "save failed", http.StatusInternalServerError)
		return
	}
	log.Printf("registered %s (%s) %s type=%s ver=%s", n.NodeID, n.Moniker, n.Peer(), n.Type, n.Version)
	peers := s.peerStrings(n.NodeID)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":               true,
		"node":             n,
		"peers":            peers,
		"persistent_peers": strings.Join(peers, ","),
	})
}

func (s *server) handlePeers(w http.ResponseWriter, r *http.Request) {
	self := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("self")))
	peers := s.peerStrings(self)
	writeJSON(w, http.StatusOK, map[string]any{
		"chain_id":         s.cfg.ChainID,
		"count":            len(peers),
		"seeds":            s.cfg.SeedPeers,
		"peers":            peers,
		"persistent_peers": strings.Join(peers, ","),
	})
}

func (s *server) handlePeersTxt(w http.ResponseWriter, r *http.Request) {
	self := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("self")))
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprint(w, strings.Join(s.peerStrings(self), ","))
}

func (s *server) handleNodes(w http.ResponseWriter, _ *http.Request) {
	nodes := s.reg.list()
	writeJSON(w, http.StatusOK, map[string]any{
		"chain_id": s.cfg.ChainID,
		"count":    len(nodes),
		"nodes":    nodes,
	})
}

func (s *server) handleMyIP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprint(w, clientIP(r))
}

// handleStateSync returns config.toml [statesync] parameters for a fresh node:
// a recent trusted light-client anchor (height+hash) plus the rpc-servers to
// fetch a snapshot from. The chain is pruned, so blocksync-from-genesis is
// impossible; new nodes MUST state sync.
func (s *server) handleStateSync(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	disabled := func(reason string) { writeJSON(w, http.StatusOK, map[string]any{"enable": false, "reason": reason}) }
	if s.cfg.StateSyncRPC == "" || s.cfg.StateSyncServers == "" {
		disabled("no state-sync rpc configured")
		return
	}
	if s.cfg.ChainID == "" || s.cfg.TrustOffset < 0 {
		disabled("invalid state-sync configuration")
		return
	}
	client := s.rpc
	if client == nil {
		client = rpcClient
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	latest, err := rpcLatestHeight(ctx, client, s.cfg.StateSyncRPC, s.cfg.ChainID)
	if err != nil {
		disabled("status query failed: " + err.Error())
		return
	}
	trustHeight := latest - int64(s.cfg.TrustOffset)
	if trustHeight < 1 {
		trustHeight = 1
	}
	hash, err := rpcBlockHash(ctx, client, s.cfg.StateSyncRPC, s.cfg.ChainID, trustHeight)
	if err != nil {
		disabled("commit query failed: " + err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"enable": true, "rpc_servers": s.cfg.StateSyncServers, "trust_height": trustHeight,
		"trust_hash": hash, "trust_period": s.cfg.TrustPeriod, "latest": latest,
	})
}

var rpcClient = &http.Client{Timeout: 8 * time.Second}

const rpcResponseLimit = 1 << 20

func rpcGet(ctx context.Context, client *http.Client, base, path string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+path, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upstream HTTP status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, rpcResponseLimit+1))
	if err != nil {
		return err
	}
	if len(body) > rpcResponseLimit {
		return errors.New("RPC response exceeds size limit")
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return err
	}
	if len(envelope) == 0 {
		return errors.New("empty RPC object")
	}
	if failure, ok := envelope["error"]; ok && string(failure) != "null" {
		return errors.New("upstream RPC error")
	}
	if result, ok := envelope["result"]; ok {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(result, &object); err != nil || len(object) == 0 {
			return errors.New("invalid RPC result")
		}
		body = result
	}
	return json.Unmarshal(body, target)
}

func rpcLatestHeight(ctx context.Context, client *http.Client, base, chain string) (int64, error) {
	var status struct {
		NodeInfo struct {
			Network string `json:"network"`
		} `json:"node_info"`
		SyncInfo struct {
			Latest int64 `json:"latest_block_height,string"`
		} `json:"sync_info"`
	}
	if err := rpcGet(ctx, client, base, "/status", &status); err != nil {
		return 0, err
	}
	if status.NodeInfo.Network != chain || status.SyncInfo.Latest <= 0 {
		return 0, errors.New("invalid status chain or height")
	}
	return status.SyncInfo.Latest, nil
}

func rpcBlockHash(ctx context.Context, client *http.Client, base, chain string, height int64) (string, error) {
	var response struct {
		SignedHeader struct {
			Header struct {
				ChainID string `json:"chain_id"`
				Height  int64  `json:"height,string"`
			} `json:"header"`
			Commit struct {
				Height  int64 `json:"height,string"`
				BlockID struct {
					Hash string `json:"hash"`
				} `json:"block_id"`
			} `json:"commit"`
		} `json:"signed_header"`
	}
	if err := rpcGet(ctx, client, base, "/commit?height="+strconv.FormatInt(height, 10), &response); err != nil {
		return "", err
	}
	header := response.SignedHeader
	if header.Header.ChainID != chain || header.Header.Height != height || header.Commit.Height != height {
		return "", errors.New("commit chain or height mismatch")
	}
	hash, err := hex.DecodeString(header.Commit.BlockID.Hash)
	if err != nil || len(hash) != sha256.Size {
		return "", errors.New("invalid 32-byte commit hash")
	}
	return strings.ToUpper(header.Commit.BlockID.Hash), nil
}

// ---- helpers ---------------------------------------------------------------

func isHexID(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

func validPublicIP(raw string) bool {
	ip := net.ParseIP(strings.TrimSpace(raw))
	return ip != nil && ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() &&
		!ip.IsUnspecified() && !ip.IsLinkLocalUnicast()
}

// clientIP returns the address observed by the trusted local reverse proxy.
// Forwarding headers are honoured only on loopback connections, where Nginx
// or the unified gateway overwrites X-Real-IP with the peer it accepted; any
// other connection is identified by its own TCP source address.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		if forwarded := strings.TrimSpace(r.Header.Get("X-Real-IP")); forwarded != "" {
			return forwarded
		}
	}
	return host
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// noCacheScripts disables caching for the install scripts + chain-info so a
// re-publish is picked up immediately; large immutable blobs keep default caching.
func noCacheScripts(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, ".sh"),
			r.URL.Path == "/hpx",
			r.URL.Path == "/install",
			r.URL.Path == "/checksums.txt",
			r.URL.Path == "/paxd.sha256",
			strings.HasSuffix(r.URL.Path, ".json"),
			strings.HasSuffix(r.URL.Path, ".toml"):
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

var publicArtifacts = map[string]string{
	"/chain-info.json":              "chain-info.json",
	"/checksums.txt":                "checksums.txt",
	"/config/fullnode/app.toml":     "config/fullnode/app.toml",
	"/config/fullnode/config.toml":  "config/fullnode/config.toml",
	"/config/validator/app.toml":    "config/validator/app.toml",
	"/config/validator/config.toml": "config/validator/config.toml",
	"/genesis.json":                 "genesis.json",
	"/get-hpx.sh":                   "get-hpx.sh",
	"/hpx":                          "hpx",
	"/install":                      "get-hpx.sh",
	"/install.sh":                   "install.sh",
	"/lib/libwasmvm.aarch64.so":     "lib/libwasmvm.aarch64.so",
	"/lib/libwasmvm.x86_64.so":      "lib/libwasmvm.x86_64.so",
	"/lib/libwasmvm152.aarch64.so":  "lib/libwasmvm152.aarch64.so",
	"/lib/libwasmvm152.x86_64.so":   "lib/libwasmvm152.x86_64.so",
	"/lib/libwasmvm155.aarch64.so":  "lib/libwasmvm155.aarch64.so",
	"/lib/libwasmvm155.x86_64.so":   "lib/libwasmvm155.x86_64.so",
	"/paxd":                         "paxd",
	"/paxd.sha256":                  "paxd.sha256",
	"/uninstall":                    "uninstall.sh",
	"/uninstall.sh":                 "uninstall.sh",
}

func artifactHandler(root string) http.Handler {
	return noCacheScripts(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		rel, ok := publicArtifacts[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		file := filepath.Join(root, filepath.FromSlash(rel))
		info, err := os.Stat(file)
		if err != nil || !info.Mode().IsRegular() {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.ServeFile(w, r, file)
	}))
}

func logreq(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", clientIP(r), r.Method, r.URL.Path)
	})
}
