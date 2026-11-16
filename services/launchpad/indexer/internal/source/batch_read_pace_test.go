package source

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestBatchReadPaceRunsReadsWithinConfiguredBound(t *testing.T) {
	var active, peak atomic.Int32
	reads, err := ParallelBatchRead(context.Background(), 64, 8, func(ctx context.Context, index int) (int, error) {
		current := active.Add(1)
		for {
			old := peak.Load()
			if current <= old || peak.CompareAndSwap(old, current) {
				break
			}
		}
		defer active.Add(-1)
		select {
		case <-time.After(2 * time.Millisecond):
			return index, nil
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(reads) != 64 || peak.Load() <= 1 || peak.Load() > 8 {
		t.Fatalf("reads=%d peak=%d", len(reads), peak.Load())
	}
	for i, got := range reads {
		if got != i {
			t.Fatalf("read %d returned %d", i, got)
		}
	}
}

func TestBatchReadPaceBoundOneRunsReadsSequentially(t *testing.T) {
	var active, peak atomic.Int32
	reads, err := ParallelBatchRead(context.Background(), 64, 1, func(ctx context.Context, index int) (int, error) {
		current := active.Add(1)
		if current > peak.Load() {
			peak.Store(current)
		}
		defer active.Add(-1)
		return index, nil
	})
	if err != nil || len(reads) != 64 || peak.Load() != 1 {
		t.Fatalf("reads=%d peak=%d err=%v", len(reads), peak.Load(), err)
	}
}

func TestBatchReadPaceFailureCancelsOutstandingReads(t *testing.T) {
	failed := errors.New("read failed")
	started := make(chan struct{}, 8)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := ParallelBatchRead(ctx, 64, 8, func(ctx context.Context, index int) (int, error) {
		if index == 0 {
			time.Sleep(10 * time.Millisecond)
			return 0, failed
		}
		started <- struct{}{}
		<-ctx.Done()
		return 0, ctx.Err()
	})
	if !errors.Is(err, failed) {
		t.Fatalf("error=%v", err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("no outstanding read observed")
	}
}

func TestBatchReadPaceCancellationStopsReads(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ParallelBatchRead(ctx, 64, 8, func(ctx context.Context, index int) (int, error) {
		return index, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
}
