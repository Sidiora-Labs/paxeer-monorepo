package app

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
)

const repairM1031Subject = "social.repair.events"

func TestRepairM_1031_BrokerReconnectAndDurableReplay(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	broker := repairM1031NewBroker(t)
	runtime := repairM1031Runtime(t, broker)
	payload := json.RawMessage(`{"body":"durable social event","revision":"1"}`)
	event := delivery.Envelope{
		EventID: "repair_m1031_event", EventType: "social.edge.changed", SchemaVersion: 1,
		ChainID: 125, SourceOrder: delivery.SourceOrder{Height: "42", TransactionIndex: "1", LogIndex: "0"},
		EmittedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), TraceID: "repair_m1031_trace",
		Payload: payload, PayloadHash: delivery.Hash(payload),
	}
	policy := delivery.Policy{Subjects: map[string]bool{repairM1031Subject: true}, EventTypes: map[string]bool{event.EventType: true}}
	wire, err := event.Encode(policy, repairM1031Subject)
	if err != nil {
		t.Fatal(err)
	}

	// The PostgreSQL peer replays the committed enqueue, failed lease/retry,
	// reopened-store claim and persistence-ack transcripts. SQL and immutable
	// bytes are checked; the keeper, lease logic, codec and publisher are real.
	claim := func(fence, attempts string) repairM1031SQLStep {
		return repairM1031SQLStep{
			contains: []string{"WITH picked AS", "FOR UPDATE SKIP LOCKED", "RETURNING o.event_id,o.subject,o.envelope,o.fence,o.attempts"},
			columns:  []uint32{25, 25, 17, 20, 23},
			rows:     [][]string{{event.EventID, repairM1031Subject, `\x` + hex.EncodeToString(wire), fence, attempts}}, tag: "UPDATE 1",
		}
	}
	budget := func(fence string) repairM1031SQLStep {
		return repairM1031SQLStep{contains: []string{"RETURNING attempts>", "'" + event.EventID + "'", "'social_repair'", "fence= '" + fence + "'"}, columns: []uint32{16}, rows: [][]string{{"f"}}, tag: "UPDATE 1"}
	}
	database := repairM1031NewSQLTranscript(t, []repairM1031SQLStep{
		{contains: []string{`INSERT INTO "repair".delivery_outbox`, "'" + event.EventID + "'", "'" + repairM1031Subject + "'", hex.EncodeToString(wire)}, tag: "INSERT 0 1"},
		claim("1", "1"), budget("1"),
		{contains: []string{"SET state=CASE WHEN attempts >=", "failure_class= 'transient'", "lease_owner=NULL,lease_until=NULL", "fence= '1'"}, tag: "UPDATE 1"},
		claim("2", "2"), budget("2"),
		{contains: []string{"state='published'", "broker_stream= 'SOCIAL'", "broker_sequence= '1'", "fence= '2'"}, tag: "UPDATE 1"},
		{contains: []string{"WITH picked AS", "FOR UPDATE SKIP LOCKED"}, columns: []uint32{25, 25, 17, 20, 23}, tag: "UPDATE 0"},
	})
	pool, store := database.open(t, ctx, policy)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Enqueue(ctx, tx, repairM1031Subject, event); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	publisher, err := natsdriver.NewPublisher(runtime.nc, "SOCIAL")
	if err != nil {
		t.Fatal(err)
	}
	relay := delivery.Relay{Store: store, Publisher: publisher, Owner: "social_repair", Batch: 1, LeaseDuration: 2 * time.Second, PublishTimeout: 150 * time.Millisecond, RetryDelay: time.Second, MaxAttempts: 10}
	if err = runtime.ready(ctx); err != nil {
		t.Fatalf("initial readiness: %v", err)
	}

	broker.stop()
	repairM1031Await(t, func() bool { return !runtime.nc.IsConnected() })
	if err = runtime.ready(ctx); err == nil {
		t.Fatal("outage remained ready")
	}
	failure := runtime.durableCycle(ctx, relay.RunOnce)
	if failure == nil {
		t.Fatal("publication during outage succeeded")
	}
	if runtime.failure.Load() != nil {
		t.Fatal("recoverable outage poisoned terminal status")
	}
	if runtime.durableFailure.Load() == nil {
		t.Fatal("failed durable cycle was not retained")
	}

	// Stop/reopen the real store and connection pool while retaining the same
	// runtime and NATS client, then replay the committed immutable outbox row.
	pool.Close()
	pool, store = database.open(t, ctx, policy)
	relay.Store = store
	broker.start()
	repairM1031Await(t, runtime.nc.IsConnected)
	if err = runtime.ready(ctx); !errors.Is(err, failure) {
		t.Fatalf("reconnect cleared pending durable work: %v", err)
	}
	// Respect the production relay's minimum durable retry delay.
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if err = runtime.durableCycle(ctx, relay.RunOnce); err != nil {
		t.Fatalf("durable replay: %v", err)
	}
	if err = runtime.ready(ctx); err != nil {
		t.Fatalf("same runtime did not recover readiness: %v", err)
	}
	if runtime.failure.Load() != nil || runtime.durableFailure.Load() != nil {
		t.Fatal("recovery left a failure latched")
	}
	select {
	case published := <-broker.publications:
		decoded, decodeErr := delivery.Decode(published, policy, repairM1031Subject)
		if decodeErr != nil || decoded.EventID != event.EventID || !bytes.Equal(decoded.Payload, payload) || !bytes.Equal(published, wire) {
			t.Fatalf("producer -> outbox -> relay -> NATS -> decoder changed replay: %+v %v", decoded, decodeErr)
		}
	case <-ctx.Done():
		t.Fatal("replay never reached the broker consumer")
	}
	runtime.close()
	reopened := repairM1031Runtime(t, broker)
	reopenedPublisher, err := natsdriver.NewPublisher(reopened.nc, "SOCIAL")
	if err != nil {
		t.Fatal(err)
	}
	pool.Close()
	_, relay.Store = database.open(t, ctx, policy)
	relay.Publisher = reopenedPublisher
	if err = reopened.durableCycle(ctx, relay.RunOnce); err != nil {
		t.Fatalf("reopened component: %v", err)
	}
	if err = reopened.ready(ctx); err != nil {
		t.Fatalf("reopened readiness: %v", err)
	}
	select {
	case <-broker.publications:
		t.Fatal("published row replayed again after reopen")
	default:
	}
	database.assertConsumed(t)
}

