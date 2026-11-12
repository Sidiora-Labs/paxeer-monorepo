package broker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func committedBroker(t *testing.T) (*nats.Conn, jetstream.JetStream) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("nats-server", "-js", "-p", strconv.Itoa(port), "-sd", t.TempDir())
	command.Stdout, command.Stderr = io.Discard, io.Discard
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	t.Cleanup(func() {
		_ = command.Process.Signal(os.Interrupt)
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = command.Process.Kill()
			<-done
		}
	})
	address := fmt.Sprintf("nats://127.0.0.1:%d", port)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		connection, err := nats.Connect(address, nats.NoReconnect(), nats.Timeout(100*time.Millisecond))
		if err == nil {
			t.Cleanup(connection.Close)
			js, err := jetstream.New(connection)
			if err != nil {
				t.Fatal(err)
			}
			return connection, js
		}
		select {
		case err := <-done:
			t.Fatalf("broker exited before readiness: %v", err)
		default:
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("broker did not become ready")
	return nil, nil
}

func TestCommittedDeliveryNotifiesAndAcknowledgesProjection(t *testing.T) {
	ctx, stream := postgresStreamStorage(t)
	frame := publicPriceFrame(t, "1")
	commitStreamFrame(t, ctx, stream, frame)
	connection, js := committedBroker(t)
	config := natsdriver.ConsumerConfig{Stream: "API_STREAM", Durable: "projection_v1", Subject: frame.Topic, Batch: 1, MaxAckPending: 4, MaxWaiting: 1, MaxDeliver: 3, FetchWait: 100 * time.Millisecond, AckWait: time.Second}
	if _, err := js.CreateStream(ctx, jetstream.StreamConfig{Name: config.Stream, Subjects: []string{frame.Topic}, Storage: jetstream.FileStorage, Replicas: 1}); err != nil {
		t.Fatal(err)
	}
	consumerConfig, err := config.BrokerConsumerConfig()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := js.CreateConsumer(ctx, config.Stream, consumerConfig); err != nil {
		t.Fatal(err)
	}
	consumer, err := natsdriver.NewConsumer(ctx, connection, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(consumer.Close)
	producer, err := natsdriver.NewPublisher(connection, config.Stream)
	if err != nil {
		t.Fatal(err)
	}
	fanout := NewTopicFanout()
	alert, stop, err := fanout.Watch([]string{frame.Topic})
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	wire, err := json.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}
	if ack, err := producer.Publish(ctx, frame.Topic, delivery.Hash([]byte(frame.EventID)), wire); err != nil || ack.Sequence != 1 {
		t.Fatalf("durable publication: %+v, %v", ack, err)
	}
	consumeCtx, cancel := context.WithCancel(ctx)
	completed := make(chan error, 1)
	go func() { completed <- stream.ConsumeCommitted(consumeCtx, consumer, fanout) }()
	select {
	case <-alert:
	case err := <-completed:
		t.Fatalf("consumer stopped before notification: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("committed topic did not notify watcher")
	}
	if fanout.Degraded() {
		t.Fatal("active consumer remained degraded")
	}
	cancel()
	select {
	case err := <-completed:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("consumer cancellation: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("consumer did not stop after cancellation")
	}
}

func TestCommittedDeliveryRejectsInsufficientStreamRetention(t *testing.T) {
	ctx, stream := postgresStreamStorage(t)
	connection, js := committedBroker(t)
	frame := publicPriceFrame(t, "1")
	config := natsdriver.ConsumerConfig{Stream: "API_STREAM", Durable: "projection_v1", Subject: frame.Topic, Batch: 1, MaxAckPending: 4, MaxWaiting: 1, MaxDeliver: 3, FetchWait: 100 * time.Millisecond, AckWait: time.Second}
	if _, err := js.CreateStream(ctx, jetstream.StreamConfig{Name: config.Stream, Subjects: []string{frame.Topic}, Storage: jetstream.FileStorage, Replicas: 1, MaxAge: time.Hour}); err != nil {
		t.Fatal(err)
	}
	fanout := NewTopicFanout()
	if err := stream.RunCommitted(ctx, nil, config, fanout); !errors.Is(err, ErrStreamUnavailable) || !fanout.Degraded() {
		t.Fatalf("missing durable connection: %v degraded=%t", err, fanout.Degraded())
	}
	invalid := config
	invalid.Batch = 0
	if err := stream.RunCommitted(ctx, connection, invalid, fanout); err == nil || !fanout.Degraded() {
		t.Fatalf("invalid durable consumer: %v degraded=%t", err, fanout.Degraded())
	}
	invalid = config
	invalid.Subject = "market.>"
	if err := stream.RunCommitted(ctx, connection, invalid, fanout); err == nil || !fanout.Degraded() {
		t.Fatalf("unbounded durable subject: %v degraded=%t", err, fanout.Degraded())
	}
	if err := stream.RunCommitted(ctx, connection, config, fanout); !errors.Is(err, ErrFrame) || !fanout.Degraded() {
		t.Fatalf("stream retention admission: %v degraded=%t", err, fanout.Degraded())
	}
}
