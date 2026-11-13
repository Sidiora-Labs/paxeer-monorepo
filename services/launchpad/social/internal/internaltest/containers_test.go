package internaltest

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	goredis "github.com/redis/go-redis/v9"
)

func TestFixtureSeedsPoolAndCanonicalSource(t *testing.T) {
	ctx := context.Background()
	db := NewPostgres(t)
	address := "0x" + strings.Repeat("1", 40)
	token := "0x" + strings.Repeat("2", 40)
	creator := "0x" + strings.Repeat("3", 40)
	SeedPool(t, db, address, token, creator, 123)
	var gotToken, gotCreator string
	if err := db.QueryRow(ctx, `SELECT token_address,creator FROM indexer.pools WHERE pool_address=$1`, address).Scan(&gotToken, &gotCreator); err != nil {
		t.Fatal(err)
	}
	if gotToken != token || gotCreator != creator {
		t.Fatalf("pool owner = %s, token = %s", gotCreator, gotToken)
	}
	SeedCanonicalSource(t, db)
	var rawCount, completeCount int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM indexer.indexer_raw_logs WHERE chain_id=125`).Scan(&rawCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `SELECT count(*) FROM indexer.indexer_block_completeness WHERE chain_id=125 AND fetch_complete`).Scan(&completeCount); err != nil {
		t.Fatal(err)
	}
	if rawCount != 1 || completeCount != 1 {
		t.Fatalf("canonical source rows = %d, complete blocks = %d", rawCount, completeCount)
	}

	uri := NewRedisURL(t)
	options, err := goredis.ParseURL(uri)
	if err != nil {
		t.Fatal(err)
	}
	client := goredis.NewClient(options)
	t.Cleanup(func() { _ = client.Close() })
	if err = client.Set(ctx, "social-fixture-key", "visible", 0).Err(); err != nil {
		t.Fatal(err)
	}
	if value, err := client.Get(ctx, "social-fixture-key").Result(); err != nil || value != "visible" {
		t.Fatalf("redis value = %q: %v", value, err)
	}
}

func TestDevelopmentDatabaseAndCoordinatorMigration(t *testing.T) {
	admin := os.Getenv(PostgresDSNEnv)
	if admin == "" {
		t.Fatalf("%s is required", PostgresDSNEnv)
	}
	ctx := context.Background()
	dsn := DevelopmentPostgresDSN(t, admin)
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := conn.Close(ctx); err != nil {
			t.Error(err)
		}
	}()
	var absent bool
	if err := conn.QueryRow(ctx, `SELECT to_regnamespace('chat') IS NULL`).Scan(&absent); err != nil || !absent {
		t.Fatalf("development schema absent = %t: %v", absent, err)
	}
	ddl := CoordinatorMigrationSQL(t, "review_schema", 380001)
	if !strings.Contains(ddl, `CREATE TABLE safety_moderator_sessions`) || !strings.Contains(ddl, `REVOKE ALL ON "review_schema"."safety_moderator_sessions"`) || strings.Contains(ddl, "SET LOCAL search_path") {
		t.Fatal("coordinator moderation schema binding is incomplete")
	}
}
