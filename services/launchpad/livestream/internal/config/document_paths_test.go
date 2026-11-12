package config

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	shareddb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	dbproduction "github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
)

func TestRuntimeDocumentsBindFileAndInlineBytes(t *testing.T) {
	raw := []byte(`{"service":"media/livestream"}`)
	digest := fmt.Sprintf("%x", sha256.Sum256(raw))
	path := filepath.Join(t.TempDir(), "schema.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	var fromFile struct {
		Service string `json:"service"`
	}
	if err := ReadBoundDocument(path, digest, &fromFile); err != nil || fromFile.Service != "media/livestream" {
		t.Fatalf("file document = %+v, %v", fromFile, err)
	}
	inline := base64.StdEncoding.EncodeToString(raw)
	cfg := Config{ExpectedSchemaDocument: inline, ExpectedSchemaSHA256: digest}
	cfg.registerInlineDocuments()
	var fromInline struct {
		Service string `json:"service"`
	}
	if err := ReadBoundDocument("", digest, &fromInline); err != nil || fromInline != fromFile {
		t.Fatalf("inline document = %+v, %v", fromInline, err)
	}
	if _, err := boundBytes(path, inline, digest); err == nil {
		t.Fatal("ambiguous document source accepted")
	}
	if _, err := boundBytes(path, "", strings.Repeat("0", 64)); err == nil {
		t.Fatal("changed document digest accepted")
	}
	if _, err := boundBytes("", "%%", digest); err == nil {
		t.Fatal("invalid inline encoding accepted")
	}
	if err := os.WriteFile(path, []byte(`{"service":"media/livestream"} {}`), 0600); err != nil {
		t.Fatal(err)
	}
	changed, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	changedDigest := fmt.Sprintf("%x", sha256.Sum256(changed))
	if err := ReadBoundDocument(path, changedDigest, &fromFile); err == nil {
		t.Fatal("trailing document accepted")
	}
}

