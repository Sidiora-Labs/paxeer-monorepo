package brokerprovision

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

func brokerFixtureRequest[T any](ctx context.Context, send func(context.Context) (T, error)) (T, error) {
	var zero T
	var lastErr error
	for {
		if err := ctx.Err(); err != nil {
			return zero, errors.Join(err, lastErr)
		}
		request, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		result, err := send(request)
		expired := errors.Is(request.Err(), context.DeadlineExceeded)
		cancel()
		if !expired || !errors.Is(err, context.DeadlineExceeded) {
			return result, err
		}
		lastErr = err
		retry := time.NewTimer(20 * time.Millisecond)
		select {
		case <-ctx.Done():
			retry.Stop()
			return zero, errors.Join(ctx.Err(), lastErr)
		case <-retry.C:
		}
	}
}

func TestBrokerFixtureRequestRetriesUnansweredRequests(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var requests []context.Context
	var starts []time.Time
	start := time.Now()
	result, err := brokerFixtureRequest(ctx, func(request context.Context) (string, error) {
		starts = append(starts, time.Now())
		requests = append(requests, request)
		deadline, ok := request.Deadline()
		if request == ctx || !ok || time.Until(deadline) > 500*time.Millisecond {
			t.Fatal("request must have its own 500 millisecond deadline")
		}
		if len(requests) <= 2 {
			<-request.Done()
			return "", request.Err()
		}
		return "ready", nil
	})
	if err != nil || result != "ready" || len(requests) != 3 {
		t.Fatalf("request result: got %q, %v after %d sends", result, err, len(requests))
	}
	if time.Since(start) >= 2*time.Second || ctx.Err() != nil {
		t.Fatal("request must answer well before the caller deadline")
	}
	for i, request := range requests {
		if request.Err() == nil {
			t.Fatal("request context remains active")
		}
		if i > 0 {
			previousDeadline, _ := requests[i-1].Deadline()
			if request == requests[i-1] || starts[i].Before(previousDeadline.Add(20*time.Millisecond)) {
				t.Fatal("retry must wait 20 milliseconds and use a new request context")
			}
		}
	}
}

func TestBrokerFixtureRequestReturnsErrorReply(t *testing.T) {
	for _, reply := range []error{jetstream.ErrStreamNotFound, jetstream.ErrStreamNameAlreadyInUse, context.DeadlineExceeded} {
		t.Run(reply.Error(), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			sends := 0
			var sent context.Context
			start := time.Now()
			result, err := brokerFixtureRequest(ctx, func(request context.Context) (string, error) {
				sends++
				sent = request
				return "reply", reply
			})
			if err != reply || result != "reply" || sends != 1 {
				t.Fatalf("error reply: got %q, %v after %d sends; want %v after one send", result, err, sends, reply)
			}
			if time.Since(start) >= 500*time.Millisecond || ctx.Err() != nil || sent.Err() != context.Canceled {
				t.Fatal("error reply must return immediately and cancel its request context")
			}
		})
	}
}

func TestBrokerFixtureRequestDeadlineRetainsLastError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var lastErr error
	var requests []context.Context
	start := time.Now()
	result, err := brokerFixtureRequest(ctx, func(request context.Context) (string, error) {
		requests = append(requests, request)
		deadline, ok := request.Deadline()
		callerDeadline, _ := ctx.Deadline()
		if request == ctx || !ok || deadline.After(callerDeadline) || time.Until(deadline) > 500*time.Millisecond {
			t.Fatal("request deadline must be within 500 milliseconds and the caller deadline")
		}
		<-request.Done()
		lastErr = fmt.Errorf("unanswered request %d: %w", len(requests), request.Err())
		return "", lastErr
	})
	if result != "" || !errors.Is(err, context.DeadlineExceeded) || ctx.Err() != context.DeadlineExceeded {
		t.Fatalf("caller deadline: result=%q err=%v caller=%v", result, err, ctx.Err())
	}
	if len(requests) < 2 || lastErr == nil || !errors.Is(err, lastErr) || !strings.Contains(err.Error(), lastErr.Error()) {
		t.Fatalf("last request error: sends=%d err=%v last=%v", len(requests), err, lastErr)
	}
	if time.Since(start) >= 3*time.Second {
		t.Fatal("request must stop at the caller deadline")
	}
	for i, request := range requests {
		if request.Err() == nil || (i > 0 && request == requests[i-1]) {
			t.Fatal("each request must have its own canceled context")
		}
	}
}