func TestRepairM_1031_ProbeAndCycleRecoverIndependently(t *testing.T) {
	broker := repairM1031NewBroker(t)
	runtime := repairM1031Runtime(t, broker)
	ctx, cancelAll := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancelAll()
	broker.answerPings.Store(false)
	probe, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	err := runtime.ready(probe)
	cancel()
	if err == nil || !runtime.nc.IsConnected() {
		t.Fatalf("unverified connected socket passed probe: %v", err)
	}
	if runtime.failure.Load() != nil {
		t.Fatal("failed readiness probe became terminal")
	}
	// A successful work cycle cannot repair the broker probe. An expired real
	// NATS flush also proves that a later successful probe cannot clear work.
	expired, expire := context.WithCancel(ctx)
	expire()
	pending := runtime.durableCycle(ctx,
		func(context.Context) error { return runtime.nc.FlushWithContext(expired) },
		func(ctx context.Context) error {
			broker.answerPings.Store(true)
			return runtime.nc.FlushWithContext(ctx)
		},
	)
	if pending == nil {
		t.Fatal("successful later work hid an earlier cycle failure")
	}
	if err = runtime.nc.FlushTimeout(time.Second); err != nil {
		t.Fatal(err)
	}
	if err = runtime.ready(ctx); !errors.Is(err, pending) {
		t.Fatalf("broker recovery cleared durable status: %v", err)
	}
	if err = runtime.durableCycle(ctx, runtime.nc.FlushWithContext); err != nil {
		t.Fatal(err)
	}
	if err = runtime.ready(ctx); err != nil {
		t.Fatalf("successful cycle did not clear its own status: %v", err)
	}
	broker.stop()
	repairM1031Await(t, func() bool { return !runtime.nc.IsConnected() })
	// The database peer's successful empty relay cycle cannot clear a broker
	// outage. Run the real store through its production claim path.
	policy := delivery.Policy{Subjects: map[string]bool{repairM1031Subject: true}, EventTypes: map[string]bool{"social.edge.changed": true}}
	db := repairM1031NewSQLTranscript(t, []repairM1031SQLStep{{contains: []string{"WITH picked AS"}, columns: []uint32{25, 25, 17, 20, 23}, tag: "UPDATE 0"}})
	_, store := db.open(t, ctx, policy)
	publisher, err := natsdriver.NewPublisher(runtime.nc, "SOCIAL")
	if err != nil {
		t.Fatal(err)
	}
	relay := delivery.Relay{Store: store, Publisher: publisher, Owner: "social_repair", Batch: 1, LeaseDuration: 2 * time.Second, PublishTimeout: 150 * time.Millisecond, RetryDelay: time.Second, MaxAttempts: 10}
	if err = runtime.durableCycle(ctx, relay.RunOnce); err != nil {
		t.Fatal(err)
	}
	if err = runtime.ready(ctx); err == nil {
		t.Fatal("successful durable cycle concealed disconnected broker")
	}
	broker.start()
	repairM1031Await(t, runtime.nc.IsConnected)
	if err = runtime.ready(ctx); err != nil {
		t.Fatalf("broker did not recover independently: %v", err)
	}
	db.assertConsumed(t)
}

