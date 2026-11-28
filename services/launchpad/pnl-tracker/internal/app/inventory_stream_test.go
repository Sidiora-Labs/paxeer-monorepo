package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

func retryInventoryStreamRequest[T any](ctx context.Context, request func(context.Context) (T, error)) (T, error) {
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	attempts := 0
	for {
		if err := bounded.Err(); err != nil {
			var zero T
			return zero, fmt.Errorf("inventory stream request after %d tries: %w", attempts, err)
		}
		attempts++
		value, err := request(bounded)
		if !errors.Is(err, jetstream.ErrStreamNotFound) {
			return value, err
		}
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-bounded.Done():
			timer.Stop()
			return value, fmt.Errorf("inventory stream request after %d tries: %w", attempts, errors.Join(err, bounded.Err()))
		case <-timer.C:
		}
	}
}

func TestInventoryStreamRequest(t *testing.T) {
	t.Run("stream becomes available", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		started := time.Now()
		attempts := 0
		value, err := retryInventoryStreamRequest(ctx, func(requestCtx context.Context) (string, error) {
			attempts++
			deadline, ok := requestCtx.Deadline()
			if !ok || time.Until(deadline) > 5*time.Second {
				t.Fatal("inventory stream request must have a five-second limit")
			}
			if attempts <= 2 {
				return "", fmt.Errorf("create consumer: %w", jetstream.ErrStreamNotFound)
			}
			return "inventory", nil
		})
		if err != nil || attempts != 3 || value != "inventory" {
			t.Fatalf("create consumer: value=%q attempts=%d error=%v", value, attempts, err)
		}
		if time.Since(started) < 20*time.Millisecond {
			t.Fatal("inventory stream requests must pause between tries")
		}
	})

	t.Run("other error returns immediately", func(t *testing.T) {
		attempts := 0
		want := errors.New("consumer unavailable")
		value, err := retryInventoryStreamRequest(context.Background(), func(context.Context) (string, error) {
			attempts++
			return "consumer", want
		})
		if err != want || attempts != 1 || value != "consumer" {
			t.Fatalf("create consumer: value=%q attempts=%d error=%v", value, attempts, err)
		}
	})

	t.Run("missing stream respects deadline", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
		defer cancel()
		attempts := 0
		_, err := retryInventoryStreamRequest(ctx, func(requestCtx context.Context) (string, error) {
			attempts++
			deadline, ok := requestCtx.Deadline()
			parentDeadline, _ := ctx.Deadline()
			if !ok || !deadline.Equal(parentDeadline) {
				t.Fatal("inventory stream request must keep the parent deadline")
			}
			return "", jetstream.ErrStreamNotFound
		})
		if !errors.Is(err, context.DeadlineExceeded) || attempts == 0 || !strings.Contains(err.Error(), fmt.Sprintf("after %d tries:", attempts)) {
			t.Fatalf("create consumer: attempts=%d error=%v", attempts, err)
		}
		canceled, stop := context.WithCancel(context.Background())
		stop()
		attempts = 0
		_, err = retryInventoryStreamRequest(canceled, func(context.Context) (string, error) {
			attempts++
			return "", jetstream.ErrStreamNotFound
		})
		if !errors.Is(err, context.Canceled) || attempts != 0 || !strings.Contains(err.Error(), "after 0 tries:") {
			t.Fatalf("create consumer: attempts=%d error=%v", attempts, err)
		}
	})
}
