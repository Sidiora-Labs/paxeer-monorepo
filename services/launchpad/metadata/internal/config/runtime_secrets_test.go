package config_test

import (
	"context"
	"strings"
	"testing"
	"time"

	runtimedb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"

	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/internaltest"
)

func TestRuntimeSecretBindingAndRotation(t *testing.T) {
	ctx := context.Background()
	cfg := edgeConfig()
	cfg.SecretsProvider = "env"
	cfg.SecretsRefreshSeconds = 30
	keys := []string{
		"DATABASE_URL", "REDIS_URL", "S3_ACCESS_KEY_ID", "S3_SECRET_ACCESS_KEY",
		"RPC_URL", "METADATA_PUBLICATION_AUDIT_KEY", "IDENTITY_SERVICE_TOKEN",
	}
	for _, key := range keys {
		t.Setenv(key, strings.ToLower(key)+"-value")
	}
	check, stop, err := cfg.ResolveProductionSecrets(ctx)
	if err != nil || check == nil || stop == nil {
		t.Fatalf("runtime secrets = %v", err)
	}
	defer stop()
	if cfg.DatabaseURL != "database_url-value" || cfg.IdentityServiceToken != "identity_service_token-value" {
		t.Fatalf("bound runtime values: database=%q identity=%q", cfg.DatabaseURL, cfg.IdentityServiceToken)
	}
	if err := check(ctx); err != nil {
		t.Fatalf("unchanged runtime secrets: %v", err)
	}
	t.Setenv("RPC_URL", "rotated-value")
	if err := check(ctx); err == nil || !strings.Contains(err.Error(), "required_secret_rotated_restart_required") {
		t.Fatalf("rotated secret = %v", err)
	}
	if err := check(ctx); err == nil {
		t.Fatal("rotated secret became ready")
	}
}

func TestPrivateRuntimeSecretRefresh(t *testing.T) {
	ctx := context.Background()
	cfg := edgeConfig()
	cfg.Ingress = config.IngressMTLS
	cfg.SecretsProvider = "env"
	cfg.SecretsRefreshSeconds = 1
	for _, key := range []string{"DATABASE_URL", "REDIS_URL", "S3_ACCESS_KEY_ID", "S3_SECRET_ACCESS_KEY", "RPC_URL", "METADATA_PUBLICATION_AUDIT_KEY", "METADATA_ASSERTION_PRIVATE_KEY_HEX"} {
		t.Setenv(key, strings.ToLower(key)+"-bound")
	}
	check, stop, err := cfg.ResolveProductionSecrets(ctx)
	if err != nil || check == nil || cfg.MetadataAssertionPrivateKeyHex != "metadata_assertion_private_key_hex-bound" {
		t.Fatalf("private secret binding = %v, signer=%q", err, cfg.MetadataAssertionPrivateKeyHex)
	}
	defer stop()
	t.Setenv("DATABASE_URL", "")
	time.Sleep(1500 * time.Millisecond)
	if err := check(ctx); err == nil || !strings.Contains(err.Error(), "required_secret_unavailable") {
		t.Fatalf("private secret refresh = %v", err)
	}
}

