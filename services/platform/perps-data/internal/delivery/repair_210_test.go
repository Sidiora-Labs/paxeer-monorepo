package delivery_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/delivery"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/delivery/natsdriver"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/publish"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/stream"
)

func repair210Envelope(t *testing.T, id, kind string, size int, policy delivery.Policy) delivery.Envelope {
	t.Helper()
	e := testEnvelope(id, kind, 1)
	e.TraceID = strings.Repeat("t", 128)
	e.SourceOrder = delivery.SourceOrder{Height: strconv.FormatInt(math.MaxInt64, 10), TransactionIndex: strconv.FormatInt(math.MaxInt64, 10), LogIndex: strconv.FormatInt(math.MaxInt64, 10)}
	e.Payload = json.RawMessage(`{"pad":""}`)
	e.PayloadHash = delivery.Hash(e.Payload)
	wire, err := e.Encode(policy, fillSubject)
	if err != nil {
		t.Fatal(err)
	}
	if size < len(wire) {
		t.Fatalf("requested envelope size %d below metadata size %d", size, len(wire))
	}
	e.Payload = json.RawMessage(`{"pad":"` + strings.Repeat("x", size-len(wire)) + `"}`)
	e.PayloadHash = delivery.Hash(e.Payload)
	return e
}

func TestRepair_210_HeaderBudget(t *testing.T) {
	// Parse the deployment's actual limit; only fixture credentials are supplied.
	t.Setenv("RAILWAY_SERVICE_NAME", "repair-210")
	t.Setenv("NATS_MAX_FILE_STORE", "19327352832")
	t.Setenv("NATS_CLUSTER_PASSWORD", "repair-cluster")
	t.Setenv("NATS_INDEXER_PASSWORD", "repair-indexer")
	t.Setenv("NATS_GATEWAY_PASSWORD", "repair-gateway")
	t.Setenv("NATS_ROUTES", "[]")
	src, err := os.ReadFile("../../deploy/nats/nats.conf")
	if err != nil {
		t.Fatal(err)
	}
	opts := &server.Options{}
	if err = opts.ProcessConfigString(string(src)); err != nil {
		t.Fatal(err)
	}
	if opts.MaxPayload != delivery.MaxTransportBytes || stream.Config(125).MaxMsgSize != delivery.MaxTransportBytes {
		t.Fatalf("transport budgets disagree: broker=%d stream=%d delivery=%d", opts.MaxPayload, stream.Config(125).MaxMsgSize, delivery.MaxTransportBytes)
	}
	// Msg.Size uses the NATS client's actual header serialization. Use maximum
	// values, including the generic driver's maximum stream name and uint64
	// expectation, even though PERPS_V1 uses shorter values in practice.
	msg := &nats.Msg{Subject: fillSubject, Header: nats.Header{}}
	msg.Header.Set(publish.HeaderSeq, strconv.FormatInt(math.MaxInt64, 10))
	msg.Header.Set(publish.HeaderBlock, strconv.FormatInt(math.MaxInt64, 10))
	msg.Header.Set(publish.HeaderKind, strings.Repeat("k", 128))
	msg.Header.Set("Nats-Msg-Id", strings.Repeat("i", 128))
	msg.Header.Set("Nats-Expected-Stream", strings.Repeat("s", 128))
	msg.Header.Set("Nats-Expected-Last-Sequence", strconv.FormatUint(math.MaxUint64, 10))
	headerBytes := msg.Size() - len(msg.Subject)
	if headerBytes != delivery.MaxTransportHeaderBytes || delivery.MaxEnvelopeBytes+headerBytes != int(opts.MaxPayload) {
		t.Fatalf("serialized headers=%d reserved=%d body=%d transport=%d", headerBytes, delivery.MaxTransportHeaderBytes, delivery.MaxEnvelopeBytes, opts.MaxPayload)
	}
	control := &nats.Msg{Subject: "perps.125.control", Header: nats.Header{}}
	control.Header.Set(delivery.HeaderControl, delivery.ControlSkipped)
	control.Header.Set(publish.HeaderSeq, strconv.FormatInt(math.MaxInt64, 10))
	control.Header.Set("Nats-Msg-Id", controlID(math.MaxInt64, strings.Repeat("i", 128)))
	control.Header.Set("Nats-Expected-Stream", strings.Repeat("s", 128))
	control.Header.Set("Nats-Expected-Last-Sequence", strconv.FormatUint(math.MaxUint64, 10))
	if n := control.Size() - len(control.Subject); n > delivery.MaxTransportHeaderBytes {
		t.Fatalf("control headers=%d exceed reserve=%d", n, delivery.MaxTransportHeaderBytes)
	}
	for _, kind := range []string{strings.Repeat("k", 129), "fill\r\nInjected: value"} {
		policy := testPolicy()
		policy.EventTypes[kind] = true
		e := testEnvelope("invalid-header-kind", kind, 1)
		if err := e.Validate(policy, fillSubject); err == nil {
			t.Fatalf("accepted unbounded or non-token kind %q", kind)
		}
	}
}

