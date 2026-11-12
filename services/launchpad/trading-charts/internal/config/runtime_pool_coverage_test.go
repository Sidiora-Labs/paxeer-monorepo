package config

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	runtimedb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	production "github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
)

func TestRuntimePoolUsesDevelopmentDatabaseAndChecksReadiness(t *testing.T) {
	ctx := context.Background()
	cfg := Config{}
	cfg.NodeEnv = "development"
	cfg.DatabaseURL = internaltest.NewDatabase(t)
	pool, err := cfg.OpenRuntimePool(ctx, "chartsd", runtimedb.PoolOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := cfg.CheckRuntimeSchema(ctx, pool); err != nil {
		t.Fatalf("development schema readiness: %v", err)
	}
	var schema string
	if err := pool.QueryRow(ctx, `SELECT current_schema()`).Scan(&schema); err != nil || schema != "public" {
		t.Fatalf("opened database schema: %q, %v", schema, err)
	}
}

func TestProductionRuntimePoolChecksBudgetAndSchemaAuthority(t *testing.T) {
	ctx := context.Background()
	dsn, admin := internaltest.NewPostgres(t)
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.ParseUint(parsed.Port(), 10, 16)
	if err != nil {
		t.Fatal(err)
	}
	var maximum int64
	if err := admin.QueryRow(ctx, `SELECT setting::bigint FROM pg_settings WHERE name='max_connections'`).Scan(&maximum); err != nil {
		t.Fatal(err)
	}
	budget := production.ConnectionBudget{
		Identity:    "chart_runtime_fixture",
		Targets:     []production.PostgresTarget{{ID: "primary", Member: "primary_member", Host: parsed.Hostname(), Port: uint16(port), MaxConnections: maximum}},
		Operational: []production.OperationalBudget{{ID: "migration", Target: "primary", Category: "migration", Connections: 2}},
	}
	for index, role := range []string{"apid:rest", "apid:sockets", "indexerd", "chartsd", "statsd", "pnld", "rankingd", "mediagatewayd", "userd", "sociald", "metadatad", "livestreamd"} {
		user := fmt.Sprintf("runtime_%d", index)
		poolID := fmt.Sprintf("other_%d", index)
		if role == "chartsd" {
			user, poolID = parsed.User.Username(), "charts_batch"
		}
		budget.Clients = append(budget.Clients, production.ClientBudget{Role: role, PoolID: poolID, Target: "primary", Transport: runtimedb.TransportDirect, Alias: strings.TrimPrefix(parsed.Path, "/"), User: user, QueryClass: runtimedb.Batch, Processes: production.LiveProcesses{Replicas: 1, Maximum: 1}, MaxConnections: 1})
	}
	if err := budget.Validate(); err != nil {
		t.Fatal(err)
	}
	wire, err := json.Marshal(budget)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{RuntimeProfile: profileOwnedLocal, BindAddress: "127.0.0.1", RuntimeBudgetDocument: base64.StdEncoding.EncodeToString(wire), RuntimeBudgetSHA256: production.Checksum(wire), RuntimePoolID: "charts_batch", RuntimeInstanceID: "charts_fixture"}
	cfg.DatabaseURL = dsn
	cfg.NodeEnv = nodeEnvironmentProduction
	opened, err := cfg.OpenRuntimePool(ctx, "chartsd", runtimedb.PoolOptions{})
	if opened != nil || err == nil || !strings.Contains(err.Error(), "runtime_schema_configuration_missing") {
		t.Fatalf("unbound production schema: pool=%v err=%v", opened, err)
	}
	opened, err = cfg.OpenRuntimePool(ctx, "unknown", runtimedb.PoolOptions{})
	if opened != nil || err == nil || !strings.Contains(err.Error(), "unregistered runtime pool") {
		t.Fatalf("unallocated runtime role: pool=%v err=%v", opened, err)
	}
}

func TestProductionRuntimePoolRequiresBoundResources(t *testing.T) {
	cases := []struct {
		name, profile, bind, want string
		port                      int
		budget, poolID, instance  string
	}{
		{"container address", "container", "127.0.0.1", "container_bind_mismatch", 8080, "", "", ""},
		{"owned address", profileOwnedLocal, "0.0.0.0", "owned_loopback_required", 3000, "", "", ""},
		{"missing profile", "other", "127.0.0.1", "runtime_profile_required", 3000, "", "", ""},
		{"missing budget", profileOwnedLocal, "127.0.0.1", "runtime_pool_configuration_missing", 3000, "", "", ""},
		{"unbound budget", profileOwnedLocal, "127.0.0.1", "runtime_budget_binding_mismatch", 3000, "invalid", "pool", "instance"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{RuntimeProfile: tc.profile, BindAddress: tc.bind, RuntimeBudgetDocument: tc.budget, RuntimePoolID: tc.poolID, RuntimeInstanceID: tc.instance}
			cfg.Port = tc.port
			cfg.NodeEnv = nodeEnvironmentProduction
			pool, err := cfg.OpenRuntimePool(context.Background(), "chartsd", runtimedb.PoolOptions{})
			if pool != nil || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("production pool bounds: %v, %v", pool, err)
			}
		})
	}
}

