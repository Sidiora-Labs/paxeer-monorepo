package migrate_test

import (
	"context"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/migrate"
)

func TestUpKeepsMigratedUserSchema(t *testing.T) {
	pool := internaltest.NewPostgres(t)
	if err := migrate.Up(context.Background(), pool.Config().ConnString()); err != nil {
		t.Fatal(err)
	}
	var users, account bool
	if err := pool.QueryRow(context.Background(), `SELECT to_regclass('users.user_profiles') IS NOT NULL, to_regclass('public.account_state') IS NOT NULL`).Scan(&users, &account); err != nil || !users || !account {
		t.Fatalf("migrated user relations = %t, %t, %v", users, account, err)
	}
}
