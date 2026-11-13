package config

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sharedconfig "github.com/Sidiora-Technologies/KindleLaunch/shared/config"
	sharedb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
)

func TestRuntimeDocumentBoundsAndSchemaBinding(t *testing.T) {
	ctx := context.Background()
	raw := []byte(`{"identity":"stats"}`)
	digestBytes := sha256.Sum256(raw)
	digest := hex.EncodeToString(digestBytes[:])
	directory := t.TempDir()
	path := filepath.Join(directory, "budget.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, path, inline, digest string }{
		{"both sources", path, base64.StdEncoding.EncodeToString(raw), digest},
		{"neither source", "", "", digest},
		{"changed checksum", path, "", strings.Repeat("0", 64)},
		{"invalid inline encoding", "", "not-base64", digest},
		{"empty inline document", "", base64.StdEncoding.EncodeToString([]byte{}), digest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := boundBytes(tc.path, tc.inline, tc.digest); err == nil {
				t.Fatal("unbound document accepted")
			}
		})
	}
	link := filepath.Join(directory, "linked.json")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := boundBytes(link, "", digest); err == nil {
		t.Fatal("linked document accepted")
	}
	large := filepath.Join(directory, "large.json")
	if err := os.WriteFile(large, make([]byte, 1048577), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := boundBytes(large, "", digest); err == nil {
		t.Fatal("oversized document accepted")
	}
	cfg := productionConfigForTests()
	if err := cfg.CheckRuntimeSchema(ctx, nil); err == nil {
		t.Fatal("missing expected schema accepted")
	}
	cfg.ExpectedSchemaDocument = base64.StdEncoding.EncodeToString(raw)
	cfg.ExpectedSchemaSHA256 = digest
	if err := cfg.CheckRuntimeSchema(ctx, nil); err == nil {
		t.Fatal("incomplete expected schema accepted")
	}
	cfg.RuntimeBudgetDocument = base64.StdEncoding.EncodeToString(raw)
	cfg.RuntimeBudgetSHA256 = digest
	cfg.RuntimePoolID, cfg.RuntimeInstanceID = "stats", "instance"
	if _, err := cfg.OpenRuntimePool(ctx, "statsd", sharedb.PoolOptions{}); err == nil {
		t.Fatal("incomplete runtime budget accepted")
	}
}

func TestProductionSecretAndBrokerSourceBounds(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*Config)
	}{
		{"unknown environment", func(c *Config) { c.NodeEnv = "unknown" }},
		{"refresh interval", func(c *Config) { c.SecretsRefreshSeconds = 0 }},
		{"secret provider", func(c *Config) { c.SecretsProvider = "unknown" }},
		{"broker credentials", func(c *Config) { c.NATSCredentialsCreds = "" }},
		{"required secret", func(c *Config) {}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := productionConfigForTests()
			tc.edit(&cfg)
			if _, stop, err := cfg.ResolveProductionSecrets(context.Background()); err == nil {
				if stop != nil {
					stop()
				}
				t.Fatal("unbound production source accepted")
			}
		})
	}
	cfg := productionConfigForTests()
	for _, tc := range []struct {
		name string
		edit func(*Config)
	}{
		{"missing credentials", func(c *Config) { c.NATSCredentialsCreds = "" }},
		{"invalid inline credentials", func(c *Config) { c.NATSCredentialsCreds = "invalid" }},
		{"oversized inline credentials", func(c *Config) { c.NATSCredentialsCreds = strings.Repeat("x", 1048577) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := cfg
			tc.edit(&changed)
			if _, err := changed.RuntimeNATSOptions(); err == nil {
				t.Fatal("unbound broker credential accepted")
			}
		})
	}
	development := Config{BaseEnv: sharedconfig.BaseEnv{NodeEnv: "development"}}
	check, stop, err := development.ResolveProductionSecrets(context.Background())
	if err != nil || check != nil || stop == nil {
		t.Fatalf("development secret resolution: %v", err)
	}
	stop()
}
