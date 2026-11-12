package config

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	runtimedb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	production "github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
)

func TestProductionSchemaRequiresBoundReadiness(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
	digest := strings.Repeat("a", 64)
	want := production.ExpectedSchema{
		Service: "core/trading-charts", RuntimeRole: "chart_runtime", RegistryDigest: digest,
		InterfaceDigest: digest, ReaderVersion: 1, Migrations: map[int64]string{1: digest},
		Schemas: map[string]string{"candles": digest}, Epoch: 1, Region: "local",
		Privileges: []production.Grant{{Schema: "candles", Object: "chart_exact", Privileges: []string{"SELECT"}}},
	}
	bind := func(value production.ExpectedSchema) Config {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{ExpectedSchemaDocument: base64.StdEncoding.EncodeToString(raw), ExpectedSchemaSHA256: production.Checksum(raw)}
		cfg.NodeEnv = nodeEnvironmentProduction
		return cfg
	}
	for _, tc := range []struct {
		name, want string
		change     func(*Config)
	}{
		{"missing authority", "runtime_schema_configuration_missing", func(c *Config) { c.ExpectedSchemaDocument = "" }},
		{"unbound document", "binding_mismatch", func(c *Config) { c.ExpectedSchemaSHA256 = digest }},
		{"malformed document", "unexpected EOF", func(c *Config) {
			raw := []byte("{")
			c.ExpectedSchemaDocument, c.ExpectedSchemaSHA256 = base64.StdEncoding.EncodeToString(raw), production.Checksum(raw)
		}},
		{"wrong service", "runtime_schema_service_mismatch", func(c *Config) {
			wrong := want
			wrong.Service = "core/other"
			*c = bind(wrong)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := bind(want)
			tc.change(&cfg)
			if err := cfg.CheckRuntimeSchema(ctx, pool); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("schema authority %s: %v", tc.name, err)
			}
		})
	}
	cfg := bind(want)
	if err := cfg.CheckRuntimeSchema(ctx, pool); err == nil {
		t.Fatal("unprovisioned runtime passed schema readiness")
	}
}

