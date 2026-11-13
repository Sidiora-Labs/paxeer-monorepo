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
)

func TestRuntimeMaterialRejectsIncompleteTLSAndInvalidDocuments(t *testing.T) {
	for _, cfg := range []Config{{IdentityCAPEM: "ca"}, {IdentityCAPEM: "ca", IdentityClientCertPEM: "certificate"}} {
		if ca, cert, key, err := cfg.IdentityTLSMaterial(); err == nil || ca != nil || cert != nil || key != nil {
			t.Fatalf("incomplete TLS material returned: %q %q %q %v", ca, cert, key, err)
		}
	}
	var out struct {
		Value string `json:"value"`
	}
	raw := []byte(`{`)
	encoded := base64.StdEncoding.EncodeToString(raw)
	digest := fmt.Sprintf("%x", sha256.Sum256(raw))
	if err := decodeBoundDocument("", encoded, digest, &out); err == nil {
		t.Fatal("malformed JSON admitted")
	}
	empty := filepath.Join(t.TempDir(), "empty.json")
	if err := os.WriteFile(empty, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := boundBytes(empty, "", strings.Repeat("0", 64)); err == nil {
		t.Fatal("empty document admitted")
	}
	if _, err := inlineNATSCredentials(strings.Repeat("x", 1048577)); err == nil {
		t.Fatal("oversized credentials admitted")
	}
	jwt := "-----BEGIN NATS USER JWT-----\na.b.c\n------END NATS USER JWT------\n"
	if _, err := inlineNATSCredentials(jwt); err == nil {
		t.Fatal("credentials without seed admitted")
	}
	cfg := Config{NodeEnv: "production", SecretsProvider: "unsupported", SecretsRefreshSeconds: 1}
	if _, _, err := cfg.ResolveProductionSecrets(context.Background()); err == nil {
		t.Fatal("unsupported secret provider admitted")
	}
}
