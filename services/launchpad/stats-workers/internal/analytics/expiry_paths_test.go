package analytics_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/analytics"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/risk"
)

func TestAnalyticsExpiryRefreshesCommittedPool(t *testing.T) {
	authority := phase2Authority(t)
	ctx := context.Background()
	pool := "0x" + strings.Repeat("1", 40)
	observed := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	if _, err := authority.Pool.Exec(ctx, "INSERT INTO "+authority.Schema+".analytics_progress(pool,height,transaction_index,log_index,block_hash,revision,observed_at) VALUES($1,1,0,0,$2,1,$3)", pool, "0x"+strings.Repeat("a", 64), observed); err != nil {
		t.Fatal(err)
	}
	projector, err := analytics.NewProjector(authority, risk.ProjectionModelV1{}, analytics.PostgresRiskInputs{Authority: authority})
	if err != nil {
		t.Fatal(err)
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- projector.RunExpiry(runCtx, time.Second, 1) }()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var count int
		if err := authority.Pool.QueryRow(ctx, "SELECT count(*) FROM "+authority.Schema+".analytics_outbox WHERE pool=$1", pool).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count >= 2 {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("expiry worker stopped: %v", err)
		default:
		}
		time.Sleep(20 * time.Millisecond)
	}
	var count int
	if err := authority.Pool.QueryRow(ctx, "SELECT count(*) FROM "+authority.Schema+".analytics_outbox WHERE pool=$1", pool).Scan(&count); err != nil || count < 2 {
		t.Fatalf("expiry snapshots: %d %v", count, err)
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("worker stop: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not stop")
	}
	if err := projector.RunExpiry(ctx, 0, 1); err == nil {
		t.Fatal("zero interval accepted")
	}
	if err := projector.RunExpiry(ctx, time.Second, 0); err == nil {
		t.Fatal("zero batch accepted")
	}
	if err := (*analytics.Projector)(nil).RunExpiry(ctx, time.Second, 1); !errors.Is(err, analytics.ErrDependencyUnavailable) {
		t.Fatalf("missing projector: %v", err)
	}
	if got, err := authority.ReadSnapshot(ctx, pool, "pressure-v1", time.Now().Unix()); err != nil || got.Snapshot.Watermark.ProjectionRevision != fmt.Sprint(2) {
		t.Fatalf("expiry snapshot: %+v %v", got, err)
	}
}
