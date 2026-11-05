package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/engine"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	sharedprocess "github.com/Sidiora-Technologies/KindleLaunch/shared/process"
	"github.com/jackc/pgx/v5"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func repair73Source() accounting.FillV1 {
	f := accounting.FillV1{
		Version:       accounting.FillVersion,
		Position:      accounting.SourcePosition{ChainID: "125", Height: "100", TransactionIndex: "0", LogIndex: "7", TransactionHash: "0x" + strings.Repeat("a", 64), BlockHash: "0x" + strings.Repeat("b", 64)},
		ActualEmitter: "0x" + strings.Repeat("1", 40), PoolAddress: "0x" + strings.Repeat("2", 40),
		Token: accounting.Asset{Address: "0x" + strings.Repeat("3", 40), Decimals: 18}, Settlement: accounting.Asset{Address: "0x" + strings.Repeat("4", 40), Decimals: 6},
		Side: "buy", TokenAmountRaw: "332665999332665", USDLAmountRaw: "5000000000", PaidInputRaw: "5000000000", FeeRaw: "15000000", InputDerivationQuality: "receipt_incomplete", FeeDisposition: "distributed_usdl",
		Attribution: accounting.Attribution{TransactionSender: "0x" + strings.Repeat("5", 40), Quality: "unavailable", Evidence: []accounting.EvidenceRef{}},
		Execution:   accounting.ExecutionPolicy{Quality: "unavailable", Evidence: []accounting.EvidenceRef{}},
		Provenance:  accounting.Provenance{DeploymentID: "chart-relay-integration", ABIRevision: "sidiora-v1", EmitterCodeEpoch: "epoch-1", PoolCodeEpoch: "epoch-1", ReceiptSHA256: strings.Repeat("c", 64), HeaderSHA256: strings.Repeat("d", 64)}, BlockTimestampSeconds: "1500000000",
	}
	f.SourceFillID = accounting.SourceFillIdentity(f.Position, f.Provenance.DeploymentID)
	return f
}

func repair73Store(t *testing.T) (string, *store.ExactStore) {
	t.Helper()
	dsn := internaltest.NewDatabase(t)
	pool := internaltest.OpenPool(t, dsn)
	for _, ddl := range []func(string) (string, error){store.ExactSchemaDDL, store.SeriesSchemaDDL, store.RebuildSchemaDDL} {
		sql, err := ddl("candles")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(context.Background(), sql); err != nil {
			t.Fatal(err)
		}
	}
	exact, err := store.NewExact(pool, "candles")
	if err != nil {
		t.Fatal(err)
	}
	return dsn, exact
}

