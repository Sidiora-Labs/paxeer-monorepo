package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/auth/agent"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/server"
)

const (
	EnvGatewayURL             = "WALLET_INVENTORY_GATEWAY_URL"
	EnvAdminTokenFile         = "WALLET_INVENTORY_ADMIN_TOKEN_FILE"
	EnvPrivateKeyFile         = "WALLET_INVENTORY_PRIVATE_KEY_FILE"
	EnvIssuer                 = "WALLET_INVENTORY_ISSUER"
	EnvTenant                 = "WALLET_INVENTORY_TENANT"
	EnvStateDir               = "WALLET_INVENTORY_STATE_DIR"
	EnvInterval               = "WALLET_INVENTORY_INTERVAL"
	EnvRenewalMargin          = "WALLET_INVENTORY_RENEWAL_MARGIN"
	EnvMaxGenerationsPerHour  = "WALLET_INVENTORY_MAX_GENERATIONS_PER_HOUR"
	EnvMaxKeysPerOwner        = "WALLET_INVENTORY_MAX_KEYS_PER_OWNER"
	EnvListenAddr             = "WALLET_INVENTORY_LISTEN_ADDR"
	EnvPort                   = "PORT"
	defaultServeInterval      = time.Minute
	defaultRenewalMargin      = 10 * 24 * time.Hour
	defaultMaxGenerationsHour = 100
	defaultMaxKeysOwner       = 1000
)

const (
	stateInventoryName = "inventory.jwt"
	stateJournalName   = "journal.ndjson"
	requestsPath       = "/v1/admin/custody-inventory/requests"
	admitPath          = "/v1/admin/custody-inventory"
)

var (
	walletKeyIDPattern  = regexp.MustCompile(`^wallet:[^:]+:[^:]+:secp256k1:[0-9]+$`)
	accountKeyIDPattern = regexp.MustCompile(`^account:[^:]+:[^:]+:ed25519:[0-9]+$`)
)

type serveSettings struct {
	gatewayURL         string
	adminToken         string
	privateKeyFile     string
	issuer             string
	tenant             string
	stateDir           string
	interval           time.Duration
	renewalMargin      time.Duration
	maxGenerationsHour int
	maxKeysOwner       int
	listenAddr         string
}

type inventoryService struct {
	settings serveSettings
	private  privateKey
	verifier *agent.SignedVerifier
	client   *http.Client

	mu        sync.RWMutex
	lastRound time.Time
}

// privateKey keeps the signing key's concrete type out of the service state.
// The command's existing loader enforces the ownership and mode requirements.
type privateKey = ecdsa.PrivateKey

type requestEnvelope struct {
	InventoryRequests []server.InventoryKey `json:"inventory_requests"`
}

type postMember struct {
	NodeID   string `json:"node_id"`
	Sequence string `json:"sequence"`
	Error    string `json:"error"`
}

type postResponse struct {
	Sequence string       `json:"sequence"`
	Members  []postMember `json:"members"`
	Retained *bool        `json:"retained"`
}

type journalLine struct {
	Kind       string   `json:"kind"`
	Time       string   `json:"time"`
	Sequence   string   `json:"sequence"`
	KeyID      string   `json:"key_id,omitempty"`
	Owner      string   `json:"owner,omitempty"`
	Curve      string   `json:"curve,omitempty"`
	Operations []string `json:"operations,omitempty"`
	Unapproved int      `json:"unapproved,omitempty"`
}

func runServe(ctx context.Context, getenv func(string) string) error {
	return runServeWithReady(ctx, getenv, nil)
}

func runServeWithReady(ctx context.Context, getenv func(string) string, ready func(string)) error {
	if getenv == nil {
		getenv = os.Getenv
	}
	settings, err := loadServeSettings(getenv)
	if err != nil {
		return err
	}
	key, err := loadPrivateKey(settings.privateKeyFile)
	if err != nil {
		return err
	}
	verifier := &agent.SignedVerifier{Key: &key.PublicKey, Issuer: settings.issuer, Tenant: settings.tenant}
	service := &inventoryService{
		settings: settings,
		private:  *key,
		verifier: verifier,
		client:   &http.Client{Timeout: 15 * time.Second},
	}
	return service.serve(ctx, ready)
}

