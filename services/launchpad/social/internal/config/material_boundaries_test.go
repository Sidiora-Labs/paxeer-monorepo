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

func TestRuntimeMaterialRejectsInvalidFilesAndEncodings(t *testing.T) {
	dir := t.TempDir()
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte("bound")))
	for _, tc := range []struct {
		name, path, inline string
	}{
		{"missing file", filepath.Join(dir, "absent"), ""},
		{"directory", dir, ""},
		{"invalid base64", "", "not-base64"},
		{"empty bytes", "", base64.StdEncoding.EncodeToString(nil)},
		{"long inline", "", strings.Repeat("A", 1398105)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := boundBytes(tc.path, tc.inline, digest); err == nil {
				t.Fatal("invalid runtime material accepted")
			}
		})
	}
	path := filepath.Join(dir, "empty")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Material(path, ""); err == nil {
		t.Fatal("empty material file accepted")
	}
	if _, err := Material(filepath.Join(dir, "absent"), ""); err == nil {
		t.Fatal("missing material file accepted")
	}
	if _, err := Material("", strings.Repeat("x", 1<<20+1)); err == nil {
		t.Fatal("oversized inline material accepted")
	}
	if _, err := Material(path, "bound"); err != nil {
		t.Fatalf("inline material rejected: %v", err)
	}
}

func TestRuntimeSecretsRejectUnboundProductionInputs(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name string
		cfg  Config
	}{
		{"development refresh", Config{NodeEnv: "development", SecretsProvider: "env"}},
		{"invalid environment", Config{NodeEnv: "staging", SecretsProvider: "env"}},
		{"invalid refresh", Config{NodeEnv: "production", RuntimeProfile: "container", SecretsProvider: "env", SecretsRefreshSeconds: -1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			check, stop, err := tc.cfg.ResolveProductionSecrets(ctx)
			if tc.name == "development refresh" {
				if err != nil || check != nil || stop == nil {
					t.Fatalf("development secret binding = %v", err)
				}
				stop()
				return
			}
			if err == nil {
				if stop != nil {
					stop()
				}
				t.Fatal("invalid production secret binding accepted")
			}
		})
	}
}
