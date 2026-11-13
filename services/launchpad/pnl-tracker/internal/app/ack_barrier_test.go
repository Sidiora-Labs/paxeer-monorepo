package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func TestCommitBarrierAcknowledgesOnlyAfterRelease(t *testing.T) {
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
	name := "PNL_BARRIER_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	subject := "pnl.barrier." + name
	if _, err = js.CreateStream(ctx, jetstream.StreamConfig{Name: name, Subjects: []string{subject}, Storage: jetstream.FileStorage}); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := js.DeleteStream(context.Background(), name); err != nil {
			t.Error(err)
		}
	}()
	cfg := natsdriver.ConsumerConfig{Stream: name, Durable: "barrier", Subject: subject, Batch: 1, MaxAckPending: 8, MaxWaiting: 1, MaxDeliver: 10, FetchWait: 100 * time.Millisecond, AckWait: time.Second}
	broker, err := cfg.BrokerConsumerConfig()
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := js.CreateOrUpdateConsumer(ctx, name, broker)
	if err != nil {
		t.Fatal(err)
	}
	transport, err := natsdriver.NewConsumer(ctx, nc, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()
	publisher, err := natsdriver.NewPublisher(nc, name)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(inventoryRuntimeDocument{Schema: "pnl", ConsumerID: "pnl"})
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"release", "cancel", "directory", "existing marker", "release lookup"} {
		t.Run(mode, func(t *testing.T) {
			id := uuid.NewString()
			if _, err := publisher.Publish(ctx, subject, id, raw); err != nil {
				t.Fatal(err)
			}
			messages, err := transport.Fetch(ctx)
			if err != nil || len(messages) != 1 || messages[0].EventID != id {
				t.Fatal(messages, err)
			}
			path := filepath.Join(t.TempDir(), "commit")
			switch mode {
			case "release":
				if err = os.WriteFile(path+".release", []byte("released"), 0600); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err = os.Mkdir(path+".release", 0700); err != nil {
					t.Fatal(err)
				}
			case "existing marker":
				if err = os.WriteFile(path+".reached", []byte("reached"), 0600); err != nil {
					t.Fatal(err)
				}
			case "release lookup":
				if err = os.Symlink("commit.release", path+".release"); err != nil {
					t.Fatal(err)
				}
			}
			operation, cancel := context.WithTimeout(ctx, 25*time.Millisecond)
			defer cancel()
			err = (fixtureCommitAck{Message: messages[0], Path: path}).Ack(operation)
			if mode == "release" {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				if err == nil {
					t.Fatal("unreleased delivery acknowledged")
				}
				if mode == "cancel" && !errors.Is(err, context.DeadlineExceeded) {
					t.Fatal(err)
				}
				if err = messages[0].Ack(ctx); err != nil {
					t.Fatal(err)
				}
			}
			marker, err := os.ReadFile(path + ".reached")
			if err != nil || !strings.Contains(string(marker), func() string {
				if mode == "existing marker" {
					return "reached"
				}
				return id
			}()) {
				t.Fatal(string(marker), err)
			}
			info, err := consumer.Info(ctx)
			if err != nil || info.NumAckPending != 0 {
				t.Fatal(info, err)
			}
		})
	}
}
