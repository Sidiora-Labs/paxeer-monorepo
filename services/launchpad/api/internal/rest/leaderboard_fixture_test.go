package rest_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	"github.com/jackc/pgx/v5/pgxpool"
)

func applyLeaderboardTransfers(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(internaltest.RepositoryRoot(t), "migrations", "190001.sql"))
	if err != nil {
		t.Fatal(err)
	}
	applied := 0
	for _, statement := range strings.Split(string(raw), ";") {
		statement = strings.TrimSpace(statement)
		if strings.HasPrefix(statement, "CREATE TABLE indexer.indexer_token_transfers (") || strings.HasPrefix(statement, "CREATE INDEX indexer_token_movements_order ") || strings.HasPrefix(statement, "CREATE VIEW indexer.indexer_token_transfers_v1 ") {
			if _, err := pool.Exec(context.Background(), statement); err != nil {
				t.Fatal(err)
			}
			applied++
		}
	}
	if applied != 3 {
		t.Fatalf("token transfer migration applied %d statements, want 3", applied)
	}
}
