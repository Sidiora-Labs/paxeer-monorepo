package source

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestParallelBatchReadCancellationDrainsReaders(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	finished := make(chan struct{})
	result := make(chan error, 1)
	var active atomic.Int32
	go func() {
		reads, err := ParallelBatchRead(ctx, 64, 1, func(ctx context.Context, index int) (int, error) {
			active.Add(1)
			defer active.Add(-1)
			if index == 0 {
				close(started)
			}
			<-ctx.Done()
			return 0, ctx.Err()
		})
		if reads != nil {
			result <- errors.New("canceled batch returned results")
		} else {
			result <- err
		}
		close(finished)
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("batch did not start")
	}
	cancel()
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("canceled batch did not finish")
	}
	if err := <-result; !errors.Is(err, context.Canceled) || active.Load() != 0 {
		t.Fatalf("canceled batch = %v, active = %d", err, active.Load())
	}
}
