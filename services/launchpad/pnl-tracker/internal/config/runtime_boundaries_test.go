package config_test

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
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/config"
	sharedconfig "github.com/Sidiora-Technologies/KindleLaunch/shared/config"
	shareddb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/nats-io/nats.go"
)

func runtimeDocument(t *testing.T, value any) (string, string) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "document")
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return path, fmt.Sprintf("%x", sha256.Sum256(raw))
}

func TestProductionSecretsRequireStableCompleteInventory(t *testing.T) {
	ctx := context.Background()
	for _, mode := range []string{"stable", "rotated", "unavailable", "refresh"} {
		t.Run(mode, func(t *testing.T) {
			cfg := config.Config{BaseEnv: sharedconfig.BaseEnv{NodeEnv: "production", SecretsProvider: "env", SecretsRefreshSeconds: 1, RPCURLFallback: "bound"}, NATSCredentialsCreds: "bound", JourneyConfigDocument: "bound", PortfolioCDNToken: "bound"}
			for _, key := range []string{"DATABASE_URL", "REDIS_URL", "RPC_URL", "RPC_URL_FALLBACK", "WEBHOOK_HMAC_SECRET", "NATS_URL", "NATS_CREDENTIALS_CREDS", "IDENTITY_SIGNING_KEY", "PNL_CURSOR_KEY", "PNL_CDN_TOKEN"} {
				t.Setenv(key, "value-"+key)
			}
			check, stop, err := cfg.ResolveProductionSecrets(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer stop()
			if cfg.DatabaseURL != "value-DATABASE_URL" || cfg.PortfolioCursorKey != "value-PNL_CURSOR_KEY" || cfg.NATSCredentialsCreds != "value-NATS_CREDENTIALS_CREDS" {
				t.Fatal("secret inventory not resolved")
			}
			if err = check(ctx); err != nil {
				t.Fatal(err)
			}
			if mode == "stable" {
				return
			}
			if mode == "unavailable" {
				t.Setenv("DATABASE_URL", "")
			} else {
				t.Setenv("DATABASE_URL", "new-database")
			}
			if mode == "refresh" {
				time.Sleep(1100 * time.Millisecond)
			}
			first := check(ctx)
			if first == nil {
				t.Fatal("changed inventory accepted")
			}
			t.Setenv("DATABASE_URL", "value-DATABASE_URL")
			if err = check(ctx); err == nil || err.Error() != first.Error() {
				t.Fatal(first, err)
			}
		})
	}
	for _, tc := range []struct {
		name string
		cfg  config.Config
		want string
	}{
		{"environment", config.Config{BaseEnv: sharedconfig.BaseEnv{NodeEnv: "invalid"}}, "environment_invalid"},
		{"refresh lower", config.Config{BaseEnv: sharedconfig.BaseEnv{NodeEnv: "production"}}, "secret_refresh_bound_invalid"},
		{"refresh upper", config.Config{BaseEnv: sharedconfig.BaseEnv{NodeEnv: "production", SecretsRefreshSeconds: 61}}, "secret_refresh_bound_invalid"},
		{"provider", config.Config{BaseEnv: sharedconfig.BaseEnv{NodeEnv: "production", SecretsRefreshSeconds: 1, SecretsProvider: "invalid"}}, ""},
		{"credentials", config.Config{BaseEnv: sharedconfig.BaseEnv{NodeEnv: "production", SecretsRefreshSeconds: 1, SecretsProvider: "env"}}, "required_secret_unavailable"},
		{"fixture profile", config.Config{BaseEnv: sharedconfig.BaseEnv{NodeEnv: "production", SecretsRefreshSeconds: 1, SecretsProvider: "env", DeploymentRealm: "private_fixture"}}, "fixture_nats_profile_mismatch"},
		{"fixture evidence", config.Config{RuntimeProfile: "owned_local", BaseEnv: sharedconfig.BaseEnv{NodeEnv: "production", SecretsRefreshSeconds: 1, SecretsProvider: "env", DeploymentRealm: "private_fixture"}}, "private_fixture"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := tc.cfg.ResolveProductionSecrets(ctx)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatal(err)
			}
		})
	}
	cfg := config.Config{BaseEnv: sharedconfig.BaseEnv{NodeEnv: "production", SecretsRefreshSeconds: 1, SecretsProvider: "env"}, NATSCredentials: "bound"}
	t.Setenv("DATABASE_URL", "")
	if _, _, err := cfg.ResolveProductionSecrets(ctx); err == nil || !strings.Contains(err.Error(), "required_secret_unavailable") {
		t.Fatal(err)
	}
}

