package config

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
)

func TestProductionSecretBindingDetectsRotation(t *testing.T) {
	values := map[string]string{
		"DATABASE_URL":                     os.Getenv("L40_TEST_POSTGRES_DSN"),
		"REDIS_URL":                        os.Getenv("L40_TEST_REDIS_URL"),
		"LIVEPEER_API_KEY":                 "fixture-provider-key",
		"LIVEPEER_WEBHOOK_SECRET":          strings.Repeat("s", 32),
		"IDENTITY_ASSERTION_SIGNING_KEY":   strings.Repeat("i", 32),
		"ACTION_AUDIT_KEY":                 strings.Repeat("a", 32),
		"LIVESTREAM_VIEWER_TOKEN_KEY":      strings.Repeat("v", 32),
		"LIVESTREAM_VAULT_TRANSIT_TOKEN":   strings.Repeat("t", 32),
		"LIVEPEER_PLAYBACK_WEBHOOK_SECRET": strings.Repeat("p", 32),
		"LIVESTREAM_BUDGET_ALERT_KEY":      strings.Repeat("b", 32),
	}
	if values["DATABASE_URL"] == "" || values["REDIS_URL"] == "" {
		t.Fatal("dedicated Postgres and Redis fixtures are required")
	}
	for name, value := range values {
		t.Setenv(name, value)
	}
	for _, connected := range []bool{false, true} {
		cfg := Config{NodeEnv: "production", SecretsProvider: "env", SecretsRefreshSeconds: 30, ProviderMode: "available", ConnectedEnabled: connected}
		if !connected {
			t.Setenv("IDENTITY_SERVICE_TOKEN", strings.Repeat("i", 32))
		}
		check, stop, err := cfg.ResolveProductionSecrets(context.Background())
		if err != nil || check == nil || stop == nil {
			t.Fatalf("secret bindings = %t %t %v", check != nil, stop != nil, err)
		}
		if cfg.DatabaseURL != values["DATABASE_URL"] || cfg.RedisURL != values["REDIS_URL"] || cfg.LivepeerAPIKey != values["LIVEPEER_API_KEY"] {
			t.Fatalf("resolved service secrets differ from configured values")
		}
		if err := check(context.Background()); err != nil {
			t.Fatal(err)
		}
		stop()
	}
	cfg := Config{NodeEnv: "production", SecretsProvider: "env", SecretsRefreshSeconds: 30, ProviderMode: "available"}
	check, stop, err := cfg.ResolveProductionSecrets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	t.Setenv("LIVEPEER_API_KEY", "rotated-provider-key")
	if err := check(context.Background()); err == nil || !strings.Contains(err.Error(), "rotated_restart_required") {
		t.Fatalf("rotated provider credential = %v", err)
	}
	if err := check(context.Background()); err == nil {
		t.Fatal("failed secret readiness recovered without restart")
	}
	cfg.SecretsRefreshSeconds = 0
	if _, _, err := cfg.ResolveProductionSecrets(context.Background()); err == nil {
		t.Fatal("unbounded refresh interval accepted")
	}
}

func TestIdentityAuthorityUsesPinnedCertificateAndSigningKey(t *testing.T) {
	pki := testpki.New(t)
	client := pki.Leaf("livestream-client", "livepeer.studio")
	peer := pki.Leaf("identity-peer", "livepeer.studio")
	signing := testpki.Ed25519(t)
	cfg := Config{IdentityURL: "https://livepeer.studio", IdentityCAFile: pki.CAFile, IdentityClientCertFile: client.CertFile, IdentityClientKeyFile: client.KeyFile, IdentityPeerCertSHA256: peer.SHA256, IdentitySigningKey: signing.StdBase64}
	authority, err := cfg.IdentityAuthority()
	if err != nil || authority == nil {
		t.Fatalf("pinned identity authority = %v", err)
	}
	cfg.IdentitySigningKey = "invalid"
	if _, err := cfg.IdentityAuthority(); err == nil {
		t.Fatal("invalid signing key accepted")
	}
	cfg.IdentitySigningKey = signing.StdBase64
	cfg.IdentityCAFile = filepath.Join(t.TempDir(), "missing-ca.pem")
	if _, err := cfg.IdentityAuthority(); err == nil {
		t.Fatal("missing identity CA accepted")
	}
	invalidCA := filepath.Join(t.TempDir(), "invalid-ca.pem")
	if err := os.WriteFile(invalidCA, []byte("not a certificate"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg.IdentityCAFile = invalidCA
	if _, err := cfg.IdentityAuthority(); err == nil || !strings.Contains(err.Error(), "ca_invalid") {
		t.Fatalf("invalid identity CA = %v", err)
	}
	cfg.IdentityCAFile = pki.CAFile
	cfg.IdentityClientCertFile = invalidCA
	if _, err := cfg.IdentityAuthority(); err == nil || !strings.Contains(err.Error(), "client_certificate_invalid") {
		t.Fatalf("invalid client certificate = %v", err)
	}
	cfg.IdentityClientCertFile = client.CertFile
	key, err := base64.StdEncoding.DecodeString(signing.StdBase64)
	if err != nil {
		t.Fatal(err)
	}
	key[len(key)-1] ^= 1
	cfg.IdentitySigningKey = base64.StdEncoding.EncodeToString(key)
	if _, err := cfg.IdentityAuthority(); err == nil || !strings.Contains(err.Error(), "signing_key_invalid") {
		t.Fatalf("changed signing key = %v", err)
	}
	cfg.IdentitySigningKey = signing.StdBase64
	cfg.IdentityPeerCertSHA256 = "invalid"
	if _, err := cfg.IdentityAuthority(); err == nil || !strings.Contains(err.Error(), "authority_invalid") {
		t.Fatalf("unbound identity peer = %v", err)
	}
	if _, err := (*Config)(nil).IdentityAuthority(); err == nil {
		t.Fatal("missing identity configuration accepted")
	}
}
