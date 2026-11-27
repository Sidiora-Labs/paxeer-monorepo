package l16

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func retryInventoryStreamRequest[T any](ctx context.Context, request func(context.Context) (T, error)) (T, error) {
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	for {
		if err := bounded.Err(); err != nil {
			var zero T
			return zero, err
		}
		value, err := request(bounded)
		if !errors.Is(err, jetstream.ErrStreamNotFound) {
			return value, err
		}
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-bounded.Done():
			timer.Stop()
			return value, errors.Join(err, bounded.Err())
		case <-timer.C:
		}
	}
}

func TestInventoryStreamRequest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	nc, err := nats.Connect(realMarketEnv(t, "PNL_TEST_NATS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("PNL_REQUEST_%d", time.Now().UnixNano())
	subject := "pnl.inventory.request." + name
	cfg := jetstream.ConsumerConfig{Name: "inventory", Durable: "inventory", FilterSubject: subject, AckPolicy: jetstream.AckExplicitPolicy}

	streamCreated := false
	t.Cleanup(func() {
		if !streamCreated {
			return
		}
		cleanup, stop := context.WithTimeout(context.Background(), time.Second)
		defer stop()
		if err := js.DeleteStream(cleanup, name); err != nil {
			t.Errorf("remove stream: %v", err)
		}
	})

	t.Run("stream becomes available", func(t *testing.T) {
		missing := make(chan struct{}, 1)
		created := make(chan error, 1)
		go func() {
			select {
			case <-ctx.Done():
				created <- ctx.Err()
				return
			case <-missing:
			}
			_, err := js.CreateStream(ctx, jetstream.StreamConfig{Name: name, Subjects: []string{subject}, Storage: jetstream.FileStorage})
			created <- err
		}()
		defer func() {
			if err := <-created; err != nil {
				t.Errorf("create stream: %v", err)
				return
			}
			streamCreated = true
		}()
		attempts := 0
		consumer, err := retryInventoryStreamRequest(ctx, func(requestCtx context.Context) (jetstream.Consumer, error) {
			attempts++
			consumer, err := js.CreateOrUpdateConsumer(requestCtx, name, cfg)
			if errors.Is(err, jetstream.ErrStreamNotFound) {
				select {
				case missing <- struct{}{}:
				default:
				}
			}
			return consumer, err
		})
		if err != nil || attempts < 2 {
			t.Fatalf("create consumer: attempts=%d error=%v", attempts, err)
		}
		info, err := consumer.Info(ctx)
		if err != nil || info.Stream != name || info.Name != cfg.Name {
			t.Fatal(info, err)
		}
	})

	t.Run("other broker error returns immediately", func(t *testing.T) {
		attempts := 0
		_, err := retryInventoryStreamRequest(ctx, func(requestCtx context.Context) (jetstream.Consumer, error) {
			attempts++
			return js.Consumer(requestCtx, name, "absent")
		})
		if !errors.Is(err, jetstream.ErrConsumerNotFound) || attempts != 1 {
			t.Fatalf("consumer lookup: attempts=%d error=%v", attempts, err)
		}
	})

	t.Run("missing stream respects deadline", func(t *testing.T) {
		deadline, stop := context.WithTimeout(ctx, 25*time.Millisecond)
		defer stop()
		attempts := 0
		_, err := retryInventoryStreamRequest(deadline, func(requestCtx context.Context) (jetstream.Consumer, error) {
			attempts++
			return js.CreateOrUpdateConsumer(requestCtx, name+"_absent", cfg)
		})
		if !errors.Is(err, context.DeadlineExceeded) || attempts == 0 {
			t.Fatalf("create consumer: attempts=%d error=%v", attempts, err)
		}
	})

	t.Run("canceled context sends no request", func(t *testing.T) {
		canceled, stop := context.WithCancel(ctx)
		stop()
		attempts := 0
		_, err := retryInventoryStreamRequest(canceled, func(requestCtx context.Context) (jetstream.Consumer, error) {
			attempts++
			return js.CreateOrUpdateConsumer(requestCtx, name, cfg)
		})
		if !errors.Is(err, context.Canceled) || attempts != 0 {
			t.Fatalf("create consumer: attempts=%d error=%v", attempts, err)
		}
	})
}