func TestRepairM_1031_TerminalWorkerFailureRequiresRestart(t *testing.T) {
	broker := repairM1031NewBroker(t)
	runtime := repairM1031Runtime(t, broker)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	terminal := errors.New("terminal social worker exit")
	runtime.worker(func(context.Context) error { return terminal })
	runtime.wg.Wait()
	if err := runtime.ready(ctx); !errors.Is(err, terminal) {
		t.Fatalf("worker exit was not terminal: %v", err)
	}
	runtime.fail(errors.New("later terminal failure"))
	broker.stop()
	repairM1031Await(t, func() bool { return !runtime.nc.IsConnected() })
	broker.start()
	repairM1031Await(t, runtime.nc.IsConnected)
	if err := runtime.durableCycle(ctx, runtime.nc.FlushWithContext); err != nil {
		t.Fatal(err)
	}
	if err := runtime.ready(ctx); !errors.Is(err, terminal) {
		t.Fatalf("recovery cleared original terminal fault: %v", err)
	}
	runtime.close()
	reopened := repairM1031Runtime(t, broker)
	if err := reopened.ready(ctx); err != nil {
		t.Fatalf("restart retained prior lifecycle fault: %v", err)
	}
}

func repairM1031Runtime(t *testing.T, broker *repairM1031Broker) *socialConnectedRuntime {
	t.Helper()
	ctx, stop := context.WithCancel(context.Background())
	runtime := &socialConnectedRuntime{ctx: ctx, stop: stop}
	if err := runtime.connectBroker("nats://"+broker.address, nats.ReconnectWait(500*time.Millisecond), nats.ReconnectJitter(0, 0)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.close)
	return runtime
}

func repairM1031Await(t *testing.T, ready func() bool) {
	t.Helper()
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for !ready() {
		select {
		case <-ticker.C:
		case <-deadline.C:
			t.Fatal("component state did not converge")
		}
	}
}

// Loopback NATS protocol fixture: supports connection probes and the exact
// header publication / persistence-ack exchange used by natsdriver.Publisher.
// Stop/start closes and reopens the listener and all accepted sockets.
type repairM1031Broker struct {
	t            *testing.T
	address      string
	listener     net.Listener
	acceptDone   chan struct{}
	mu           sync.Mutex
	sockets      map[net.Conn]bool
	wg           sync.WaitGroup
	answerPings  atomic.Bool
	publications chan []byte
	persisted    []byte
}

func repairM1031NewBroker(t *testing.T) *repairM1031Broker {
	t.Helper()
	broker := &repairM1031Broker{t: t, address: "127.0.0.1:0", sockets: map[net.Conn]bool{}, publications: make(chan []byte, 8)}
	broker.answerPings.Store(true)
	broker.start()
	t.Cleanup(broker.stop)
	return broker
}

