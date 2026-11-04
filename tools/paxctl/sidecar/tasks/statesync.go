package tasks

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	tmtypes "github.com/sei-protocol/sei-chain/sei-tendermint/types"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sei-protocol/seictl/internal/patch"
	"github.com/sei-protocol/seictl/sidecar/engine"
	"github.com/sei-protocol/seictl/sidecar/rpc"
	"github.com/sei-protocol/seilog"
)

var ssLog = seilog.NewLogger("seictl", "task", "state-sync")

const (
	stateSyncMarkerFile = ".sei-sidecar-statesync-done"
	trustHeightOffset   = 2000
	rpcPort             = "26657"
	witnessProbeTimeout = 10 * time.Second
	tlsPort             = "443"
)

// StateSyncConfig holds the trust point and RPC servers for Tendermint state sync.
type StateSyncConfig struct {
	TrustHeight      int64
	TrustHash        string
	TrustPeriod      string
	RpcServers       string
	UseLocalSnapshot bool
	BackfillBlocks   int64
}

// StateSyncConfigurer discovers a trust point from peers and writes the config file.
type StateSyncConfigurer struct {
	homeDir    string
	httpClient rpc.HTTPDoer
}

// NewStateSyncConfigurer creates a configurer targeting the given home directory.
func NewStateSyncConfigurer(homeDir string, client rpc.HTTPDoer) *StateSyncConfigurer {
	if client == nil {
		client = &http.Client{}
	}
	return &StateSyncConfigurer{homeDir: homeDir, httpClient: client}
}

// Handler returns an engine.TaskHandler.
func (s *StateSyncConfigurer) Handler() engine.TaskHandler {
	return engine.TypedHandler(func(ctx context.Context, params StateSyncRequest) error {
		return s.Configure(ctx, params)
	})
}

// StateSyncRequest groups the caller-provided parameters for state-sync configuration.
type StateSyncRequest struct {
	UseLocalSnapshot bool   `json:"useLocalSnapshot"`
	TrustPeriod      string `json:"trustPeriod"`
	BackfillBlocks   int64  `json:"backfillBlocks"`
	// RpcServers are explicit light-client witness endpoints ("host:port").
	// When non-empty they are used verbatim; otherwise witnesses are derived
	// from persistent-peers.
	RpcServers []string `json:"rpcServers"`
}

// Configure determines the state-sync light-client witnesses, queries one for a
// trust point, and writes the settings to config.toml.
//
// Witnesses come from p.RpcServers when provided, otherwise are derived from
// persistent-peers. Only witnesses that answer /status are written: a peer that
// serves P2P but not RPC (e.g. an external P2P NLB hostname) would otherwise make
// seid exit on "no witnesses connected" and crashloop. With UseLocalSnapshot the
// trust height comes from the restored snapshot instead of a query.
func (s *StateSyncConfigurer) Configure(ctx context.Context, p StateSyncRequest) error {
	cfg, err := s.Preflight(ctx, p)
	if err != nil {
		return err
	}

	ssLog.Info("writing config", "trust-height", cfg.TrustHeight, "trust-hash", cfg.TrustHash,
		"trust-period", p.TrustPeriod, "rpc-servers", cfg.RpcServers,
		"use-local-snapshot", p.UseLocalSnapshot, "backfill-blocks", p.BackfillBlocks)
	if err := writeStateSyncToConfig(s.homeDir, cfg); err != nil {
		return fmt.Errorf("configure-state-sync: writing config.toml: %w", err)
	}

	return writeMarker(s.homeDir, stateSyncMarkerFile)
}

