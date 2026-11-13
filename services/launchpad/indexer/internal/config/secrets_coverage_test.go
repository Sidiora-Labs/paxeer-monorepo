package config

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestProductionSecretsBindAndDetectRotation(t *testing.T) {
	for _, key := range []string{"DATABASE_URL", "REDIS_URL", "RPC_URL", "RPC_URL_FALLBACK", "INDEXER_HISTORICAL_RPC_URL", "WEBHOOK_HMAC_SECRET", "NATS_URL", "NATS_CREDENTIALS"} {
		t.Setenv(key, "value-one")
	}
	t.Setenv("WEBHOOK_HMAC_SECRETS", "first, second")
	c := Config{NodeEnv: "production", SecretsProvider: "env", SecretsRefreshSeconds: 60, NATSCredentials: "selected", RPCURLFallback: "selected", HistoricalRPCURL: "selected", WebhookHMACSecret: "selected", WebhookHMACSecrets: []string{"selected", "selected"}, WebhookURLs: []string{"first", "second"}}
	check, stop, err := c.ResolveProductionSecrets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	if c.DatabaseURL != "value-one" || c.NATSCredentials != "value-one" || c.RPCURLFallback != "value-one" || c.HistoricalRPCURL != "value-one" || c.WebhookHMACSecret != "value-one" || len(c.WebhookHMACSecrets) != 2 || c.WebhookHMACSecrets[0] != "first" || c.WebhookHMACSecrets[1] != "second" {
		t.Fatalf("resolved production bindings: %+v", c)
	}
	if err := check(context.Background()); err != nil {
		t.Fatalf("unchanged secret bindings: %v", err)
	}
	t.Setenv("DATABASE_URL", "value-two")
	if err := check(context.Background()); err == nil || !strings.Contains(err.Error(), "required_secret_rotated_restart_required") {
		t.Fatalf("rotated secret accepted: %v", err)
	}
	t.Setenv("DATABASE_URL", "value-one")
	if err := check(context.Background()); err == nil || !strings.Contains(err.Error(), "required_secret_rotated_restart_required") {
		t.Fatalf("rotation failure was not retained: %v", err)
	}
}

func TestProductionSecretsBackgroundRefreshRetainsRotation(t *testing.T) {
	for _, key := range []string{"DATABASE_URL", "REDIS_URL", "RPC_URL", "NATS_URL", "NATS_CREDENTIALS"} {
		t.Setenv(key, "original")
	}
	c := Config{NodeEnv: "production", SecretsProvider: "env", SecretsRefreshSeconds: 1, NATSCredentials: "selected"}
	check, stop, err := c.ResolveProductionSecrets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	time.Sleep(1100 * time.Millisecond)
	if err = check(context.Background()); err != nil {
		t.Fatalf("unchanged background binding: %v", err)
	}
	t.Setenv("DATABASE_URL", "rotated")
	time.Sleep(1100 * time.Millisecond)
	t.Setenv("DATABASE_URL", "original")
	if err = check(context.Background()); err == nil || !strings.Contains(err.Error(), "required_secret_rotated_restart_required") {
		t.Fatalf("background rotation was not retained: %v", err)
	}
}