func (b *repairM1031Broker) start() {
	listener, err := net.Listen("tcp", b.address)
	if err != nil {
		b.t.Fatal(err)
	}
	b.listener, b.address = listener, listener.Addr().String()
	b.acceptDone = make(chan struct{})
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		defer close(b.acceptDone)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			b.mu.Lock()
			b.sockets[conn] = true
			b.mu.Unlock()
			b.wg.Add(1)
			go func() {
				defer b.wg.Done()
				defer conn.Close()
				defer func() { b.mu.Lock(); delete(b.sockets, conn); b.mu.Unlock() }()
				b.serve(conn)
			}()
		}
	}()
}

func (b *repairM1031Broker) stop() {
	if b.listener == nil {
		return
	}
	b.listener.Close()
	<-b.acceptDone
	b.mu.Lock()
	for conn := range b.sockets {
		conn.Close()
	}
	b.mu.Unlock()
	b.wg.Wait()
	b.listener = nil
}

func (b *repairM1031Broker) serve(conn net.Conn) {
	fmt.Fprint(conn, "INFO {\"server_id\":\"repair_m1031\",\"version\":\"2.10.0\",\"proto\":1,\"headers\":true,\"max_payload\":1048576,\"jetstream\":true}\r\n")
	reader := bufio.NewReader(conn)
	var inbox, sid string
	pendingPongs := 0
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			b.t.Error("empty NATS command")
			return
		}
		switch fields[0] {
		case "CONNECT", "PONG", "UNSUB":
		case "PING":
			pendingPongs++
			if b.answerPings.Load() {
				for pendingPongs > 0 {
					if _, err = fmt.Fprint(conn, "PONG\r\n"); err != nil {
						return
					}
					pendingPongs--
				}
			}
		case "SUB":
			if len(fields) != 3 {
				b.t.Errorf("unexpected subscription: %q", line)
				return
			}
			inbox, sid = fields[1], fields[2]
		case "HPUB":
			if len(fields) != 5 || fields[1] != repairM1031Subject || !strings.HasPrefix(fields[2], strings.TrimSuffix(inbox, "*")) {
				b.t.Errorf("unexpected publication: %q inbox=%q", line, inbox)
				return
			}
			headerSize, e1 := strconv.Atoi(fields[3])
			total, e2 := strconv.Atoi(fields[4])
			if e1 != nil || e2 != nil || headerSize < 1 || total < headerSize || total > delivery.MaxEnvelopeBytes+1024 {
				b.t.Error("invalid NATS publication bounds")
				return
			}
			raw := make([]byte, total+2)
			if _, err = io.ReadFull(reader, raw); err != nil {
				return
			}
			header := string(raw[:headerSize])
			if !strings.Contains(header, "Nats-Msg-Id: repair_m1031_event\r\n") || !strings.Contains(header, "Nats-Expected-Stream: SOCIAL\r\n") || string(raw[total:]) != "\r\n" {
				b.t.Errorf("invalid publication headers: %q", header)
				return
			}
			b.mu.Lock()
			duplicate := len(b.persisted) != 0
			if duplicate && !bytes.Equal(b.persisted, raw[headerSize:total]) {
				b.mu.Unlock()
				b.t.Error("duplicate message identity changed immutable payload")
				return
			}
			if !duplicate {
				b.persisted = append([]byte(nil), raw[headerSize:total]...)
				b.publications <- append([]byte(nil), b.persisted...)
			}
			b.mu.Unlock()
			ack := fmt.Sprintf(`{"stream":"SOCIAL","seq":1,"duplicate":%t}`, duplicate)
			if _, err = fmt.Fprintf(conn, "MSG %s %s %d\r\n%s\r\n", fields[2], sid, len(ack), ack); err != nil {
				return
			}
		default:
			b.t.Errorf("unrecorded NATS command: %q", line)
			return
		}
	}
}

// These are committed PostgreSQL wire transcripts, not a replacement Store.
// Each exchange matches actual production SQL; no SQL engine or binary runs.
type repairM1031SQLStep struct {
	contains []string
	columns  []uint32
	rows     [][]string
	tag      string
}

type repairM1031SQLTranscript struct {
	t        *testing.T
	listener net.Listener
	mu       sync.Mutex
	sockets  map[net.Conn]bool
	steps    []repairM1031SQLStep
	next     int
	wg       sync.WaitGroup
}

