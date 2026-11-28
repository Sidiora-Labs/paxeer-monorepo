package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	inventory "github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l16"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func TestAccountingConsumerTermination(t *testing.T) {
	ctx := context.Background()
	nc, err := nats.Connect(os.Getenv("PNL_TEST_NATS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	name := "PNL_LIFECYCLE_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	subject := "pnl.lifecycle." + name
	if _, err = js.CreateStream(ctx, jetstream.StreamConfig{Name: name, Subjects: []string{subject}, Storage: jetstream.FileStorage}); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := js.DeleteStream(ctx, name); err != nil {
			t.Error(err)
		}
	}()
	for _, barrier := range []bool{false, true} {
		for _, closed := range []bool{false, true} {
			label := "canceled"
			if closed {
				label = "closed"
			}
			if barrier {
				label += "_barrier"
			}
			t.Run(label, func(t *testing.T) {
				cfg := natsdriver.ConsumerConfig{Stream: name, Durable: label, Subject: subject, Batch: 1, MaxAckPending: 8, MaxWaiting: 1, MaxDeliver: 10, FetchWait: 100 * time.Millisecond, AckWait: time.Second}
				broker, err := cfg.BrokerConsumerConfig()
				if err != nil {
					t.Fatal(err)
				}
				if _, err = retryInventoryStreamRequest(ctx, func(requestCtx context.Context) (jetstream.Consumer, error) {
					return js.CreateOrUpdateConsumer(requestCtx, name, broker)
				}); err != nil {
					t.Fatal("create consumer:", err)
				}
				transport, err := natsdriver.NewConsumer(ctx, nc, cfg)
				if err != nil {
					t.Fatal(err)
				}
				defer transport.Close()
				a := &App{accounting: &inventory.AccountingConsumer{}, transport: transport}
				if barrier {
					a.fixtureAckBarrier = filepath.Join(t.TempDir(), "commit")
				}
				operation, cancel := context.WithTimeout(ctx, time.Second)
				defer cancel()
				want := context.Canceled
				if closed {
					transport.Close()
					want = natsdriver.ErrClosed
				} else {
					cancel()
				}
				err = a.runAccounting(operation)
				if !errors.Is(err, want) {
					t.Fatalf("consumer termination = %v, want %v", err, want)
				}
				failure := accountingRuntimeError(operation, err)
				if closed && !errors.Is(failure, want) || !closed && failure != nil {
					t.Fatalf("runtime termination = %v for consumer result %v", failure, err)
				}
				if closed && operation.Err() != nil {
					t.Fatalf("consumer closed with active context: %v", operation.Err())
				}
			})
		}
	}
}

func TestAccountingRuntimeReportsUnexpectedTermination(t *testing.T) {
	ctx := context.Background()
	if err := accountingRuntimeError(ctx, nil); err == nil || !strings.Contains(err.Error(), "inventory consumer stopped unexpectedly") {
		t.Fatalf("successful consumer termination with an active service = %v", err)
	}
	if err := accountingRuntimeError(ctx, context.Canceled); !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "inventory consumer failed") {
		t.Fatalf("consumer cancellation with an active service = %v", err)
	}
	stopped, stop := context.WithCancel(ctx)
	stop()
	if err := accountingRuntimeError(stopped, fmt.Errorf("consumer stopped: %w", context.Canceled)); err != nil {
		t.Fatalf("service shutdown reported as a consumer fault: %v", err)
	}
	if err := accountingRuntimeError(stopped, natsdriver.ErrClosed); !errors.Is(err, natsdriver.ErrClosed) {
		t.Fatalf("shutdown lost the consumer fault: %v", err)
	}
}