func loadServeSettings(getenv func(string) string) (serveSettings, error) {
	setting := func(name string, aliases ...string) string {
		if value := strings.TrimSpace(getenv(name)); value != "" {
			return value
		}
		for _, alias := range aliases {
			if value := strings.TrimSpace(getenv(alias)); value != "" {
				return value
			}
		}
		return ""
	}
	settings := serveSettings{
		gatewayURL:         strings.TrimRight(setting(EnvGatewayURL, "WALLET_INVENTORY_GATEWAY"), "/"),
		adminToken:         setting(EnvAdminTokenFile, "WALLET_INVENTORY_BEARER_FILE"),
		privateKeyFile:     setting(EnvPrivateKeyFile, "WALLET_INVENTORY_KEY_FILE"),
		issuer:             setting(EnvIssuer),
		tenant:             setting(EnvTenant),
		stateDir:           setting(EnvStateDir),
		interval:           defaultServeInterval,
		renewalMargin:      defaultRenewalMargin,
		maxGenerationsHour: defaultMaxGenerationsHour,
		maxKeysOwner:       defaultMaxKeysOwner,
	}
	for name, value := range map[string]string{
		EnvGatewayURL:     settings.gatewayURL,
		EnvAdminTokenFile: settings.adminToken,
		EnvPrivateKeyFile: settings.privateKeyFile,
		EnvIssuer:         settings.issuer,
		EnvTenant:         settings.tenant,
		EnvStateDir:       settings.stateDir,
	} {
		if value == "" {
			return serveSettings{}, fmt.Errorf("%s is required", name)
		}
	}
	adminTokenRaw, err := readBounded(settings.adminToken, 1<<20)
	if err != nil || strings.TrimSpace(string(adminTokenRaw)) == "" {
		return serveSettings{}, fmt.Errorf("%s: bearer file is unavailable", EnvAdminTokenFile)
	}
	settings.adminToken = strings.TrimSpace(string(adminTokenRaw))
	u, err := url.Parse(settings.gatewayURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
		return serveSettings{}, fmt.Errorf("%s must be an absolute HTTP(S) URL", EnvGatewayURL)
	}
	if settings.interval, err = parseServeDuration(getenv(EnvInterval), defaultServeInterval); err != nil {
		return serveSettings{}, fmt.Errorf("%s: %w", EnvInterval, err)
	}
	if settings.renewalMargin, err = parseServeDuration(getenv(EnvRenewalMargin), defaultRenewalMargin); err != nil {
		return serveSettings{}, fmt.Errorf("%s: %w", EnvRenewalMargin, err)
	}
	if settings.maxGenerationsHour, err = parseServeLimit(getenv(EnvMaxGenerationsPerHour), defaultMaxGenerationsHour); err != nil {
		return serveSettings{}, fmt.Errorf("%s: %w", EnvMaxGenerationsPerHour, err)
	}
	if settings.maxKeysOwner, err = parseServeLimit(getenv(EnvMaxKeysPerOwner), defaultMaxKeysOwner); err != nil {
		return serveSettings{}, fmt.Errorf("%s: %w", EnvMaxKeysPerOwner, err)
	}
	listen := strings.TrimSpace(getenv(EnvListenAddr))
	if listen == "" {
		port := strings.TrimSpace(getenv(EnvPort))
		if port == "" {
			port = "8080"
		}
		if n, parseErr := strconv.Atoi(port); parseErr != nil || n < 1 || n > 65535 {
			return serveSettings{}, fmt.Errorf("%s must be a valid TCP port", EnvPort)
		}
		listen = ":" + port
	}
	settings.listenAddr = listen
	return settings, nil
}

func parseServeDuration(raw string, fallback time.Duration) (time.Duration, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil || d <= 0 {
		return 0, errors.New("must be a positive duration")
	}
	return d, nil
}

func parseServeLimit(raw string, fallback int) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < 1 || n > 65536 {
		return 0, errors.New("must be an integer from 1 through 65536")
	}
	return n, nil
}

