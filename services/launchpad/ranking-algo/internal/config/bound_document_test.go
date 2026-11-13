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

	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/internaltest"
	runtimedb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/jackc/pgx/v5"
)

func TestBoundRuntimeDocumentDecoding(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "runtime.json")
	raw := []byte(`{"version":1,"rankingSchema":"ranking"}`)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])
	var document struct {
		Version       int    `json:"version"`
		RankingSchema string `json:"rankingSchema"`
	}
	if err := config.ReadBoundDocument(path, digest, &document); err != nil {
		t.Fatal(err)
	}
	if document.Version != 1 || document.RankingSchema != "ranking" {
		t.Fatalf("decoded runtime document = %+v", document)
	}
	for _, tc := range []struct {
		name   string
		body   []byte
		digest string
	}{
		{"checksum", raw, strings.Repeat("0", 64)},
		{"unknown field", []byte(`{"version":1,"extra":true}`), ""},
		{"trailing value", []byte(`{"version":1} {"version":2}`), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file := filepath.Join(root, tc.name+".json")
			if err := os.WriteFile(file, tc.body, 0600); err != nil {
				t.Fatal(err)
			}
			checksum := tc.digest
			if checksum == "" {
				sum := sha256.Sum256(tc.body)
				checksum = hex.EncodeToString(sum[:])
			}
			if err := config.ReadBoundDocument(file, checksum, &document); err == nil {
				t.Fatal("invalid runtime document accepted")
			}
		})
	}
	link := filepath.Join(root, "linked.json")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if err := config.ReadBoundDocument(link, digest, &document); err == nil {
		t.Fatal("linked runtime document accepted")
	}
}

func TestDevelopmentSecretResolution(t *testing.T) {
	cfg := config.Config{}
	cfg.NodeEnv = "development"
	check, stop, err := cfg.ResolveProductionSecrets(context.Background())
	if err != nil || check != nil || stop == nil {
		t.Fatalf("development secret resolution: check present=%t, stop present=%t, error=%v", check != nil, stop != nil, err)
	}
	stop()
	cfg.NodeEnv = "unexpected"
	if _, _, err := cfg.ResolveProductionSecrets(context.Background()); err == nil {
		t.Fatal("unknown environment accepted")
	}
	cfg.NodeEnv = "production"
	cfg.SecretsRefreshSeconds = 0
	if _, _, err := cfg.ResolveProductionSecrets(context.Background()); err == nil {
		t.Fatal("unbounded refresh interval accepted")
	}
}

func TestInlineRuntimeDocumentBinding(t *testing.T) {
	env := validEnv()
	raw := []byte(`{"version":1}`)
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])
	env["DB_BUDGET_DOCUMENT"] = base64.StdEncoding.EncodeToString(raw)
	env["DB_BUDGET_SHA256"] = digest
	applyEnv(t, env)
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	var document struct {
		Version int `json:"version"`
	}
	if err := config.ReadBoundDocument("", digest, &document); err != nil || document.Version != 1 {
		t.Fatalf("inline runtime document = %+v, %v", document, err)
	}
	if err := config.ReadBoundDocument("", strings.Repeat("0", 64), &document); err == nil {
		t.Fatal("unregistered runtime document accepted")
	}
	t.Setenv("DB_BUDGET_FILE", filepath.Join(t.TempDir(), "budget.json"))
	if _, err := config.Load(); err == nil {
		t.Fatal("two runtime document sources accepted")
	}
}

func TestProductionRuntimeRequiresBoundProfile(t *testing.T) {
	cfg := config.Config{}
	cfg.NodeEnv = "production"
	cfg.RuntimeProfile = "unknown"
	if _, err := cfg.OpenRuntimePool(context.Background(), "rankingd", runtimedb.PoolOptions{}); err == nil {
		t.Fatal("unknown runtime profile accepted")
	}
	cfg.RuntimeProfile = "container"
	if _, err := cfg.OpenRuntimePool(context.Background(), "rankingd", runtimedb.PoolOptions{}); err == nil {
		t.Fatal("container with unmatched bind accepted")
	}
	cfg.Port = 8080
	cfg.BindAddress = "0.0.0.0"
	if _, err := cfg.OpenRuntimePool(context.Background(), "rankingd", runtimedb.PoolOptions{}); err == nil {
		t.Fatal("runtime without budget accepted")
	}
	cfg.RuntimeBudgetDocument = "not-base64"
	cfg.RuntimeBudgetSHA256 = strings.Repeat("0", 64)
	cfg.RuntimePoolID = "ranking_batch"
	cfg.RuntimeInstanceID = "ranking-1"
	if _, err := cfg.OpenRuntimePool(context.Background(), "rankingd", runtimedb.PoolOptions{}); err == nil {
		t.Fatal("unbound budget accepted")
	}
	if err := cfg.CheckRuntimeSchema(context.Background(), nil); err == nil {
		t.Fatal("runtime without schema document accepted")
	}
}

