package consumer

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/projectionstats"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
)

func accountingBatchBroker(t *testing.T) (*natsdriver.Consumer, *natsdriver.Publisher) {
	t.Helper()
	ctx := context.Background()
	nc := internaltest.ConnectNATS(t, os.Getenv("L20_NATS_URL"))
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	stream := "STATS_BATCH_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := js.CreateStream(ctx, jetstream.StreamConfig{Name: stream, Subjects: []string{accounting.MarketSubject}, Storage: jetstream.FileStorage, MaxBytes: 1048576}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := js.DeleteStream(context.Background(), stream); err != nil {
			t.Error(err)
		}
	})
	cfg := natsdriver.ConsumerConfig{Stream: stream, Durable: "stats", Subject: accounting.MarketSubject, Batch: 64, MaxAckPending: 64, MaxWaiting: 1, MaxDeliver: 2, FetchWait: time.Second, AckWait: time.Second}
	native, err := cfg.BrokerConsumerConfig()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := js.CreateConsumer(ctx, stream, native); err != nil {
		t.Fatal(err)
	}
	consumer, err := natsdriver.NewConsumer(ctx, nc, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(consumer.Close)
	publisher, err := natsdriver.NewPublisher(nc, stream)
	if err != nil {
		t.Fatal(err)
	}
	return consumer, publisher
}

func TestAccountingBatchRejectsChangedEnvelopeAndRollsBackSQL(t *testing.T) {
	ctx := context.Background()
	chain, log, timestamp := localAccountingChain(t)
	for _, tc := range []struct {
		name, sql string
		edit      func(*delivery.Envelope)
		class     delivery.FailureClass
	}{
		{name: "inner payload", edit: func(e *delivery.Envelope) { e.Payload = []byte(`{}`); e.PayloadHash = delivery.Hash(e.Payload) }, class: delivery.InnerSchema},
		{name: "publication identity", edit: func(e *delivery.Envelope) { e.EventID = uuid.NewString() }, class: delivery.PartitionBinding},
		{name: "projection constraint", sql: `ALTER TABLE stats_projection.trade_facts ADD CONSTRAINT positive_document CHECK (false)`},
		{name: "publication constraint", sql: `ALTER TABLE stats_projection.accounting_market_commits ADD CONSTRAINT committed_document CHECK (false)`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool, store, partitions, repo, authority := localAccountingDatabase(t, log)
			publication := localAccountingPublication(t, pool, log, timestamp)
			consumer, err := NewSourceVerifiedAccountingConsumer(SourceVerifiedAccountingConfig{Pool: pool, Deliveries: store, Partitions: partitions, Repository: repo, Authority: authority, ConsumerID: "stats", Generation: "g1", ProjectionVersion: "projection-v1", Owner: "stats-worker", SourceStream: "canonical", SourcePartitionSchema: "indexer", RPC: chain, MaxReplayBlocks: 16, MaxReplayFills: 16})
			if err != nil {
				t.Fatal(err)
			}
			transport, publisher := accountingBatchBroker(t)
			payload, err := accounting.CanonicalPublicationV1(publication)
			if err != nil {
				t.Fatal(err)
			}
			envelope := delivery.Envelope{EventID: publication.PublicationID, EventType: "accounting.market.v1", SchemaVersion: 1, ChainID: 125, SourceOrder: delivery.SourceOrder{Height: publication.Fill.Position.Height, TransactionIndex: publication.Fill.Position.TransactionIndex, LogIndex: publication.Fill.Position.LogIndex}, EmittedAt: time.Now().UTC(), TraceID: "stats_batch", PayloadHash: delivery.Hash(payload), Payload: payload}
			if tc.edit != nil {
				tc.edit(&envelope)
			}
			wire, err := envelope.Encode(delivery.Policy{Subjects: map[string]bool{accounting.MarketSubject: true}, EventTypes: map[string]bool{"accounting.market.v1": true}}, accounting.MarketSubject)
			if err != nil {
				t.Fatal(err)
			}
			if tc.sql != "" {
				if _, err := pool.Exec(ctx, tc.sql); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := publisher.Publish(ctx, accounting.MarketSubject, envelope.EventID, wire); err != nil {
				t.Fatal(err)
			}
			messages, err := transport.Fetch(ctx)
			if err != nil || len(messages) != 1 {
				t.Fatalf("broker delivery: %d %v", len(messages), err)
			}
			err = consumer.ConsumeBatch(ctx, messages)
			if err == nil {
				t.Fatal("invalid batch committed")
			}
			if tc.class != "" {
				var failure *delivery.IntegrityFailure
				if !errors.As(err, &failure) || failure.Class != tc.class {
					t.Fatalf("integrity classification: %v", err)
				}
			} else {
				var failure *delivery.OrderedBatchFailure
				if !errors.As(err, &failure) {
					t.Fatalf("transaction classification: %v", err)
				}
			}
			var trades, commits, offsets int
			if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM stats_projection.trade_facts),(SELECT count(*) FROM stats_projection.accounting_market_commits),(SELECT count(*) FROM stats_delivery.delivery_partition_commits)`).Scan(&trades, &commits, &offsets); err != nil || trades != 0 || commits != 0 || offsets != 0 {
				t.Fatalf("partial batch: %d %d %d %v", trades, commits, offsets, err)
			}
			if _, err := consumer.lease(ctx, *publication.Fill); tc.class == "" && err != nil {
				t.Fatalf("recoverable lease: %v", err)
			}
		})
	}
	var absent *SourceVerifiedAccountingConsumer
	if err := absent.Run(ctx, nil); !errors.Is(err, projectionstats.ErrInvalid) {
		t.Fatal(err)
	}
}
