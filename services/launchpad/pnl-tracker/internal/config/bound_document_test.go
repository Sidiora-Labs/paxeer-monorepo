package config_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/internaltest"
)

func TestBoundRuntimeDocumentSources(t *testing.T) {
	setBaseEnv(t)
	data := []byte(`{"service":"core/pnl-tracker"}`)
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	path := filepath.Join(t.TempDir(), "schema.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	var fromFile struct {
		Service string `json:"service"`
	}
	if err := config.ReadBoundDocument(path, digest, &fromFile); err != nil || fromFile.Service != "core/pnl-tracker" {
		t.Fatalf("file document = %+v, error = %v", fromFile, err)
	}
	t.Setenv("EXPECTED_SCHEMA_DOCUMENT", base64.StdEncoding.EncodeToString(data))
	t.Setenv("EXPECTED_SCHEMA_SHA256", digest)
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	var fromInline struct {
		Service string `json:"service"`
	}
	if err := config.ReadBoundDocument("", digest, &fromInline); err != nil || fromInline != fromFile {
		t.Fatalf("inline document = %+v, error = %v", fromInline, err)
	}
	if err := config.ReadBoundDocument(path, strings.Repeat("0", 64), &fromFile); err == nil {
		t.Fatal("accepted a different document digest")
	}
	if err := os.WriteFile(path, []byte(`{"service":"core/pnl-tracker"} true`), 0600); err != nil {
		t.Fatal(err)
	}
	trailing, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	trailingSum := sha256.Sum256(trailing)
	if err := config.ReadBoundDocument(path, hex.EncodeToString(trailingSum[:]), &fromFile); err == nil {
		t.Fatal("accepted trailing document data")
	}
}

func TestDevelopmentSchemaCheckUsesDatabase(t *testing.T) {
	pool := internaltest.NewPostgres(t)
	cfg := config.Config{}
	cfg.NodeEnv = "development"
	if err := cfg.CheckRuntimeSchema(context.Background(), pool); err != nil {
		t.Fatalf("development database check: %v", err)
	}
	check, stop, err := cfg.ResolveProductionSecrets(context.Background())
	if err != nil || check != nil || stop == nil {
		t.Fatalf("development secret resolution: check=%v stop=%v error=%v", check != nil, stop != nil, err)
	}
	stop()
}

func TestLoadRejectsConflictingRuntimeDocuments(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("DB_BUDGET_FILE", "/tmp/budget.json")
	t.Setenv("DB_BUDGET_DOCUMENT", base64.StdEncoding.EncodeToString([]byte(`{}`)))
	if _, err := config.Load(); err == nil || !strings.Contains(err.Error(), "DB_BUDGET_FILE and DB_BUDGET_DOCUMENT") {
		t.Fatalf("conflicting budget sources: %v", err)
	}
}

func TestLoadRejectsOutOfRangeReconcileInterval(t *testing.T) {
	for _, interval := range []string{"abc", "0", "9223372036855"} {
		t.Run(interval, func(t *testing.T) {
			setBaseEnv(t)
			t.Setenv("PNL_RECONCILE_INTERVAL_MS", interval)
			if _, err := config.Load(); err == nil || !strings.Contains(err.Error(), "PNL_RECONCILE_INTERVAL_MS") {
				t.Fatalf("interval %q: %v", interval, err)
			}
		})
	}
}
