package config

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	runtimedb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
)

func TestProductionRuntimeDocumentBoundaries(t *testing.T) {
	raw := []byte(`{"service":"other"}`)
	digest := fmt.Sprintf("%x", sha256.Sum256(raw))
	encoded := base64.StdEncoding.EncodeToString(raw)
	config := Config{NodeEnv: literalProduction, RuntimeProfile: "owned_local", BindAddress: "127.0.0.1", RuntimePoolID: "pool-a", RuntimeInstanceID: "instance-a"}
	for _, tc := range []struct {
		name, document, hash, want string
	}{
		{"wrong digest", encoded, strings.Repeat("0", 64), "runtime_budget_binding_mismatch"},
		{"invalid budget", encoded, digest, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := config
			candidate.RuntimeBudgetDocument, candidate.RuntimeBudgetSHA256 = tc.document, tc.hash
			pool, err := candidate.OpenRuntimePool(context.Background(), "apid:rest", runtimedb.PoolOptions{})
			if pool != nil {
				pool.Close()
				t.Fatal("invalid runtime budget opened a pool")
			}
			if err == nil || tc.want != "" && !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("runtime budget error: %v", err)
			}
		})
	}
	config.ExpectedSchemaDocument, config.ExpectedSchemaSHA256 = encoded, digest
	if err := config.CheckRuntimeSchema(context.Background(), nil); err == nil {
		t.Fatal("invalid expected schema accepted")
	}
	config.ExpectedSchemaSHA256 = strings.Repeat("0", 64)
	if err := config.CheckRuntimeSchema(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "binding_mismatch") {
		t.Fatalf("unbound expected schema: %v", err)
	}
}

func TestRuntimeMaterialAndInlineDocumentBoundaries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "material")
	if err := os.WriteFile(path, []byte("identity-material"), 0600); err != nil {
		t.Fatal(err)
	}
	sym := filepath.Join(dir, "link")
	if err := os.Symlink(path, sym); err != nil {
		t.Fatal(err)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte("identity-material")))
	for _, tc := range []struct {
		name, path, inline, hash string
	}{
		{"symlink", sym, "", digest},
		{"missing", filepath.Join(dir, "missing"), "", digest},
		{"empty inline", "", base64.StdEncoding.EncodeToString(nil), digest},
		{"large inline", "", strings.Repeat("A", 1398105), digest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if data, err := boundBytes(tc.path, tc.inline, tc.hash); err == nil || data != nil {
				t.Fatalf("unbound document: %q %v", data, err)
			}
		})
	}
	if _, err := materialBytes("FILE", filepath.Join(dir, "missing"), "INLINE", ""); err == nil {
		t.Fatal("missing identity file accepted")
	}
	if _, err := materialBytes("FILE", "", "INLINE", strings.Repeat("x", 1048577)); err == nil {
		t.Fatal("oversized identity material accepted")
	}
	for _, tc := range []struct {
		name string
		cfg  Config
	}{
		{"missing CA", Config{NodeEnv: literalProduction, RuntimeProfile: "container"}},
		{"missing client certificate", Config{NodeEnv: literalProduction, RuntimeProfile: "container", IdentityCAPEM: "ca"}},
		{"missing client key", Config{NodeEnv: literalProduction, RuntimeProfile: "container", IdentityCAPEM: "ca", IdentityClientCertPEM: "cert"}},
		{"missing broker credentials", Config{NodeEnv: literalProduction, RuntimeProfile: "container", APIRole: "sockets", IdentityCAPEM: "ca", IdentityClientCertPEM: "cert", IdentityClientKeyPEM: "key"}},
		{"conflicting broker credentials", Config{NodeEnv: literalProduction, RuntimeProfile: "container", APIRole: "sockets", IdentityCAPEM: "ca", IdentityClientCertPEM: "cert", IdentityClientKeyPEM: "key", NATSCredentials: path, NATSCredentialsCreds: "inline"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.cfg.checkInlineMaterial(); err == nil {
				t.Fatal("incomplete runtime material accepted")
			}
		})
	}
}
