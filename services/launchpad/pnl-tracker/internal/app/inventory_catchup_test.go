package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	inventory "github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l16"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	sharedprocess "github.com/Sidiora-Technologies/KindleLaunch/shared/process"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go/jetstream"
)

type inventoryCatchUpQueries struct{ passes atomic.Int64 }

func (q *inventoryCatchUpQueries) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.HasPrefix(data.SQL, "SELECT p.payload FROM indexer.indexer_accounting_publications_v1 p") {
		q.passes.Add(1)
	}
	return ctx
}

func (*inventoryCatchUpQueries) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func TestInventoryCatchUpPassRunsBesideTheConsumer(t *testing.T) {
	fixture := newJourneyCompositionFixture(t)
	ctx, cancel := context.WithTimeout(fixture.ctx, 10*time.Second)
	defer cancel()
	observer := new(inventoryCatchUpQueries)
	poolConfig := fixture.pool.Config()
	poolConfig.ConnConfig.Tracer = observer
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	consumer, err := inventory.NewAccountingConsumer(pool, "pnl", "pnl", "g1", "pnl-1", "inventory-v1", fixture.baseline)
	if err != nil {
		t.Fatal(err)
	}
	js, err := jetstream.New(fixture.nc)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("PNL_CATCHUP_JOBS_%d", time.Now().UnixNano())
	if _, err = js.CreateStream(ctx, jetstream.StreamConfig{Name: name, Subjects: []string{accounting.InventorySubject}, Storage: jetstream.FileStorage}); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := js.DeleteStream(context.Background(), name); err != nil {
			t.Error(err)
		}
	}()
	config := natsdriver.ConsumerConfig{Stream: name, Durable: "inventory", Subject: accounting.InventorySubject, Batch: 1, MaxAckPending: 8, MaxWaiting: 1, MaxDeliver: 100, FetchWait: 250 * time.Millisecond, AckWait: 30 * time.Second}
	brokerConfig, err := config.BrokerConsumerConfig()
	if err != nil {
		t.Fatal(err)
	}
	broker, err := retryInventoryStreamRequest(ctx, func(requestCtx context.Context) (jetstream.Consumer, error) {
		return js.CreateOrUpdateConsumer(requestCtx, name, brokerConfig)
	})
	if err != nil {
		t.Fatal("create consumer:", err)
	}
	transport, err := natsdriver.NewConsumer(ctx, fixture.nc, config)
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()
	pass := inventory.NewInventoryCatchUp(consumer)
	pass.Period = time.Millisecond
	app := &App{accounting: consumer, transport: transport, tracker: sharedprocess.NewTracker(), logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if err = app.tracker.SetReady(); err != nil {
		t.Fatal(err)
	}
	runtimeErr := make(chan error, 1)
	if !app.startAccountingJobs(ctx, runtimeErr, cancel, pass) {
		t.Fatal("inventory jobs were not started")
	}
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		info, err := broker.Info(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if observer.passes.Load() > 0 && info.NumWaiting > 0 {
			break
		}
		select {
		case err := <-runtimeErr:
			t.Fatal(err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-ticker.C:
		}
	}
	cancel()
	drain, stopDrain := context.WithTimeout(context.Background(), time.Second)
	defer stopDrain()
	if err = app.tracker.Drain(drain); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-runtimeErr:
		t.Fatal(err)
	default:
	}
	before := observer.passes.Load()
	withoutConsumer := &App{tracker: sharedprocess.NewTracker(), logger: app.logger}
	if err = withoutConsumer.tracker.SetReady(); err != nil {
		t.Fatal(err)
	}
	if withoutConsumer.startAccountingJobs(context.Background(), runtimeErr, func() {}, pass) {
		t.Fatal("inventory jobs started without the inventory consumer")
	}
	time.Sleep(20 * time.Millisecond)
	if observer.passes.Load() != before {
		t.Fatal("catch up ran without the inventory consumer")
	}
}
