// Package internaltest provides real infrastructure (Postgres, Redis) for the
// social service's integration tests — never fakes and never containers: the
// helpers use the already running servers named by SOCIAL_TEST_POSTGRES_DSN /
// SOCIAL_TEST_REDIS_URL and fail the test immediately when those are unset.
// It is imported only by *_test.go files, so it adds nothing to the production
// binary.
package internaltest

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
)

// PostgresDSNEnv / RedisURLEnv name the already running Postgres / Redis the
// tests use.
//
//   - PostgresDSNEnv is an admin DSN whose role may CREATE DATABASE and CREATE ROLE. Each call
//     gets a throwaway database, dropped WITH (FORCE) on cleanup.
//   - RedisURLEnv is a redis:// URL that must name the logical database
//     reserved for these tests; it is flushed before and after each test, so
//     tests sharing it must not run concurrently.
const (
	PostgresDSNEnv = "SOCIAL_TEST_POSTGRES_DSN"
	RedisURLEnv    = "SOCIAL_TEST_REDIS_URL"
)

// NewPostgres bootstraps the coordinator-owned social schema and its producer
// interfaces on the server named by PostgresDSNEnv.
func NewPostgres(t *testing.T) *pgxpool.Pool {
	t.Helper()
	admin := os.Getenv(PostgresDSNEnv)
	if admin == "" {
		t.Fatalf("%s is not set: point it at a running PostgreSQL whose role may CREATE DATABASE and CREATE ROLE", PostgresDSNEnv)
	}
	return NewPostgresAt(t, admin)
}

func NewPostgresAt(t *testing.T, admin string) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	database := coordinatortest.Bootstrap(t, admin, "chat_runtime")
	cfg, err := pgxpool.ParseConfig(database.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = "chat,public"
	cfg.MaxConns = 8
	cfg.MaxConnIdleTime = 30 * time.Second
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// DevelopmentPostgresDSN retains the registered owner interfaces while letting
// the embedded social migrations construct chat from an empty schema.
func DevelopmentPostgresDSN(t *testing.T, admin string) string {
	t.Helper()
	database := coordinatortest.Bootstrap(t, admin, "chat_runtime")
	conn, err := pgx.Connect(context.Background(), database.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	if _, err = conn.Exec(context.Background(), "DROP SCHEMA chat CASCADE"); err != nil {
		t.Fatal(err)
	}
	return database.AdminDSN
}

func CoordinatorMigrationSQL(t *testing.T, schema string, version int) string {
	t.Helper()
	path := filepath.Join(coordinatortest.RepositoryRoot(t), "launchpad/spec/production-system/integration/migrations", fmt.Sprintf("%d.sql", version))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	ddl := regexp.MustCompile(`(?m)^SET LOCAL search_path[^\n]*\n`).ReplaceAllString(string(raw), "")
	quoted := pgx.Identifier{schema}.Sanitize()
	return strings.NewReplacer(`"chat".`, quoted+".", "chat.", quoted+".").Replace(ddl)
}

// SeedPool inserts a row into indexer.pools so creator-authorized deletes resolve.
func SeedPool(t *testing.T, pool *pgxpool.Pool, poolAddr, tokenAddr, creator string, createdAt int64) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO indexer.pools (pool_address, token_address, creator, created_at, optical, pool_id, created_block, tx_hash)
		 VALUES ($1,$2,$3,$4,$1,$5,1,$5)
		 ON CONFLICT (pool_address) DO UPDATE SET token_address = EXCLUDED.token_address, creator = EXCLUDED.creator, created_at = EXCLUDED.created_at`,
		poolAddr, tokenAddr, creator, createdAt, "0x"+strings.Repeat("0", 24)+strings.TrimPrefix(poolAddr, "0x")); err != nil {
		t.Fatalf("seed pool: %v", err)
	}
}

// NewRedisURL returns the RedisURLEnv URL after flushing its logical database;
// the database is flushed again on cleanup.
func NewRedisURL(t *testing.T) string {
	t.Helper()
	uri := os.Getenv(RedisURLEnv)
	if uri == "" {
		t.Fatalf("%s is not set: point it at a running Redis logical database reserved for these tests (redis://:password@host:port/N); tests never start containers", RedisURLEnv)
	}
	u, err := url.Parse(uri)
	if err != nil {
		t.Fatalf("parse %s: %v", RedisURLEnv, err)
	}
	pinned := strings.Trim(u.Path, "/")
	if pinned == "" {
		t.Fatalf("%s must name a logical database reserved for these tests (redis://:password@host:port/N)", RedisURLEnv)
	}
	if db, convErr := strconv.Atoi(pinned); convErr != nil || db < 0 || db > 15 {
		t.Fatalf("%s database %q is not 0..15", RedisURLEnv, pinned)
	}
	opt, err := goredis.ParseURL(uri)
	if err != nil {
		t.Fatalf("parse %s: %v", RedisURLEnv, err)
	}
	flush := func() {
		client := goredis.NewClient(opt)
		defer client.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := client.FlushDB(ctx).Err(); err != nil {
			t.Errorf("flush %s: %v", RedisURLEnv, err)
		}
	}
	flush()
	t.Cleanup(flush)
	return uri
}
