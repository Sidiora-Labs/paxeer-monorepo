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

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
	sharedconfig "github.com/Sidiora-Technologies/KindleLaunch/shared/config"
	runtimedb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
)

func TestReadBoundDocumentUsesExactInlineAndFileBytes(t *testing.T) {
	setBaseEnv(t)
	raw := []byte(`{"service":"stats-workers","revision":7}`)
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])
	t.Setenv("DB_BUDGET_DOCUMENT", base64.StdEncoding.EncodeToString(raw))
	t.Setenv("DB_BUDGET_SHA256", digest)
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
	var inline struct {
		Service  string `json:"service"`
		Revision int    `json:"revision"`
	}
	if err := ReadBoundDocument("", digest, &inline); err != nil || inline.Service != "stats-workers" || inline.Revision != 7 {
		t.Fatalf("inline document: %+v, %v", inline, err)
	}
	path := filepath.Join(t.TempDir(), "budget.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	var file struct {
		Service  string `json:"service"`
		Revision int    `json:"revision"`
	}
	if err := ReadBoundDocument(path, digest, &file); err != nil || file != inline {
		t.Fatalf("file document: %+v, %v", file, err)
	}
	if err := ReadBoundDocument(path, strings.Repeat("0", 64), &file); err == nil {
		t.Fatal("changed checksum accepted")
	}
	if err := os.WriteFile(path, append(raw, []byte(` {}`)...), 0600); err != nil {
		t.Fatal(err)
	}
	trailing := append(append([]byte(nil), raw...), []byte(` {}`)...)
	trailingSum := sha256.Sum256(trailing)
	if err := ReadBoundDocument(path, hex.EncodeToString(trailingSum[:]), &file); err == nil {
		t.Fatal("trailing document accepted")
	}
}

func TestDevelopmentRuntimePoolReadsMigratedSchema(t *testing.T) {
	ctx := context.Background()
	dsn, _ := internaltest.NewPostgresWithDSN(t)
	cfg := Config{BaseEnv: sharedconfig.BaseEnv{DatabaseURL: dsn}, HolderRefreshIntervalMS: 60000}
	pool, err := cfg.OpenRuntimePool(ctx, "stats", runtimedb.PoolOptions{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := cfg.CheckRuntimeSchema(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var exists bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('stats.pool_stats') IS NOT NULL").Scan(&exists); err != nil || !exists {
		t.Fatalf("migrated statistics schema: exists=%v err=%v", exists, err)
	}
	if got := cfg.HolderRefreshInterval(); got.String() != "1m0s" {
		t.Fatalf("refresh interval=%s", got)
	}
}
