package internaltest

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

func TestFixtureServices(t *testing.T) {
	ctx := context.Background()
	separate := NewPostgres(t)
	var one int
	if err := separate.QueryRow(ctx, "SELECT 1").Scan(&one); err != nil || one != 1 {
		t.Fatalf("separate database = %d, %v", one, err)
	}
	pool, dsn := NewPostgresURL(t)
	poolAddress := "0x" + strings.Repeat("1", 40)
	tokenAddress := "0x" + strings.Repeat("2", 40)
	creator := "0x" + strings.Repeat("3", 40)
	SeedPool(t, pool, poolAddress, tokenAddress, creator)
	var storedToken, storedCreator string
	if err := pool.QueryRow(ctx, "SELECT token_address, creator FROM indexer.pools WHERE pool_address=$1", poolAddress).Scan(&storedToken, &storedCreator); err != nil || storedToken != tokenAddress || storedCreator != creator {
		t.Fatalf("seeded pool = %q, %q, %v", storedToken, storedCreator, err)
	}
	connection, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("fixture database connection: %v", err)
	}
	defer connection.Close(ctx)

	redisURL := NewRedisURL(t)
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(options)
	defer client.Close()
	if err := client.Set(ctx, "fixture-object", "present", 0).Err(); err != nil {
		t.Fatal(err)
	}
	if got, err := client.Get(ctx, "fixture-object").Result(); err != nil || got != "present" {
		t.Fatalf("fixture Redis value = %q, %v", got, err)
	}

	objects := NewMinIO(t, "metadata-fixture")
	body := []byte("fixture image bytes")
	if err := objects.Store.Put(ctx, "objects/image", bytes.NewReader(body), int64(len(body)), "application/octet-stream"); err != nil {
		t.Fatal(err)
	}
	got, _, err := objects.Store.GetBytes(ctx, "objects/image", 1024)
	if err != nil || !bytes.Equal(got, body) {
		t.Fatalf("fixture object = %q, %v", got, err)
	}
	separateStore := NewStore(t, "metadata-fixture-store")
	if err := separateStore.Put(ctx, "objects/separate", bytes.NewReader(body), int64(len(body)), "application/octet-stream"); err != nil {
		t.Fatal(err)
	}
	separateBody, _, err := separateStore.GetBytes(ctx, "objects/separate", 1024)
	if err != nil || !bytes.Equal(separateBody, body) {
		t.Fatalf("separate object = %q, %v", separateBody, err)
	}
	var log lockedBuffer
	if n, err := log.Write([]byte("fixture ready")); err != nil || n != len("fixture ready") || log.String() != "fixture ready" {
		t.Fatalf("fixture log = %q, bytes=%d, %v", log.String(), n, err)
	}
	host, port := NewClamd(t)
	if (host == "") != (port == 0) {
		t.Fatalf("scanner address = %q:%d", host, port)
	}

	gateway := NewGateway(t, dsn, redisURL, objects, "https://app.sidiora.test")
	wallet, token := gateway.SignIn(t)
	if len(wallet) != 42 || token == "" {
		t.Fatalf("signed in identity = %q", wallet)
	}
	response, err := http.Get(gateway.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if _, err := io.Copy(io.Discard, response.Body); err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("gateway health = %d, %v", response.StatusCode, err)
	}
}
