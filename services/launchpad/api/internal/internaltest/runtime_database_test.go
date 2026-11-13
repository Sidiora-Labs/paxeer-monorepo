package internaltest

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestAPIRuntimeDatabaseGrantsReviewedReadRole(t *testing.T) {
	pool := NewPostgres(t)
	var schema string
	if err := pool.QueryRow(context.Background(), "SELECT to_regclass('stats.pool_stats')::text").Scan(&schema); err != nil || schema != "stats.pool_stats" {
		t.Fatalf("read schema: %q, %v", schema, err)
	}
	redis := NewRedis(t)
	if err := redis.Set(context.Background(), "fixture:ready", "yes", 0).Err(); err != nil {
		t.Fatal(err)
	}
	if value, err := redis.Get(context.Background(), "fixture:ready").Result(); err != nil || value != "yes" {
		t.Fatalf("redis fixture: %q, %v", value, err)
	}

	db := NewAPIRuntimeDatabase(t, "api_rest_runtime", "api_stream", "recovery")
	if db.RuntimeDSN == "" || db.AdminDSN == "" || db.RuntimeRole == "" || db.Epoch != 1 || db.Region != RuntimeRegion {
		t.Fatalf("runtime database metadata: %+v", db)
	}
	conn, err := pgx.Connect(context.Background(), db.RuntimeDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := conn.Close(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	var role string
	if err := conn.QueryRow(context.Background(), "SELECT current_user").Scan(&role); err != nil {
		t.Fatal(err)
	}
	if role != db.RuntimeRole {
		t.Fatalf("runtime role: %q", role)
	}
	admin, err := pgx.Connect(context.Background(), db.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := admin.Close(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	var epoch int64
	var region string
	if err := admin.QueryRow(context.Background(), "SELECT epoch,region FROM recovery.recovery_control WHERE singleton").Scan(&epoch, &region); err != nil {
		t.Fatal(err)
	}
	if epoch != db.Epoch || region != db.Region {
		t.Fatalf("recovery state: epoch=%d region=%q", epoch, region)
	}
}