func TestProductionSecretsRequireStableValues(t *testing.T) {
	environment := validEnv()
	for _, key := range []string{"DATABASE_URL", "REDIS_URL", "RPC_URL"} {
		t.Setenv(key, environment[key])
	}
	cfg := config.Config{}
	cfg.NodeEnv = "production"
	cfg.SecretsProvider = "env"
	cfg.SecretsRefreshSeconds = 1
	check, stop, err := cfg.ResolveProductionSecrets(context.Background())
	if err != nil || check == nil || stop == nil {
		t.Fatalf("production secret resolution: check present=%t, stop present=%t, error=%v", check != nil, stop != nil, err)
	}
	defer stop()
	if cfg.DatabaseURL == "" || cfg.RedisURL == "" || cfg.RPCURL == "" {
		t.Fatal("required production values were not resolved")
	}
	if err := check(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Setenv("REDIS_URL", environment["REDIS_URL"]+"/3")
	if err := check(context.Background()); err == nil {
		t.Fatal("rotated secret accepted")
	}
	t.Setenv("REDIS_URL", environment["REDIS_URL"])
	if err := check(context.Background()); err == nil {
		t.Fatal("secret rotation did not require restart")
	}
	cfg.SecretsProvider = "unsupported"
	if _, _, err := cfg.ResolveProductionSecrets(context.Background()); err == nil {
		t.Fatal("unsupported production secret provider accepted")
	}
	cfg.SecretsProvider = "env"
	t.Setenv("DATABASE_URL", "")
	if _, _, err := cfg.ResolveProductionSecrets(context.Background()); err == nil {
		t.Fatal("missing required production secret accepted")
	}
}

func TestProductionPoolUsesBoundReadRole(t *testing.T) {
	db := internaltest.NewRankingRuntimeDatabase(t)
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, db.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	var maxConnections int64
	if err := admin.QueryRow(ctx, "SELECT setting::bigint FROM pg_settings WHERE name='max_connections'").Scan(&maxConnections); err != nil {
		t.Fatal(err)
	}
	if err := admin.Close(ctx); err != nil {
		t.Fatal(err)
	}
	budget := production.ConnectionBudget{
		Identity:    "ranking_config_test",
		Targets:     []production.PostgresTarget{{ID: "fixture_pg", Member: "fixture_pg_primary", Host: db.Host, Port: db.Port, MaxConnections: maxConnections}},
		Operational: []production.OperationalBudget{{ID: "migration", Target: "fixture_pg", Category: "migration", Connections: 2}},
	}
	for _, member := range []struct{ role, user, pool, class string }{
		{"mediagatewayd", "identity_runtime", "gateway_identity", "interactive"},
		{"userd", "users_runtime", "userd_primary", "interactive"},
		{"metadatad", "metadata_runtime", "metadata_primary", "interactive"},
		{"sociald", "chat_runtime", "social_primary", "interactive"},
		{"livestreamd", "livestream_runtime", "livestream_primary", "interactive"},
		{"pnld", "pnl_runtime", "pnl_batch", "batch"},
		{"apid:rest", "api_rest_runtime", "api_rest", "interactive"},
		{"apid:sockets", "api_sockets_runtime", "api_sockets", "interactive"},
		{"statsd", "stats_runtime", "stats_batch", "batch"},
		{"chartsd", "candles_runtime", "charts_batch", "batch"},
		{"rankingd", db.RuntimeRole, "ranking_batch", "batch"},
		{"indexerd", "indexer_runtime", "indexer_primary", "batch"},
	} {
		budget.Clients = append(budget.Clients, production.ClientBudget{Role: member.role, PoolID: member.pool, Target: "fixture_pg", Transport: runtimedb.TransportDirect, Alias: db.Database, User: member.user, QueryClass: runtimedb.QueryClass(member.class), Processes: production.LiveProcesses{Replicas: 1, Surge: 1, Maximum: 2}, MaxConnections: 3})
	}
	raw, err := json.Marshal(budget)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := production.ParseConnectionBudget(raw); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{}
	cfg.NodeEnv = "production"
	cfg.RuntimeProfile = "owned_local"
	cfg.BindAddress = "127.0.0.1"
	cfg.DatabaseURL = db.RuntimeDSN
	cfg.RuntimeBudgetDocument = base64.StdEncoding.EncodeToString(raw)
	cfg.RuntimeBudgetSHA256 = production.Checksum(raw)
	cfg.RuntimePoolID = "ranking_batch"
	cfg.RuntimeInstanceID = "ranking-1"
	cfg.ExpectedSchemaDocument = base64.StdEncoding.EncodeToString(db.ExpectedSchema)
	cfg.ExpectedSchemaSHA256 = production.Checksum(db.ExpectedSchema)
	pool, err := cfg.OpenRuntimePool(ctx, "rankingd", runtimedb.PoolOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer runtimedb.ClosePool(pool)
	var role string
	if err := pool.QueryRow(ctx, "SELECT current_user").Scan(&role); err != nil || role != db.RuntimeRole {
		t.Fatalf("runtime database role = %q, %v", role, err)
	}
	if err := cfg.CheckRuntimeSchema(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var expected production.ExpectedSchema
	if err := json.Unmarshal(db.ExpectedSchema, &expected); err != nil {
		t.Fatal(err)
	}
	expected.Service = "core/other"
	changed, err := json.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ExpectedSchemaDocument = base64.StdEncoding.EncodeToString(changed)
	cfg.ExpectedSchemaSHA256 = production.Checksum(changed)
	if err := cfg.CheckRuntimeSchema(ctx, pool); err == nil {
		t.Fatal("schema document for another service accepted")
	}
}
