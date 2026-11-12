package projectionstats

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCanonicalWindowExpiryRefreshesPoolAndPlatform(t *testing.T) {
	ctx, pool, service, address := localReadFixture(t)
	scheduler := ExpiryScheduler{Service: service, Interval: time.Second, Windows: []int64{60}, Clock: func() time.Time { return time.Unix(200, 0) }}
	if err := scheduler.Refresh(ctx, 200); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM stats_local.phase2_windows WHERE pool=$1 OR pool=''`, address).Scan(&count); err != nil || count != 2 {
		t.Fatalf("window snapshots: %d %v", count, err)
	}
	window, err := service.LastWindow(ctx, address, 60)
	if err != nil || window.WindowStart != 140 || window.WindowEnd != 200 {
		t.Fatalf("pool window: %+v %v", window, err)
	}
	runCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if err := scheduler.Run(runCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("scheduler stop: %v", err)
	}
	if err := (ExpiryScheduler{}).Refresh(ctx, 1); !errors.Is(err, ErrInvalid) {
		t.Fatalf("missing service: %v", err)
	}
	if err := (ExpiryScheduler{Service: service, Windows: []int64{0}}).Refresh(ctx, 1); !errors.Is(err, ErrInvalid) {
		t.Fatalf("zero window: %v", err)
	}
	if err := (ExpiryScheduler{Service: service, Interval: time.Millisecond, Windows: []int64{60}}).Run(ctx); !errors.Is(err, ErrInvalid) {
		t.Fatalf("short interval: %v", err)
	}
}
