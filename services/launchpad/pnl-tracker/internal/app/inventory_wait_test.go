package app

import (
	"context"
	"testing"
	"time"
)

func TestInventoryRetryWaitExpires(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	waitInventoryRun(ctx, 0)
	if err := ctx.Err(); err != nil {
		t.Fatalf("timer did not release the retry before the context deadline: %v", err)
	}
}

func TestInventoryRetryWaitStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() {
		waitInventoryRun(ctx, time.Hour)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cancelled retry remained waiting for its timer")
	}
}
