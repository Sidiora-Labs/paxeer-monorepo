package config

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
	runtimedb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestProductionRuntimePoolRequiresRegisteredReadiness(t *testing.T) {
	dsn, databasePool := internaltest.NewPostgresWithDSN(t)
	connection, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	budget := production.ConnectionBudget{Identity: "stats_runtime_budget", Targets: []production.PostgresTarget{{ID: "primary", Member: "primary", Host: connection.ConnConfig.Host, Port: connection.ConnConfig.Port, MaxConnections: 100}}, Operational: []production.OperationalBudget{{ID: "migrator", Target: "primary", Category: "migration", Connections: 1}}}
	for _, role := range []string{"apid:rest", "apid:sockets", "indexerd", "chartsd", "statsd", "pnld", "rankingd", "mediagatewayd", "userd", "sociald", "metadatad", "livestreamd"} {
		user := strings.ReplaceAll(role, ":", "_")
		if role == "statsd" {
			user = connection.ConnConfig.User
		}
		budget.Clients = append(budget.Clients, production.ClientBudget{Role: role, PoolID: "runtime", Target: "primary", Transport: runtimedb.TransportDirect, Alias: connection.ConnConfig.Database, User: user, QueryClass: runtimedb.Batch, Processes: production.LiveProcesses{Replicas: 1, Maximum: 1}, MaxConnections: 2})
	}
	if err = budget.Validate(); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(budget)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	cfg := productionConfigForTests()
	cfg.DatabaseURL = dsn
	cfg.RuntimePoolID = "runtime"
	cfg.RuntimeInstanceID = "stats-one"
	cfg.RuntimeBudgetDocument = base64.StdEncoding.EncodeToString(raw)
	cfg.RuntimeBudgetSHA256 = hex.EncodeToString(sum[:])
	expected := production.ExpectedSchema{Service: "core/stats-workers", RuntimeRole: connection.ConnConfig.User, RegistryDigest: strings.Repeat("a", 64), InterfaceDigest: strings.Repeat("b", 64), ReaderVersion: 1, Migrations: map[int64]string{1: strings.Repeat("c", 64)}, Schemas: map[string]string{"stats": strings.Repeat("d", 64)}, Epoch: 1, Region: "fixture-loopback", Privileges: []production.Grant{{Schema: "stats", Object: "pool_stats", Privileges: []string{"SELECT"}}}}
	expectedRaw, err := json.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ExpectedSchemaDocument = base64.StdEncoding.EncodeToString(expectedRaw)
	cfg.ExpectedSchemaSHA256 = production.Checksum(expectedRaw)
	if err = cfg.CheckRuntimeSchema(context.Background(), databasePool); err == nil {
		t.Fatal("runtime schema without registered readiness admitted")
	}
	if p, err := cfg.OpenRuntimePool(context.Background(), "statsd", runtimedb.PoolOptions{}); err == nil || p != nil {
		if p != nil {
			p.Close()
		}
		t.Fatalf("unregistered readiness admitted: %v", err)
	}
	for _, tc := range []struct {
		name, want string
		edit       func(*Config)
	}{
		{"unregistered pool", "unregistered runtime pool", func(c *Config) { c.RuntimePoolID = "absent" }},
		{"database binding", "", func(c *Config) { c.DatabaseURL = "postgres://%" }},
		{"missing schema", "runtime_schema_configuration_missing", func(c *Config) { c.ExpectedSchemaDocument = "" }},
		{"schema checksum", "", func(c *Config) { c.ExpectedSchemaSHA256 = strings.Repeat("0", 64) }},
		{"schema service", "runtime_schema_service_mismatch", func(c *Config) {
			var schema production.ExpectedSchema
			if err := json.Unmarshal(expectedRaw, &schema); err != nil {
				t.Fatal(err)
			}
			schema.Service = "core/other"
			raw, err := json.Marshal(schema)
			if err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(raw)
			c.ExpectedSchemaDocument = base64.StdEncoding.EncodeToString(raw)
			c.ExpectedSchemaSHA256 = hex.EncodeToString(sum[:])
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := cfg
			c.RuntimeInstanceID += strings.ReplaceAll(tc.name, " ", "_")
			tc.edit(&c)
			p, err := c.OpenRuntimePool(context.Background(), "statsd", runtimedb.PoolOptions{})
			if p != nil {
				p.Close()
			}
			if err == nil || p != nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("runtime readiness: initialized=%t %v", p != nil, err)
			}
		})
	}
}

func TestProductionSecretsRefreshDetectsUnavailableValues(t *testing.T) {
	cfg := productionConfigForTests()
	cfg.SecretsRefreshSeconds = 1
	cfg.NATSCredentials = "credentials"
	cfg.NATSCredentialsCreds = ""
	cfg.RPCURLFallback = "http://127.0.0.1:8546"
	values := map[string]string{"DATABASE_URL": "postgres://localhost/stats", "REDIS_URL": "redis://localhost:6379", "RPC_URL": "http://localhost:8545", "RPC_URL_FALLBACK": cfg.RPCURLFallback, "WEBHOOK_HMAC_SECRET": strings.Repeat("h", 32), "NATS_URL": "nats://localhost:4222", "NATS_CREDENTIALS": cfg.NATSCredentials, "STATS_CURSOR_KEY": cfg.StatsCursorKey, "STATS_AUDIT_KEY": cfg.StatsAuditKey, "IDENTITY_ASSERTION_SIGNING_KEY": cfg.IdentitySigningKey, "IDENTITY_MTLS_CLIENT_KEY_PEM": cfg.IdentityMTLSClientKeyPEM}
	for k, v := range values {
		t.Setenv(k, v)
	}
	check, stop, err := cfg.ResolveProductionSecrets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	if err = check(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RPC_URL_FALLBACK", "")
	timer := time.NewTimer(1200 * time.Millisecond)
	defer timer.Stop()
	<-timer.C
	if err = check(context.Background()); err == nil || err.Error() != "required_secret_unavailable: RPC_URL_FALLBACK" {
		t.Fatalf("secret availability: %v", err)
	}
	t.Setenv("RPC_URL_FALLBACK", values["RPC_URL_FALLBACK"])
	if err = check(context.Background()); err == nil || err.Error() != "required_secret_unavailable: RPC_URL_FALLBACK" {
		t.Fatalf("unavailable secret state: %v", err)
	}
	options, err := cfg.RuntimeNATSOptions()
	if err != nil || len(options) != 3 {
		t.Fatalf("file credential options: %d %v", len(options), err)
	}
}
