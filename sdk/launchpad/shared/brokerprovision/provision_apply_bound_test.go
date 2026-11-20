package brokerprovision

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

type boundBroker struct {
	jetstream.JetStream
	holdCreate string
	holdRead   string
	holdUpdate string
	holdAll    bool
	change     string
	creates    int
	reads      int
	updates    int
	exists     int
	requests   []context.Context
}

func (b *boundBroker) CreateStream(ctx context.Context, config jetstream.StreamConfig) (jetstream.Stream, error) {
	if b.holdCreate != kindStream {
		return b.JetStream.CreateStream(ctx, config)
	}
	b.creates++
	b.requests = append(b.requests, ctx)
	if b.creates == 1 {
		if b.change == "equivalent" {
			config.Subjects = append([]string(nil), config.Subjects...)
			for i, j := 0, len(config.Subjects)-1; i < j; i, j = i+1, j-1 {
				config.Subjects[i], config.Subjects[j] = config.Subjects[j], config.Subjects[i]
			}
		} else if b.change == "different" {
			config.MaxMsgs++
		}
	}
	stream, err := b.JetStream.CreateStream(ctx, config)
	if errors.Is(err, jetstream.ErrStreamNameAlreadyInUse) {
		b.exists++
	}
	if b.holdAll || (err == nil && b.creates == 1) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return stream, err
}

func (b *boundBroker) CreateConsumer(ctx context.Context, stream string, config jetstream.ConsumerConfig) (jetstream.Consumer, error) {
	if b.holdCreate != kindDurable {
		return b.JetStream.CreateConsumer(ctx, stream, config)
	}
	b.creates++
	b.requests = append(b.requests, ctx)
	if b.creates == 1 {
		if b.change == "equivalent" {
			info, err := b.JetStream.Stream(ctx, stream)
			if err != nil {
				return nil, err
			}
			config.Replicas = info.CachedInfo().Config.Replicas
		} else if b.change == "different" {
			config.AckWait += time.Second
		}
	}
	consumer, err := b.JetStream.CreateConsumer(ctx, stream, config)
	if errors.Is(err, jetstream.ErrConsumerExists) {
		b.exists++
	}
	if b.holdAll || (err == nil && b.creates == 1) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return consumer, err
}

func (b *boundBroker) UpdateStream(ctx context.Context, config jetstream.StreamConfig) (jetstream.Stream, error) {
	stream, err := b.JetStream.UpdateStream(ctx, config)
	if b.holdUpdate == kindStream {
		b.updates++
		b.requests = append(b.requests, ctx)
		if err == nil && b.updates == 1 {
			<-ctx.Done()
			return nil, ctx.Err()
		}
	}
	return stream, err
}

func (b *boundBroker) UpdateConsumer(ctx context.Context, stream string, config jetstream.ConsumerConfig) (jetstream.Consumer, error) {
	consumer, err := b.JetStream.UpdateConsumer(ctx, stream, config)
	if b.holdUpdate == kindDurable {
		b.updates++
		b.requests = append(b.requests, ctx)
		if err == nil && b.updates == 1 {
			<-ctx.Done()
			return nil, ctx.Err()
		}
	}
	return consumer, err
}

func (b *boundBroker) Stream(ctx context.Context, name string) (jetstream.Stream, error) {
	stream, err := b.JetStream.Stream(ctx, name)
	if b.holdRead == "stream" {
		b.reads++
		b.requests = append(b.requests, ctx)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, err
	}
	return &boundStream{Stream: stream, broker: b}, nil
}

func (b *boundBroker) Consumer(ctx context.Context, stream, durable string) (jetstream.Consumer, error) {
	consumer, err := b.JetStream.Consumer(ctx, stream, durable)
	if b.holdRead == "consumer" {
		b.reads++
		b.requests = append(b.requests, ctx)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, err
	}
	return &boundConsumer{Consumer: consumer, broker: b}, nil
}

type boundStream struct {
	jetstream.Stream
	broker *boundBroker
}

