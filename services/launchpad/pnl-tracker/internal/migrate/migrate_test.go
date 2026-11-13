package migrate_test

import (
	"context"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/internaltest"
)

func TestMigrationCreatesPositionSchema(t *testing.T) {
	pool := internaltest.NewPostgres(t)
	var present bool
	if err := pool.QueryRow(context.Background(), `SELECT to_regclass('pnl.user_positions') IS NOT NULL`).Scan(&present); err != nil {
		t.Fatal(err)
	}
	if !present {
		t.Fatal("pnl positions table is absent after migration")
	}
}
