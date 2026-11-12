package config

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/internaltest"
	runtimedb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	dbproduction "github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
)

func TestBoundRuntimeDocumentLoadsFileAndInlineBytes(t *testing.T) {
	raw := []byte(`{"version":2,"gatewayDNS":"gateway.example"}`)
	digest := fmt.Sprintf("%x", sha256.Sum256(raw))
	path := filepath.Join(t.TempDir(), "social.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	var fromFile SocialRuntimeBinding
	if err := ReadBoundDocument(path, digest, &fromFile); err != nil {
		t.Fatal(err)
	}
	if fromFile.Version != 2 || fromFile.GatewayDNS != "gateway.example" {
		t.Fatalf("file binding = %+v", fromFile)
	}
	cfg := Config{SocialRuntimeDocument: base64.StdEncoding.EncodeToString(raw), SocialRuntimeSHA256: digest}
	cfg.registerInlineDocuments()
	var fromInline SocialRuntimeBinding
	if err := ReadBoundDocument("", digest, &fromInline); err != nil || fromInline.Version != fromFile.Version || fromInline.GatewayDNS != fromFile.GatewayDNS {
		t.Fatalf("inline binding = %+v: %v", fromInline, err)
	}
	if _, err := boundBytes(path, cfg.SocialRuntimeDocument, digest); err == nil {
		t.Fatal("two document sources accepted")
	}
	if _, err := boundBytes(path, "", strings.Repeat("0", 64)); err == nil {
		t.Fatal("different digest accepted")
	}
	invalid := []byte(`{"version":2,"unrecognized":true}`)
	if err := os.WriteFile(path, invalid, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ReadBoundDocument(path, fmt.Sprintf("%x", sha256.Sum256(invalid)), &fromFile); err == nil {
		t.Fatal("unknown runtime field accepted")
	}
	trailing := []byte(`{"version":2} {"version":3}`)
	if err := os.WriteFile(path, trailing, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ReadBoundDocument(path, fmt.Sprintf("%x", sha256.Sum256(trailing)), &fromFile); err == nil {
		t.Fatal("trailing runtime document accepted")
	}
}

func TestDevelopmentRuntimePoolChecksLiveSchema(t *testing.T) {
	ctx := context.Background()
	admin := os.Getenv("SOCIAL_FIXTURE_POSTGRES_DSN")
	if admin == "" {
		admin = "postgres://kindle_fixture:kindle_fixture_password@127.0.0.1:25432/kindle_fixture?sslmode=disable"
	}
	cfg := Config{NodeEnv: "development", DatabaseURL: internaltest.DevelopmentPostgresDSN(t, admin)}
	pool, err := cfg.OpenRuntimePool(ctx, "sociald", runtimedb.PoolOptions{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := cfg.CheckRuntimeSchema(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var one int
	if err := pool.QueryRow(ctx, "SELECT 1").Scan(&one); err != nil || one != 1 {
		t.Fatalf("runtime pool query = %d: %v", one, err)
	}
	cfg.NodeEnv = "production"
	cfg.RuntimeProfile = "container"
	cfg.BindAddress = "127.0.0.1"
	cfg.Port = 8080
	if _, err := cfg.OpenRuntimePool(ctx, "sociald", runtimedb.PoolOptions{}); err == nil {
		t.Fatal("container profile accepted a loopback bind")
	}
}

func TestPlatformRuntimePoolChecksCoordinatorSchema(t *testing.T) {
	admin := os.Getenv("SOCIAL_FIXTURE_POSTGRES_DSN")
	if admin == "" {
		admin = "postgres://kindle_fixture:kindle_fixture_password@127.0.0.1:25432/kindle_fixture?sslmode=disable"
	}
	database := coordinatortest.Bootstrap(t, admin, "chat_runtime")
	cfg := Config{NodeEnv: "production", RuntimeProfile: PlatformProfile, DatabaseURL: database.Runtimes["chat_runtime"].DSN, DBMaxConns: 10}
	cfg.ExpectedSchemaFile, cfg.ExpectedSchemaSHA256 = database.ExpectedSchemaFile(t, "chat_runtime")
	pool, err := cfg.OpenRuntimePool(context.Background(), "sociald", runtimedb.PoolOptions{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := cfg.CheckRuntimeSchema(context.Background(), pool); err != nil {
		t.Fatal(err)
	}
	var one int
	if err := pool.QueryRow(context.Background(), `SELECT 1`).Scan(&one); err != nil || one != 1 {
		t.Fatalf("runtime query = %d: %v", one, err)
	}
}

func TestProductionSecretsRequireRestartAfterRotation(t *testing.T) {
	t.Setenv("DATABASE_URL", "database-value-a")
	t.Setenv("REDIS_URL", "redis-value-a")
	cfg := Config{NodeEnv: "production", RuntimeProfile: "container", SecretsProvider: "env", SecretsRefreshSeconds: 30}
	check, stop, err := cfg.ResolveProductionSecrets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	if cfg.DatabaseURL != "database-value-a" || cfg.RedisURL != "redis-value-a" || check(context.Background()) != nil {
		t.Fatalf("bound runtime values = %q, %q", cfg.DatabaseURL, cfg.RedisURL)
	}
	t.Setenv("REDIS_URL", "redis-value-b")
	if err := check(context.Background()); err == nil || !strings.Contains(err.Error(), "required_secret_rotated_restart_required") {
		t.Fatalf("rotated authority = %v", err)
	}
	t.Setenv("REDIS_URL", "redis-value-a")
	if err := check(context.Background()); err == nil {
		t.Fatal("restoring a rotated value cleared restart requirement")
	}
}

func TestProductionSecretsReportMissingRequiredValues(t *testing.T) {
	t.Setenv("DATABASE_URL", "fixture-database")
	t.Setenv("REDIS_URL", "fixture-redis")
	t.Setenv("CHAT_ADMIN_API_KEY", "fixture-admin")
	cfg := Config{NodeEnv: "production", RuntimeProfile: "container", SecretsProvider: "env", SecretsRefreshSeconds: 30, ChatAdminAPIKey: "configured"}
	check, stop, err := cfg.ResolveProductionSecrets(context.Background())
	if err != nil || check == nil || cfg.ChatAdminAPIKey != "fixture-admin" {
		t.Fatalf("bound admin secret = %q: %v", cfg.ChatAdminAPIKey, err)
	}
	defer stop()
	t.Setenv("REDIS_URL", "")
	if err := check(context.Background()); err == nil || !strings.Contains(err.Error(), "required_secret_unavailable") {
		t.Fatalf("missing runtime secret = %v", err)
	}
	t.Setenv("CHAT_ADMIN_API_KEY", "")
	missing := Config{NodeEnv: "production", RuntimeProfile: "container", SecretsProvider: "env", SecretsRefreshSeconds: 30, ChatAdminAPIKey: "configured"}
	if _, stop, err := missing.ResolveProductionSecrets(context.Background()); err == nil || !strings.Contains(err.Error(), "required_secret_unavailable") {
		if stop != nil {
			stop()
		}
		t.Fatalf("missing admin secret = %v", err)
	}
}

func TestProductionRuntimePoolRequiresBoundProfileAndSchema(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name string
		cfg  Config
		want string
	}{
		{"platform connection limit", Config{NodeEnv: "production", RuntimeProfile: PlatformProfile, DBMaxConns: 0}, "runtime_pool_connection_limit_invalid"},
		{"container address", Config{NodeEnv: "production", RuntimeProfile: "container", BindAddress: "127.0.0.1", Port: 8080}, "container_bind_mismatch"},
		{"loopback address", Config{NodeEnv: "production", RuntimeProfile: "owned_local", BindAddress: "0.0.0.0"}, "owned_loopback_required"},
		{"missing profile", Config{NodeEnv: "production", RuntimeProfile: "unknown"}, "runtime_profile_required"},
		{"missing budget", Config{NodeEnv: "production", RuntimeProfile: "container", BindAddress: "0.0.0.0", Port: 8080}, "runtime_pool_configuration_missing"},
		{"unbound budget", Config{NodeEnv: "production", RuntimeProfile: "container", BindAddress: "0.0.0.0", Port: 8080, RuntimePoolID: "social", RuntimeInstanceID: "social-1", RuntimeBudgetDocument: base64.StdEncoding.EncodeToString([]byte(`{}`)), RuntimeBudgetSHA256: strings.Repeat("0", 64)}, "runtime_budget_binding_mismatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.cfg.OpenRuntimePool(ctx, "sociald", runtimedb.PoolOptions{})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("pool validation = %v; want %s", err, tc.want)
			}
		})
	}
	cfg := Config{NodeEnv: "production"}
	if err := cfg.CheckRuntimeSchema(ctx, nil); err == nil || !strings.Contains(err.Error(), "runtime_schema_configuration_missing") {
		t.Fatalf("schema binding = %v", err)
	}
}

func TestProductionRuntimePoolRejectsUnregisteredAndPooledAllocations(t *testing.T) {
	budget := dbproduction.ConnectionBudget{
		Identity:    "budget_v2",
		Targets:     []dbproduction.PostgresTarget{{ID: "primary", Member: "postgres_a", Host: "postgres-a.internal", Port: 5432, MaxConnections: 600, ProtectedSlots: 60}},
		Poolers:     []dbproduction.PoolerBudget{{ID: "pooler", Target: "primary", Processes: dbproduction.LiveProcesses{Replicas: 3, Maximum: 3}, MaxClientConnections: 100, Mode: "transaction", Host: "pooler.internal", Port: 6432, Version: "1.24.1", ImageDigest: "sha256:" + strings.Repeat("a", 64), ConfigurationSHA256: strings.Repeat("b", 64), Aliases: []dbproduction.AliasLimit{{Alias: "kindle", MaxConnections: 120}}}},
		Operational: []dbproduction.OperationalBudget{{ID: "migrator", Target: "primary", Category: "migration", Connections: 1}, {ID: "admin", Target: "primary", Category: "admin", Connections: 119}},
	}
	for _, role := range []string{"apid:rest", "apid:sockets", "indexerd", "chartsd", "statsd", "pnld", "rankingd", "mediagatewayd", "userd", "sociald", "metadatad", "livestreamd"} {
		user := strings.ReplaceAll(role, ":", "_")
		budget.Poolers[0].Users = append(budget.Poolers[0].Users, dbproduction.UserLimit{User: user, MaxConnections: 120})
		budget.Poolers[0].Pairs = append(budget.Poolers[0].Pairs, dbproduction.PairLimit{Alias: "kindle", User: user, PoolSize: 120})
		budget.Clients = append(budget.Clients, dbproduction.ClientBudget{Role: role, PoolID: "interactive", Target: "primary", Transport: runtimedb.TransportTransactionPooler, Pooler: "pooler", Alias: "kindle", User: user, QueryClass: runtimedb.Interactive, Processes: dbproduction.LiveProcesses{Replicas: 1, Maximum: 1}, MaxConnections: 1})
	}
	raw, err := json.Marshal(budget)
	if err != nil {
		t.Fatal(err)
	}
	base := Config{NodeEnv: "production", RuntimeProfile: "container", BindAddress: "0.0.0.0", Port: 8080, RuntimeInstanceID: "social-a", RuntimeBudgetDocument: base64.StdEncoding.EncodeToString(raw), RuntimeBudgetSHA256: fmt.Sprintf("%x", sha256.Sum256(raw))}
	for _, tc := range []struct {
		name, pool, want string
	}{
		{"unregistered pool", "other", "unregistered runtime pool"},
		{"transaction pooler", "interactive", "runtime_transaction_pooler_seam_unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base
			cfg.RuntimePoolID = tc.pool
			if _, err := cfg.OpenRuntimePool(context.Background(), "sociald", runtimedb.PoolOptions{}); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("runtime allocation = %v; want %s", err, tc.want)
			}
		})
	}
	invalid := base
	invalid.RuntimePoolID = "interactive"
	invalid.RuntimeBudgetDocument = base64.StdEncoding.EncodeToString([]byte(`{"unknown":true}`))
	invalid.RuntimeBudgetSHA256 = fmt.Sprintf("%x", sha256.Sum256([]byte(`{"unknown":true}`)))
	if _, err := invalid.OpenRuntimePool(context.Background(), "sociald", runtimedb.PoolOptions{}); err == nil || !strings.Contains(err.Error(), "invalid connection budget document") {
		t.Fatalf("invalid allocated budget = %v", err)
	}
}
