package migrate

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/internaltest"
)

func TestUpAppliesSocialSchemaAndIsRepeatable(t *testing.T) {
	ctx := context.Background()
	admin := os.Getenv("SOCIAL_FIXTURE_POSTGRES_DSN")
	if admin == "" {
		admin = "postgres://kindle_fixture:kindle_fixture_password@127.0.0.1:25432/kindle_fixture?sslmode=disable"
	}
	dsn := internaltest.DevelopmentPostgresDSN(t, admin)
	for range 2 {
		if err := Up(ctx, dsn); err != nil {
			t.Fatal(err)
		}
	}
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	var current int64
	if err = conn.QueryRow(ctx, `SELECT max(version_id) FROM public.goose_db_version WHERE is_applied`).Scan(&current); err != nil {
		t.Fatal(err)
	}
	if current != 5 {
		t.Fatalf("migration version = %d, want 5", current)
	}
	var exists bool
	if err = conn.QueryRow(ctx, `SELECT to_regclass('chat.interaction_comments') IS NOT NULL`).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("social comment relation is absent")
	}
}
