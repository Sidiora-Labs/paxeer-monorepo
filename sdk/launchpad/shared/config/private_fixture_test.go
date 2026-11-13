package config

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrivateFixtureEvidenceRequiresBoundLocalManifest(t *testing.T) {
	validEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.DeploymentRealm = valuePrivateFixture
	cfg.RPCURL = "http://127.0.0.1:8545"
	cfg.RPCURLFallback = ""
	if _, err := cfg.PrivateFixtureEvidence(); err == nil || !strings.Contains(err.Error(), "private_fixture_manifest_binding_missing") {
		t.Fatalf("missing manifest binding: %v", err)
	}

	path := filepath.Join(t.TempDir(), "manifest.json")
	write := func(data []byte) {
		t.Helper()
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(data)
		cfg.PrivateFixtureManifestFile = path
		cfg.PrivateFixtureManifestSHA256 = hex.EncodeToString(digest[:])
	}
	write([]byte(`{}`))
	if _, err := cfg.PrivateFixtureEvidence(); err == nil || !strings.Contains(err.Error(), "private_fixture_fields_missing") {
		t.Fatalf("manifest without required fields: %v", err)
	}

	cfg.RPCURL = "https://example.test:8545"
	if _, err := cfg.PrivateFixtureEvidence(); err == nil || !strings.Contains(err.Error(), "private_fixture_rpc_not_owned_loopback") {
		t.Fatalf("nonlocal endpoint: %v", err)
	}
	cfg.RPCURL = "http://127.0.0.1:8545"
	cfg.RPCURLFallback = "http://127.0.0.1:9545"
	if _, err := cfg.PrivateFixtureEvidence(); err == nil || !strings.Contains(err.Error(), "private_fixture_rpc_not_owned_loopback") {
		t.Fatalf("unbound fallback endpoint: %v", err)
	}
	cfg.RPCURLFallback = ""

	cfg.PrivateFixtureManifestSHA256 = strings.Repeat("0", 64)
	if _, err := cfg.PrivateFixtureEvidence(); err == nil || !strings.Contains(err.Error(), "private_fixture_manifest_hash_mismatch") {
		t.Fatalf("manifest digest mismatch: %v", err)
	}
	write(nil)
	if _, err := cfg.PrivateFixtureEvidence(); err == nil || !strings.Contains(err.Error(), "private_fixture_manifest_size_invalid") {
		t.Fatalf("empty manifest: %v", err)
	}
	write([]byte(`{"realm":"private-fixture","realm":"private-fixture"}`))
	if _, err := cfg.PrivateFixtureEvidence(); err == nil || !strings.Contains(err.Error(), "private_fixture_manifest_malformed") {
		t.Fatalf("duplicate manifest field: %v", err)
	}
	write([]byte(`{"schemaVersion":1}`))
	if _, err := cfg.PrivateFixtureEvidence(); err == nil || !strings.Contains(err.Error(), "private_fixture_fields_missing") {
		t.Fatalf("incomplete manifest: %v", err)
	}
}
