package interactions

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestViewerCacheCoalescesReadsAndCancelsWaitingViewer(t *testing.T) {
	f := connected(t)
	actor, peer := f.wallets[0], f.wallets[1]
	ctx := f.ctx[actor]
	mustConnected(t, f.store.Follow(ctx, Request{Actor: actor, ID: peer, Key: "cached-edge"}))
	var zero ViewerCache
	stats, _, err := f.store.FollowStats(ctx, peer, actor, &zero)
	if err != nil || stats.Followers != "1" || !stats.IsFollowing || zero.Metrics().OriginFills != 1 {
		t.Fatalf("initial cache fill = %+v: %v", stats, err)
	}
	cache := NewViewerCache(f.redis)
	tx, err := f.db.Begin(ctx)
	mustConnected(t, err)
	defer rollback(tx)
	_, err = tx.Exec(ctx, `LOCK TABLE interaction_follows IN ACCESS EXCLUSIVE MODE`)
	mustConnected(t, err)
	type outcome struct {
		stats FollowStats
		err   error
	}
	read := func(ctx context.Context, done chan<- outcome) {
		v, _, err := f.store.FollowStats(ctx, peer, actor, cache)
		done <- outcome{v, err}
	}
	first, second, cancelled := make(chan outcome, 1), make(chan outcome, 1), make(chan outcome, 1)
	run, stop := context.WithTimeout(ctx, 5*time.Second)
	defer stop()
	go read(run, first)
	wait := func(condition func() bool) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for !condition() {
			if time.Now().After(deadline) {
				t.Fatal("cache fill did not reach expected state")
			}
			time.Sleep(time.Millisecond)
		}
	}
	wait(func() bool { return cache.Metrics().ActiveFills == 1 })
	go read(run, second)
	waiting, cancel := context.WithCancel(run)
	defer cancel()
	go read(waiting, cancelled)
	wait(func() bool { return cache.Metrics().CoalescedWaiters == 2 })
	cancel()
	result := <-cancelled
	if !errors.Is(result.err, context.Canceled) {
		t.Fatalf("waiting viewer cancellation = %v", result.err)
	}
	mustConnected(t, tx.Rollback(ctx))
	for _, done := range []<-chan outcome{first, second} {
		result := <-done
		if result.err != nil || result.stats.Followers != "1" || !result.stats.IsFollowing {
			t.Fatalf("coalesced cache read = %+v: %v", result.stats, result.err)
		}
	}
	metrics := cache.Metrics()
	if metrics.OriginFills != 1 || metrics.CoalescedWaiters != 2 || metrics.Hits != 1 || metrics.ActiveFills != 0 {
		t.Fatalf("coalesced cache metrics = %+v", metrics)
	}
}
