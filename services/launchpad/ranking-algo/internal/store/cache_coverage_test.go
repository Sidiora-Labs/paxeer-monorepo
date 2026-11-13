package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/ranker"
)

func TestSnapshotCachePreservesPageIdentity(t *testing.T) {
	rdb := internaltest.NewRedis(t)
	if _, err := NewSnapshotCache(nil, "ranking-test"); !errors.Is(err, ranker.ErrUnavailable) {
		t.Fatalf("missing cache client accepted: %v", err)
	}
	if _, err := NewSnapshotCache(rdb, "!"); !errors.Is(err, ranker.ErrUnavailable) {
		t.Fatalf("invalid cache namespace accepted: %v", err)
	}
	cache, err := NewSnapshotCache(rdb, "ranking-test")
	if err != nil {
		t.Fatal(err)
	}
	h := ranker.Header{
		RunID: "be0ebdd7-e6e2-4f9a-b856-68e7fb31bf22", Category: "trending",
		InputDigest: strings.Repeat("a", 64), OutputDigest: strings.Repeat("b", 64),
		AlgorithmVersion: ranker.AlgorithmVersion, CutoffTime: 1000,
		CompletedAt: time.Unix(1001, 0), Revision: 1, Total: 0,
	}
	page := ranker.StoredPage{Header: h, Items: []ranker.RankedItem{}, Offset: 0}
	ctx := context.Background()
	if err := cache.Put(ctx, page, 100); err != nil {
		t.Fatal(err)
	}
	got, err := cache.Get(ctx, h, 0, 100)
	actual, expected := got.Header, h
	actual.CompletedAt = time.Time{}
	expected.CompletedAt = time.Time{}
	if err != nil || actual != expected || !got.Header.CompletedAt.Equal(h.CompletedAt) || len(got.Items) != 0 {
		t.Fatalf("cached page = %+v, %v", got, err)
	}
	if _, err := cache.Get(ctx, h, 0, 101); !errors.Is(err, ranker.ErrUnavailable) {
		t.Fatalf("out-of-range page accepted: %v", err)
	}
	if err := cache.Put(ctx, page, 0); !errors.Is(err, ranker.ErrUnavailable) {
		t.Fatalf("out-of-range limit accepted: %v", err)
	}
	incomplete := page
	incomplete.Header.Total = 1
	if err := cache.Put(ctx, incomplete, 100); !errors.Is(err, ranker.ErrUnavailable) {
		t.Fatalf("incomplete cached page accepted: %v", err)
	}
	changed := page
	changed.Header.Revision++
	raw, err := json.Marshal(changed)
	if err != nil {
		t.Fatal(err)
	}
	if err := rdb.Set(ctx, cache.key(h, 0, 100), raw, time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.Get(ctx, h, 0, 100); !errors.Is(err, ranker.ErrUnavailable) {
		t.Fatalf("changed cached revision accepted: %v", err)
	}
	if err := rdb.Set(ctx, cache.key(h, 0, 100), "{bad", time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.Get(ctx, h, 0, 100); err == nil {
		t.Fatal("malformed cached page accepted")
	}
	if err := rdb.Set(ctx, cache.key(h, 0, 100), strings.Repeat("x", 65537), time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.Get(ctx, h, 0, 100); !errors.Is(err, ranker.ErrUnavailable) {
		t.Fatalf("oversized cached page accepted: %v", err)
	}
}
