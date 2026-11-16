package internaltest

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/jackc/pgx/v5"
)

// RuntimeDatabase contains the coordinator-owned schema and reviewed runtime
// privileges for the ranking service.
type RuntimeDatabase struct {
	// AdminDSN reaches the private database as the cluster administrator.
	AdminDSN string
	// RuntimeDSN logs in as the least-privilege ranking runtime role.
	RuntimeDSN  string
	Host        string
	Port        uint16
	Database    string
	RuntimeRole string
	// ExpectedSchema is the canonical ExpectedSchema JSON the runtime loads
	// through EXPECTED_SCHEMA_FILE / EXPECTED_SCHEMA_SHA256.
	ExpectedSchema []byte
	Epoch          int64
	Region         string
}

// rankingService is the ExpectedSchema.service the ranking runtime checks.
const rankingService = "core/ranking-algo"

// RuntimeRegion is the recovery region the local fixture bootstrap admits.
const RuntimeRegion = "fixture-loopback"

// RepositoryRoot is the monorepo root (the reviewed migration inputs live under
// spec/production-system).
func RepositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller unavailable")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..")
}

func readBound(t *testing.T, root, relative, want string) []byte {
	t.Helper()
	switch {
	case strings.HasPrefix(relative, "core/"):
		relative = "services/launchpad/" + strings.TrimPrefix(relative, "core/")
	case strings.HasPrefix(relative, "media/"):
		relative = "services/launchpad/" + strings.TrimPrefix(relative, "media/")
	case strings.HasPrefix(relative, "spec/"):
		relative = "launchpad/" + relative
	}
	f, err := os.OpenInRoot(root, relative)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			t.Errorf("close reviewed input: %v", closeErr)
		}
	}()
	raw, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	if want != "" && production.Checksum(raw) != want {
		t.Fatalf("%s differs from its reviewed sha256", relative)
	}
	return raw
}

// NewRankingRuntimeDatabase uses the shared migration coordinator bootstrap
// for a private database with the reviewed ranking runtime role.
func NewRankingRuntimeDatabase(t *testing.T) RuntimeDatabase {
	t.Helper()
	admin := os.Getenv(PostgresDSNEnv)
	if admin == "" {
		t.Fatalf("%s is not set: point it at a running PostgreSQL whose role may CREATE DATABASE and CREATE ROLE; tests never start containers", PostgresDSNEnv)
	}
	database := coordinatortest.Bootstrap(t, admin, "ranking_runtime")
	runtime := database.Runtimes["ranking_runtime"]
	config, err := pgx.ParseConfig(runtime.DSN)
	if err != nil {
		t.Fatal(err)
	}
	return RuntimeDatabase{
		AdminDSN: database.AdminDSN, RuntimeDSN: runtime.DSN,
		Host: config.Host, Port: config.Port, Database: database.Name,
		RuntimeRole: runtime.Role, ExpectedSchema: runtime.ExpectedSchema,
		Epoch: coordinatortest.Epoch, Region: coordinatortest.Region,
	}
}