func TestRuntimeSecretAdmissionAndReadiness(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name string
		cfg  config.Config
		want string
	}{
		{"invalid environment", config.Config{NodeEnv: "unknown"}, "environment_invalid"},
		{"short refresh", config.Config{NodeEnv: "production", SecretsRefreshSeconds: 0}, "secret_refresh_bound_invalid"},
		{"long refresh", config.Config{NodeEnv: "production", SecretsRefreshSeconds: 61}, "secret_refresh_bound_invalid"},
		{"unknown provider", config.Config{NodeEnv: "production", SecretsRefreshSeconds: 1, SecretsProvider: "missing"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := tc.cfg.ResolveProductionSecrets(ctx)
			if err == nil || (tc.want != "" && !strings.Contains(err.Error(), tc.want)) {
				t.Fatalf("secret admission = %v, want %q", err, tc.want)
			}
		})
	}
	for _, env := range []string{"", "test", "development"} {
		cfg := config.Config{NodeEnv: env}
		check, stop, err := cfg.ResolveProductionSecrets(ctx)
		if err != nil || check != nil || stop == nil {
			t.Fatalf("%q secret binding = %v", env, err)
		}
		stop()
	}
	base := edgeConfig()
	base.SecretsProvider = "env"
	base.SecretsRefreshSeconds = 30
	keys := []string{"DATABASE_URL", "REDIS_URL", "S3_ACCESS_KEY_ID", "S3_SECRET_ACCESS_KEY", "RPC_URL", "METADATA_PUBLICATION_AUDIT_KEY", "IDENTITY_SERVICE_TOKEN"}
	for _, key := range keys {
		t.Setenv(key, strings.ToLower(key)+"-bound")
	}
	missing := base
	t.Setenv("RPC_URL", "")
	if _, _, err := missing.ResolveProductionSecrets(ctx); err == nil || !strings.Contains(err.Error(), "required_secret_unavailable") {
		t.Fatalf("missing required secret = %v", err)
	}
	t.Setenv("RPC_URL", "rpc_url-bound")
	base.ModerationSource = config.ModerationService
	base.ModerationBearerKey = ""
	if _, _, err := base.ResolveProductionSecrets(ctx); err == nil || !strings.Contains(err.Error(), "moderation_bearer_key_missing") {
		t.Fatalf("missing moderation key = %v", err)
	}
	base.ModerationBearerKey = "METADATA_MODERATION_TOKEN"
	if _, _, err := base.ResolveProductionSecrets(ctx); err == nil || !strings.Contains(err.Error(), "moderation_bearer_unavailable") {
		t.Fatalf("missing moderation credential = %v", err)
	}
	t.Setenv("METADATA_MODERATION_TOKEN", "moderation-bound")
	check, stop, err := base.ResolveProductionSecrets(ctx)
	if err != nil || check == nil || stop == nil {
		t.Fatalf("bound moderation = %v", err)
	}
	defer stop()
	if err := check(ctx); err != nil {
		t.Fatalf("moderation readiness = %v", err)
	}
	t.Setenv("METADATA_MODERATION_TOKEN", "")
	if err := check(ctx); err == nil || !strings.Contains(err.Error(), "moderation_bearer_unavailable") {
		t.Fatalf("missing moderation readiness = %v", err)
	}
	if err := check(ctx); err == nil {
		t.Fatal("failed readiness became ready")
	}
}

func TestRuntimePoolConfiguration(t *testing.T) {
	ctx := context.Background()
	pool, dsn := internaltest.NewPostgresURL(t)
	cfg := config.Config{NodeEnv: "development", DatabaseURL: dsn}
	if err := cfg.CheckRuntimeSchema(ctx, pool); err != nil {
		t.Fatalf("development schema readiness: %v", err)
	}
	opened, err := cfg.OpenRuntimePool(ctx, "metadata", runtimedb.PoolOptions{})
	if err != nil {
		t.Fatalf("development pool: %v", err)
	}
	defer opened.Close()
	if err := opened.Ping(ctx); err != nil {
		t.Fatalf("opened pool: %v", err)
	}
	cfg.NodeEnv = "production"
	cfg.RuntimeProfile = "invalid"
	if _, err := cfg.OpenRuntimePool(ctx, "metadata", runtimedb.PoolOptions{}); err == nil || !strings.Contains(err.Error(), "runtime_profile_required") {
		t.Fatalf("invalid runtime profile = %v", err)
	}
	cfg.RuntimeProfile = "container"
	cfg.Port = 8080
	cfg.BindAddress = "0.0.0.0"
	if _, err := cfg.OpenRuntimePool(ctx, "metadata", runtimedb.PoolOptions{}); err == nil || !strings.Contains(err.Error(), "runtime_pool_configuration_missing") {
		t.Fatalf("missing runtime budget = %v", err)
	}
	if err := cfg.CheckRuntimeSchema(ctx, pool); err == nil || !strings.Contains(err.Error(), "runtime_schema_configuration_missing") {
		t.Fatalf("missing production schema binding = %v", err)
	}
}
