package config

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"

	runtimedb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	dbproduction "github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestProductionPoolSplitEnforcesReviewedAllocation(t *testing.T) {
	dsn := os.Getenv("L41_POSTGRES_URL")
	if dsn == "" {
		t.Fatal("L41_POSTGRES_URL is required for the gateway database fixture")
	}
	endpoint, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.ParseUint(endpoint.Port(), 10, 16)
	if err != nil {
		t.Fatal(err)
	}
	alias := strings.TrimPrefix(endpoint.Path, "/")
	budget := dbproduction.ConnectionBudget{
		Identity: "gateway_fixture", Targets: []dbproduction.PostgresTarget{{ID: "primary", Member: "fixture_primary", Host: endpoint.Hostname(), Port: uint16(port), MaxConnections: 100}},
		Operational: []dbproduction.OperationalBudget{{ID: "migrator", Target: "primary", Category: "migration", Connections: 1}},
	}
	for _, role := range []string{"apid:rest", "apid:sockets", "indexerd", "chartsd", "statsd", "pnld", "rankingd", "mediagatewayd", "userd", "sociald", "metadatad", "livestreamd"} {
		user := strings.ReplaceAll(role, ":", "_")
		maximum := int64(1)
		if role == "mediagatewayd" {
			user = "identity_runtime"
			maximum = 3
		}
		budget.Clients = append(budget.Clients, dbproduction.ClientBudget{Role: role, PoolID: "identity", Target: "primary", Transport: runtimedb.TransportDirect, Alias: alias, User: user, QueryClass: runtimedb.Interactive, Processes: dbproduction.LiveProcesses{Replicas: 1, Maximum: 1}, MaxConnections: maximum})
	}
	raw, err := json.Marshal(budget)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dbproduction.ParseConnectionBudget(raw); err != nil {
		t.Fatalf("reviewed budget fixture: %v", err)
	}
	digest := sha256.Sum256(raw)
	cfg := Config{NodeEnv: nodeEnvironmentProduction, RuntimeProfile: "owned_local", BindAddress: "127.0.0.1", RuntimeBudgetDocument: base64.StdEncoding.EncodeToString(raw), RuntimeBudgetSHA256: hex.EncodeToString(digest[:]), RuntimePoolID: "identity", RuntimeInstanceID: "gateway_fixture", DatabaseURL: dsn, SecretsProvider: "env"}
	split := RuntimePoolSplit{IdentityPoolID: "identity", IdentityMaxConnections: 2, ControlPoolID: "control", ControlMaxConnections: 2, ControlCredentialReference: "GATEWAY_TEST_CONTROL_DATABASE"}
	for _, tc := range []struct {
		name string
		edit func(*Config)
		want string
	}{
		{"container listener", func(c *Config) { c.RuntimeProfile = "container" }, "container_bind_mismatch"},
		{"owned listener", func(c *Config) { c.BindAddress = "0.0.0.0" }, "owned_loopback_required"},
		{"runtime profile", func(c *Config) { c.RuntimeProfile = "unknown" }, "runtime_profile_required"},
		{"missing pool identity", func(c *Config) { c.RuntimePoolID = "" }, "runtime_pool_configuration_missing"},
		{"unbound budget", func(c *Config) { c.RuntimeBudgetSHA256 = strings.Repeat("0", 64) }, "runtime_budget_binding_mismatch"},
		{"invalid budget", func(c *Config) {
			invalid := []byte(`{}`)
			digest := sha256.Sum256(invalid)
			c.RuntimeBudgetDocument = base64.StdEncoding.EncodeToString(invalid)
			c.RuntimeBudgetSHA256 = hex.EncodeToString(digest[:])
		}, ""},
	} {
		t.Run("runtime pool "+tc.name, func(t *testing.T) {
			candidate := cfg
			tc.edit(&candidate)
			pool, err := candidate.OpenRuntimePool(context.Background(), "mediagatewayd", runtimedb.PoolOptions{})
			if pool != nil || err == nil || !strings.Contains(err.Error(), tc.want) {
				if pool != nil {
					pool.Close()
				}
				t.Fatalf("runtime pool binding: pool=%v err=%v, want %s", pool, err, tc.want)
			}
		})
	}
	if pool, err := (&Config{NodeEnv: "development", DatabaseURL: dsn}).OpenRuntimePool(context.Background(), "mediagatewayd", runtimedb.PoolOptions{}); err != nil || pool == nil {
		t.Fatalf("development runtime pool: pool=%v err=%v", pool, err)
	} else {
		if err := pool.Ping(context.Background()); err != nil {
			t.Fatalf("development runtime ping: %v", err)
		}
		pool.Close()
	}
	if pool, err := cfg.OpenRuntimePool(context.Background(), "unreviewed_role", runtimedb.PoolOptions{}); pool != nil || err == nil {
		if pool != nil {
			pool.Close()
		}
		t.Fatalf("unreviewed runtime role: pool=%v err=%v", pool, err)
	}
	invalidSchema := []byte(`{}`)
	invalidSchemaDigest := sha256.Sum256(invalidSchema)
	schemaConfig := cfg
	schemaConfig.ExpectedSchemaDocument = base64.StdEncoding.EncodeToString(invalidSchema)
	schemaConfig.ExpectedSchemaSHA256 = hex.EncodeToString(invalidSchemaDigest[:])
	if err := schemaConfig.CheckRuntimeSchema(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "complete trusted expected-schema requirements required") {
		t.Fatalf("incomplete bound schema: %v", err)
	}
	for _, tc := range []struct {
		name string
		edit func(*Config, *RuntimePoolSplit)
		want string
	}{
		{"container listener", func(c *Config, _ *RuntimePoolSplit) { c.RuntimeProfile = "container" }, "container_bind_mismatch"},
		{"runtime profile", func(c *Config, _ *RuntimePoolSplit) { c.RuntimeProfile = "unknown" }, "runtime_profile_required"},
		{"budget binding", func(c *Config, _ *RuntimePoolSplit) { c.RuntimeBudgetSHA256 = strings.Repeat("0", 64) }, "runtime_budget_binding_mismatch"},
		{"invalid budget", func(c *Config, _ *RuntimePoolSplit) {
			invalid := []byte(`{}`)
			digest := sha256.Sum256(invalid)
			c.RuntimeBudgetDocument = base64.StdEncoding.EncodeToString(invalid)
			c.RuntimeBudgetSHA256 = hex.EncodeToString(digest[:])
		}, ""},
		{"pool budget", func(c *Config, _ *RuntimePoolSplit) { c.RuntimePoolID = "unreviewed" }, "explicit_runtime_pool_split_required"},
		{"pool budget lookup", func(c *Config, s *RuntimePoolSplit) { c.RuntimePoolID = "unreviewed"; s.IdentityPoolID = "unreviewed" }, ""},
		{"instance budget", func(c *Config, _ *RuntimePoolSplit) { c.RuntimeInstanceID = "unreviewed" }, ""},
		{"secrets provider", func(c *Config, s *RuntimePoolSplit) { c.SecretsProvider = "unsupported"; s.ControlMaxConnections = 1 }, ""},
	} {
		t.Run("split pool "+tc.name, func(t *testing.T) {
			candidate, bounds := cfg, split
			tc.edit(&candidate, &bounds)
			identity, control, err := candidate.OpenSplitRuntimePools(context.Background(), bounds)
			if identity != nil || control != nil || err == nil || (tc.want != "" && !strings.Contains(err.Error(), tc.want)) {
				if identity != nil {
					identity.Close()
				}
				if control != nil {
					control.Close()
				}
				t.Fatalf("split pool binding: identity=%v control=%v err=%v, want %s", identity, control, err, tc.want)
			}
		})
	}
	identity, control, err := cfg.OpenSplitRuntimePools(context.Background(), split)
	if identity != nil || control != nil || err == nil || !strings.Contains(err.Error(), "runtime_pool_split_exceeds_reviewed_allocation") {
		t.Fatalf("overallocated pool split: identity=%v control=%v err=%v", identity, control, err)
	}
	split.ControlMaxConnections = 1
	identity, control, err = cfg.OpenSplitRuntimePools(context.Background(), split)
	if identity != nil || control != nil || err == nil || !strings.Contains(err.Error(), "control_database_credential_unavailable") {
		t.Fatalf("unbound control credential: identity=%v control=%v err=%v", identity, control, err)
	}
	t.Setenv("GATEWAY_TEST_CONTROL_DATABASE", dsn)
	identity, control, err = cfg.OpenSplitRuntimePools(context.Background(), split)
	if identity != nil || control != nil || err == nil || strings.Contains(err.Error(), "control_database_credential_unavailable") {
		t.Fatalf("admitted credential must still satisfy runtime pool roles: identity=%v control=%v err=%v", identity, control, err)
	}
	if identity != nil {
		identity.Close()
	}
	if control != nil {
		control.Close()
	}
	if pool, err := cfg.OpenRuntimePool(context.Background(), "mediagatewayd", runtimedb.PoolOptions{}); err == nil {
		if pool != nil {
			pool.Close()
		}
		t.Fatal("fixture role was admitted as production identity runtime")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := (&Config{NodeEnv: "development"}).CheckRuntimeSchema(context.Background(), pool); err != nil {
		t.Fatalf("development schema connection: %v", err)
	}
	if err := cfg.CheckRuntimeSchema(context.Background(), pool); err == nil || !strings.Contains(err.Error(), "runtime_schema_configuration_missing") {
		t.Fatalf("missing production schema binding: %v", err)
	}
}
