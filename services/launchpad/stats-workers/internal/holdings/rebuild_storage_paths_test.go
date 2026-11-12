package holdings

import (
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestCanonicalRebuildRejectsUnavailableRelations(t *testing.T) {
	for _, tc := range []struct{ name, setup string }{
		{"creation source", "ALTER VIEW indexer.indexer_canonical_creations_v1 RENAME TO saved_creations"},
		{"block source", "ALTER VIEW indexer.indexer_block_completeness_v1 RENAME TO saved_blocks"},
		{"raw source", "ALTER VIEW indexer.indexer_raw_logs_v1 RENAME TO saved_logs"},
		{"token insert", "ALTER TABLE rebuild_storage.tokens ADD CHECK (false) NOT VALID"},
		{"transfer insert", "ALTER TABLE rebuild_storage.transfers ADD CHECK (false) NOT VALID"},
		{"checkpoint insert", "ALTER TABLE rebuild_storage.checkpoints ADD CHECK (false) NOT VALID"},
		{"coverage update", "ALTER TABLE rebuild_storage.tokens ADD CHECK (coverage <> 'complete') NOT VALID"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, pool, mint := canonicalLocalFixture(t)
			ddl, err := SchemaSQL("rebuild_storage")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = pool.Exec(ctx, ddl); err != nil {
				t.Fatal(err)
			}
			repo, err := NewRepository(pool, "rebuild_storage")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = pool.Exec(ctx, tc.setup); err != nil {
				t.Fatal(err)
			}
			_, err = repo.RebuildFromCanonical(ctx, pool, "canonical", mint.Token, 6, "10", "10", "holdings_v1")
			var storage *pgconn.PgError
			code := "42P01"
			if strings.Contains(tc.setup, "CHECK") {
				code = "23514"
			}
			if !errors.As(err, &storage) || storage.Code != code {
				t.Fatalf("rebuild storage: %v, want SQLSTATE %s", err, code)
			}
			var checkpoints int
			if err = pool.QueryRow(ctx, "SELECT count(*) FROM rebuild_storage.checkpoints").Scan(&checkpoints); err != nil || checkpoints != 0 {
				t.Fatalf("uncommitted checkpoint: %d %v", checkpoints, err)
			}
		})
	}
}

func TestCanonicalRebuildValidatesRangeAndDestination(t *testing.T) {
	ctx, pool, mint := canonicalLocalFixture(t)
	ddl, err := SchemaSQL("rebuild_destination")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	repo, err := NewRepository(pool, "rebuild_destination")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, token, creation, through string
		want                           string
	}{
		{"token", "invalid", "10", "10", "invalid address"},
		{"creation integer", mint.Token, "01", "10", "invalid uint256"},
		{"range overflow", mint.Token, "18446744073709551616", "18446744073709551616", ErrOrder.Error()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := repo.RebuildFromCanonical(ctx, pool, "canonical", tc.token, 6, tc.creation, tc.through, "holdings_v1"); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("range admission: %v", err)
			}
		})
	}
	if _, err = repo.RebuildFromCanonical(ctx, nil, "canonical", mint.Token, 6, "10", "10", "holdings_v1"); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("source admission: %v", err)
	}
	if err = repo.DiscoverCanonical(ctx, mint.Token, "canonical"); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.RebuildFromCanonical(ctx, pool, "canonical", mint.Token, 6, "10", "10", "holdings_v1"); !errors.Is(err, ErrConflict) {
		t.Fatalf("live destination replaced: %v", err)
	}
	var count int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM rebuild_destination.transfers").Scan(&count); err != nil || count != 0 {
		t.Fatalf("live destination mutated: %d %v", count, err)
	}
}