func TestProductionSecretsRequireCurrentBoundValues(t *testing.T) {
	features := ChartRuntimeFeatures{
		SchemaVersion: 2, EndpointSetVersion: "main", DeploymentVersion: deployment.Current().DeploymentVersion,
		AnchorHeight: 1, AnchorHash: "0x" + strings.Repeat("a", 64),
		CanonicalAPIOrigin: "http://localhost", ReplayGatewayOrigin: "http://localhost", CanonicalSeriesModel: "exact-v1",
		DisplayPrecision: 6, MaxHistoryOrigins: 2, RollupBatch: 4, RollupIntervalMillis: 1000,
	}
	raw, err := json.Marshal(features)
	if err != nil {
		t.Fatal(err)
	}
	base := Config{FeaturesConfigDocument: base64.StdEncoding.EncodeToString(raw), FeaturesConfigSHA256: fmt.Sprintf("%x", sha256.Sum256(raw)), NATSCredentialsCreds: "configured"}
	base.NodeEnv, base.SecretsProvider, base.SecretsRefreshSeconds = nodeEnvironmentProduction, "env", 1
	for _, name := range []string{"DATABASE_URL", "REDIS_URL", "RPC_URL", "WEBHOOK_HMAC_SECRET", "NATS_URL", "NATS_CREDENTIALS_CREDS", "RPC_URL_FALLBACK", "S3_ACCESS_KEY_ID", "S3_SECRET_ACCESS_KEY", "CHART_OPERATOR_TOKEN"} {
		t.Setenv(name, "configured")
	}
	for _, tc := range []struct {
		name, want string
		change     func(*Config)
	}{
		{"unknown environment", "environment_invalid", func(c *Config) { c.NodeEnv = "unreviewed" }},
		{"refresh outside bound", "secret_refresh_bound_invalid", func(c *Config) { c.SecretsRefreshSeconds = 0 }},
		{"missing broker credential mode", "NATS_CREDENTIALS or NATS_CREDENTIALS_CREDS", func(c *Config) { c.NATSCredentialsCreds = "" }},
		{"invalid feature binding", "runtime_document", func(c *Config) { c.FeaturesConfigSHA256 = strings.Repeat("0", 64) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base
			tc.change(&cfg)
			check, stop, err := cfg.ResolveProductionSecrets(context.Background())
			if stop != nil {
				stop()
			}
			if check != nil || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("secret admission %s: %v", tc.name, err)
			}
		})
	}
	cfg := base
	cfg.RPCURLFallback = "configured"
	check, stop, err := cfg.ResolveProductionSecrets(context.Background())
	if err != nil || stop == nil || check == nil || cfg.NATSCredentialsCreds != "configured" || cfg.RPCURLFallback != "configured" {
		t.Fatalf("bound broker and fallback: %+v, %v", cfg, err)
	}
	defer stop()
	t.Setenv("DATABASE_URL", "")
	if err := check(context.Background()); err == nil || !strings.Contains(err.Error(), "required_secret_unavailable") {
		t.Fatalf("removed provider binding: %v", err)
	}
	missing := base
	if check, stop, err := missing.ResolveProductionSecrets(context.Background()); check != nil || stop != nil || err == nil || !strings.Contains(err.Error(), "required_secret_unavailable") {
		t.Fatalf("missing initial provider binding: %v", err)
	}
	for _, tc := range []struct{ name, profile, want string }{
		{"wrong fixture profile", "container", "private_fixture_manifest_binding_missing"},
		{"missing fixture evidence", profileOwnedLocal, "fixture"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := base
			fixture.DeploymentRealm, fixture.RuntimeProfile = realmPrivateFixture, tc.profile
			if check, stop, err := fixture.ResolveProductionSecrets(context.Background()); check != nil || stop != nil || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("fixture broker authority: %v", err)
			}
		})
	}
	t.Setenv("DATABASE_URL", "configured")
	features.Archive = &ChartArchiveFeatures{Endpoint: "http://localhost", Region: "local", Bucket: "charts"}
	raw, err = json.Marshal(features)
	if err != nil {
		t.Fatal(err)
	}
	archive := base
	archive.FeaturesConfigDocument, archive.FeaturesConfigSHA256 = base64.StdEncoding.EncodeToString(raw), fmt.Sprintf("%x", sha256.Sum256(raw))
	archiveCheck, archiveStop, err := archive.ResolveProductionSecrets(context.Background())
	if err != nil || archiveCheck == nil || archiveStop == nil || archive.ArchiveAccessKeyID != "configured" || archive.ArchiveSecretAccessKey != "configured" || archive.OperatorToken != "configured" {
		t.Fatalf("archive secret authority: %v", err)
	}
	defer archiveStop()
	if err := archiveCheck(context.Background()); err != nil {
		t.Fatalf("stable archive bindings: %v", err)
	}
	t.Setenv("S3_ACCESS_KEY_ID", "")
	time.Sleep(1200 * time.Millisecond)
	if err := archiveCheck(context.Background()); err == nil || !strings.Contains(err.Error(), "required_secret_unavailable") {
		t.Fatalf("refreshed archive binding: %v", err)
	}
}

func TestRuntimeEnvironmentAndBudgetRequireParsedAuthority(t *testing.T) {
	t.Setenv("WEBHOOK_HMAC_SECRET", "")
	if cfg, err := Load(); err == nil || cfg.WebhookHMACSecret != "" {
		t.Fatalf("missing environment secret: %+v, %v", cfg, err)
	}
	development := Config{}
	development.NodeEnv = "development"
	check, stop, err := development.ResolveProductionSecrets(context.Background())
	if err != nil || check != nil || stop == nil {
		t.Fatalf("development secret binding: %v", err)
	}
	stop()
	invalidProvider := Config{}
	invalidProvider.NodeEnv, invalidProvider.SecretsProvider, invalidProvider.SecretsRefreshSeconds = nodeEnvironmentProduction, "unknown", 1
	if check, stop, err := invalidProvider.ResolveProductionSecrets(context.Background()); err == nil || check != nil || stop != nil {
		t.Fatalf("unknown secret provider: %v", err)
	}
	raw := []byte(`{}`)
	budget := Config{RuntimeProfile: profileOwnedLocal, BindAddress: "127.0.0.1", RuntimeBudgetDocument: base64.StdEncoding.EncodeToString(raw), RuntimeBudgetSHA256: production.Checksum(raw), RuntimePoolID: "charts_batch", RuntimeInstanceID: "chart_local"}
	budget.NodeEnv = nodeEnvironmentProduction
	if pool, err := budget.OpenRuntimePool(context.Background(), "chartsd", runtimedb.PoolOptions{}); pool != nil || err == nil {
		t.Fatalf("unallocated runtime budget: %v", err)
	}
}