func (s *boundStream) Info(ctx context.Context, opts ...jetstream.StreamInfoOpt) (*jetstream.StreamInfo, error) {
	info, err := s.Stream.Info(ctx, opts...)
	if s.broker.holdRead == "stream info" {
		s.broker.reads++
		s.broker.requests = append(s.broker.requests, ctx)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return info, err
}

type boundConsumer struct {
	jetstream.Consumer
	broker *boundBroker
}

func (c *boundConsumer) Info(ctx context.Context) (*jetstream.ConsumerInfo, error) {
	info, err := c.Consumer.Info(ctx)
	if c.broker.holdRead == "consumer info" {
		c.broker.reads++
		c.broker.requests = append(c.broker.requests, ctx)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return info, err
}

func shortenRequestWait(t *testing.T) {
	t.Helper()
	original := requestWait
	requestWait = time.Second
	t.Cleanup(func() { requestWait = original })
}

func assertBoundRequests(t *testing.T, requests []context.Context, caller context.Context) {
	t.Helper()
	if len(requests) < 2 {
		t.Fatalf("request count: got %d want at least two", len(requests))
	}
	for i, request := range requests {
		deadline, ok := request.Deadline()
		callerDeadline, _ := caller.Deadline()
		if request == caller || !ok || deadline.After(callerDeadline) {
			t.Fatal("request must have a deadline within the caller deadline")
		}
		if request.Err() == nil {
			t.Fatal("request context remains active")
		}
		if i > 0 && request == requests[i-1] {
			t.Fatal("retry must have its own context")
		}
	}
}

func TestProvisionApplyBoundStreamCreation(t *testing.T) {
	testBoundCreation(t, kindStream)
}

func TestProvisionApplyBoundDurableCreation(t *testing.T) {
	testBoundCreation(t, kindDurable)
}

func testBoundCreation(t *testing.T, kind string) {
	t.Helper()
	_, js, _ := realBroker(t)
	shortenRequestWait(t)
	for _, change := range []string{"same", "equivalent", "different"} {
		t.Run(change, func(t *testing.T) {
			d := smallDocument(t)
			d.Streams[0].Name += "_" + strings.ToUpper(change)
			d.Streams[0].Subjects = []string{"bound." + change + ".one", "bound." + change + ".two"}
			d.Durables[0].Stream = d.Streams[0].Name
			d.Durables[0].Subject = d.Streams[0].Subjects[0]
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			want := []Difference{{Kind: kindStream, Stream: d.Streams[0].Name, State: stateAbsent}}
			if kind == kindDurable {
				if _, err := js.CreateStream(ctx, d.Streams[0].BrokerConfig()); err != nil {
					t.Fatal(err)
				}
				want = []Difference{{Kind: kindDurable, Stream: d.Streams[0].Name, Durable: d.Durables[0].Durable, State: stateAbsent}}
			} else {
				want = append(want, Difference{Kind: kindDurable, Stream: d.Streams[0].Name, Durable: d.Durables[0].Durable, State: stateAbsent})
			}
			broker := &boundBroker{JetStream: js, holdCreate: kind, change: change}
			start := time.Now()
			applied, err := Provision(ctx, broker, d, Updates{})
			if time.Since(start) >= 10*time.Second || ctx.Err() != nil {
				t.Fatal("provision must finish well before the caller deadline")
			}
			assertBoundRequests(t, broker.requests, ctx)
			if broker.creates != 2 {
				t.Fatalf("creation requests: got %d want two", broker.creates)
			}
			if change != "same" && broker.exists != 1 {
				t.Fatalf("existing resource replies: got %d want one", broker.exists)
			}
			if change == "different" {
				cause := jetstream.ErrStreamNameAlreadyInUse
				if kind == kindDurable {
					cause = jetstream.ErrConsumerExists
				}
				if !errors.Is(err, cause) || len(applied) != 0 {
					t.Fatalf("unequal resource: applied=%v err=%v", applied, err)
				}
				wantError := "brokerprovision: cannot apply " + want[0].String() + ": " + cause.Error()
				if err.Error() != wantError {
					t.Fatalf("apply error: got %q want %q", err, wantError)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(applied, want) {
				t.Fatalf("creation result: applied=%v want=%v err=%v", applied, want, err)
			}
			remaining, err := Check(ctx, js, d)
			if err != nil || len(remaining) != 0 {
				t.Fatalf("broker configuration: differences=%v err=%v", remaining, err)
			}
		})
	}
}

func TestProvisionApplyBoundCheckDeadline(t *testing.T) {
	_, js, _ := realBroker(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	d := smallDocument(t)
	if _, err := Provision(ctx, js, d, Updates{}); err != nil {
		t.Fatal(err)
	}
	shortenRequestWait(t)
	for _, read := range []string{"stream", "stream info", "consumer", "consumer info"} {
		t.Run(read, func(t *testing.T) {
			caller, stop := context.WithTimeout(ctx, 2500*time.Millisecond)
			defer stop()
			broker := &boundBroker{JetStream: js, holdRead: read}
			differences, err := Check(caller, broker, d)
			if !errors.Is(err, context.DeadlineExceeded) || differences != nil {
				t.Fatalf("check deadline: differences=%v err=%v", differences, err)
			}
			name := "stream " + d.Streams[0].Name
			if strings.HasPrefix(read, "consumer") {
				name = "durable " + d.Durables[0].Stream + "/" + d.Durables[0].Durable
			}
			want := "brokerprovision: cannot inspect " + name + ": " + context.DeadlineExceeded.Error()
			if err.Error() != want || caller.Err() != context.DeadlineExceeded || broker.reads < 2 {
				t.Fatalf("check result: reads=%d err=%v caller=%v", broker.reads, err, caller.Err())
			}
			assertBoundRequests(t, broker.requests, caller)
		})
	}
}

func TestProvisionApplyBoundUpdates(t *testing.T) {
	_, js, _ := realBroker(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	d := smallDocument(t)
	if _, err := Provision(ctx, js, d, Updates{}); err != nil {
		t.Fatal(err)
	}
	shortenRequestWait(t)
	for _, kind := range []string{kindStream, kindDurable} {
		t.Run(kind, func(t *testing.T) {
			updates := Updates{}
			want := Difference{Kind: kind, Stream: d.Streams[0].Name, State: stateDiffers}
			if kind == kindStream {
				d.Streams[0].Limits.MaxMessages++
				updates.Streams = []string{d.Streams[0].Name}
			} else {
				d.Durables[0].AckWait += time.Second
				updates.Durables = []string{d.Durables[0].Stream + "/" + d.Durables[0].Durable}
				want.Durable = d.Durables[0].Durable
			}
			broker := &boundBroker{JetStream: js, holdUpdate: kind}
			start := time.Now()
			applied, err := Provision(ctx, broker, d, updates)
			if err != nil || !reflect.DeepEqual(applied, []Difference{want}) {
				t.Fatalf("update result: applied=%v err=%v", applied, err)
			}
			if broker.updates != 2 || time.Since(start) >= 10*time.Second || ctx.Err() != nil {
				t.Fatalf("update retry: requests=%d caller=%v", broker.updates, ctx.Err())
			}
			assertBoundRequests(t, broker.requests, ctx)
		})
	}
}

func TestProvisionApplyBoundApplyDeadline(t *testing.T) {
	_, js, _ := realBroker(t)
	shortenRequestWait(t)
	for _, kind := range []string{kindStream, kindDurable} {
		t.Run(kind, func(t *testing.T) {
			d := smallDocument(t)
			d.Streams[0].Name += "_" + strings.ToUpper(kind)
			d.Streams[0].Subjects = []string{"bound.deadline." + kind}
			d.Durables[0].Stream = d.Streams[0].Name
			d.Durables[0].Subject = d.Streams[0].Subjects[0]
			setup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if kind == kindDurable {
				if _, err := js.CreateStream(setup, d.Streams[0].BrokerConfig()); err != nil {
					t.Fatal(err)
				}
			}
			caller, stop := context.WithTimeout(setup, 2500*time.Millisecond)
			defer stop()
			broker := &boundBroker{JetStream: js, holdCreate: kind, holdAll: true}
			applied, err := Provision(caller, broker, d, Updates{})
			if !errors.Is(err, context.DeadlineExceeded) || len(applied) != 0 {
				t.Fatalf("apply deadline: applied=%v err=%v", applied, err)
			}
			failed := Difference{Kind: kind, Stream: d.Streams[0].Name, State: stateAbsent}
			if kind == kindDurable {
				failed.Durable = d.Durables[0].Durable
			}
			want := "brokerprovision: cannot apply " + failed.String() + ": " + context.DeadlineExceeded.Error()
			if err.Error() != want || caller.Err() != context.DeadlineExceeded || broker.creates < 2 {
				t.Fatalf("apply deadline result: requests=%d err=%v caller=%v", broker.creates, err, caller.Err())
			}
			assertBoundRequests(t, broker.requests, caller)
		})
	}
}
