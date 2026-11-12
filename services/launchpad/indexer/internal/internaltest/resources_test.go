package internaltest

import (
	"context"
	"github.com/redis/go-redis/v9"
	"testing"
)

func TestEphemeralDatabaseAndRedisExposeRealResources(t *testing.T) {
	dsn, pool := NewPostgres(t)
	if dsn == "" || pool == nil {
		t.Fatal("database resources absent")
	}
	var exists bool
	if err := pool.QueryRow(context.Background(), `SELECT to_regclass('indexer.cursors') IS NOT NULL`).Scan(&exists); err != nil || !exists {
		t.Fatalf("migrated cursor table %t %v", exists, err)
	}
	url := NewRedisURL(t)
	options, err := redis.ParseURL(url)
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(options)
	defer client.Close()
	if err = client.Set(context.Background(), "indexer-resource-lifecycle", "available", 0).Err(); err != nil {
		t.Fatal(err)
	}
	if got, err := client.Get(context.Background(), "indexer-resource-lifecycle").Result(); err != nil || got != "available" {
		t.Fatalf("redis round trip %q %v", got, err)
	}
}
