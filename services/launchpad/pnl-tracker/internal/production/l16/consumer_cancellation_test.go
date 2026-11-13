package l16

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l17"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func cancellationAccountingConsumer(t *testing.T, ctx context.Context) (*AccountingConsumer, *internaltest.QueryCancellation, delivery.Envelope) {
	t.Helper()
	service, pool, frames := currentRecordedInventoryFrames(t, ctx)
	config := pool.Config()
	trace := new(internaltest.QueryCancellation)
	config.ConnConfig.Tracer = trace
	traced, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(traced.Close)
	a := *service.authority
	a.pool = traced
	consumer, err := NewAccountingConsumerWithRuntime(traced, "pnl", a.consumerID, a.generation, a.owner, a.projectionVersion, &a, a.epochGuard, delivery.StreamWriter{Schema: "api_stream"})
	if err != nil {
		t.Fatal(err)
	}
	consumer, err = consumer.WithFullPortfolio(l17.Repository{Pool: traced}, service.outbox, "portfolio.snapshot.v2")
	if err != nil {
		t.Fatal(err)
	}
	var publication accounting.PublicationV1
	if err = json.Unmarshal(frames[0].Payload, &publication); err != nil {
		t.Fatal(err)
	}
	if err = consumer.PreparePartition(ctx, publication.Positions[0].PartitionKey); err != nil {
		t.Fatal(err)
	}
	if _, err = consumer.partitions.Acquire(ctx, delivery.ConsumerKey{ConsumerID: a.consumerID, ProjectionGeneration: a.generation, PartitionKind: "inventory", PartitionKey: publication.Positions[0].PartitionKey}, a.owner, 30*time.Second); err != nil {
		t.Fatal(err)
	}
	return consumer, trace, frames[0]
}

func TestAccountingDeliveryCancellationPreservesAtomicEffects(t *testing.T) {
	ctx := context.Background()
	probe, trace, frame := cancellationAccountingConsumer(t, ctx)
	candidate, cancellation, candidateFrame := cancellationAccountingConsumer(t, ctx)
	raw, err := json.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}
	candidateRaw, err := json.Marshal(candidateFrame)
	if err != nil || string(raw) != string(candidateRaw) {
		t.Fatal("accounting fixture identities differ", err)
	}
	nc, err := nats.Connect(realMarketEnv(t, "PNL_TEST_NATS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("PNL_CANCELLATION_%d", time.Now().UnixNano())
	if _, err = js.CreateStream(ctx, jetstream.StreamConfig{Name: name, Subjects: []string{accounting.InventorySubject}, Storage: jetstream.FileStorage}); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := js.DeleteStream(context.Background(), name); err != nil {
			t.Error(err)
		}
	}()
	cfg := natsdriver.ConsumerConfig{Stream: name, Durable: "inventory", Subject: accounting.InventorySubject, Batch: 1, MaxAckPending: 8, MaxWaiting: 1, MaxDeliver: 100, FetchWait: time.Second, AckWait: 30 * time.Second}
	broker, err := cfg.BrokerConsumerConfig()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = js.CreateOrUpdateConsumer(ctx, name, broker); err != nil {
		t.Fatal(err)
	}
	publisher, err := natsdriver.NewPublisher(nc, name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = publisher.Publish(ctx, accounting.InventorySubject, frame.EventID, raw); err != nil {
		t.Fatal(err)
	}
	transport, err := natsdriver.NewConsumer(ctx, nc, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()
	messages, err := transport.Fetch(ctx)
	if err != nil || len(messages) != 1 {
		t.Fatal(messages, err)
	}
	trace.Start(0)
	err = probe.Consume(ctx, accounting.InventorySubject, raw, messages[0])
	queries := trace.Stop()
	if err != nil || len(queries) == 0 {
		t.Fatal(queries, err)
	}
	var committed int
	if err = probe.pool.QueryRow(ctx, `SELECT count(*) FROM pnl.portfolio_snapshots`).Scan(&committed); err != nil || committed != 1 {
		t.Fatal(committed, err)
	}
	for i, query := range queries {
		if strings.EqualFold(query, "rollback") {
			continue
		}
		t.Run(fmt.Sprintf("query_%03d", i+1), func(t *testing.T) {
			cancellation.Start(i + 1)
			err := candidate.Consume(ctx, accounting.InventorySubject, candidateRaw, messages[0])
			observed := cancellation.Stop()
			if !errors.Is(err, context.Canceled) || len(observed) != i+1 || observed[i] != query {
				t.Fatal(observed, query, err)
			}
			var snapshots, effects, streams int
			if err = candidate.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM pnl.portfolio_snapshots),(SELECT count(*) FROM pnl.inventory_l16_effects),(SELECT count(*) FROM api_stream.stream_delivery_outbox)`).Scan(&snapshots, &effects, &streams); err != nil || snapshots != 0 || effects != 0 || streams != 0 {
				t.Fatal(snapshots, effects, streams, err)
			}
		})
	}
}
