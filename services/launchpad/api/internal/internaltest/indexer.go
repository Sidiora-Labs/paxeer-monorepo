package internaltest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ApplyIndexerTables(t *testing.T, pool *pgxpool.Pool, tables ...string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(RepositoryRoot(t), "services/launchpad/indexer/migrations/00001_init.sql"))
	if err != nil {
		t.Fatal(err)
	}
	up := strings.SplitN(string(raw), "-- +goose Down", 2)[0]
	applied := 0
	for _, block := range strings.Split(up, "-- +goose StatementBegin") {
		statement := strings.SplitN(block, "-- +goose StatementEnd", 2)[0]
		for _, table := range tables {
			if strings.Contains(statement, `CREATE TABLE "indexer"."`+table+`"`) {
				if _, err := pool.Exec(context.Background(), statement); err != nil {
					t.Fatalf("indexer table %s: %v", table, err)
				}
				applied++
			}
		}
	}
	if applied != len(tables) {
		t.Fatalf("indexer migration applied %d of %v", applied, tables)
	}
}

func ApplyReadMigrations(t *testing.T, pool *pgxpool.Pool, names ...string) {
	t.Helper()
	for _, name := range names {
		raw, err := os.ReadFile(filepath.Join(RepositoryRoot(t), "migrations", name+".sql"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(context.Background(), string(raw)); err != nil {
			t.Fatalf("read migration %s: %v", name, err)
		}
	}
}
