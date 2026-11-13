package internaltest

import (
	"context"
	"strings"
	"testing"

	goredis "github.com/redis/go-redis/v9"
)

func TestInfrastructureFixturesUseIsolatedResources(t *testing.T) {
	ctx := context.Background()
	pool := NewPostgres(t)
	const wallet = "0x1111111111111111111111111111111111111111"
	const address = "0x2222222222222222222222222222222222222222"
	SeedPool(t, pool, address, address, wallet, 1760000000)
	var creator string
	if err := pool.QueryRow(ctx, `SELECT creator FROM indexer.pools WHERE pool_address=$1`, address).Scan(&creator); err != nil || creator != wallet {
		t.Fatalf("isolated indexer pool = %q, %v", creator, err)
	}
	uri := NewRedisURL(t)
	options, err := goredis.ParseURL(uri)
	if err != nil {
		t.Fatal(err)
	}
	redis := goredis.NewClient(options)
	t.Cleanup(func() {
		if err := redis.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := redis.Ping(ctx).Err(); err != nil {
		t.Fatalf("isolated Redis database: %v", err)
	}
	objects := NewStore(t, "user-fixture")
	if _, err := objects.Head(ctx, "absent-object"); err == nil {
		t.Fatal("new bucket already contains an object")
	}
	t.Setenv("USER_TEST_CLAMAV_ADDR", "")
	if host, port := NewClamd(t); host != "" || port != 0 {
		t.Fatalf("optional scanner = %q:%d", host, port)
	}
	t.Setenv("USER_TEST_CLAMAV_ADDR", "127.0.0.1:3310")
	if host, port := NewClamd(t); host != "127.0.0.1" || port != 3310 {
		t.Fatalf("configured scanner = %q:%d", host, port)
	}
}

func TestScannerAddressValidatesHostAndPort(t *testing.T) {
	for _, tc := range []struct {
		address string
		host    string
		port    int
		valid   bool
	}{
		{"scanner.example.org:3310", "scanner.example.org", 3310, true},
		{"[::1]:3310", "::1", 3310, true},
		{"scanner.example.org", "", 0, false},
		{"scanner.example.org:0", "", 0, false},
		{"scanner.example.org:65536", "", 0, false},
		{"scanner.example.org:abc", "", 0, false},
	} {
		t.Run(tc.address, func(t *testing.T) {
			host, port, err := parseClamdAddress(tc.address)
			if tc.valid && (err != nil || host != tc.host || port != tc.port) {
				t.Fatalf("scanner address = %q:%d, %v", host, port, err)
			}
			if !tc.valid && (err == nil || host != "" || port != 0) {
				t.Fatalf("invalid scanner address = %q:%d, %v", host, port, err)
			}
			if !tc.valid && !strings.Contains(err.Error(), "port") {
				t.Fatalf("scanner address error = %v", err)
			}
		})
	}
}