func repair73Broker(t *testing.T) (string, *nats.Conn, jetstream.Stream) {
	t.Helper()
	binary, err := exec.LookPath("nats-server")
	if err != nil {
		t.Fatal("real JetStream server required:", err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err = listener.Close(); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	log, err := os.Create(filepath.Join(dir, "broker.log"))
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(binary, "-js", "-a", "127.0.0.1", "-p", strconv.Itoa(port), "-sd", filepath.Join(dir, "data"))
	command.Stdout, command.Stderr = log, log
	if err = command.Start(); err != nil {
		log.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = command.Process.Signal(syscall.SIGINT)
		if err := command.Wait(); err != nil {
			t.Error(err)
		}
		_ = log.Close()
	})
	url := fmt.Sprintf("nats://127.0.0.1:%d", port)
	var nc *nats.Conn
	repair73Wait(t, func() bool {
		nc, err = nats.Connect(url, nats.NoReconnect(), nats.Timeout(100*time.Millisecond))
		return err == nil
	})
	t.Cleanup(nc.Close)
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := js.CreateStream(context.Background(), jetstream.StreamConfig{Name: "CHART_RELAY", Subjects: []string{"candle.updated.v2"}, Storage: jetstream.FileStorage, Duplicates: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	return url, nc, stream
}

func repair73Wait(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("real relay did not reach the expected state before deadline")
}

func repair73Start(t *testing.T, st *store.ExactStore, nc *nats.Conn) *App {
	t.Helper()
	publisher, err := natsdriver.NewPublisher(nc, "CHART_RELAY")
	if err != nil {
		t.Fatal(err)
	}
	tracker := sharedprocess.NewTracker()
	if err = tracker.SetReady(); err != nil {
		t.Fatal(err)
	}
	a := &App{exact: st, chartPublisher: publisher, chartOutbox: chartOutboxConfig{Stream: "CHART_RELAY", Subject: "candle.updated.v2", Batch: 2, IntervalMillis: 20}, tracker: tracker, nats: nc}
	if err = a.startChartOutbox(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	return a
}

func repair73Apply(t *testing.T, st *store.ExactStore) pgx.Tx {
	t.Helper()
	ctx := context.Background()
	tx, err := st.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	if err = engine.ApplySeriesTx(ctx, tx, st, repair73Source(), 1, engine.SeriesCheckpoint{Height: 100, Timestamp: 1500000060}); err != nil {
		t.Fatal(err)
	}
	return tx
}

func repair73Pending(t *testing.T, st *store.ExactStore) int {
	t.Helper()
	var n int
	if err := st.Pool.QueryRow(context.Background(), "SELECT count(*) FROM candles.chart_outbox WHERE published_at IS NULL").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func repair73Messages(t *testing.T, stream jetstream.Stream) uint64 {
	t.Helper()
	info, err := stream.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return info.State.Msgs
}

func repair73Read(t *testing.T, nc *nats.Conn, want int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := js.Stream(ctx, "CHART_RELAY")
	if err != nil {
		t.Fatal(err)
	}
	cfg := natsdriver.ConsumerConfig{Stream: "CHART_RELAY", Durable: "chart_relay_reader", Subject: "candle.updated.v2", Batch: 64, MaxAckPending: 64, MaxWaiting: 1, MaxDeliver: 10, FetchWait: 100 * time.Millisecond, AckWait: time.Second}
	brokerCfg, err := cfg.BrokerConsumerConfig()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = stream.CreateOrUpdateConsumer(ctx, brokerCfg); err != nil {
		t.Fatal(err)
	}
	reader, err := natsdriver.NewConsumer(ctx, nc, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	seen := map[string]bool{}
	for len(seen) < want {
		messages, err := reader.Fetch(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, message := range messages {
			var bar engine.SeriesCandle
			if err = json.Unmarshal(message.Wire, &bar); err != nil {
				t.Fatal(err)
			}
			expected := "candle_" + accounting.HashTuple("chart-outbox-v2", "125", bar.PoolAddress, bar.Interval, strconv.FormatInt(bar.Start, 10), strconv.FormatUint(bar.Revision, 10))
			if message.EventID != expected || seen[expected] || bar.Revision != 1 || bar.Trades != 1 || bar.VolumeUsdlRaw != "5000000000" || bar.CloseSource.SourceFillID != repair73Source().SourceFillID || bar.AlgorithmVersion != engine.SeriesAlgorithm {
				t.Fatalf("unexpected committed snapshot: id=%s bar=%+v", message.EventID, bar)
			}
			seen[expected] = true
			if err = message.Ack(ctx); err != nil {
				t.Fatal(err)
			}
		}
	}
	if len(seen) != want {
		t.Fatalf("received %d identities, want %d", len(seen), want)
	}
}

func TestRepair_73_CommittedOnlyPublication(t *testing.T) {
	_, st := repair73Store(t)
	_, nc, stream := repair73Broker(t)
	tx := repair73Apply(t, st)
	a := repair73Start(t, st, nc)
	time.Sleep(100 * time.Millisecond)
	if got := repair73Messages(t, stream); got != 0 {
		t.Fatalf("uncommitted rows published: %d", got)
	}
	if err := tx.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	if repair73Pending(t, st) != 0 {
		t.Fatal("rolled back rows survived")
	}
	tx = repair73Apply(t, st)
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	repair73Wait(t, func() bool { return repair73Pending(t, st) == 0 })
	if got := repair73Messages(t, stream); got != uint64(len(engine.ExactIntervals)) {
		t.Fatalf("publication count=%d", got)
	}
	repair73Read(t, nc, len(engine.ExactIntervals))
	a.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := a.tracker.Drain(ctx); err != nil {
		t.Fatal("relay participant not released:", err)
	}
}

func TestRepair_73_RetryRestartAndGuardedRetention(t *testing.T) {
	dsn, st := repair73Store(t)
	url, nc, _ := repair73Broker(t)
	ctx := context.Background()
	tx := repair73Apply(t, st)
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	const fault = `CREATE SEQUENCE candles.relay_mark_attempts;
CREATE FUNCTION candles.reject_relay_mark() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM nextval('candles.relay_mark_attempts'); RAISE EXCEPTION 'publication mark interrupted'; END $$;
CREATE TRIGGER reject_relay_mark BEFORE UPDATE ON candles.chart_outbox FOR EACH ROW EXECUTE FUNCTION candles.reject_relay_mark()`
	if _, err := st.Pool.Exec(ctx, fault); err != nil {
		t.Fatal(err)
	}
	a := repair73Start(t, st, nc)
	repair73Wait(t, func() bool {
		var attempts int
		if err := st.Pool.QueryRow(ctx, "SELECT last_value FROM candles.relay_mark_attempts").Scan(&attempts); err != nil {
			t.Fatal(err)
		}
		return attempts >= 2
	})
	a.Close()
	if n := repair73Pending(t, st); n != len(engine.ExactIntervals) {
		t.Fatalf("failed mark lost pending rows: %d", n)
	}
	if got := repair73MessagesAfterReconnect(t, url); got != 1 {
		t.Fatalf("retry was not duplicate safe: %d messages", got)
	}
	source := repair73Source()
	from, err := engine.BucketStart(1500000000, "1m")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = st.Pool.Exec(ctx, "INSERT INTO candles.chart_archive_checkpoints(pool,from_time,to_time,manifest_digest) VALUES($1,$2,$3,$4)", source.PoolAddress, from, from+60, strings.Repeat("e", 64)); err != nil {
		t.Fatal(err)
	}
	if deleted, err := st.DeleteRetainedRange(ctx, source.PoolAddress, "1m", from, from+60); err == nil || deleted != 0 {
		t.Fatal("unpublished range passed retention guard", deleted, err)
	}
	st.Pool.Close()
	pool := internaltest.OpenPool(t, dsn)
	reopened, err := store.NewExact(pool, "candles")
	if err != nil {
		t.Fatal(err)
	}
	if repair73Pending(t, reopened) != len(engine.ExactIntervals) {
		t.Fatal("pending publication state not recovered")
	}
	if _, err = pool.Exec(ctx, "DROP TRIGGER reject_relay_mark ON candles.chart_outbox"); err != nil {
		t.Fatal(err)
	}
	reconnected, err := nats.Connect(url, nats.NoReconnect())
	if err != nil {
		t.Fatal(err)
	}
	restarted := repair73Start(t, reopened, reconnected)
	repair73Wait(t, func() bool { return repair73Pending(t, reopened) == 0 })
	if got := repair73MessagesAfterReconnect(t, url); got != uint64(len(engine.ExactIntervals)) {
		t.Fatalf("recovery duplicated or lost publications: %d", got)
	}
	repair73Read(t, reconnected, len(engine.ExactIntervals))
	if err = reopened.HoldReplayRange(ctx, "live-reader", source.PoolAddress, from, from+60, time.Minute); err != nil {
		t.Fatal(err)
	}
	if deleted, err := reopened.DeleteRetainedRange(ctx, source.PoolAddress, "1m", from, from+60); err == nil || deleted != 0 {
		t.Fatal("live reference guard bypassed", deleted, err)
	}
	if _, err = pool.Exec(ctx, "DELETE FROM candles.chart_replay_references WHERE reference_id='live-reader'"); err != nil {
		t.Fatal(err)
	}
	if deleted, err := reopened.DeleteRetainedRange(ctx, source.PoolAddress, "1m", from, from+60); err != nil || deleted != 1 {
		t.Fatal("drained retained range did not progress", deleted, err)
	}
	restarted.Close()
}

func repair73MessagesAfterReconnect(t *testing.T, url string) uint64 {
	t.Helper()
	nc, err := nats.Connect(url, nats.NoReconnect())
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := js.Stream(context.Background(), "CHART_RELAY")
	if err != nil {
		t.Fatal(err)
	}
	return repair73Messages(t, stream)
}

func TestRepair_73_AdmissionAndShutdown(t *testing.T) {
	c := chartOutboxConfig{Stream: "CHART_RELAY", Subject: "candle.updated.v2", Batch: 2, IntervalMillis: 20}
	policy := delivery.Policy{Subjects: map[string]bool{c.Subject: true}}
	if err := c.validate(policy); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*chartOutboxConfig){func(c *chartOutboxConfig) { c.Stream = "" }, func(c *chartOutboxConfig) { c.Subject = "other.subject" }, func(c *chartOutboxConfig) { c.Batch = 0 }, func(c *chartOutboxConfig) { c.Batch = 65 }, func(c *chartOutboxConfig) { c.IntervalMillis = 0 }, func(c *chartOutboxConfig) { c.IntervalMillis = 10001 }} {
		bad := c
		change(&bad)
		if bad.validate(policy) == nil {
			t.Fatalf("invalid binding admitted: %+v", bad)
		}
	}
	if c.validate(delivery.Policy{}) == nil {
		t.Fatal("unadmitted output subject accepted")
	}
	_, st := repair73Store(t)
	_, nc, _ := repair73Broker(t)
	a := repair73Start(t, st, nc)
	if err := a.startChartOutbox(context.Background(), slog.Default()); err == nil {
		t.Fatal("duplicate participant admitted")
	}
	a.tracker.BeginDrain()
	drainCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := a.tracker.Drain(drainCtx); err != context.DeadlineExceeded {
		t.Fatal("running relay was not tracked:", err)
	}
	a.Close()
	if err := a.tracker.Drain(context.Background()); err != nil {
		t.Fatal(err)
	}
	a.Close()
}
