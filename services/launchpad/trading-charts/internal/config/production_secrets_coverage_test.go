package config

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
)

func TestProductionSecretsBindAndDetectRotation(t *testing.T) {
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
	cfg := Config{FeaturesConfigDocument: base64.StdEncoding.EncodeToString(raw), FeaturesConfigSHA256: fmt.Sprintf("%x", sha256.Sum256(raw))}
	cfg.NodeEnv = nodeEnvironmentProduction
	cfg.SecretsProvider = "env"
	cfg.SecretsRefreshSeconds = 1
	cfg.NATSCredentials = "credentials"
	for _, name := range []string{"DATABASE_URL", "REDIS_URL", "RPC_URL", "WEBHOOK_HMAC_SECRET", "NATS_URL", "NATS_CREDENTIALS"} {
		t.Setenv(name, "configured")
	}
	check, stop, err := cfg.ResolveProductionSecrets(context.Background())
	if err != nil || check == nil || stop == nil {
		t.Fatalf("resolved production bindings: %v", err)
	}
	defer stop()
	if cfg.DatabaseURL != "configured" || cfg.NATSCredentials != "configured" || cfg.WebhookHMACSecret != "configured" {
		t.Fatalf("required bindings not loaded")
	}
	if err = check(context.Background()); err != nil {
		t.Fatalf("stable bindings: %v", err)
	}
	t.Setenv("DATABASE_URL", "updated")
	if err = check(context.Background()); err == nil || !strings.Contains(err.Error(), "rotated_restart_required") {
		t.Fatalf("changed database binding: %v", err)
	}
	if err = check(context.Background()); err == nil || !strings.Contains(err.Error(), "rotated_restart_required") {
		t.Fatalf("latched binding change: %v", err)
	}
}