func repairM1031NewSQLTranscript(t *testing.T, steps []repairM1031SQLStep) *repairM1031SQLTranscript {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	db := &repairM1031SQLTranscript{t: t, listener: listener, sockets: map[net.Conn]bool{}, steps: steps}
	acceptDone := make(chan struct{})
	db.wg.Add(1)
	go func() {
		defer db.wg.Done()
		defer close(acceptDone)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			db.mu.Lock()
			db.sockets[conn] = true
			db.mu.Unlock()
			db.wg.Add(1)
			go func() {
				defer db.wg.Done()
				defer conn.Close()
				defer func() { db.mu.Lock(); delete(db.sockets, conn); db.mu.Unlock() }()
				db.serve(conn)
			}()
		}
	}()
	t.Cleanup(func() {
		listener.Close()
		<-acceptDone
		db.mu.Lock()
		for conn := range db.sockets {
			conn.Close()
		}
		db.mu.Unlock()
		db.wg.Wait()
	})
	return db
}

func (db *repairM1031SQLTranscript) open(t *testing.T, ctx context.Context, policy delivery.Policy) (*pgxpool.Pool, *delivery.Store) {
	t.Helper()
	cfg, err := pgxpool.ParseConfig("postgres://repair:repair@" + db.listener.Addr().String() + "/repair?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	cfg.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	store, err := delivery.NewStore(pool, "repair", policy)
	if err != nil {
		t.Fatal(err)
	}
	return pool, store
}

func (db *repairM1031SQLTranscript) assertConsumed(t *testing.T) {
	t.Helper()
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.next != len(db.steps) {
		t.Fatalf("SQL transcript consumed %d/%d statements", db.next, len(db.steps))
	}
}

func (db *repairM1031SQLTranscript) serve(conn net.Conn) {
	backend := pgproto3.NewBackend(conn, conn)
	if _, err := backend.ReceiveStartupMessage(); err != nil {
		db.t.Error(err)
		return
	}
	backend.Send(&pgproto3.AuthenticationOk{})
	backend.Send(&pgproto3.ParameterStatus{Name: "server_version", Value: "16.0"})
	backend.Send(&pgproto3.ParameterStatus{Name: "client_encoding", Value: "UTF8"})
	backend.Send(&pgproto3.ParameterStatus{Name: "standard_conforming_strings", Value: "on"})
	backend.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
	if err := backend.Flush(); err != nil {
		return
	}
	status := byte('I')
	for {
		message, err := backend.Receive()
		if err != nil {
			return
		}
		switch query := message.(type) {
		case *pgproto3.Terminate:
			return
		case *pgproto3.Query:
			sql := query.String
			switch sql {
			case "begin":
				status = 'T'
				backend.Send(&pgproto3.CommandComplete{CommandTag: []byte("BEGIN")})
			case "commit":
				status = 'I'
				backend.Send(&pgproto3.CommandComplete{CommandTag: []byte("COMMIT")})
			default:
				db.mu.Lock()
				if db.next >= len(db.steps) {
					db.mu.Unlock()
					db.t.Errorf("unrecorded SQL: %s", sql)
					return
				}
				step := db.steps[db.next]
				db.next++
				db.mu.Unlock()
				for _, part := range step.contains {
					if !strings.Contains(sql, part) {
						db.t.Errorf("SQL transcript missing %q in %s", part, sql)
						return
					}
				}
				if status != 'T' {
					db.t.Errorf("durable SQL outside transaction: %s", sql)
					return
				}
				if len(step.columns) > 0 {
					fields := make([]pgproto3.FieldDescription, len(step.columns))
					for i, oid := range step.columns {
						fields[i] = pgproto3.FieldDescription{Name: []byte(fmt.Sprintf("column_%d", i)), DataTypeOID: oid, DataTypeSize: -1, TypeModifier: -1}
					}
					backend.Send(&pgproto3.RowDescription{Fields: fields})
					for _, row := range step.rows {
						values := make([][]byte, len(row))
						for i, value := range row {
							values[i] = []byte(value)
						}
						backend.Send(&pgproto3.DataRow{Values: values})
					}
				}
				backend.Send(&pgproto3.CommandComplete{CommandTag: []byte(step.tag)})
			}
			backend.Send(&pgproto3.ReadyForQuery{TxStatus: status})
			if err := backend.Flush(); err != nil {
				return
			}
		default:
			db.t.Errorf("unrecorded PostgreSQL message: %T", message)
			return
		}
	}
}
