package config

import (
	"strings"
	"testing"
)

func TestPlatformConfigurationRequiresBoundPrivateAndPublicAuthorities(t *testing.T) {
	good := Config{
		NodeEnv: "production", RuntimeProfile: PlatformProfile, ProviderMode: "unavailable", Port: 3000,
		PrivateListenAddress: "127.0.0.1:8443", PrivateTLSCertPEM: "certificate", PrivateTLSClientCAPEM: "client roots", PrivateTLSKeyPEM: "key",
		GatewayPeerCertSHA256: strings.Repeat("a", 64), ViewerTokenKey: strings.Repeat("v", 32), DBMaxConns: 5,
		ExpectedSchemaFile: "schema.json", ExpectedSchemaSHA256: strings.Repeat("b", 64), CORSAllowedOrigins: "https://launchpad.test",
	}
	if err := good.ValidatePlatform(); err != nil {
		t.Fatalf("platform authority = %v", err)
	}
	for _, tc := range []struct {
		name, expected string
		change         func(*Config)
	}{
		{"connected", "LIVESTREAM_CONNECTED_ENABLED", func(c *Config) { c.ConnectedEnabled = true }},
		{"provider", "LIVEPEER_PROVIDER_MODE", func(c *Config) { c.ProviderMode = "available" }},
		{"bind", "BIND_ADDRESS", func(c *Config) { c.BindAddress = "127.0.0.1" }},
		{"private listener", "LIVESTREAM_PRIVATE_LISTEN_ADDRESS", func(c *Config) { c.PrivateListenAddress = "127.0.0.1:3000" }},
		{"certificate source", "LIVESTREAM_TLS_CERT_FILE", func(c *Config) { c.PrivateTLSCertFile = "certificate.pem" }},
		{"client roots", "LIVESTREAM_TLS_CLIENT_CA_FILE", func(c *Config) { c.PrivateTLSClientCAPEM = "" }},
		{"private key", "LIVESTREAM_TLS_KEY_FILE", func(c *Config) { c.PrivateTLSKeyPEM = "" }},
		{"gateway pin", "GATEWAY_PEER_CERT_SHA256", func(c *Config) { c.GatewayPeerCertSHA256 = strings.Repeat("A", 64) }},
		{"viewer token", "LIVESTREAM_VIEWER_TOKEN_KEY", func(c *Config) { c.ViewerTokenKey = "short" }},
		{"pool limit", "DB_MAX_CONNS", func(c *Config) { c.DBMaxConns = 201 }},
		{"schema binding", "EXPECTED_SCHEMA_FILE", func(c *Config) { c.ExpectedSchemaSHA256 = "bad" }},
		{"origin", "CORS_ALLOWED_ORIGINS entries", func(c *Config) { c.CORSAllowedOrigins = "http://launchpad.test" }},
		{"missing origin", "CORS_ALLOWED_ORIGINS must list", func(c *Config) { c.CORSAllowedOrigins = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := good
			tc.change(&cfg)
			if err := cfg.ValidatePlatform(); err == nil || !strings.Contains(err.Error(), tc.expected) {
				t.Fatalf("platform validation = %v", err)
			}
		})
	}
}

func TestBaseConfigurationValidatesProviderAndRequestEdges(t *testing.T) {
	good := Config{
		NodeEnv: "development", ProviderMode: "available", DatabaseURL: "postgres://localhost/livestream", RedisURL: "redis://localhost:6379",
		LivepeerAPIURL: "https://livepeer.studio/api", LivepeerAPIKey: "key", LivepeerWebhookSecret: strings.Repeat("s", 32),
		LogLevel: "info", Port: 3000, MaxStreamsPerWallet: 3,
	}
	if err := good.Validate(); err != nil {
		t.Fatalf("base configuration = %v", err)
	}
	for _, tc := range []struct {
		name, expected string
		change         func(*Config)
	}{
		{"provider mode", "LIVEPEER_PROVIDER_MODE", func(c *Config) { c.ProviderMode = "unknown" }},
		{"webhook secret", "LIVEPEER_WEBHOOK_SECRET", func(c *Config) { c.LivepeerWebhookSecret = "short" }},
		{"database URL", "DATABASE_URL", func(c *Config) { c.DatabaseURL = "no-host" }},
		{"redis URL", "REDIS_URL", func(c *Config) { c.RedisURL = "" }},
		{"provider URL", "LIVEPEER_API_URL", func(c *Config) { c.LivepeerAPIURL = "http://livepeer.studio/api" }},
		{"provider key", "LIVEPEER_API_KEY", func(c *Config) { c.LivepeerAPIKey = "" }},
		{"proxy CIDR", "TRUSTED_PROXY_CIDRS", func(c *Config) { c.TrustedProxyCIDRs = "not-a-cidr" }},
		{"log level", "LOG_LEVEL", func(c *Config) { c.LogLevel = "trace" }},
		{"port", "PORT", func(c *Config) { c.Port = 65536 }},
		{"wallet capacity", "MAX_STREAMS_PER_WALLET", func(c *Config) { c.MaxStreamsPerWallet = 0 }},
		{"document sources", "runtime_document_source_ambiguous", func(c *Config) { c.ExpectedSchemaFile, c.ExpectedSchemaDocument = "schema.json", "e30=" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := good
			tc.change(&cfg)
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), tc.expected) {
				t.Fatalf("configuration validation = %v", err)
			}
		})
	}
}
