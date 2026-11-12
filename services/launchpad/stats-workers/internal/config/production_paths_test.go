package config

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"

	sharedconfig "github.com/Sidiora-Technologies/KindleLaunch/shared/config"
	sharedb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
)

func productionConfigForTests() Config {
	key := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	return Config{
		BaseEnv:        sharedconfig.BaseEnv{NodeEnv: "production", SecretsProvider: "env", SecretsRefreshSeconds: 60, Port: 8080},
		StatsCursorKey: strings.Repeat("a", 64), StatsAuditKey: strings.Repeat("b", 64),
		RuntimeProfile: "container", BindAddress: "0.0.0.0",
		IdentityURL: "https://identity.example", IdentityMTLSCAPEM: "ca", IdentityMTLSClientCertPEM: "cert", IdentityMTLSClientKeyPEM: "key",
		IdentityMTLSPeerCertSHA256: strings.Repeat("c", 64), IdentitySigningKey: base64.StdEncoding.EncodeToString(key),
		NATSCredentialsCreds:  "inline credentials",
		PublicStatsIntervalMS: 2000, PublicStatsBatch: 50, PublicStreamSchema: "api_stream", LegacyBridgeIntervalMS: 1000, LegacyBridgeBatch: 100,
	}
}

func TestProductionSecretsAreBoundAndRotationIsVisible(t *testing.T) {
	cfg := productionConfigForTests()
	values := map[string]string{
		"DATABASE_URL": "postgres://localhost/stats", "REDIS_URL": "redis://localhost:6379", "RPC_URL": "http://localhost:8545",
		"WEBHOOK_HMAC_SECRET": strings.Repeat("h", 32), "NATS_URL": "nats://localhost:4222",
		"STATS_CURSOR_KEY": cfg.StatsCursorKey, "STATS_AUDIT_KEY": cfg.StatsAuditKey,
		"IDENTITY_ASSERTION_SIGNING_KEY": cfg.IdentitySigningKey, "IDENTITY_MTLS_CLIENT_KEY_PEM": cfg.IdentityMTLSClientKeyPEM,
		"NATS_CREDENTIALS_CREDS": cfg.NATSCredentialsCreds,
	}
	for name, value := range values {
		t.Setenv(name, value)
	}
	check, stop, err := cfg.ResolveProductionSecrets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	if cfg.DatabaseURL != values["DATABASE_URL"] || cfg.NATSURL != values["NATS_URL"] || cfg.WebhookHMACSecret != values["WEBHOOK_HMAC_SECRET"] {
		t.Fatalf("resolved fields: %+v", cfg)
	}
	if err := check(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WEBHOOK_HMAC_SECRET", strings.Repeat("z", 32))
	if err := check(context.Background()); err == nil || !strings.Contains(err.Error(), "required_secret_rotated_restart_required") {
		t.Fatalf("rotation: %v", err)
	}
	if err := check(context.Background()); err == nil {
		t.Fatal("rotation was forgotten")
	}
}

func TestProductionValidationBounds(t *testing.T) {
	cfg := productionConfigForTests()
	if err := cfg.ValidateProductionWorkers(); err != nil {
		t.Fatal(err)
	}
	if err := cfg.ValidateIdentityClient(); err != nil {
		t.Fatal(err)
	}
	cursor, audit, err := cfg.Phase2Keys()
	if err != nil || len(cursor) != 32 || len(audit) != 32 || hex.EncodeToString(cursor) != cfg.StatsCursorKey {
		t.Fatalf("phase2 keys: %x %x %v", cursor, audit, err)
	}
	cases := []struct {
		name     string
		edit     func(*Config)
		identity bool
	}{
		{"worker interval", func(c *Config) { c.PublicStatsIntervalMS = 100 }, false},
		{"worker batch", func(c *Config) { c.LegacyBridgeBatch = 1001 }, false},
		{"stream schema", func(c *Config) { c.PublicStreamSchema = "Api_Stream" }, false},
		{"identity origin", func(c *Config) { c.IdentityURL += "/path" }, true},
		{"identity source", func(c *Config) { c.IdentityMTLSCAFile = "/etc/ca.pem" }, true},
		{"identity pin", func(c *Config) { c.IdentityMTLSPeerCertSHA256 = strings.ToUpper(c.IdentityMTLSPeerCertSHA256) }, true},
		{"identity key", func(c *Config) { c.IdentitySigningKey = "invalid" }, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			copy := cfg
			tc.edit(&copy)
			var err error
			if tc.identity {
				err = copy.ValidateIdentityClient()
			} else {
				err = copy.ValidateProductionWorkers()
			}
			if err == nil {
				t.Fatal("invalid production config accepted")
			}
		})
	}
	for _, key := range []string{"StatsCursorKey", "StatsAuditKey"} {
		copy := cfg
		if key == "StatsCursorKey" {
			copy.StatsCursorKey = "bad"
		} else {
			copy.StatsAuditKey = "bad"
		}
		if _, _, err := copy.Phase2Keys(); err == nil {
			t.Fatalf("%s accepted", key)
		}
	}
}

func TestProductionPoolRequiresBoundRuntimeDocuments(t *testing.T) {
	for _, tc := range []struct {
		name     string
		edit     func(*Config)
		contains string
	}{
		{"profile", func(c *Config) { c.RuntimeProfile = "other" }, "runtime_profile_required"},
		{"container bind", func(c *Config) { c.BindAddress = "127.0.0.1" }, "container_bind_mismatch"},
		{"owned bind", func(c *Config) { c.RuntimeProfile = "owned_local" }, "owned_loopback_required"},
		{"budget", func(c *Config) { c.RuntimeBudgetDocument = "" }, "runtime_pool_configuration_missing"},
		{"digest", func(c *Config) {
			c.RuntimeBudgetDocument = "e30="
			c.RuntimePoolID = "stats"
			c.RuntimeInstanceID = "one"
		}, "runtime_budget_binding_mismatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := productionConfigForTests()
			cfg.RuntimeBudgetDocument = "e30="
			cfg.RuntimePoolID = "stats"
			cfg.RuntimeInstanceID = "one"
			tc.edit(&cfg)
			_, err := cfg.OpenRuntimePool(context.Background(), "statsd", sharedb.PoolOptions{})
			if err == nil || !strings.Contains(err.Error(), tc.contains) {
				t.Fatalf("pool error: %v", err)
			}
		})
	}
}
