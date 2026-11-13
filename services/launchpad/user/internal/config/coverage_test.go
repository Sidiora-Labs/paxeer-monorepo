package config_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/internaltest"
	shareddb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	dbproduction "github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestBoundRuntimeDocuments(t *testing.T) {
	plain := []byte(`{"trade":"account.events"}`)
	digest := sha256.Sum256(plain)
	pin := hex.EncodeToString(digest[:])
	path := filepath.Join(t.TempDir(), "subjects.json")
	if err := os.WriteFile(path, plain, 0o600); err != nil {
		t.Fatal(err)
	}
	var subjects map[string]string
	if err := config.ReadBoundDocument(path, pin, &subjects); err != nil || subjects["trade"] != "account.events" {
		t.Fatalf("bound file = %v, %v", subjects, err)
	}
	for _, tc := range []struct {
		name, body string
	}{
		{"unknown field", `{"trade":"account.events","other":1}`},
		{"trailing document", `{"trade":"account.events"}{}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte(tc.body)
			sum := sha256.Sum256(data)
			file := filepath.Join(t.TempDir(), "subjects.json")
			if err := os.WriteFile(file, data, 0o600); err != nil {
				t.Fatal(err)
			}
			var decoded struct {
				Trade string `json:"trade"`
			}
			if err := config.ReadBoundDocument(file, hex.EncodeToString(sum[:]), &decoded); err == nil {
				t.Fatal("noncanonical runtime document accepted")
			}
		})
	}
	if err := config.ReadBoundDocument(path, strings.Repeat("0", 64), &subjects); err == nil {
		t.Fatal("changed document digest accepted")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "target"), path); err != nil {
		t.Fatal(err)
	}
	if err := config.ReadBoundDocument(path, pin, &subjects); err == nil {
		t.Fatal("symbolic link accepted as a bound document")
	}
}

func TestInlineRuntimeDocumentFromLoad(t *testing.T) {
	plain := []byte(`{"trade":"account.events"}`)
	digest := sha256.Sum256(plain)
	pin := hex.EncodeToString(digest[:])
	setEnv(t, baseEnv())
	t.Setenv("ACCOUNT_SUBJECTS_DOCUMENT", base64.StdEncoding.EncodeToString(plain))
	t.Setenv("ACCOUNT_SUBJECTS_SHA256", pin)
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AccountSubjectsFile != "" || cfg.AccountSubjectsDocument == "" {
		t.Fatalf("inline document was not loaded: %+v", cfg)
	}
	var subjects map[string]string
	if err := config.ReadBoundDocument(cfg.AccountSubjectsFile, cfg.AccountSubjectsSHA256, &subjects); err != nil || subjects["trade"] != "account.events" {
		t.Fatalf("inline document = %v, %v", subjects, err)
	}
	if err := config.ReadBoundDocument("", strings.Repeat("1", 64), &subjects); err == nil {
		t.Fatal("unregistered inline document accepted")
	}
}

func TestAccountRuntimeEndpointAndMaterialValidation(t *testing.T) {
	valid := accountConfig()
	for _, tc := range []struct {
		name   string
		mutate func(*config.Config)
		want   string
	}{
		{"peer pin", func(c *config.Config) { c.IdentityCertSHA256 = strings.ToUpper(c.IdentityCertSHA256) }, "account_peer_pin_invalid"},
		{"http endpoint", func(c *config.Config) { c.IdentityURL = "http://identity.example" }, "account_https_endpoint_invalid"},
		{"shared bucket", func(c *config.Config) { c.AccountExportBucket = c.S3Bucket }, "account_runtime_configuration_missing"},
		{"broker transport", func(c *config.Config) { c.AccountNATSURL = "nats://broker.example" }, "account_broker_tls_required"},
		{"chain endpoint", func(c *config.Config) { c.AccountRPCURL = "invalid" }, "ACCOUNT_CHAIN_RPC_URL"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := valid
			tc.mutate(&cfg)
			if err := cfg.ValidateAccountRuntime(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("validation error = %v, want %s", err, tc.want)
			}
		})
	}
	if _, err := config.DecodeAccountKey(base64.StdEncoding.EncodeToString(make([]byte, 31))); err == nil {
		t.Fatal("short key accepted")
	}
	if _, err := config.Material("", strings.Repeat("x", (1<<20)+1)); err == nil {
		t.Fatal("oversized inline material accepted")
	}
	path := filepath.Join(t.TempDir(), "empty.pem")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Material(path, ""); err == nil {
		t.Fatal("empty file material accepted")
	}
	cfg := config.Config{NodeEnv: "development"}
	check, stop, err := cfg.ResolveProductionSecrets(context.Background())
	if err != nil || check != nil || stop == nil {
		t.Fatalf("development secret resolution: check present=%t, stop present=%t, error=%v", check != nil, stop != nil, err)
	}
	stop()
	cfg.NodeEnv = "unknown"
	if _, _, err := cfg.ResolveProductionSecrets(context.Background()); err == nil {
		t.Fatal("unknown runtime environment accepted")
	}
}

func TestRuntimePoolAndSchemaCheck(t *testing.T) {
	pool := internaltest.NewPostgres(t)
	cfg := config.Config{NodeEnv: "development", DatabaseURL: pool.Config().ConnString()}
	opened, err := cfg.OpenRuntimePool(context.Background(), "userd", shareddb.PoolOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	if err := cfg.CheckRuntimeSchema(context.Background(), opened); err != nil {
		t.Fatal(err)
	}
	cfg.NodeEnv = "production"
	cfg.RuntimeProfile = "container"
	cfg.Port = 3000
	if _, err := cfg.OpenRuntimePool(context.Background(), "userd", shareddb.PoolOptions{}); err == nil || !strings.Contains(err.Error(), "container_bind_mismatch") {
		t.Fatalf("container bind validation = %v", err)
	}
	cfg.RuntimeProfile = "owned_local"
	cfg.BindAddress = "0.0.0.0"
	if _, err := cfg.OpenRuntimePool(context.Background(), "userd", shareddb.PoolOptions{}); err == nil || !strings.Contains(err.Error(), "owned_loopback_required") {
		t.Fatalf("owned-local bind validation = %v", err)
	}
	cfg.BindAddress = "127.0.0.1"
	if _, err := cfg.OpenRuntimePool(context.Background(), "userd", shareddb.PoolOptions{}); err == nil || !strings.Contains(err.Error(), "runtime_pool_configuration_missing") {
		t.Fatalf("missing budget validation = %v", err)
	}
	if err := cfg.CheckRuntimeSchema(context.Background(), opened); err == nil || !strings.Contains(err.Error(), "runtime_schema_configuration_missing") {
		t.Fatalf("missing schema validation = %v", err)
	}
}

func TestProductionSecretsRequireRestartAfterRotation(t *testing.T) {
	keys := []string{
		"DATABASE_URL", "REDIS_URL", "S3_ACCESS_KEY_ID", "S3_SECRET_ACCESS_KEY",
		"ACCOUNT_CURSOR_KEY", "ACCOUNT_SERVICE_SIGNING_SEED", "ACCOUNT_EXPORT_KEY", "ACCOUNT_ENDPOINT_KEY",
		"ACCOUNT_ENDPOINT_FINGERPRINT_KEY", "ACCOUNT_AUDIT_KEY", "NOTIFICATION_SERVICE_TOKEN", "NOTIFICATION_D11_TOKEN",
		"ACCOUNT_NATS_CREDS", "ACCOUNT_NATS_KEY_PEM", "ACCOUNT_TLS_KEY_PEM",
	}
	for _, key := range keys {
		t.Setenv(key, "initial-"+key)
	}
	cfg := config.Config{NodeEnv: "production", SecretsProvider: "env", SecretsRefreshSeconds: 1}
	check, stop, err := cfg.ResolveProductionSecrets(context.Background())
	if err != nil || check == nil || stop == nil {
		t.Fatalf("production secret resolution: check present=%t, stop present=%t, error=%v", check != nil, stop != nil, err)
	}
	defer stop()
	if cfg.DatabaseURL != "initial-DATABASE_URL" || cfg.AccountCursorKey != "initial-ACCOUNT_CURSOR_KEY" {
		t.Fatal("configured secrets were not installed")
	}
	if err := check(context.Background()); err != nil {
		t.Fatalf("unchanged secrets require restart: %v", err)
	}
	t.Setenv("ACCOUNT_CURSOR_KEY", "rotated-ACCOUNT_CURSOR_KEY")
	time.Sleep(2 * time.Second)
	if err := check(context.Background()); err == nil || !strings.Contains(err.Error(), "required_secret_rotated_restart_required") {
		t.Fatalf("rotated secret response = %v", err)
	}
	t.Setenv("ACCOUNT_CURSOR_KEY", "initial-ACCOUNT_CURSOR_KEY")
	if err := check(context.Background()); err == nil {
		t.Fatal("secret rotation was cleared without restart")
	}
	cfg.SecretsRefreshSeconds = 0
	if _, _, err := cfg.ResolveProductionSecrets(context.Background()); err == nil || !strings.Contains(err.Error(), "secret_refresh_bound_invalid") {
		t.Fatalf("invalid refresh interval = %v", err)
	}
}

func TestProductionRuntimeRejectsUnboundBudgetAndSchema(t *testing.T) {
	pool := internaltest.NewPostgres(t)
	cfg := config.Config{
		NodeEnv: "production", RuntimeProfile: "owned_local", BindAddress: "127.0.0.1",
		DatabaseURL: pool.Config().ConnString(), RuntimePoolID: "interactive", RuntimeInstanceID: "user-1",
	}
	if _, err := cfg.OpenRuntimePool(context.Background(), "userd", shareddb.PoolOptions{}); err == nil || !strings.Contains(err.Error(), "runtime_pool_configuration_missing") {
		t.Fatalf("missing runtime budget = %v", err)
	}
	document := []byte(`{}`)
	sum := sha256.Sum256(document)
	cfg.RuntimeBudgetDocument = base64.StdEncoding.EncodeToString(document)
	cfg.RuntimeBudgetSHA256 = strings.Repeat("0", 64)
	if _, err := cfg.OpenRuntimePool(context.Background(), "userd", shareddb.PoolOptions{}); err == nil || !strings.Contains(err.Error(), "runtime_budget_binding_mismatch") {
		t.Fatalf("unbound runtime budget = %v", err)
	}
	cfg.RuntimeBudgetSHA256 = hex.EncodeToString(sum[:])
	if _, err := cfg.OpenRuntimePool(context.Background(), "userd", shareddb.PoolOptions{}); err == nil {
		t.Fatal("empty connection budget accepted")
	}
	cfg.ExpectedSchemaDocument = base64.StdEncoding.EncodeToString(document)
	cfg.ExpectedSchemaSHA256 = hex.EncodeToString(sum[:])
	if err := cfg.CheckRuntimeSchema(context.Background(), pool); err == nil {
		t.Fatal("empty expected schema accepted")
	}
	cfg.ExpectedSchemaDocument = "not-base64"
	if err := cfg.CheckRuntimeSchema(context.Background(), pool); err == nil {
		t.Fatal("malformed expected schema accepted")
	}
}

func TestProductionSchemaReadinessUsesBoundDocument(t *testing.T) {
	database := coordinatortest.Bootstrap(t, os.Getenv("USER_TEST_POSTGRES_DSN"), "chat_runtime")
	runtime := database.Runtimes["chat_runtime"]
	path, digest := database.ExpectedSchemaFile(t, "chat_runtime")
	pool, err := pgxpool.New(context.Background(), runtime.DSN)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	cfg := config.Config{NodeEnv: "production", ExpectedSchemaFile: path, ExpectedSchemaSHA256: digest}
	if err := cfg.CheckRuntimeSchema(context.Background(), pool); err == nil || !strings.Contains(err.Error(), "runtime_schema_service_mismatch") {
		t.Fatalf("foreign runtime schema response = %v", err)
	}
	parsed, err := pgxpool.ParseConfig(runtime.DSN)
	if err != nil {
		t.Fatal(err)
	}
	budget := dbproduction.ConnectionBudget{
		Identity:    "user_test_budget",
		Targets:     []dbproduction.PostgresTarget{{ID: "primary", Member: "test_primary", Host: parsed.ConnConfig.Host, Port: parsed.ConnConfig.Port, MaxConnections: 100}},
		Operational: []dbproduction.OperationalBudget{{ID: "migrator", Target: "primary", Category: "migration", Connections: 1}},
	}
	for _, role := range []string{"apid:rest", "apid:sockets", "indexerd", "chartsd", "statsd", "pnld", "rankingd", "mediagatewayd", "userd", "sociald", "metadatad", "livestreamd"} {
		user := strings.ReplaceAll(role, ":", "_")
		if role == "userd" {
			user = parsed.ConnConfig.User
		}
		budget.Clients = append(budget.Clients, dbproduction.ClientBudget{Role: role, PoolID: "interactive", Target: "primary", Transport: shareddb.TransportDirect, Alias: parsed.ConnConfig.Database, User: user, QueryClass: shareddb.Interactive, Processes: dbproduction.LiveProcesses{Replicas: 1, Maximum: 1}, MaxConnections: 1})
	}
	document, err := json.Marshal(budget)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(document)
	cfg.DatabaseURL = runtime.DSN
	cfg.RuntimeProfile = "owned_local"
	cfg.BindAddress = "127.0.0.1"
	cfg.RuntimeBudgetDocument = base64.StdEncoding.EncodeToString(document)
	cfg.RuntimeBudgetSHA256 = hex.EncodeToString(sum[:])
	cfg.RuntimePoolID = "interactive"
	cfg.RuntimeInstanceID = "user_test"
	if _, err := cfg.OpenRuntimePool(context.Background(), "userd", shareddb.PoolOptions{}); err == nil || !strings.Contains(err.Error(), "runtime_schema_service_mismatch") {
		t.Fatalf("foreign schema admitted as user runtime: %v", err)
	}
	cfg.ExpectedSchemaSHA256 = strings.Repeat("0", 64)
	if err := cfg.CheckRuntimeSchema(context.Background(), pool); err == nil {
		t.Fatal("schema document accepted under another digest")
	}
}