func TestRuntimeDocumentRejectsMissingOversizedAndMalformedSources(t *testing.T) {
	path := filepath.Join(t.TempDir(), "authority.json")
	raw := []byte(`{"service":"media/livestream"}`)
	digest := fmt.Sprintf("%x", sha256.Sum256(raw))
	for _, source := range []struct{ path, inline, digest string }{
		{path, "", digest},
		{"", "", digest},
		{"", base64.StdEncoding.EncodeToString(raw), "short"},
		{"", strings.Repeat("A", 1398105), digest},
		{"", base64.StdEncoding.EncodeToString(nil), digest},
		{"", base64.StdEncoding.EncodeToString(raw), strings.Repeat("0", 64)},
	} {
		if _, err := boundBytes(source.path, source.inline, source.digest); err == nil {
			t.Fatalf("unbound document accepted: path=%q inline length=%d", source.path, len(source.inline))
		}
	}
	if err := os.Symlink(path, path+".link"); err != nil {
		t.Fatal(err)
	}
	if _, err := boundBytes(path+".link", "", digest); err == nil {
		t.Fatal("symbolic link accepted as a document")
	}
	if err := os.WriteFile(path, []byte(`{"unknown":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	changed := []byte(`{"unknown":true}`)
	var value struct {
		Service string `json:"service"`
	}
	if err := ReadBoundDocument(path, fmt.Sprintf("%x", sha256.Sum256(changed)), &value); err == nil {
		t.Fatal("unknown document field accepted")
	}
}

func TestRuntimePoolAndConnectedConfigurationBoundaries(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("L40_TEST_POSTGRES_DSN")
	redisURL := os.Getenv("L40_TEST_REDIS_URL")
	if dsn == "" || redisURL == "" {
		t.Fatal("dedicated Postgres and Redis fixtures are required")
	}
	development := Config{NodeEnv: "development", DatabaseURL: dsn}
	pool, err := development.OpenRuntimePool(ctx, "livestreamd", shareddb.PoolOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := development.CheckRuntimeSchema(ctx, pool); err != nil {
		t.Fatal(err)
	}
	production := development
	production.NodeEnv = "production"
	production.RuntimeProfile = PlatformProfile
	if _, err := production.OpenRuntimePool(ctx, "livestreamd", shareddb.PoolOptions{}); err == nil {
		t.Fatal("unbounded platform pool accepted")
	}
	production.RuntimeProfile = "container"
	if _, err := production.OpenRuntimePool(ctx, "livestreamd", shareddb.PoolOptions{}); err == nil {
		t.Fatal("incorrect container bind accepted")
	}
	production.RuntimeProfile = "owned_local"
	if _, err := production.OpenRuntimePool(ctx, "livestreamd", shareddb.PoolOptions{}); err == nil {
		t.Fatal("unbound owned local pool accepted")
	}
	if err := production.CheckRuntimeSchema(ctx, pool); err == nil {
		t.Fatal("unbound production schema accepted")
	}

	connected := Config{
		NodeEnv: "production", ProviderMode: "available", DatabaseURL: dsn, RedisURL: redisURL, LivepeerAPIURL: "https://livepeer.studio/api", LivepeerAPIKey: "fixture", LivepeerWebhookSecret: strings.Repeat("s", 32), LogLevel: "info", Port: 3000, MaxStreamsPerWallet: 3,
		PlaybackWebhookSecret: strings.Repeat("p", 32), BudgetAlertKey: strings.Repeat("b", 32), PlaybackWebhookID: "hook_1", BudgetAlertURL: "https://livepeer.studio/alert", PlaybackWebhookURL: "https://livepeer.studio/access",
		BudgetPolicyDocument: base64.StdEncoding.EncodeToString([]byte(`{}`)), BudgetPolicySHA256: strings.Repeat("a", 64), WebhookTimestampUnit: "milliseconds", MaxGlobalStreams: 20, ActionEnvironment: "test", ActionAuditKey: strings.Repeat("a", 32), ViewerTokenKey: strings.Repeat("v", 32), VaultTransitToken: strings.Repeat("t", 32), VaultTransitKey: "key_1", VaultTransitURL: "https://livepeer.studio", IdentityURL: "https://livepeer.studio", IngestEndpoint: "rtmps://rtmp.livepeer.studio/live",
	}
	if err := connected.ValidateConnected(); err != nil {
		t.Fatalf("connected configuration = %v", err)
	}
	for _, tc := range []struct {
		name, want string
		change     func(*Config)
	}{
		{"provider authority", "connected_livestream_provider_endpoint_invalid", func(c *Config) { c.LivepeerAPIURL = "https://other.test/api" }},
		{"playback authority", "livestream_usage_or_playback_authority_missing", func(c *Config) { c.PlaybackWebhookID = "" }},
		{"action authority", "connected_livestream_authority_missing", func(c *Config) { c.ActionAuditKey = "short" }},
		{"identity endpoint", "connected_livestream_authority_endpoint_invalid", func(c *Config) { c.IdentityURL = "https://identity.test/path" }},
		{"ingest endpoint", "connected_livestream_ingest_endpoint_invalid", func(c *Config) { c.IngestEndpoint = "http://rtmp.livepeer.studio/live" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := connected
			tc.change(&candidate)
			if err := candidate.ValidateConnected(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("connected authority = %v", err)
			}
		})
	}
	connected.PlaybackWebhookURL = "http://livepeer.studio/access"
	if err := connected.ValidateConnected(); err == nil {
		t.Fatal("plaintext callback accepted")
	}
}

func TestProductionPoolRejectsUnboundRuntimeChoices(t *testing.T) {
	ctx := context.Background()
	cfg := Config{NodeEnv: "production", RuntimeProfile: "container", Port: 8080, BindAddress: "0.0.0.0", DatabaseURL: os.Getenv("L40_TEST_POSTGRES_DSN")}
	for _, tc := range []struct {
		name   string
		change func(*Config)
		want   string
	}{
		{"unsupported profile", func(c *Config) { c.RuntimeProfile = "unknown" }, "runtime_profile_required"},
		{"owned listener", func(c *Config) { c.RuntimeProfile = "owned_local" }, "owned_loopback_required"},
		{"container listener", func(c *Config) { c.BindAddress = "127.0.0.1" }, "container_bind_mismatch"},
		{"missing budget", func(*Config) {}, "runtime_pool_configuration_missing"},
		{"changed budget", func(c *Config) {
			c.RuntimeBudgetDocument = base64.StdEncoding.EncodeToString([]byte(`{}`))
			c.RuntimeBudgetSHA256 = strings.Repeat("0", 64)
			c.RuntimePoolID = "interactive"
			c.RuntimeInstanceID = "instance_1"
		}, "runtime_budget_binding_mismatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := cfg
			tc.change(&candidate)
			if _, err := candidate.OpenRuntimePool(ctx, "livestreamd", shareddb.PoolOptions{}); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("runtime selection = %v", err)
			}
		})
	}
}

func TestDevelopmentConfigurationLoadsAndValidatesEnvironment(t *testing.T) {
	if os.Getenv("L40_TEST_POSTGRES_DSN") == "" || os.Getenv("L40_TEST_REDIS_URL") == "" {
		t.Fatal("dedicated Postgres and Redis fixtures are required")
	}
	for name, value := range map[string]string{
		"NODE_ENV":               "development",
		"DATABASE_URL":           os.Getenv("L40_TEST_POSTGRES_DSN"),
		"REDIS_URL":              os.Getenv("L40_TEST_REDIS_URL"),
		"LIVEPEER_PROVIDER_MODE": "unavailable",
		"LIVEPEER_API_URL":       "https://livepeer.studio/api",
		"LOG_LEVEL":              "info",
		"PORT":                   "3000",
		"MAX_STREAMS_PER_WALLET": "3",
	} {
		t.Setenv(name, value)
	}
	cfg, err := Load()
	if err != nil || cfg.NodeEnv != "development" || cfg.ProviderMode != "unavailable" || cfg.Port != 3000 {
		t.Fatalf("development configuration = %+v, %v", cfg, err)
	}
	t.Setenv("PORT", "not-a-port")
	if _, err := Load(); err == nil {
		t.Fatal("malformed process port accepted")
	}
	t.Setenv("PORT", "0")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "PORT") {
		t.Fatalf("out-of-range process port = %v", err)
	}
	t.Setenv("PORT", "3000")
	check, stop, err := cfg.ResolveProductionSecrets(context.Background())
	if err != nil || check != nil || stop == nil {
		t.Fatalf("development secret resolution = check:%t stop:%t error:%v", check != nil, stop != nil, err)
	}
	stop()
	bad := cfg
	bad.NodeEnv = "staging"
	if _, _, err := bad.ResolveProductionSecrets(context.Background()); err == nil {
		t.Fatal("unknown environment accepted")
	}
	bad = cfg
	bad.Port = 0
	if err := bad.Validate(); err == nil || !strings.Contains(err.Error(), "PORT") {
		t.Fatalf("invalid port = %v", err)
	}
	bad = cfg
	bad.TrustedProxyCIDRs = strings.Repeat("127.0.0.0/8,", 33)
	if _, err := bad.TrustedProxies(); err == nil {
		t.Fatal("unbounded proxy list accepted")
	}
}

func TestRuntimeMaterialReadsOnlyBoundedRegularFiles(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "certificate.pem")
	if err := os.WriteFile(path, []byte("certificate bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	material, err := Material(path, "")
	if err != nil || string(material) != "certificate bytes" {
		t.Fatalf("file material = %q, %v", material, err)
	}
	material, err = Material("", "inline bytes")
	if err != nil || string(material) != "inline bytes" {
		t.Fatalf("inline material = %q, %v", material, err)
	}
	if _, err := Material(directory, ""); err == nil {
		t.Fatal("directory accepted as material")
	}
	if _, err := Material("", strings.Repeat("x", 1<<20+1)); err == nil {
		t.Fatal("oversized inline material accepted")
	}
}

func TestPlatformRuntimePoolChecksCoordinatorSchema(t *testing.T) {
	adminDSN := os.Getenv("L40_TEST_APP_POSTGRES_DSN")
	if adminDSN == "" {
		t.Fatal("coordinator PostgreSQL fixture is required")
	}
	database := coordinatortest.Bootstrap(t, adminDSN, "livestream_runtime")
	file, digest := database.ExpectedSchemaFile(t, "livestream_runtime")
	cfg := Config{
		NodeEnv: "production", RuntimeProfile: PlatformProfile,
		DatabaseURL: database.Runtimes["livestream_runtime"].DSN,
		DBMaxConns:  5, ExpectedSchemaFile: file, ExpectedSchemaSHA256: digest,
	}
	pool, err := cfg.OpenRuntimePool(context.Background(), "livestreamd", shareddb.PoolOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := cfg.CheckRuntimeSchema(context.Background(), pool); err != nil {
		t.Fatalf("coordinator schema readiness = %v", err)
	}
	cfg.ExpectedSchemaSHA256 = strings.Repeat("0", 64)
	if err := cfg.CheckRuntimeSchema(context.Background(), pool); err == nil {
		t.Fatal("changed schema binding accepted")
	}
}

func TestContainerRuntimePoolUsesBoundDirectBudget(t *testing.T) {
	adminDSN := os.Getenv("L40_TEST_APP_POSTGRES_DSN")
	if adminDSN == "" {
		t.Fatal("coordinator PostgreSQL fixture is required")
	}
	database := coordinatortest.Bootstrap(t, adminDSN, "livestream_runtime")
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
		Identity:    "runtime_budget_1",
		Targets:     []dbproduction.PostgresTarget{{ID: "primary", Member: "postgres_1", Host: endpoint.Hostname(), Port: uint16(port), MaxConnections: 100, ProtectedSlots: 10}},
		Operational: []dbproduction.OperationalBudget{{ID: "migrator", Target: "primary", Category: "migration", Connections: 1}},
	}
	for _, role := range []string{"apid:rest", "apid:sockets", "indexerd", "chartsd", "statsd", "pnld", "rankingd", "mediagatewayd", "userd", "sociald", "metadatad", "livestreamd"} {
		user := strings.ReplaceAll(role, ":", "_")
		if role == "livestreamd" {
			user = endpoint.User.Username()
		}
		budget.Clients = append(budget.Clients, dbproduction.ClientBudget{Role: role, PoolID: "interactive", Target: "primary", Transport: shareddb.TransportDirect, Alias: strings.TrimPrefix(endpoint.Path, "/"), User: user, QueryClass: shareddb.Interactive, Processes: dbproduction.LiveProcesses{Replicas: 1, Maximum: 1}, MaxConnections: 1})
	}
	raw, err := json.Marshal(budget)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "budget.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	schemaFile, schemaDigest := database.ExpectedSchemaFile(t, "livestream_runtime")
	cfg := Config{NodeEnv: "production", RuntimeProfile: "container", Port: 8080, BindAddress: "0.0.0.0", DatabaseURL: dsn, RuntimeBudgetFile: path, RuntimeBudgetSHA256: fmt.Sprintf("%x", sha256.Sum256(raw)), RuntimePoolID: "interactive", RuntimeInstanceID: "instance_1", ExpectedSchemaFile: schemaFile, ExpectedSchemaSHA256: schemaDigest}
	pool, err := cfg.OpenRuntimePool(context.Background(), "livestreamd", shareddb.PoolOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.CheckRuntimeSchema(context.Background(), pool); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	pool.Close()
	cfg.RuntimeProfile, cfg.BindAddress, cfg.RuntimeInstanceID = "owned_local", "127.0.0.1", "instance_2"
	pool, err = cfg.OpenRuntimePool(context.Background(), "livestreamd", shareddb.PoolOptions{})
	if err != nil {
		t.Fatalf("owned local pool = %v", err)
	}
	pool.Close()
}