func (s *inventoryService) serve(ctx context.Context, ready func(string)) error {
	if err := os.MkdirAll(s.settings.stateDir, 0o700); err != nil {
		return fmt.Errorf("state directory: %w", err)
	}
	listener, err := net.Listen("tcp", s.settings.listenAddr)
	if err != nil {
		return fmt.Errorf("inventory listener: %w", err)
	}
	serverHTTP := &http.Server{Handler: s.healthHandler(), ReadHeaderTimeout: 5 * time.Second}
	serveErr := make(chan error, 1)
	go func() { serveErr <- serverHTTP.Serve(listener) }()
	if ready != nil {
		ready(listener.Addr().String())
	}

	go func() {
		_ = s.round(ctx)
		ticker := time.NewTicker(s.settings.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = s.round(ctx)
			}
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = serverHTTP.Shutdown(shutdownCtx)
		return nil
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func (s *inventoryService) healthHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		s.mu.RLock()
		last := s.lastRound
		s.mu.RUnlock()
		if last.IsZero() || time.Since(last) > 3*s.settings.interval {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok\n")
	})
	return mux
}

func (s *inventoryService) round(ctx context.Context) error {
	current, err := readInventory(filepath.Join(s.settings.stateDir, stateInventoryName), s.verifier)
	if err != nil {
		return err
	}
	requests, err := s.fetchRequests(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	candidate, approvals, unapproved, changed, err := s.approve(current, requests, now)
	if err != nil {
		return err
	}
	renew := current.ExpiresAt-now.Unix() <= int64(s.settings.renewalMargin/time.Second)
	if !changed && !renew {
		if err := s.appendJournal(journalLine{Kind: "round", Time: now.UTC().Format(time.RFC3339Nano), Sequence: current.Sequence, Unapproved: unapproved}); err != nil {
			return err
		}
		s.markRound(time.Now())
		return nil
	}
	sequence, err := nextInventorySequence(current.Sequence)
	if err != nil {
		return err
	}
	candidate.Sequence = sequence
	if renew {
		candidate.IssuedAt = now.Unix()
		candidate.ExpiresAt = candidate.IssuedAt + int64(maxLifetime/time.Second)
	}
	token, err := encodeInventory(&s.private, candidate)
	if err != nil {
		return err
	}
	if err := s.postConfirmed(ctx, token, candidate); err != nil {
		return err
	}
	if err := writeInventory(filepath.Join(s.settings.stateDir, stateInventoryName), []byte(token+"\n")); err != nil {
		return err
	}
	for _, key := range approvals {
		if err := s.appendJournal(journalLine{Kind: "approval", Time: now.UTC().Format(time.RFC3339Nano), Sequence: candidate.Sequence, KeyID: key.KeyID, Owner: key.Owner, Curve: key.Curve, Operations: append([]string(nil), key.Operations...)}); err != nil {
			return err
		}
	}
	if err := s.appendJournal(journalLine{Kind: "round", Time: now.UTC().Format(time.RFC3339Nano), Sequence: candidate.Sequence, Unapproved: unapproved}); err != nil {
		return err
	}
	s.markRound(time.Now())
	return nil
}

func nextInventorySequence(sequence string) (string, error) {
	n, err := strconv.ParseUint(sequence, 10, 64)
	if err != nil || n == ^uint64(0) {
		return "", errors.New("inventory sequence exhausted")
	}
	return strconv.FormatUint(n+1, 10), nil
}

func (s *inventoryService) fetchRequests(ctx context.Context) ([]server.InventoryKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.settings.gatewayURL+requestsPath, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.settings.adminToken)
	response, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gateway request list returned status %d", response.StatusCode)
	}
	var envelope requestEnvelope
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxTokenBytes+1))
	if err := decoder.Decode(&envelope); err != nil {
		return nil, errors.New("gateway request list is invalid")
	}
	if len(envelope.InventoryRequests) > 65536 {
		return nil, errors.New("gateway request list exceeds key limit")
	}
	return envelope.InventoryRequests, nil
}

func (s *inventoryService) postConfirmed(ctx context.Context, token string, candidate server.WalletInventory) error {
	body, err := json.Marshal(struct {
		Token string `json:"token"`
	}{Token: token})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.settings.gatewayURL+admitPath, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.settings.adminToken)
	req.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("gateway inventory post returned status %d", response.StatusCode)
	}
	var result postResponse
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	if err := decoder.Decode(&result); err != nil || result.Sequence != candidate.Sequence {
		return errors.New("gateway did not confirm the inventory sequence")
	}
	if result.Retained != nil && !*result.Retained {
		return errors.New("gateway did not retain the inventory")
	}
	ids := make(map[string]bool, len(candidate.Members))
	for _, member := range candidate.Members {
		ids[member.ID] = false
	}
	if len(result.Members) != len(candidate.Members) {
		return errors.New("gateway did not confirm every inventory member")
	}
	for _, member := range result.Members {
		if member.Error != "" || member.Sequence != candidate.Sequence {
			return errors.New("gateway did not confirm every inventory member")
		}
		if _, ok := ids[member.NodeID]; !ok || ids[member.NodeID] {
			return errors.New("gateway returned an unexpected inventory member")
		}
		ids[member.NodeID] = true
	}
	for _, held := range ids {
		if !held {
			return errors.New("gateway did not confirm every inventory member")
		}
	}
	return nil
}