// Preflight establishes independent witnesses before either configuration or erasure.
func (s *StateSyncConfigurer) Preflight(ctx context.Context, p StateSyncRequest) (StateSyncConfig, error) {
	fail := func(err error) (StateSyncConfig, error) {
		return StateSyncConfig{}, fmt.Errorf("configure-state-sync: %w", err)
	}
	chainID := os.Getenv("SEI_CHAIN_ID")
	if doc, err := tmtypes.GenesisDocFromFile(filepath.Join(s.homeDir, "config", "genesis.json")); err == nil {
		if chainID != "" && doc.ChainID != chainID {
			return fail(fmt.Errorf("configured chain differs from genesis"))
		}
		chainID = doc.ChainID
	}
	if chainID == "" {
		return fail(fmt.Errorf("configured chain ID required"))
	}
	candidates, err := s.witnessCandidates(p)
	if err != nil {
		return fail(err)
	}
	seenEndpoint := map[string]bool{}
	seenID := map[string]bool{}
	var reachable []string
	var latest int64
	for _, ep := range candidates {
		host, port, err := net.SplitHostPort(ep)
		if err != nil || host == "" || port == "" {
			return fail(fmt.Errorf("invalid witness %q", ep))
		}
		ep = net.JoinHostPort(strings.ToLower(strings.TrimSuffix(host, ".")), port)
		if seenEndpoint[ep] {
			continue
		}
		seenEndpoint[ep] = true
		pctx, cancel := context.WithTimeout(ctx, witnessProbeTimeout)
		raw, err := s.rpcClientForEndpoint(ep).Get(pctx, "/status")
		cancel()
		if err != nil {
			continue
		}
		var status rpc.StatusResult
		if err := json.Unmarshal(raw, &status); err != nil {
			return fail(err)
		}
		if status.NodeInfo.Network != chainID {
			return fail(fmt.Errorf("witness %s is on chain %q, expected %q", ep, status.NodeInfo.Network, chainID))
		}
		if len(status.NodeInfo.ID) != 40 {
			return fail(fmt.Errorf("witness %s has invalid node identity", ep))
		}
		if _, err := hex.DecodeString(status.NodeInfo.ID); err != nil {
			return fail(err)
		}
		id := strings.ToLower(status.NodeInfo.ID)
		if seenID[id] {
			return fail(fmt.Errorf("witnesses share node identity %s", id))
		}
		seenID[id] = true
		h, err := strconv.ParseInt(status.SyncInfo.LatestBlockHeight, 10, 64)
		if err != nil || h <= 0 {
			return fail(fmt.Errorf("witness %s has invalid height", ep))
		}
		if latest == 0 || h < latest {
			latest = h
		}
		reachable = append(reachable, ep)
	}
	if len(reachable) < 2 {
		return fail(fmt.Errorf("at least two distinct reachable RPC witnesses required"))
	}
	height := latest - trustHeightOffset
	if height < 1 {
		height = 1
	}
	if p.UseLocalSnapshot {
		height, err = discoverLocalSnapshotHeight(s.homeDir)
		if err != nil {
			return fail(fmt.Errorf("discovering local snapshot height: %w", err))
		}
	}
	var hash string
	for _, ep := range reachable {
		pctx, cancel := context.WithTimeout(ctx, witnessProbeTimeout)
		got, err := s.queryVerifiedBlock(pctx, ep, height, chainID)
		cancel()
		if err != nil {
			return fail(err)
		}
		if hash != "" && !strings.EqualFold(hash, got) {
			return fail(fmt.Errorf("witness trust hashes disagree at height %d", height))
		}
		hash = got
	}
	return StateSyncConfig{TrustHeight: height, TrustHash: hash, TrustPeriod: p.TrustPeriod, RpcServers: strings.Join(reachable, ","), UseLocalSnapshot: p.UseLocalSnapshot, BackfillBlocks: p.BackfillBlocks}, nil
}

func (s *StateSyncConfigurer) queryVerifiedBlock(ctx context.Context, ep string, height int64, chainID string) (string, error) {
	raw, err := s.rpcClientForEndpoint(ep).Get(ctx, "/block?height="+url.QueryEscape(strconv.FormatInt(height, 10)))
	if err != nil {
		return "", err
	}
	var block struct {
		BlockID rpc.BlockID `json:"block_id"`
		Block   struct {
			Header struct {
				ChainID string `json:"chain_id"`
				Height  string `json:"height"`
			} `json:"header"`
		} `json:"block"`
	}
	if err := json.Unmarshal(raw, &block); err != nil {
		return "", err
	}
	if block.Block.Header.ChainID != chainID || block.Block.Header.Height != strconv.FormatInt(height, 10) {
		return "", fmt.Errorf("witness returned a different chain or height")
	}
	if block.BlockID.Hash == "" {
		return "", fmt.Errorf("empty block hash at height %d", height)
	}
	if len(block.BlockID.Hash) != 64 {
		return "", fmt.Errorf("unexpected block hash length at height %d", height)
	}
	if _, err := hex.DecodeString(block.BlockID.Hash); err != nil {
		return "", err
	}
	return block.BlockID.Hash, nil
}

