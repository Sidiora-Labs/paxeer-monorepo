package l18

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/internaltest"
	"github.com/jackc/pgx/v5"
)

func fixtureRoot() string {
	_, source, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(source), "../../../../../.."))
}

func localChainFixture(t *testing.T) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	_, fixture := internaltest.NewAnvilFixture(t, ctx, fixtureRoot())
	raw, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "fixture.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func phase2Migration(t *testing.T, schema string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(fixtureRoot(), "migrations/180001.sql"))
	if err != nil {
		t.Fatal(err)
	}
	quoted := pgx.Identifier{schema}.Sanitize()
	migration := strings.ReplaceAll(string(raw), `"pnl"`, quoted)
	return strings.ReplaceAll(migration, "pnl.", quoted+".")
}
