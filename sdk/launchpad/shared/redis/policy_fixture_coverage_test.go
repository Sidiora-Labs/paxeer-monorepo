package redis

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	r "github.com/redis/go-redis/v9"
)

func TestPolicySnapshotOnBoundedRedisFixture(t *testing.T) {
	endpoint := os.Getenv("SHARED_TEST_REDIS_URL")
	if endpoint == "" {
		t.Fatal("private Redis fixture is required")
	}
	options, err := r.ParseURL(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	client := r.NewClient(options)
	defer client.Close()
	ctx := context.Background()
	key := "policy_fixture:" + strconv.FormatInt(time.Now().UnixNano(), 10)
	defer client.Del(ctx, key)
	now := time.Now()
	snapshot := PolicySnapshot{Generation: "g1", Revision: "5", Visibility: valuePublic, Class: valuePublicSnapshot, SourceWatermark: json.RawMessage(`{"height":"125"}`), FetchedAt: now, FreshUntil: now.Add(time.Second), RetainUntil: now.Add(time.Minute), Body: json.RawMessage(`{"value":"current","requestId":"one"}`)}
	for _, tc := range []struct {
		left, right string
		want        int
	}{{"1", "2", -1}, {"12", "2", 1}, {"2", "1", 1}, {"2", "2", 0}} {
		got, err := CompareRevision(tc.left, tc.right)
		if err != nil || got != tc.want {
			t.Fatalf("revision order %s/%s = %d, %v", tc.left, tc.right, got, err)
		}
	}
	if _, err := CompareRevision("0", "1"); err == nil {
		t.Fatal("noncanonical revision accepted")
	}
	if ok, err := PutPolicySnapshot(ctx, client, key, snapshot, "", ""); err != nil || !ok {
		t.Fatalf("put policy snapshot = %t, %v", ok, err)
	}
	read, ok, err := GetPolicySnapshot(ctx, client, key)
	if err != nil || !ok || read.Revision != snapshot.Revision || read.ContentDigest != PolicySnapshotDigest(snapshot) {
		t.Fatalf("read policy snapshot = %+v, %t, %v", read, ok, err)
	}
	changedTrace := snapshot
	changedTrace.Body = json.RawMessage(`{"value":"current","requestId":"two"}`)
	if PolicyProjectionDigest(changedTrace) != PolicyProjectionDigest(snapshot) {
		t.Fatal("request trace changed authoritative projection")
	}
	changed := snapshot
	changed.Body = json.RawMessage(`{"value":"changed"}`)
	if ok, err := PutPolicySnapshot(ctx, client, key, changed, "", ""); ok || err == nil {
		t.Fatalf("equal revision changed projection = %t, %v", ok, err)
	}
	older := snapshot
	older.Revision = "4"
	if ok, err := PutPolicySnapshot(ctx, client, key, older, "", ""); ok || err != nil {
		t.Fatalf("older projection replaced current = %t, %v", ok, err)
	}
	floor := snapshot
	floor.Revision = "6"
	if ok, err := InvalidatePolicySnapshot(ctx, client, key, floor); err != nil || !ok {
		t.Fatalf("invalidation floor = %t, %v", ok, err)
	}
	if _, ok, err := GetPolicySnapshot(ctx, client, key); err != nil || ok {
		t.Fatalf("invalidation floor exposed as value = %t, %v", ok, err)
	}
	for _, raw := range []string{`{`, `{"generation":"g1"}`, `{"generation":"g1","revision":"1","visibility":"public","class":"public_snapshot","sourceWatermark":null,"fetchedAt":"2020-01-01T00:00:00Z","freshUntil":"2020-01-01T00:00:01Z","retainUntil":"2020-01-01T00:00:01Z","body":{}}`} {
		if err := client.Set(ctx, key, raw, time.Minute).Err(); err != nil {
			t.Fatal(err)
		}
		if _, ok, err := GetPolicySnapshot(ctx, client, key); err == nil || ok {
			t.Fatalf("invalid policy encoding served: %t, %v", ok, err)
		}
	}
	if err := client.Del(ctx, key).Err(); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := GetPolicySnapshot(ctx, client, key); err != nil || ok {
		t.Fatalf("missing snapshot served: %t, %v", ok, err)
	}
	if _, _, err := GetPolicySnapshot(ctx, client, ""); err == nil {
		t.Fatal("empty snapshot key accepted")
	}
	if ok, err := PutPolicySnapshot(ctx, client, "", snapshot, "", ""); ok || err == nil {
		t.Fatalf("empty policy destination accepted: %t, %v", ok, err)
	}
	expired := snapshot
	expired.FreshUntil = now.Add(-time.Second)
	expired.RetainUntil = now.Add(-time.Millisecond)
	if ok, err := PutPolicySnapshot(ctx, client, key, expired, "", ""); ok || err == nil {
		t.Fatalf("expired policy snapshot accepted: %t, %v", ok, err)
	}
	if ok, err := InvalidatePolicySnapshot(ctx, client, "", floor); ok || err == nil {
		t.Fatalf("empty invalidation key accepted: %t, %v", ok, err)
	}
	invalid := snapshot
	invalid.Visibility = "internal"
	if err := invalid.Validate(); err == nil {
		t.Fatal("unknown policy visibility accepted")
	}
	invalid = snapshot
	invalid.Class = "private_account"
	if err := invalid.Validate(); err == nil {
		t.Fatal("private class accepted in public cache")
	}
	invalid = snapshot
	invalid.Body = json.RawMessage(`"` + strings.Repeat("x", MaxSnapshotBytes) + `"`)
	if err := invalid.Validate(); err == nil {
		t.Fatal("oversized policy body accepted")
	}
}