func TestRuntimePoolRequiresBoundBudgetAndMatchingSchema(t *testing.T) {
	ctx := context.Background()
	db := coordinatortest.Bootstrap(t, os.Getenv("PNL_TEST_POSTGRES_DSN"), "indexer_runtime", "pnl_runtime")
	cfg := config.Config{BaseEnv: sharedconfig.BaseEnv{NodeEnv: "production", DatabaseURL: db.Runtimes["pnl_runtime"].DSN}, RuntimeProfile: "owned_local", BindAddress: "127.0.0.1", RuntimePoolID: "interactive", RuntimeInstanceID: "pnl_config"}
	cfg.ExpectedSchemaFile, cfg.ExpectedSchemaSHA256 = db.ExpectedSchemaFile(t, "pnl_runtime")
	route, err := url.Parse(cfg.DatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.ParseUint(route.Port(), 10, 16)
	if err != nil {
		t.Fatal(err)
	}
	budget := production.ConnectionBudget{Identity: "pnl_config", Targets: []production.PostgresTarget{{ID: "primary", Member: "fixture", Host: route.Hostname(), Port: uint16(port), MaxConnections: 100}}, Operational: []production.OperationalBudget{{ID: "admin", Target: "primary", Category: "admin", Connections: 1}, {ID: "migration", Target: "primary", Category: "migration", Connections: 1}}}
	for _, role := range []string{"apid:rest", "apid:sockets", "indexerd", "chartsd", "statsd", "pnld", "rankingd", "mediagatewayd", "userd", "sociald", "metadatad", "livestreamd"} {
		user := strings.ReplaceAll(role, ":", "_")
		if role == "pnld" {
			user = route.User.Username()
		}
		budget.Clients = append(budget.Clients, production.ClientBudget{Role: role, PoolID: "interactive", Target: "primary", Transport: shareddb.TransportDirect, Alias: strings.TrimPrefix(route.Path, "/"), User: user, QueryClass: shareddb.Interactive, Processes: production.LiveProcesses{Replicas: 1, Maximum: 1}, MaxConnections: 2})
	}
	cfg.RuntimeBudgetFile, cfg.RuntimeBudgetSHA256 = runtimeDocument(t, budget)
	pool, err := cfg.OpenRuntimePool(ctx, "pnld", shareddb.PoolOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = cfg.CheckRuntimeSchema(ctx, pool); err != nil {
		t.Fatal(err)
	}
	journey := cfg
	journey.RuntimeInstanceID = "pnl_config_journey"
	journey.JourneyConfigDocument = "bound"
	jp, err := journey.OpenRuntimePool(ctx, "pnld", shareddb.PoolOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var path string
	if err = jp.QueryRow(ctx, "SHOW search_path").Scan(&path); err != nil || path != "pnl,pg_catalog" {
		t.Fatal(path, err)
	}
	jp.Close()
	for i, tc := range []struct {
		name   string
		change func(*config.Config)
		want   string
	}{
		{"container", func(c *config.Config) { c.RuntimeProfile = "container" }, "container_bind_mismatch"},
		{"owned", func(c *config.Config) { c.BindAddress = "0.0.0.0" }, "owned_loopback_required"},
		{"profile", func(c *config.Config) { c.RuntimeProfile = "" }, "runtime_profile_required"},
		{"budget absent", func(c *config.Config) { c.RuntimeBudgetFile = "" }, "runtime_pool_configuration_missing"},
		{"budget digest", func(c *config.Config) { c.RuntimeBudgetSHA256 = strings.Repeat("0", 64) }, "runtime_budget_binding_mismatch"},
		{"budget schema", func(c *config.Config) {
			c.RuntimeBudgetFile, c.RuntimeBudgetSHA256 = runtimeDocument(t, map[string]any{})
		}, ""},
		{"pool binding", func(c *config.Config) { c.RuntimePoolID = "other" }, ""},
		{"schema absent", func(c *config.Config) { c.ExpectedSchemaFile = "" }, "runtime_schema_configuration_missing"},
		{"schema digest", func(c *config.Config) { c.ExpectedSchemaSHA256 = strings.Repeat("0", 64) }, "binding_mismatch"},
		{"schema malformed", func(c *config.Config) {
			c.ExpectedSchemaFile, c.ExpectedSchemaSHA256 = runtimeDocument(t, map[string]any{})
		}, ""},
		{"journey route", func(c *config.Config) { c.JourneyConfigDocument = "bound"; c.DatabaseURL = "invalid" }, "pnl_journey_database_route_invalid"},
		{"journey search path", func(c *config.Config) {
			c.JourneyConfigDocument = "bound"
			route, err := url.Parse(c.DatabaseURL)
			if err != nil {
				t.Fatal(err)
			}
			query := route.Query()
			query.Set("search_path", "public")
			route.RawQuery = query.Encode()
			c.DatabaseURL = route.String()
		}, "pnl_journey_search_path_conflict"},
		{"database route", func(c *config.Config) { c.DatabaseURL = "invalid" }, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := cfg
			candidate.RuntimeInstanceID = fmt.Sprintf("pnl_config_%d", i)
			tc.change(&candidate)
			p, err := candidate.OpenRuntimePool(ctx, "pnld", shareddb.PoolOptions{})
			if err == nil {
				p.Close()
				t.Fatal("unbound runtime pool accepted")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatal(err)
			}
		})
	}
	dev := cfg
	dev.NodeEnv = "development"
	dp, err := dev.OpenRuntimePool(ctx, "pnld", shareddb.PoolOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err = dp.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	dp.Close()
}

func TestRuntimeDocumentsRejectAmbiguousAndUnboundedInputs(t *testing.T) {
	for _, tc := range []struct{ name, raw string }{
		{"unknown", `{"unknown":true}`}, {"empty", ""}, {"oversized", strings.Repeat("x", 1048577)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "document")
			if err := os.WriteFile(path, []byte(tc.raw), 0600); err != nil {
				t.Fatal(err)
			}
			digest := fmt.Sprintf("%x", sha256.Sum256([]byte(tc.raw)))
			var value struct{}
			if err := config.ReadBoundDocument(path, digest, &value); err == nil {
				t.Fatal("invalid document accepted")
			}
		})
	}
	for _, kind := range []string{"directory", "symlink", "missing"} {
		t.Run(kind, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "document")
			switch kind {
			case "directory":
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Symlink(t.TempDir(), path); err != nil {
					t.Fatal(err)
				}
			}
			if err := config.ReadBoundDocument(path, strings.Repeat("0", 64), new(any)); err == nil {
				t.Fatal("nonregular document accepted")
			}
		})
	}
	for _, inline := range []string{"!", base64.StdEncoding.EncodeToString(nil), strings.Repeat("x", 1398105)} {
		t.Run(fmt.Sprint(len(inline)), func(t *testing.T) {
			setBaseEnv(t)
			t.Setenv("EXPECTED_SCHEMA_DOCUMENT", inline)
			t.Setenv("EXPECTED_SCHEMA_SHA256", strings.Repeat("0", 64))
			if _, err := config.Load(); err != nil {
				t.Fatal(err)
			}
			if err := config.ReadBoundDocument("", strings.Repeat("0", 64), new(any)); err == nil {
				t.Fatal("invalid inline document accepted")
			}
		})
	}
	for _, creds := range []string{"invalid", strings.Repeat("x", 1048577), "-----BEGIN NATS USER JWT-----\na.b.c\n------END NATS USER JWT------"} {
		cfg := config.Config{NATSCredentialsCreds: creds}
		if _, err := cfg.RuntimeNATSOptions(); err == nil {
			t.Fatal("invalid broker identity accepted")
		}
	}
	for _, cfg := range []config.Config{{}, {BaseEnv: sharedconfig.BaseEnv{DeploymentRealm: "private_fixture"}}, {RuntimeProfile: "owned_local", NATSUsername: "u", NATSPassword: "p", BaseEnv: sharedconfig.BaseEnv{DeploymentRealm: "private_fixture"}}} {
		if _, err := cfg.RuntimeNATSOptions(); err == nil {
			t.Fatal("missing broker authority accepted")
		}
	}
	jwt, creds := inlineNATSCreds(t)
	path := filepath.Join(t.TempDir(), "credentials")
	if err := os.WriteFile(path, []byte(creds), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{NATSCredentials: path}
	options, err := cfg.RuntimeNATSOptions()
	if err != nil || len(options) != 3 {
		t.Fatal(options, err)
	}
	var applied nats.Options
	for _, option := range options {
		if err = option(&applied); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := applied.UserJWT(); err != nil || got != jwt {
		t.Fatal(got, err)
	}
}
