package migrate_test

import (
	"context"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/migrate"
)

func TestUpKeepsMetadataSchema(t *testing.T) {
	pool, dsn := internaltest.NewPostgresURL(t)
	ctx := context.Background()
	if err := migrate.Up(ctx, dsn); err != nil {
		t.Fatal(err)
	}
	var version int
	if err := pool.QueryRow(ctx, "SELECT max(version_id) FROM goose_db_version WHERE is_applied").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 3 {
		t.Fatalf("migration version = %d, want 3", version)
	}
	var table string
	if err := pool.QueryRow(ctx, "SELECT to_regclass('metadata.legacy_publications')::text").Scan(&table); err != nil || table != "metadata.legacy_publications" {
		t.Fatalf("publication table = %q, %v", table, err)
	}
}
