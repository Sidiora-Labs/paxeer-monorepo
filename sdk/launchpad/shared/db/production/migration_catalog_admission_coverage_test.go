package production

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestMigrationPublicationRequiresCompleteCatalogAuthority(t *testing.T) {
	for _, runner := range []string{"registered", "coordinated", "online"} {
		for _, mode := range []string{"seeded", "unreadable identity", "unknown legacy", "missing legacy", "changed legacy", "bound legacy", "missing attempts", "attempt constraint", "history constraint", "approval constraint", "invalid manifest"} {
			if (mode == "missing attempts" || mode == "attempt constraint") && runner == "registered" {
				continue
			}
			t.Run(runner+"/"+mode, func(t *testing.T) {
				pool, dsn := phase2MigrationFixture(t)
				ctx := context.Background()
				conn, err := pgx.Connect(ctx, dsn)
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = conn.Close(ctx) }()
				plan := OnlineIndexPlan{Schema: "stats", Table: "entries", Name: "entries_idx", Columns: []string{"id"}}
				sql := "CREATE TABLE stats.catalog_admitted(id bigint)"
				if runner == "online" {
					exec(t, pool, "SET ROLE stats_owner;CREATE TABLE stats.entries(id bigint);RESET ROLE")
					sql, err = plan.CreateSQL()
					if err != nil {
						t.Fatal(err)
					}
				}
				m := phase2RegisteredMigration(70001, sql, nil)
				if runner == "online" {
					m.TransactionMode = "nontransactional"
				}
				seed := Checksum([]byte("predecessor catalog"))
				_, err = pool.Exec(ctx, "INSERT INTO migration_control.applied_migrations(migration_id,service,schema_name,checksum,registry_digest) VALUES(70000,'core/stats-workers','stats',$1,$2)", seed, m.ApprovalManifestDigest)
				if err != nil {
					t.Fatal(err)
				}
				legacy := LegacyMigrationAlias{Service: "core/stats-workers", OriginalVersion: 1, SourceSHA256: seed, UpSHA256: seed, StructuralFingerprint: seed}
				switch mode {
				case "unreadable identity":
					exec(t, pool, "ALTER TABLE migration_control.applied_migrations RENAME TO original_applied_migrations;CREATE VIEW migration_control.applied_migrations AS SELECT 'unreadable'::text AS migration_id,checksum FROM migration_control.original_applied_migrations")
				case "unknown legacy":
					legacy.Service = "unknown"
					m.LegacyPredecessors = []LegacyMigrationAlias{legacy}
				case "missing legacy", "changed legacy", "bound legacy":
					m.LegacyPredecessors = []LegacyMigrationAlias{legacy}
					if mode != "missing legacy" {
						source := seed
						if mode == "changed legacy" {
							source = Checksum([]byte("other source"))
						}
						_, err = pool.Exec(ctx, "INSERT INTO migration_control.legacy_baselines(service,original_version,source_checksum,up_checksum,structural_fingerprint) VALUES($1,$2,$3,$4,$4)", legacy.Service, legacy.OriginalVersion, source, seed)
						if err != nil {
							t.Fatal(err)
						}
					}
				case "missing attempts":
					exec(t, pool, "ALTER TABLE migration_control.migration_attempts RENAME TO archived_attempts")
				case "attempt constraint":
					exec(t, pool, "ALTER TABLE migration_control.migration_attempts ADD CONSTRAINT admission_guard CHECK(migration_id<>70001)")
				case "history constraint":
					exec(t, pool, "ALTER TABLE migration_control.stats_workers_versions ADD CONSTRAINT admission_guard CHECK(version_id<>70001)")
				case "approval constraint":
					exec(t, pool, "ALTER TABLE migration_control.applied_migrations ADD CONSTRAINT admission_guard CHECK(migration_id<>70001)")
				case "invalid manifest":
					m.SHA256 = Checksum([]byte("different source"))
				}
				registry := map[int64]MigrationManifest{m.MigrationID: m}
				switch runner {
				case "registered":
					err = ApplyRegisteredMigration(ctx, conn, m, []byte(sql), registry, m.InterfaceDigest, m.ApprovalManifestDigest, 1)
				case "coordinated":
					err = ApplyCoordinatedMigration(ctx, conn, m, []byte(sql), registry, m.InterfaceDigest, m.ApprovalManifestDigest, 1)
				case "online":
					err = ApplyOnlineIndexMigration(ctx, conn, m, plan, registry, m.InterfaceDigest, m.ApprovalManifestDigest, 1, nil)
				}
				admitted := mode == "seeded" || mode == "bound legacy"
				if admitted && err != nil {
					t.Fatal(err)
				}
				if !admitted && err == nil {
					t.Fatal("migration without complete catalog authority admitted")
				}
				catalog := "migration_control.applied_migrations"
				if mode == "unreadable identity" {
					catalog = "migration_control.original_applied_migrations"
				}
				var count int
				if e := pool.QueryRow(ctx, "SELECT count(*) FROM "+catalog+" WHERE migration_id=70001").Scan(&count); e != nil {
					t.Fatal(e)
				}
				if (admitted && count != 1) || (!admitted && count != 0) {
					t.Fatalf("publication count %d, admitted %t", count, admitted)
				}
				var role string
				if e := conn.QueryRow(ctx, "SELECT current_user").Scan(&role); e != nil {
					t.Fatal(e)
				}
				if strings.HasSuffix(role, "_owner") {
					t.Fatalf("migration retained owner role %s", role)
				}
			})
		}
	}
}