func TestRuntimeNATSOptionsRequireCredentials(t *testing.T) {
	cases := []struct {
		name, file, inline, want string
		count                    int
	}{
		{"missing", "", "", "broker_credentials_missing", 0},
		{"invalid inline", "", "invalid", "broker_credentials_invalid", 0},
		{"credentials file", "/tmp/charts.creds", "", "", 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{NATSCredentials: tc.file, NATSCredentialsCreds: tc.inline, RuntimeInstanceID: "chart-1"}
			options, err := cfg.RuntimeNATSOptions()
			if tc.want != "" {
				if err == nil || !strings.Contains(err.Error(), tc.want) || options != nil {
					t.Fatalf("credential refusal: %d options, %v", len(options), err)
				}
				return
			}
			if err != nil || len(options) != tc.count {
				t.Fatalf("credential options: %d, %v", len(options), err)
			}
		})
	}
}

func TestRuntimeFeaturesAcceptArchiveAndRejectInvalidArchive(t *testing.T) {
	features := ChartRuntimeFeatures{
		SchemaVersion: 2, EndpointSetVersion: "main", DeploymentVersion: deployment.Current().DeploymentVersion,
		AnchorHeight: 1, AnchorHash: "0x" + strings.Repeat("a", 64),
		CanonicalAPIOrigin: "https://charts.example.org", ReplayGatewayOrigin: "https://replay.example.org",
		CanonicalSeriesModel: "exact-v1", DisplayPrecision: 6, MaxHistoryOrigins: 2,
		RollupBatch: 4, RollupIntervalMillis: 1000,
		Archive: &ChartArchiveFeatures{Endpoint: "https://objects.example.org", Region: "local", Bucket: "charts"},
	}
	encode := func(value ChartRuntimeFeatures) Config {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return Config{FeaturesConfigDocument: base64.StdEncoding.EncodeToString(raw), FeaturesConfigSHA256: fmt.Sprintf("%x", sha256.Sum256(raw))}
	}
	cfg := encode(features)
	got, err := cfg.RuntimeFeatures()
	if err != nil || got.Archive == nil || got.Archive.Bucket != "charts" {
		t.Fatalf("archive feature: %+v, %v", got.Archive, err)
	}
	features.Archive.Endpoint = "http://objects.example.org"
	cfg = encode(features)
	if _, err := cfg.RuntimeFeatures(); err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("remote archive origin: %v", err)
	}
	features.Archive.Endpoint = "https://objects.example.org"
	features.Archive.Bucket = ""
	cfg = encode(features)
	if _, err := cfg.RuntimeFeatures(); err == nil || !strings.Contains(err.Error(), "archive_config_invalid") {
		t.Fatalf("empty archive bucket: %v", err)
	}
}
