package redis

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestBoundedCacheRequiresHealthyRedisAndCanonicalEntries(t *testing.T) {
	endpoint := os.Getenv("SHARED_TEST_REDIS_URL")
	client, err := NewBoundedClient(endpoint, 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	options := client.Options()
	if !options.ContextTimeoutEnabled || options.MaxRetries != 0 || options.ReadTimeout != 100*time.Millisecond || options.DialTimeout != 100*time.Millisecond || options.PoolTimeout != 100*time.Millisecond {
		t.Fatalf("unbounded Redis options %+v", options)
	}
	for _, timeout := range []time.Duration{0, 2 * time.Second} {
		if _, err := NewBoundedClient(endpoint, timeout); err == nil {
			t.Fatal("invalid Redis timeout admitted")
		}
	}
	if _, err := NewBoundedClient("invalid", time.Second); err == nil {
		t.Fatal("invalid Redis URL admitted")
	}
	ctx := context.Background()
	key := "cache-boundary:" + strconv.FormatInt(time.Now().UnixNano(), 10)
	defer client.Del(ctx, key)
	for _, wire := range []string{strings.Repeat("x", MaxSnapshotBytes+1), `{`, `{"generation":"g1","revision":"0"}`} {
		if err := client.Set(ctx, key, wire, time.Minute).Err(); err != nil {
			t.Fatal(err)
		}
		if _, found, err := GetSnapshot(ctx, client, key); err == nil || found {
			t.Fatal("corrupt snapshot served")
		}
	}
	now := time.Now()
	snapshot := Snapshot{Generation: "g1", Revision: "1", SourceWatermark: json.RawMessage(`{}`), FetchedAt: now, ExpiresAt: now.Add(time.Minute), Body: json.RawMessage(`{}`)}
	if err := client.Del(ctx, key).Err(); err != nil {
		t.Fatal(err)
	}
	if _, found, err := GetSnapshot(ctx, client, key); err != nil || found {
		t.Fatalf("cache miss: %t %v", found, err)
	}
	if ok, err := PutSnapshot(ctx, client, key, snapshot); err != nil || !ok {
		t.Fatalf("snapshot put %t %v", ok, err)
	}
	other := snapshot
	other.Generation = "g2"
	if ok, err := PutSnapshot(ctx, client, key, other); err == nil || ok {
		t.Fatal("cross-generation snapshot replaced")
	}
	encoded := snapshot
	encoded.Body = json.RawMessage(`"` + strings.Repeat("x", MaxSnapshotBytes-4) + `"`)
	if ok, err := PutSnapshot(ctx, client, key, encoded); err == nil || ok {
		t.Fatal("encoded entry exceeded byte bound")
	}
	for _, args := range []struct{ base, spread time.Duration }{{0, 0}, {time.Second, -1}, {time.Second, time.Second}, {25 * time.Hour, 0}} {
		if _, err := JitterTTL(args.base, args.spread); err == nil {
			t.Fatal("invalid jitter admitted")
		}
	}
	if ttl, err := JitterTTL(time.Second, 0); err != nil || ttl != time.Second {
		t.Fatalf("zero jitter %v %v", ttl, err)
	}
	if _, err := AcquireRefresh(ctx, client, "", time.Second); err == nil {
		t.Fatal("empty refresh lease admitted")
	}
	if _, err := ReleaseRefresh(ctx, client, key, "short"); err == nil {
		t.Fatal("invalid refresh token admitted")
	}
	if _, err := InvalidateSnapshot(ctx, client, key, "g1", "0"); err == nil {
		t.Fatal("invalid snapshot floor admitted")
	}
	if _, err := Admit(ctx, client, "", 1, time.Second); err == nil {
		t.Fatal("empty admission key admitted")
	}
	if err := client.Set(ctx, key, "5", 0).Err(); err != nil {
		t.Fatal(err)
	}
	if _, err := Admit(ctx, client, key, 1, time.Second); err == nil {
		t.Fatal("counter without expiry admitted")
	}
	if (Admission{}).RetryAfterSeconds() != "1" {
		t.Fatal("retry minimum changed")
	}
	if err := client.Del(ctx, key).Err(); err != nil {
		t.Fatal(err)
	}
	fetched := 0
	for i := 0; i < 2; i++ {
		v, err := CacheGetOrSet(ctx, client, key, func() (int, error) { fetched++; return 7, nil }, time.Minute)
		if err != nil || v != 7 {
			t.Fatalf("cached value %d %v", v, err)
		}
	}
	if fetched != 1 {
		t.Fatalf("cache fetched %d times", fetched)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	cancelled, stop := context.WithCancel(ctx)
	stop()
	sentinel := errors.New("loader unavailable")
	for name, call := range map[string]func() error{
		"get":        func() error { _, _, err := CacheGet[int](ctx, client, key); return err },
		"set":        func() error { return CacheSet(ctx, client, key, 7, time.Minute) },
		"invalidate": func() error { return CacheInvalidate(ctx, client, key) },
		"read-through": func() error {
			_, err := CacheGetOrSet(ctx, client, key, func() (int, error) { t.Error("closed cache invoked loader"); return 0, sentinel }, time.Minute)
			return err
		},
		"snapshot":       func() error { _, _, err := GetSnapshot(ctx, client, key); return err },
		"write snapshot": func() error { _, err := PutSnapshot(ctx, client, key, snapshot); return err },
		"admission":      func() error { _, err := Admit(ctx, client, key, 1, time.Second); return err },
		"refresh":        func() error { _, err := AcquireRefresh(ctx, client, key, time.Second); return err },
		"publish":        func() error { return (&Publisher{rdb: client}).Publish(ctx, key, map[string]int{"value": 1}) },
		"raw publish":    func() error { return PublishJSON(cancelled, client, key, []byte(`{}`)) },
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Fatal("unavailable cache admitted operation")
			}
		})
	}
}

func TestProjectionKeysRejectUnverifiedScopesAndParameters(t *testing.T) {
	for _, tc := range []struct {
		scope  KeyScope
		params url.Values
	}{
		{KeyScope{}, nil},
		{KeyScope{RouteVersion: "v2", Generation: "g1", Visibility: "unknown"}, nil},
		{KeyScope{RouteVersion: "v2", Generation: "g1", Visibility: valuePrivate}, nil},
		{KeyScope{RouteVersion: "v2", Generation: "g1", Visibility: valuePublic, Principal: "owner"}, nil},
		{KeyScope{RouteVersion: "v2", Generation: "g1", Visibility: valuePrivate, Principal: strings.Repeat("x", 257), AuthorizationRevision: "1"}, nil},
		{KeyScope{RouteVersion: "v2", Generation: "g1", Visibility: valuePublic}, url.Values{"page": {"1", "2"}}},
	} {
		if _, err := ProjectionKey(tc.scope, tc.params); err == nil {
			t.Fatal("unverified cache scope admitted")
		}
	}
}
