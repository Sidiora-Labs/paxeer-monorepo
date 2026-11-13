package natsdriver

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func TestConsumerRequiresBoundedFileBackedDurable(t *testing.T) {
	nc, js := realBroker(t)
	_, consumer, cfg := provision(t, nc, js)
	ctx := context.Background()
	for name, call := range map[string]func() error{
		"publisher connection":   func() error { _, err := NewPublisher(nil, "STREAM"); return err },
		"publisher stream":       func() error { _, err := NewPublisher(nc, "invalid stream"); return err },
		"consumer context":       func() error { _, err := NewConsumer(nil, nc, cfg); return err },
		"consumer configuration": func() error { _, err := NewConsumer(ctx, nc, ConsumerConfig{}); return err },
		"consumer absent stream": func() error {
			candidate := cfg
			candidate.Stream = "ABSENT"
			_, err := NewConsumer(ctx, nc, candidate)
			return err
		},
		"consumer absent durable": func() error {
			candidate := cfg
			candidate.Durable = "absent"
			_, err := NewConsumer(ctx, nc, candidate)
			return err
		},
		"fetch context":    func() error { _, err := consumer.Fetch(nil); return err },
		"fetch consumer":   func() error { _, err := (&Consumer{}).Fetch(ctx); return err },
		"run callback":     func() error { return consumer.Run(ctx, nil) },
		"message position": func() error { _, err := (&Message{}).BrokerPosition(); return err },
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Fatal("unconfigured broker operation admitted")
			}
		})
	}
	candidate := cfg
	candidate.Stream = "MEMORY"
	if _, err := js.CreateStream(ctx, jetstream.StreamConfig{Name: candidate.Stream, Subjects: []string{"memory.data"}, Storage: jetstream.MemoryStorage}); err != nil {
		t.Fatal(err)
	}
	if _, err := NewConsumer(ctx, nc, candidate); err == nil {
		t.Fatal("memory stream admitted")
	}
	for _, edit := range []func(*ConsumerConfig){func(c *ConsumerConfig) { c.Batch = 0 }, func(c *ConsumerConfig) { c.MaxWaiting = 33 }, func(c *ConsumerConfig) { c.MaxDeliver = 101 }, func(c *ConsumerConfig) { c.FetchWait = time.Nanosecond }, func(c *ConsumerConfig) { c.AckWait = 2 * time.Hour }, func(c *ConsumerConfig) { c.Subject = "invalid.*" }} {
		candidate := cfg
		edit(&candidate)
		if _, err := candidate.BrokerConsumerConfig(); err == nil {
			t.Fatal("unbounded consumer configuration admitted")
		}
	}
}

func TestConsumerRunCommitsOnlyValidBrokerMessages(t *testing.T) {
	for _, mode := range []string{"accepted", "effect rejection", "missing identity", "empty body", "oversize body"} {
		t.Run(mode, func(t *testing.T) {
			nc, js := realBroker(t)
			_, consumer, cfg := provision(t, nc, js)
			message := nats.NewMsg(cfg.Subject)
			message.Data = []byte("durable bytes")
			message.Header.Set("Nats-Msg-Id", "event_valid")
			switch mode {
			case "missing identity":
				message.Header.Del("Nats-Msg-Id")
			case "empty body":
				message.Data = nil
			case "oversize body":
				message.Data = []byte(strings.Repeat("x", delivery.MaxEnvelopeBytes+1))
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if _, err := js.PublishMsg(ctx, message); err != nil {
				t.Fatal(err)
			}
			if mode == "missing identity" || mode == "empty body" || mode == "oversize body" {
				fetched, err := consumer.Fetch(ctx)
				if err == nil || len(fetched) != 0 {
					t.Fatalf("invalid broker message delivered: %d %v", len(fetched), err)
				}
				return
			}
			effectErr := errors.New("effect rejected")
			var consumed int
			err := consumer.Run(ctx, func(ctx context.Context, msg *Message) error {
				consumed++
				position, err := msg.BrokerPosition()
				if err != nil || position.Stream != cfg.Stream || position.EventID != "event_valid" || position.WireSHA256 != delivery.Hash(message.Data) {
					t.Fatalf("broker position: %+v %v", position, err)
				}
				if mode == "effect rejection" {
					return effectErr
				}
				if err := msg.Ack(ctx); err != nil {
					t.Fatal(err)
				}
				cancel()
				return nil
			})
			if consumed != 1 {
				t.Fatalf("consumer invocation count: %d", consumed)
			}
			if mode == "effect rejection" {
				if !errors.Is(err, effectErr) {
					t.Fatal(err)
				}
			} else if !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
		})
	}
}
