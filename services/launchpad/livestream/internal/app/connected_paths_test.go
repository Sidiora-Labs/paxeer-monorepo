package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/livestream/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/media/livestream/internal/streams"
	shareddb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	dbproduction "github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
)

func connectedChain(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, stop := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "anvil", "--chain-id", "125", "--host", "127.0.0.1", "--port", strconv.Itoa(port), "--silent")
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	if err := cmd.Start(); err != nil {
		stop()
		t.Fatal(err)
	}
	t.Cleanup(func() { stop(); _ = cmd.Wait() })
	endpoint := fmt.Sprintf("http://127.0.0.1:%d", port)
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		response, err := client.Post(endpoint, "application/json", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}`))
		if err == nil {
			var result struct {
				Result string `json:"result"`
			}
			decodeErr := json.NewDecoder(response.Body).Decode(&result)
			_ = response.Body.Close()
			if decodeErr == nil && result.Result == "0x7d" {
				return endpoint
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("chain-125 fixture did not become ready")
	return ""
}

func connectedAppConfig(t *testing.T) config.Config {
	database, _ := platformDatabase(t)
	redisURL := os.Getenv("L40_TEST_REDIS_URL")
	if redisURL == "" {
		t.Fatal("dedicated Redis fixture is required")
	}
	dsn := database.Runtimes["livestream_runtime"].DSN
	endpoint, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(endpoint.Port())
	if err != nil {
		t.Fatal(err)
	}
	budget := dbproduction.ConnectionBudget{
		Identity:    "connected_budget_1",
		Targets:     []dbproduction.PostgresTarget{{ID: "primary", Member: "postgres_1", Host: endpoint.Hostname(), Port: uint16(port), MaxConnections: 100, ProtectedSlots: 10}},
		Operational: []dbproduction.OperationalBudget{{ID: "migrator", Target: "primary", Category: "migration", Connections: 1}},
	}
	for _, role := range []string{"apid:rest", "apid:sockets", "indexerd", "chartsd", "statsd", "pnld", "rankingd", "mediagatewayd", "userd", "sociald", "metadatad", "livestreamd"} {
		user := strings.ReplaceAll(role, ":", "_")
		maxConns := int64(1)
		if role == "livestreamd" {
			user, maxConns = endpoint.User.Username(), 5
		}
		budget.Clients = append(budget.Clients, dbproduction.ClientBudget{Role: role, PoolID: "interactive", Target: "primary", Transport: shareddb.TransportDirect, Alias: strings.TrimPrefix(endpoint.Path, "/"), User: user, QueryClass: shareddb.Interactive, Processes: dbproduction.LiveProcesses{Replicas: 1, Maximum: 1}, MaxConnections: maxConns})
	}
	budgetRaw, err := json.Marshal(budget)
	if err != nil {
		t.Fatal(err)
	}
	budgetPath := filepath.Join(t.TempDir(), "connection-budget.json")
	if err := os.WriteFile(budgetPath, budgetRaw, 0600); err != nil {
		t.Fatal(err)
	}
	policy := streams.BudgetPolicy{VendorUserID: "account_1", DeliveryPricePerMinute: "1", TranscodePricePerMinute: "1", StoragePricePerMinute: "1", MonthlyUnits: "1000", BroadcastReserveUnits: "1", Currency: "USD", PeakDurationSeconds: 600, ApprovalReference: "approved", DeliveryPricePerTB: "1"}
	policyRaw, err := json.Marshal(policy)
	if err != nil {
		t.Fatal(err)
	}
	policyPath := filepath.Join(t.TempDir(), "spend-policy.json")
	if err := os.WriteFile(policyPath, policyRaw, 0600); err != nil {
		t.Fatal(err)
	}
	pki := testpki.New(t)
	identityClient := pki.Leaf("livestream-client", "livestream.test")
	identityPeer := pki.Leaf("identity-peer", "identity.test")
	signing := testpki.Ed25519(t)
	for name, value := range map[string]string{
		"DATABASE_URL": dsn, "REDIS_URL": redisURL, "LIVEPEER_API_KEY": "fixture-provider-key",
		"LIVEPEER_WEBHOOK_SECRET":          strings.Repeat("s", 32),
		"IDENTITY_ASSERTION_SIGNING_KEY":   signing.StdBase64,
		"ACTION_AUDIT_KEY":                 strings.Repeat("a", 32),
		"LIVESTREAM_VIEWER_TOKEN_KEY":      strings.Repeat("v", 32),
		"LIVESTREAM_VAULT_TRANSIT_TOKEN":   strings.Repeat("t", 32),
		"LIVEPEER_PLAYBACK_WEBHOOK_SECRET": strings.Repeat("p", 32),
		"LIVESTREAM_BUDGET_ALERT_KEY":      strings.Repeat("b", 32),
	} {
		t.Setenv(name, value)
	}
	schemaFile, schemaDigest := database.ExpectedSchemaFile(t, "livestream_runtime")
	cfg := config.Config{
		NodeEnv: "production", RuntimeProfile: "container", RuntimeZone: "zone-a", RuntimeMetricsEnvironment: "production", SecretsProvider: "env", SecretsRefreshSeconds: 30,
		Port: 8080, BindAddress: "0.0.0.0", LogLevel: "info", DatabaseURL: dsn, RedisURL: redisURL,
		RuntimeBudgetFile: budgetPath, RuntimeBudgetSHA256: fmt.Sprintf("%x", sha256.Sum256(budgetRaw)), RuntimePoolID: "interactive", RuntimeInstanceID: "connected_instance", ExpectedSchemaFile: schemaFile, ExpectedSchemaSHA256: schemaDigest,
		ConnectedEnabled: true, ProviderMode: "available", LivepeerAPIURL: "https://livepeer.studio/api", LivepeerWebhookSecret: strings.Repeat("s", 32), WebhookTimestampUnit: "milliseconds",
		RPCURL: connectedChain(t), MaxStreamsPerWallet: 3, MaxGlobalStreams: 20,
		PlaybackWebhookID: "playback_hook", PlaybackWebhookURL: "https://livestream.test/webhooks/livepeer/playback-access", BudgetAlertURL: "https://livestream.test/budget-alert", BudgetPolicyFile: policyPath, BudgetPolicySHA256: fmt.Sprintf("%x", sha256.Sum256(policyRaw)),
		ActionEnvironment: "test", VaultTransitURL: "https://vault.test", VaultTransitKey: "transit_1", IngestEndpoint: "rtmps://rtmp.livepeer.studio/live",
		IdentityURL: "https://identity.test", IdentityCAFile: pki.CAFile, IdentityClientCertFile: identityClient.CertFile, IdentityClientKeyFile: identityClient.KeyFile, IdentityPeerCertSHA256: identityPeer.SHA256,
	}
	return cfg
}

func TestConnectedAppBootsWorkersAgainstCoordinatorStores(t *testing.T) {
	cfg := connectedAppConfig(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, err := New(ctx, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if a.Router == nil || a.PrivateRouter != nil || a.rpc == nil || a.workerDone == nil {
		t.Fatal("connected runtime components missing")
	}
	if err := a.workerReady(ctx); err != nil {
		t.Fatalf("connected workers = %v", err)
	}
	readiness := httptest.NewRecorder()
	a.Router.ServeHTTP(readiness, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if readiness.Code != http.StatusServiceUnavailable || !bytes.Contains(readiness.Body.Bytes(), []byte("stream_action_chain")) || !bytes.Contains(readiness.Body.Bytes(), []byte("stream_worker_schema")) {
		t.Fatalf("connected readiness = %d %s", readiness.Code, readiness.Body.String())
	}
	response := httptest.NewRecorder()
	a.Router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte("livestream")) {
		t.Fatalf("connected metrics = %d %s", response.Code, response.Body.String())
	}
	a.pool.Close()
	select {
	case failure := <-a.failure:
		if failure == nil || failure.Error() != "livestream_worker_failure" {
			t.Fatalf("worker failure signal = %v", failure)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("workers did not report a closed coordinator store")
	}
	cancel()
	a.Close()
	if err := a.workerReady(context.Background()); !errors.Is(err, streams.ErrUnavailable) {
		t.Fatalf("drained connected workers = %v", err)
	}
}

func TestConnectedAppRejectsInvalidRuntimeAuthority(t *testing.T) {
	if file := os.Getenv("LIVESTREAM_CONNECTED_CONFIG_FILE"); file != "" {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var cfg config.Config
		if err := json.Unmarshal(raw, &cfg); err != nil {
			t.Fatal(err)
		}
		var expected string
		switch os.Getenv("LIVESTREAM_CONNECTED_CASE") {
		case "rpc":
			cfg.RPCURL, expected = "://invalid", "livestream_rpc_unavailable"
		case "timestamp":
			cfg.WebhookTimestampUnit, expected = "seconds", "livepeer_webhook_source_requires_milliseconds"
		case "provider":
			cfg.LivepeerAPIURL, expected = "https://other.test/api", "connected_livestream_provider_endpoint_invalid"
		case "budget":
			cfg.BudgetPolicySHA256, expected = strings.Repeat("0", 64), "runtime_document_binding_mismatch"
		default:
			t.Fatal("unknown connected authority case")
		}
		if _, err := New(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil))); err == nil || !strings.Contains(err.Error(), expected) {
			t.Fatalf("%s authority = %v", os.Getenv("LIVESTREAM_CONNECTED_CASE"), err)
		}
		return
	}
	cfg := connectedAppConfig(t)
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "connected-config.json")
	if err := os.WriteFile(file, raw, 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"rpc", "timestamp", "provider", "budget"} {
		cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestConnectedAppRejectsInvalidRuntimeAuthority$")
		cmd.Env = append(os.Environ(), "LIVESTREAM_CONNECTED_CONFIG_FILE="+file, "LIVESTREAM_CONNECTED_CASE="+name)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s authority check = %v: %s", name, err, output)
		}
	}
}