// witnessCandidates returns the candidate witness endpoints ("host:port"):
// caller-provided RpcServers verbatim, otherwise each persistent-peer host with
// the RPC port attached. The peer-derived form holds for peers that serve RPC on
// the P2P host (EC2 peers, internal cluster DNS); reachableWitnesses drops those
// that don't.
func (s *StateSyncConfigurer) witnessCandidates(p StateSyncRequest) ([]string, error) {
	if len(p.RpcServers) > 0 {
		ssLog.Info("using caller-provided rpc witnesses", "count", len(p.RpcServers))
		return p.RpcServers, nil
	}

	peers, err := readPeersFromConfig(s.homeDir)
	if err != nil {
		return nil, err
	}
	if len(peers) == 0 {
		return nil, fmt.Errorf("no peers in config.toml")
	}
	ssLog.Debug("found peers", "count", len(peers))

	hosts := extractRPCHosts(peers, 2)
	if len(hosts) == 0 {
		return nil, fmt.Errorf("could not extract RPC hosts from peers")
	}
	endpoints := make([]string, len(hosts))
	for i, h := range hosts {
		endpoints[i] = h + ":" + rpcPort
	}
	return endpoints, nil
}

// reachableWitnesses returns the candidate endpoints whose /status responds.
func (s *StateSyncConfigurer) reachableWitnesses(ctx context.Context, candidates []string) []string {
	reachable := make([]string, 0, len(candidates))
	for _, ep := range candidates {
		if err := s.probeWitness(ctx, ep); err != nil {
			ssLog.Warn("state-sync witness unreachable, skipping", "endpoint", ep, "err", err)
			continue
		}
		reachable = append(reachable, ep)
	}
	return reachable
}

// probeWitness reports whether endpoint answers /status. The context deadline is
// load-bearing: the sidecar's http.Client has no Timeout, so a black-holed
// endpoint (TCP accepted, no response) is bounded only by this cancellation.
func (s *StateSyncConfigurer) probeWitness(ctx context.Context, endpoint string) error {
	pctx, cancel := context.WithTimeout(ctx, witnessProbeTimeout)
	defer cancel()
	_, err := s.rpcClientForEndpoint(endpoint).Get(pctx, "/status")
	return err
}

// discoverLocalSnapshotHeight scans the Tendermint snapshots directory for the
// highest available snapshot height. Snapshots are stored as
// <home>/data/snapshots/<height>/<format>/.
func discoverLocalSnapshotHeight(homeDir string) (int64, error) {
	snapshotDir := filepath.Join(homeDir, "data", "snapshots")
	entries, err := os.ReadDir(snapshotDir)
	if err != nil {
		return 0, fmt.Errorf("reading snapshots directory: %w", err)
	}

	var maxHeight int64
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		h, err := strconv.ParseInt(e.Name(), 10, 64)
		if err != nil {
			continue
		}
		if h > maxHeight {
			maxHeight = h
		}
	}

	if maxHeight == 0 {
		return 0, fmt.Errorf("no snapshot found in %s", snapshotDir)
	}
	return maxHeight, nil
}

// extractRPCHosts extracts up to maxHosts host addresses from peer strings
// in "nodeId@host:port" format.
func extractRPCHosts(peers []string, maxHosts int) []string {
	var hosts []string
	for _, p := range peers {
		if len(hosts) >= maxHosts {
			break
		}
		parts := strings.SplitN(p, "@", 2)
		if len(parts) != 2 {
			continue
		}
		hostPort := parts[1]
		host := hostPort
		if idx := strings.LastIndex(hostPort, ":"); idx >= 0 {
			host = hostPort[:idx]
		}
		if host != "" {
			hosts = append(hosts, host)
		}
	}
	return hosts
}