func repair210Database(t *testing.T, ctx context.Context) *sql.DB {
	t.Helper()
	dsn := os.Getenv(testDatabaseEnv)
	if dsn == "" {
		t.Fatal("repair proof requires " + testDatabaseEnv)
	}
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		var err error
		dsn, err = pq.ParseURL(dsn)
		if err != nil {
			t.Fatal(err)
		}
	}
	admin, err := store.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	name := "repair_210_" + strings.ToLower(rand.Text())
	if _, err = admin.ExecContext(ctx, "CREATE DATABASE "+pq.QuoteIdentifier(name)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec("DROP DATABASE " + pq.QuoteIdentifier(name)); err != nil {
			t.Error(err)
		}
	})
	db, err := store.Open(dsn + " dbname=" + name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = store.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `CREATE TABLE perps.repair_210_effects(event_id text PRIMARY KEY, payload bytea NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestRepair_210_EnvelopeTransportBudget(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db := repair210Database(t, ctx)
	kind := strings.Repeat("k", 128)
	policy := testPolicy()
	policy.EventTypes[kind] = true
	outbox, err := delivery.NewStore(db, "perps", policy)
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range []int{delivery.MaxEnvelopeBytes + 1, delivery.MaxTransportBytes, delivery.MaxTransportBytes + 1} {
		e := repair210Envelope(t, "oversized", kind, size, policy)
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		_, enqueueErr := outbox.Enqueue(ctx, tx, fillSubject, e)
		if err = tx.Rollback(); err != nil {
			t.Fatal(err)
		}
		if enqueueErr == nil {
			t.Fatalf("admitted %d-byte envelope", size)
		}
		wire, err := json.Marshal(e)
		if err != nil || len(wire) != size {
			t.Fatalf("oversized fixture length=%d want=%d error=%v", len(wire), size, err)
		}
		if _, err = delivery.Decode(wire, policy, fillSubject); err == nil {
			t.Fatalf("consumer accepted %d-byte envelope", size)
		}
	}
	if n := count(t, db, `SELECT count(*) FROM perps.delivery_outbox`); n != 0 {
		t.Fatalf("refused envelopes left %d outbox rows", n)
	}
	ns, err := server.NewServer(&server.Options{Host: "127.0.0.1", Port: server.RANDOM_PORT, JetStream: true, MaxPayload: delivery.MaxTransportBytes, StoreDir: t.TempDir(), NoLog: true, NoSigs: true})
	if err != nil {
		t.Fatal(err)
	}
	go ns.Start()
	t.Cleanup(ns.Shutdown)
	if !ns.ReadyForConnections(10 * time.Second) {
		t.Fatal("NATS did not become ready")
	}
	nc, err := nats.Connect(ns.ClientURL())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	cfg := stream.Config(125)
	cfg.Replicas = 1
	perps, err := js.CreateStream(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	consumerCfg := natsdriver.ConsumerConfig{Stream: stream.Name, Durable: "repair-210", Subject: fillSubject, Batch: 2, MaxAckPending: 2, MaxWaiting: 1, MaxDeliver: 2, FetchWait: time.Second, AckWait: 10 * time.Second}
	brokerCfg, err := consumerCfg.BrokerConsumerConfig()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = js.CreateConsumer(ctx, stream.Name, brokerCfg); err != nil {
		t.Fatal(err)
	}
	consumer, err := natsdriver.NewConsumer(ctx, nc, consumerCfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(consumer.Close)
	events := []delivery.Envelope{
		repair210Envelope(t, strings.Repeat("a", 128), kind, delivery.MaxEnvelopeBytes, policy),
		repair210Envelope(t, strings.Repeat("b", 128), kind, delivery.MaxEnvelopeBytes, policy),
	}
	// The isolated database lets the real producer allocate full-width sequence
	// values without modifying immutable outbox rows or another test's sequence.
	// MaxInt64 itself is the existing claim query's exclusive barrier sentinel;
	// keep all three rows below it while retaining 19-digit header values.
	if _, err = db.ExecContext(ctx, `SELECT setval(pg_get_serial_sequence('perps.delivery_outbox','perps_seq'),$1,false)`, int64(math.MaxInt64-3)); err != nil {
		t.Fatal(err)
	}
	enqueueCommitted(t, ctx, db, outbox, fillSubject, events...)
	p, err := publish.New(db, nc, policy, publish.Config{Owner: "repair-210", Batch: 2, MaxAttempts: 2, LeaseDuration: 10 * time.Second, PublishTimeout: 2 * time.Second, RetryDelay: time.Second, IdleInterval: time.Millisecond, MaxHeadAge: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	runCtx, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- p.Run(runCtx) }()
	defer func() {
		stop()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Errorf("publisher stopped: %v", err)
		}
	}()
	waitDrained := func() {
		t.Helper()
		for count(t, db, `SELECT count(*) FROM perps.delivery_outbox WHERE state='pending' OR (state='quarantined' AND broker_sequence IS NULL)`) != 0 {
			select {
			case <-ctx.Done():
				t.Fatal("outbox did not drain: ", ctx.Err())
			case <-time.After(5 * time.Millisecond):
			}
		}
	}
	waitDrained()
	messages, err := consumer.Fetch(ctx)
	if err != nil || len(messages) != len(events) {
		t.Fatalf("consumer received %d events: %v", len(messages), err)
	}
	for i, e := range events {
		wire, err := e.Encode(policy, fillSubject)
		if err != nil || len(wire) != delivery.MaxEnvelopeBytes {
			t.Fatalf("largest admitted body=%d, error=%v", len(wire), err)
		}
		row := readOutbox(t, db, e.EventID)
		if row.state != "published" || row.failureClass != "" || row.attempts != 1 || row.brokerSequence != int64(i+1) {
			t.Fatalf("admitted event was lost or retried: %+v", row)
		}
		m := messages[i]
		if m.EventID != e.EventID || m.Subject != fillSubject || m.Sequence != uint64(i+1) || !bytes.Equal(m.Wire, wire) {
			t.Fatalf("consumer changed event %d (received %d bytes)", i, len(m.Wire))
		}
		if m.Header.Get(publish.HeaderSeq) != strconv.FormatInt(int64(math.MaxInt64-3+i), 10) || m.Header.Get(publish.HeaderBlock) != e.SourceOrder.Height || m.Header.Get(publish.HeaderKind) != kind || m.Header.Get("Nats-Expected-Stream") != stream.Name {
			t.Fatalf("missing publisher headers: %v", m.Header)
		}
		if i == 1 && m.Header.Get("Nats-Expected-Last-Sequence") != "1" {
			t.Fatalf("pipelined event lacks sequence expectation: %v", m.Header)
		}
		if n := (&nats.Msg{Header: m.Header, Data: m.Wire}).Size(); n > delivery.MaxTransportBytes {
			t.Fatalf("actual body and headers=%d exceed transport budget", n)
		}
		if err = outbox.Consume(ctx, "repair-210", "v1", m.Subject, m.Wire, m, func(ctx context.Context, tx *sql.Tx, received delivery.Envelope) error {
			if received.EventID != e.EventID || received.PayloadHash != e.PayloadHash || !bytes.Equal(received.Payload, e.Payload) {
				t.Fatal("transactional consumer changed payload")
			}
			_, err := tx.ExecContext(ctx, `INSERT INTO perps.repair_210_effects(event_id,payload) VALUES($1,$2)`, received.EventID, []byte(received.Payload))
			return err
		}); err != nil {
			t.Fatal(err)
		}
		var payload []byte
		if err = db.QueryRowContext(ctx, `SELECT payload FROM perps.repair_210_effects WHERE event_id=$1`, e.EventID).Scan(&payload); err != nil || !bytes.Equal(payload, e.Payload) {
			t.Fatalf("committed consumer payload differs: %v", err)
		}
	}
	if n := count(t, db, `SELECT count(*) FROM perps.delivery_inbox`); n != 2 || ackPending(t, ctx, mustRepair210Consumer(t, ctx, js, consumerCfg.Durable)) != 0 {
		t.Fatalf("consumer did not commit and acknowledge both events: inbox=%d", n)
	}
	// Only legacy-invalid data bypasses admission. The real relay must retain
	// its permanent quarantine and ordered skipped-control behavior.
	legacy := repair210Envelope(t, strings.Repeat("c", 128), kind, delivery.MaxTransportBytes, policy)
	legacyWire, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO perps.delivery_outbox(event_id,schema_version,subject,payload_hash,envelope) VALUES($1,1,$2,$3,$4)`, legacy.EventID, fillSubject, legacy.PayloadHash, legacyWire); err != nil {
		t.Fatal(err)
	}
	waitDrained()
	row := readOutbox(t, db, legacy.EventID)
	if row.state != "quarantined" || row.failureClass != string(delivery.SchemaViolation) || row.brokerSequence != 3 {
		t.Fatalf("legacy-invalid row was not quarantined and announced: %+v", row)
	}
	control, err := perps.GetMsg(ctx, 3)
	if err != nil {
		t.Fatal(err)
	}
	subject, id, body, err := delivery.ControlMessage(delivery.Lease{EventID: legacy.EventID, Subject: fillSubject, PerpsSeq: math.MaxInt64 - 1, Control: true})
	if err != nil || control.Subject != subject || control.Header.Get("Nats-Msg-Id") != id || control.Header.Get(delivery.HeaderControl) != delivery.ControlSkipped || !bytes.Equal(control.Data, body) {
		t.Fatalf("legacy row did not produce its skipped control: %v", err)
	}
	if info, err := perps.Info(ctx); err != nil || info.State.Msgs != 3 {
		t.Fatalf("unexpected persisted events/control: %v, %v", info, err)
	}
}

func mustRepair210Consumer(t *testing.T, ctx context.Context, js jetstream.JetStream, name string) jetstream.Consumer {
	t.Helper()
	c, err := js.Consumer(ctx, stream.Name, name)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
