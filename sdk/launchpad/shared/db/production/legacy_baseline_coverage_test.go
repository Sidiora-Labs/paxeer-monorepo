package production

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestLegacyBaselineImportsRecordedHistory(t *testing.T) {
	pool, dsn := phase2MigrationFixture(t)
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err := pool.Exec(ctx, "CREATE TABLE stats.legacy_source(value bigint NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, err := CatalogFingerprint(ctx, tx, "stats")
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	history, err := HistoryTable("core/stats-workers", "stats")
	if err != nil {
		t.Fatal(err)
	}
	source, up := []byte("recorded source migration"), []byte("CREATE TABLE stats.fixture(value bigint)")
	alias := LegacyMigrationAlias{Service: "core/stats-workers", OriginalVersion: 1, SourceSHA256: Checksum(source), UpSHA256: Checksum(up), StructuralFingerprint: fingerprint}

	for _, mode := range []string{"connection", "alias", "schema", "structure", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			candidate := alias
			connection := conn
			schema := "stats"
			callCtx := ctx
			switch mode {
			case "connection":
				connection = nil
			case "alias":
				candidate.Service = "unbound"
			case "schema":
				schema = "invalid schema"
			case "structure":
				candidate.StructuralFingerprint = Checksum([]byte("another structure"))
			case "cancelled":
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				callCtx = cancelled
			}
			if err := ImportLegacyBaseline(callCtx, connection, candidate, schema, source, up); err == nil {
				t.Fatal("unbound legacy baseline admitted")
			}
		})
	}
	if conn.IsClosed() {
		conn, err = pgx.Connect(ctx, dsn)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close(ctx)
	}
	locked, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := locked.Exec(ctx, "SELECT pg_advisory_lock($1)", CoordinatorKey); err != nil {
		t.Fatal(err)
	}
	if err := ImportLegacyBaseline(ctx, conn, alias, "stats", source, up); !errors.Is(err, ErrMigrationInProgress) {
		t.Fatalf("concurrent baseline import: %v", err)
	}
	if err := locked.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if err := ImportLegacyBaseline(ctx, conn, alias, "stats", source, up); err == nil {
		t.Fatal("unrecorded source migration admitted")
	}
	if _, err := pool.Exec(ctx, "INSERT INTO "+history+"(version_id,is_applied) VALUES($1,true)", alias.OriginalVersion); err != nil {
		t.Fatal(err)
	}
	if err := ImportLegacyBaseline(ctx, conn, alias, "stats", source, up); err != nil {
		t.Fatalf("recorded baseline import: %v", err)
	}
	var sourceDigest, upDigest, structure string
	if err := pool.QueryRow(ctx, "SELECT source_checksum,up_checksum,structural_fingerprint FROM migration_control.legacy_baselines WHERE service=$1 AND original_version=$2", alias.Service, alias.OriginalVersion).Scan(&sourceDigest, &upDigest, &structure); err != nil || sourceDigest != alias.SourceSHA256 || upDigest != alias.UpSHA256 || structure != fingerprint {
		t.Fatalf("imported baseline: %s, %s, %s, %v", sourceDigest, upDigest, structure, err)
	}
	if err := ImportLegacyBaseline(ctx, conn, alias, "stats", []byte("changed source"), up); err == nil {
		t.Fatal("changed source checksum admitted")
	}
	if err := ImportLegacyBaseline(ctx, conn, alias, "stats", source, up); err == nil {
		t.Fatal("baseline imported twice")
	}
}
