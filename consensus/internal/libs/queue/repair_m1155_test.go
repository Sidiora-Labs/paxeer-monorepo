package queue

import (
	"context"
	"errors"
	"sync"
	"testing"
	"testing/synctest"
)

// queue-multiple-waiters-not-woken is covered by
// TestRepairM_1155_MultipleWaitersNotWoken; queue-cancellation-lost-wakeup is
// covered by TestRepairM_1155_CancellationLostWakeup. No external wiring is needed.

func TestRepairM_1155_MultipleWaitersNotWoken(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		q := mustQueue(t, Options{SoftQuota: 2, HardLimit: 2})
		defer q.Close()

		type result struct {
			item interface{}
			err  error
		}
		results := make(chan result, 2)
		for range 2 {
			go func() {
				item, err := q.Wait(t.Context())
				results <- result{item, err}
			}()
		}
		// Both consumers are parked in the real queue before the producer runs.
		synctest.Wait()
		q.mustAdd("first")
		q.mustAdd("second")
		synctest.Wait()

		seen := make(map[interface{}]int)
		for range 2 {
			select {
			case got := <-results:
				if got.err != nil {
					t.Fatalf("Wait returned error: %v", got.err)
				}
				seen[got.item]++
			default:
				t.Fatal("a consumer is still blocked after two successful enqueues")
			}
		}
		if len(seen) != 2 || seen["first"] != 1 || seen["second"] != 1 {
			t.Fatalf("consumed items = %v, want each enqueued item exactly once", seen)
		}
		if item, ok := q.Remove(); ok || item != nil {
			t.Fatalf("Remove after consumers finished = (%v, %v), want (nil, false)", item, ok)
		}
	})
}

// This wraps a real cancelable context to cancel precisely after observing a
// live context at the queue's predicate check, before it enters Cond.Wait.
// Cancellation state and notifications still come from context.WithCancel.
type repairM1155CancelAtCheck struct {
	context.Context
	cancel context.CancelFunc
	once   sync.Once
}

func (ctx *repairM1155CancelAtCheck) Err() error {
	err := ctx.Context.Err()
	ctx.once.Do(ctx.cancel)
	return err
}

func TestRepairM_1155_CancellationLostWakeup(t *testing.T) {
	t.Run("AtWaitBoundary", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			q := mustQueue(t, Options{HardLimit: 1})
			defer q.Close()
			parent, cancel := context.WithCancel(t.Context())
			defer cancel()
			ctx := &repairM1155CancelAtCheck{Context: parent, cancel: cancel}
			finished := false
			go func() {
				item, err := q.Wait(ctx)
				if item != nil || !errors.Is(err, context.Canceled) {
					t.Errorf("Wait = (%v, %v), want (nil, context.Canceled)", item, err)
				}
				finished = true
			}()
			synctest.Wait()
			if !finished {
				t.Fatal("cancellation between the predicate check and Cond.Wait stranded the consumer")
			}
			if !errors.Is(parent.Err(), context.Canceled) {
				t.Fatal("the wait boundary did not trigger real context cancellation")
			}
			q.mustAdd("after cancellation")
			q.mustRemove("after cancellation")
		})
	})

	t.Run("AlreadyCanceled", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			q := mustQueue(t, Options{HardLimit: 1})
			defer q.Close()
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			item, err := q.Wait(ctx)
			if item != nil || !errors.Is(err, context.Canceled) {
				t.Fatalf("Wait = (%v, %v), want (nil, context.Canceled)", item, err)
			}
		})
	})

	t.Run("IndependentWaiters", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			q := mustQueue(t, Options{HardLimit: 1})
			defer q.Close()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			canceled, consumed := false, false
			go func() {
				item, err := q.Wait(ctx)
				if item != nil || !errors.Is(err, context.Canceled) {
					t.Errorf("canceled Wait = (%v, %v), want (nil, context.Canceled)", item, err)
				}
				canceled = true
			}()
			go func() {
				item, err := q.Wait(t.Context())
				if item != "survivor" || err != nil {
					t.Errorf("live Wait = (%v, %v), want (survivor, nil)", item, err)
				}
				consumed = true
			}()
			synctest.Wait()
			cancel()
			synctest.Wait()
			if !canceled || consumed {
				t.Fatalf("after cancellation: canceled waiter finished = %v, live waiter finished = %v", canceled, consumed)
			}
			q.mustAdd("survivor")
			synctest.Wait()
			if !consumed {
				t.Fatal("cancellation stranded the remaining live consumer")
			}
		})
	})
}