func (s *inventoryService) approve(current server.WalletInventory, requests []server.InventoryKey, now time.Time) (server.WalletInventory, []server.InventoryKey, int, bool, error) {
	candidate := current
	candidate.Keys = append([]server.InventoryKey(nil), current.Keys...)
	index := make(map[string]int, len(candidate.Keys))
	ownerCounts := make(map[string]int)
	for i, key := range candidate.Keys {
		index[key.KeyID] = i
		ownerCounts[key.Owner]++
	}
	generationCount, err := s.recentGenerations(now)
	if err != nil {
		return current, nil, 0, false, err
	}
	approved := make([]server.InventoryKey, 0, len(requests))
	seen := make(map[string]bool, len(requests))
	unapproved := 0
	for _, request := range requests {
		if request.KeyID == "" || seen[request.KeyID] || !validKey(request) {
			unapproved++
			continue
		}
		seen[request.KeyID] = true
		priorIndex, exists := index[request.KeyID]
		if isGenerationRequest(request) {
			if exists || generationCount >= s.settings.maxGenerationsHour || !generationKeyID(request.KeyID, request.Curve) || ownerCounts[request.Owner] >= s.settings.maxKeysOwner {
				unapproved++
				continue
			}
			index[request.KeyID] = len(candidate.Keys)
			candidate.Keys = append(candidate.Keys, request)
			ownerCounts[request.Owner]++
			generationCount++
			approved = append(approved, request)
			continue
		}
		if !isSigningRequest(request) || !exists || !hasOperation(candidate.Keys[priorIndex], "generate") || candidate.Keys[priorIndex].Owner != request.Owner || candidate.Keys[priorIndex].Curve != request.Curve {
			unapproved++
			continue
		}
		candidate.Keys[priorIndex] = request
		approved = append(approved, request)
	}
	if len(approved) == 0 {
		return current, approved, unapproved, false, nil
	}
	if err := validateInventory(candidate); err != nil {
		return current, nil, unapproved, false, err
	}
	return candidate, approved, unapproved, true, nil
}

func generationKeyID(keyID, curve string) bool {
	return (curve == "secp256k1" && walletKeyIDPattern.MatchString(keyID)) || (curve == "ed25519" && accountKeyIDPattern.MatchString(keyID))
}

func isGenerationRequest(key server.InventoryKey) bool {
	return key.PublicKey == "" && key.Epoch == 0 && len(key.Operations) == 1 && key.Operations[0] == "generate"
}

func isSigningRequest(key server.InventoryKey) bool {
	return key.PublicKey != "" && hasOperation(key, "sign")
}

func hasOperation(key server.InventoryKey, wanted string) bool {
	for _, operation := range key.Operations {
		if operation == wanted {
			return true
		}
	}
	return false
}

func (s *inventoryService) recentGenerations(now time.Time) (int, error) {
	path := filepath.Join(s.settings.stateDir, stateJournalName)
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	defer file.Close()
	cutoff := now.Add(-time.Hour)
	count := 0
	decoder := json.NewDecoder(file)
	for {
		var line journalLine
		err := decoder.Decode(&line)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return 0, errors.New("inventory journal is invalid")
		}
		at, parseErr := time.Parse(time.RFC3339Nano, line.Time)
		if parseErr != nil {
			return 0, errors.New("inventory journal time is invalid")
		}
		if line.Kind == "approval" && at.After(cutoff) && !at.After(now) && hasString(line.Operations, "generate") {
			count++
		}
	}
	return count, nil
}

func hasString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func (s *inventoryService) appendJournal(line journalLine) error {
	path := filepath.Join(s.settings.stateDir, stateJournalName)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	raw, err := json.Marshal(line)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(raw, '\n')); err != nil {
		return err
	}
	return file.Sync()
}

func (s *inventoryService) markRound(at time.Time) {
	s.mu.Lock()
	s.lastRound = at
	s.mu.Unlock()
}