// rpcClientForEndpoint builds an rpc.Client targeting a full "host:port" RPC
// endpoint. The scheme is derived from the port: a :443 witness is a public TLS
// gateway (Istio HTTPRoute) and must be reached over https; every other port is
// the plaintext in-cluster CometBFT RPC. Hardcoding http:// here previously made
// a :443 witness fail the /status probe with an immediate EOF (plaintext request
// to a TLS listener), which blocked every new state-syncing node.
func (s *StateSyncConfigurer) rpcClientForEndpoint(endpoint string) *rpc.Client {
	return rpc.NewClient(witnessScheme(endpoint)+"://"+endpoint, s.httpClient)
}

// witnessScheme returns the URL scheme for a "host:port" witness endpoint:
// https for :443, http otherwise. An endpoint with no parseable port defaults
// to http (the in-cluster plaintext form).
func witnessScheme(endpoint string) string {
	if _, port, err := net.SplitHostPort(endpoint); err == nil && port == tlsPort {
		return "https"
	}
	return "http"
}

func (s *StateSyncConfigurer) queryLatestHeight(ctx context.Context, endpoint string) (int64, error) {
	raw, err := s.rpcClientForEndpoint(endpoint).Get(ctx, "/status")
	if err != nil {
		return 0, err
	}

	var status rpc.StatusResult
	if err := json.Unmarshal(raw, &status); err != nil {
		return 0, fmt.Errorf("parsing status response: %w", err)
	}

	height, err := strconv.ParseInt(status.SyncInfo.LatestBlockHeight, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing height %q: %w", status.SyncInfo.LatestBlockHeight, err)
	}
	return height, nil
}

func (s *StateSyncConfigurer) queryBlockHash(ctx context.Context, endpoint string, height int64) (string, error) {
	path := fmt.Sprintf("/block?height=%d", height)
	raw, err := s.rpcClientForEndpoint(endpoint).Get(ctx, path)
	if err != nil {
		return "", err
	}

	var block rpc.BlockResult
	if err := json.Unmarshal(raw, &block); err != nil {
		return "", fmt.Errorf("parsing block response: %w", err)
	}
	hash := block.BlockID.Hash
	if hash == "" {
		return "", fmt.Errorf("empty block hash at height %d", height)
	}
	const sha256HexLen = 64
	if len(hash) != sha256HexLen {
		return "", fmt.Errorf("unexpected block hash length at height %d: got %d, want %d", height, len(hash), sha256HexLen)
	}
	return hash, nil
}

func writeStateSyncToConfig(homeDir string, cfg StateSyncConfig) error {
	configPath := filepath.Join(homeDir, "config", "config.toml")
	ss := map[string]any{
		"enable":             true,
		"trust-height":       cfg.TrustHeight,
		"trust-hash":         cfg.TrustHash,
		"rpc-servers":        cfg.RpcServers,
		"use-local-snapshot": cfg.UseLocalSnapshot,
	}
	if cfg.TrustPeriod != "" {
		ss["trust-period"] = cfg.TrustPeriod
	}
	if cfg.BackfillBlocks > 0 {
		ss["backfill-blocks"] = cfg.BackfillBlocks
	}
	return mergeAndWrite(configPath, map[string]any{"statesync": ss})
}

// readPeersFromConfig reads the persistent-peers value from config.toml and
// splits it into individual peer strings.
func readPeersFromConfig(homeDir string) ([]string, error) {
	configPath := filepath.Join(homeDir, "config", "config.toml")
	doc, err := patch.ReadTOML(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading config.toml: %w", err)
	}

	p2p, ok := doc["p2p"].(map[string]any)
	if !ok {
		return nil, nil
	}

	raw, _ := p2p["persistent-peers"].(string)
	if raw == "" {
		return nil, nil
	}

	var peers []string
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			peers = append(peers, p)
		}
	}
	return peers, nil
}
