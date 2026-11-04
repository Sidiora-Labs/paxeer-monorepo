package consumer_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/consumer"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/httpapi"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/migrations"
	"github.com/Sidiora-Technologies/KindleLaunch/protocol/bindings"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
	_ "modernc.org/sqlite"
)

// repairM1042SQL exposes a real file-backed SQLite database over the PostgreSQL
// simple-query protocol. Production pgx, Store, projection and holder arithmetic
// run unchanged. Only schema qualification and lock syntax are translated:
// BEGIN IMMEDIATE supplies a database-wide writer lock instead of advisory locks.
// Savepoints, conflicts, reads, writes and crash recovery execute in SQLite.
// This fixture does not certify PostgreSQL-specific concurrency behaviour.
type repairM1042SQL struct {
	db          *sql.DB
	listener    net.Listener
	accepted    chan struct{}
	connections sync.WaitGroup
	drop        atomic.Value
	hits        atomic.Int32
}

func repairM1042OpenSQL(t *testing.T, path string) *repairM1042SQL {
	t.Helper()
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	s := &repairM1042SQL{db: db, listener: listener, accepted: make(chan struct{})}
	s.drop.Store("")
	go func() {
		defer close(s.accepted)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			s.connections.Add(1)
			go func() { defer s.connections.Done(); s.serve(conn) }()
		}
	}()
	return s
}

func (s *repairM1042SQL) close() {
	_ = s.listener.Close()
	<-s.accepted
	s.connections.Wait()
	_ = s.db.Close()
}

func (s *repairM1042SQL) serve(conn net.Conn) {
	defer conn.Close()
	backend := pgproto3.NewBackend(conn, conn)
	if _, err := backend.ReceiveStartupMessage(); err != nil {
		return
	}
	backend.Send(&pgproto3.AuthenticationOk{})
	for key, value := range map[string]string{"client_encoding": "UTF8", "server_encoding": "UTF8", "standard_conforming_strings": "on", "server_version": "16.0"} {
		backend.Send(&pgproto3.ParameterStatus{Name: key, Value: value})
	}
	backend.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
	if err := backend.Flush(); err != nil {
		return
	}
	ctx := context.Background()
	dbconn, err := s.db.Conn(ctx)
	if err != nil {
		return
	}
	defer dbconn.Close()
	defer func() { _, _ = dbconn.ExecContext(ctx, "ROLLBACK") }()
	status := byte('I')
	for {
		message, err := backend.Receive()
		if err != nil {
			return
		}
		switch message := message.(type) {
		case *pgproto3.Terminate:
			return
		case *pgproto3.Query:
			query := strings.TrimSpace(message.String)
			normalized := strings.ToUpper(strings.Join(strings.Fields(query), " "))
			translated := strings.ReplaceAll(query, "stats.", "")
			translated = strings.ReplaceAll(translated, "::int", "")
			if normalized == "BEGIN" {
				translated = "BEGIN IMMEDIATE"
			}
			if strings.HasPrefix(normalized, "SELECT PG_ADVISORY_XACT_LOCK(") {
				translated = "SELECT 1"
			}
			tag := ""
			if status == 'E' && !strings.HasPrefix(normalized, "ROLLBACK") {
				err = fmt.Errorf("current transaction is aborted")
			} else if strings.HasPrefix(strings.ToUpper(translated), "SELECT") {
				err = repairM1042Rows(ctx, backend, dbconn, translated)
			} else {
				var result sql.Result
				result, err = dbconn.ExecContext(ctx, translated)
				if err == nil {
					affected, countErr := result.RowsAffected()
					if countErr != nil {
						return
					}
					verb := strings.Fields(normalized)[0]
					switch verb {
					case "INSERT":
						tag = fmt.Sprintf("INSERT 0 %d", affected)
					case "UPDATE", "DELETE":
						tag = fmt.Sprintf("%s %d", verb, affected)
					default:
						tag = verb
					}
				}
			}
			if err != nil {
				backend.Send(&pgproto3.ErrorResponse{Severity: "ERROR", Code: "XX000", Message: err.Error()})
				if status == 'T' {
					status = 'E'
				}
			} else {
				if normalized == "BEGIN" {
					status = 'T'
				}
				if normalized == "COMMIT" || normalized == "ROLLBACK" {
					status = 'I'
				}
				if strings.HasPrefix(normalized, "ROLLBACK TO") {
					status = 'T'
				}
				if target := s.drop.Load().(string); target != "" && strings.HasPrefix(normalized, target) && s.drop.CompareAndSwap(target, "") {
					s.hits.Add(1)
					return // actual write executed; discard its acknowledgement and disconnect
				}
				if tag != "" {
					backend.Send(&pgproto3.CommandComplete{CommandTag: []byte(tag)})
				}
			}
			backend.Send(&pgproto3.ReadyForQuery{TxStatus: status})
			if err := backend.Flush(); err != nil {
				return
			}
		default:
			return // the client explicitly selects simple-query mode
		}
	}
}

func repairM1042Rows(ctx context.Context, backend *pgproto3.Backend, conn *sql.Conn, query string) error {
	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()
	columns, err := rows.ColumnTypes()
	if err != nil {
		return err
	}
	fields := make([]pgproto3.FieldDescription, len(columns))
	for i, column := range columns {
		oid := uint32(pgtype.TextOID)
		typeName := strings.ToUpper(column.DatabaseTypeName())
		if strings.Contains(typeName, "INT") {
			oid = pgtype.Int8OID
		}
		if typeName == "BOOLEAN" {
			oid = pgtype.BoolOID
		}
		fields[i] = pgproto3.FieldDescription{Name: []byte(column.Name()), DataTypeOID: oid, DataTypeSize: -1}
	}
	var data [][][]byte
	for rows.Next() {
		values := make([]any, len(fields))
		dest := make([]any, len(fields))
		for i := range values {
			dest[i] = &values[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return err
		}
		wire := make([][]byte, len(fields))
		for i, value := range values {
			if value == nil {
				continue
			}
			switch value := value.(type) {
			case int64:
				fields[i].DataTypeOID = pgtype.Int8OID
				wire[i] = []byte(strconv.FormatInt(value, 10))
			case bool:
				fields[i].DataTypeOID = pgtype.BoolOID
				if value {
					wire[i] = []byte("t")
				} else {
					wire[i] = []byte("f")
				}
			case []byte:
				wire[i] = value
			default:
				wire[i] = []byte(fmt.Sprint(value))
			}
		}
		data = append(data, wire)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	backend.Send(&pgproto3.RowDescription{Fields: fields})
	for _, row := range data {
		backend.Send(&pgproto3.DataRow{Values: row})
	}
	backend.Send(&pgproto3.CommandComplete{CommandTag: []byte(fmt.Sprintf("SELECT %d", len(data)))})
	return nil
}

type repairM1042Fixture struct {
	path   string
	db     *repairM1042SQL
	pool   *pgxpool.Pool
	store  *store.Store
	swap   *consumer.SwapConsumer
	holder *consumer.HolderTracker
	redis  *goredis.Client
}

func newRepairM1042Fixture(t *testing.T) *repairM1042Fixture {
	t.Helper()
	f := &repairM1042Fixture{path: filepath.Join(t.TempDir(), "stats.sqlite")}
	f.open(t)
	t.Cleanup(f.close)
	ddl, err := migrations.FS.ReadFile("00001_init.sql")
	if err != nil {
		t.Fatal(err)
	}
	up := strings.Split(string(ddl), "-- +goose Down")[0]
	var lines []string
	for _, line := range strings.Split(up, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "--") {
			lines = append(lines, line)
		}
	}
	for _, statement := range strings.Split(strings.Join(lines, "\n"), ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" || strings.HasPrefix(statement, "CREATE SCHEMA") {
			continue
		}
		statement = strings.ReplaceAll(statement, `"stats".`, "")
		statement = strings.ReplaceAll(statement, " USING btree", "")
		if _, err := f.db.db.Exec(statement); err != nil {
			t.Fatalf("registered migration: %v: %s", err, statement)
		}
	}
	return f
}

func (f *repairM1042Fixture) open(t *testing.T) {
	t.Helper()
	f.db = repairM1042OpenSQL(t, f.path)
	cfg, err := pgxpool.ParseConfig("postgres://fixture@" + f.db.listener.Addr().String() + "/stats?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	cfg.ConnConfig.ConnectTimeout = 3 * time.Second
	cfg.MaxConns = 4
	f.pool, err = pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	f.store = store.New(f.pool)
	// A closed production Redis client fails deterministically without networking.
	// Seeded-pool tests assert this post-commit failure and retry its delivery.
	f.redis = goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:1"})
	if err := f.redis.Close(); err != nil {
		t.Fatal(err)
	}
	f.swap = consumer.NewSwapConsumer(f.store, f.redis, discardLogger())
	f.holder = consumer.NewHolderTracker(f.store, f.redis, discardLogger(), time.Hour)
}

func (f *repairM1042Fixture) close() {
	f.holder.Close()
	f.pool.Close()
	f.db.close()
}

func (f *repairM1042Fixture) restart(t *testing.T) {
	t.Helper()
	f.close()
	f.open(t) // reopen the on-disk database and create all consumer/store instances anew
}

func repairM1042Events(t *testing.T) []consumer.SwapEvent {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "projectionstats", "testdata", "real_pool_125.json"))
	if err != nil {
		t.Fatal(err)
	}
	var source realLegacyPool
	if err := json.Unmarshal(raw, &source); err != nil {
		t.Fatal(err)
	}
	filterer, err := bindings.NewEventEmitterFilterer(common.HexToAddress(source.EventEmitter), nil)
	if err != nil {
		t.Fatal(err)
	}
	var events []consumer.SwapEvent
	for _, tx := range source.Transactions {
		for _, log := range tx.Logs {
			if log.Address != source.EventEmitter || len(log.Topics) == 0 || log.Topics[0] != legacySwapTopic {
				continue
			}
			data, err := hex.DecodeString(strings.TrimPrefix(log.Data, "0x"))
			if err != nil {
				t.Fatal(err)
			}
			topics := make([]common.Hash, len(log.Topics))
			for i, topic := range log.Topics {
				topics[i] = common.HexToHash(topic)
			}
			decoded, err := filterer.ParseSwap(types.Log{Address: common.HexToAddress(log.Address), Topics: topics, Data: data, TxHash: common.HexToHash(tx.Hash), Index: uint(log.LogIndex), BlockNumber: uint64(tx.BlockNumber)})
			if err != nil {
				t.Fatal(err)
			}
			events = append(events, consumer.SwapEvent{PoolAddress: source.Pool, Sender: strings.ToLower(decoded.Sender.Hex()), IsBuy: decoded.IsBuy, AmountIn: decoded.AmountIn.String(), AmountOut: decoded.AmountOut.String(), Price: decoded.Price.String(), Fee: decoded.Fee.String(), BlockTimestamp: source.Blocks[strconv.FormatInt(tx.BlockNumber, 10)].Timestamp, TxHash: tx.Hash, LogIndex: int(log.LogIndex)})
		}
	}
	if len(events) != 2 || !events[0].IsBuy || events[1].IsBuy {
		t.Fatalf("recorded producer buy/sell: %+v", events)
	}
	return events
}

func (f *repairM1042Fixture) assertEmpty(t *testing.T) {
	t.Helper()
	for _, table := range []string{"pool_transactions", "volume_buckets", "bucket_traders", "price_snapshots", "pool_holders"} {
		var count int
		if err := f.db.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("partial durable effect in %s: %d rows", table, count)
		}
	}
}

func (f *repairM1042Fixture) assertBuy(t *testing.T, ev consumer.SwapEvent, holders bool) {
	t.Helper()
	var count, buys, sells int
	var volume string
	if err := f.db.db.QueryRow("SELECT COUNT(*) FROM pool_transactions").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("inbox rows=%d, want 1", count)
	}
	if err := f.db.db.QueryRow("SELECT volume_usdl,buy_count,sell_count FROM volume_buckets WHERE pool_address=? AND bucket_start=?", ev.PoolAddress, ev.BlockTimestamp/60*60).Scan(&volume, &buys, &sells); err != nil {
		t.Fatal(err)
	}
	if volume != ev.AmountIn || buys != 1 || sells != 0 {
		t.Fatalf("bucket volume=%s buys=%d sells=%d, want %s/1/0", volume, buys, sells, ev.AmountIn)
	}
	for _, table := range []string{"bucket_traders", "price_snapshots"} {
		if err := f.db.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("%s rows=%d, want 1", table, count)
		}
	}
	if holders {
		balance, ok, err := f.store.GetHolderBalance(context.Background(), ev.PoolAddress, ev.Sender)
		if err != nil || !ok || balance != ev.AmountOut {
			t.Fatalf("holder balance=%s exists=%v err=%v, want %s", balance, ok, err, ev.AmountOut)
		}
	}
}

func repairM1042CacheFailure(t *testing.T, err error) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), "consumer: cache pool stats: redis: client is closed") {
		t.Fatalf("want post-commit closed-cache failure, got %v", err)
	}
}

func TestRepairM_1042_RedeliveryAndRestart(t *testing.T) {
	ctx := context.Background()
	f := newRepairM1042Fixture(t)
	ev := repairM1042Events(t)[0]
	seedPoolStats(t, ctx, f.store, ev.PoolAddress, nil, "10000000000000")
	for delivery := 0; delivery < 3; delivery++ {
		repairM1042CacheFailure(t, f.swap.ProcessWebhookEvent(ctx, ev, f.holder))
		f.assertBuy(t, ev, true)
		row, err := f.store.GetPoolStats(ctx, ev.PoolAddress)
		if err != nil || row == nil || row.Price != ev.Price {
			t.Fatalf("committed price: %+v %v", row, err)
		}
		f.restart(t)
		f.assertBuy(t, ev, true)
	}
	// A distinct recorded sell applies exactly once to that recovered balance.
	sell := repairM1042Events(t)[1]
	for delivery := 0; delivery < 2; delivery++ {
		repairM1042CacheFailure(t, f.swap.ProcessWebhookEvent(ctx, sell, f.holder))
		balance, ok, err := f.store.GetHolderBalance(ctx, sell.PoolAddress, sell.Sender)
		if err != nil || !ok || balance != "4803406952560" {
			t.Fatalf("recovered buy/sell balance=%s exists=%v err=%v", balance, ok, err)
		}
		f.restart(t)
	}
}

func TestRepairM_1042_FailureAfterEachWrite(t *testing.T) {
	boundaries := []string{
		"INSERT INTO STATS.POOL_TRANSACTIONS", "INSERT INTO STATS.VOLUME_BUCKETS",
		"INSERT INTO STATS.BUCKET_TRADERS", "UPDATE STATS.POOL_STATS SET VOLUME_5M",
		"INSERT INTO STATS.PRICE_SNAPSHOTS", "UPDATE STATS.POOL_STATS SET PRICE",
		"INSERT INTO STATS.POOL_HOLDERS",
	}
	for _, boundary := range boundaries {
		t.Run(boundary, func(t *testing.T) {
			ctx := context.Background()
			f := newRepairM1042Fixture(t)
			ev := repairM1042Events(t)[0]
			seedPoolStats(t, ctx, f.store, ev.PoolAddress, nil, "10000000000000")
			f.db.drop.Store(boundary)
			if err := f.swap.ProcessWebhookEvent(ctx, ev, f.holder); err == nil {
				t.Fatal("interrupted transaction acknowledged")
			}
			if f.db.hits.Load() != 1 {
				t.Fatalf("write boundary was not reached: %s", boundary)
			}
			f.restart(t)
			f.assertEmpty(t)
			row, err := f.store.GetPoolStats(ctx, ev.PoolAddress)
			if err != nil || row == nil || row.Price != "10000000000000" {
				t.Fatalf("partial pool update survived: %+v %v", row, err)
			}
			repairM1042CacheFailure(t, f.swap.ProcessWebhookEvent(ctx, ev, f.holder))
			f.assertBuy(t, ev, true)
		})
	}
}

func TestRepairM_1042_CommitAcknowledgementLost(t *testing.T) {
	ctx := context.Background()
	f := newRepairM1042Fixture(t)
	ev := repairM1042Events(t)[0]
	seedPoolStats(t, ctx, f.store, ev.PoolAddress, nil, "10000000000000")
	f.db.drop.Store("COMMIT")
	if err := f.swap.ProcessWebhookEvent(ctx, ev, f.holder); err == nil || !strings.Contains(err.Error(), "consumer: commit swap:") {
		t.Fatalf("want ambiguous commit failure, got %v", err)
	}
	if f.db.hits.Load() != 1 {
		t.Fatal("commit boundary did not execute")
	}
	f.restart(t)
	f.assertBuy(t, ev, true)
	repairM1042CacheFailure(t, f.swap.ProcessWebhookEvent(ctx, ev, f.holder))
	f.assertBuy(t, ev, true)
}

func TestRepairM_1042_ProcessEventAtomicRecovery(t *testing.T) {
	ctx := context.Background()
	f := newRepairM1042Fixture(t)
	ev := repairM1042Events(t)[0]
	f.db.drop.Store("INSERT INTO STATS.VOLUME_BUCKETS")
	if err := f.swap.ProcessEvent(ctx, ev); err == nil {
		t.Fatal("interrupted standalone projection acknowledged")
	}
	if f.db.hits.Load() != 1 {
		t.Fatal("bucket write did not execute")
	}
	f.restart(t)
	f.assertEmpty(t)
	for delivery := 0; delivery < 3; delivery++ {
		if err := f.swap.ProcessEvent(ctx, ev); err != nil {
			t.Fatal(err)
		}
		f.assertBuy(t, ev, false)
		f.restart(t)
	}
}

func TestRepairM_1042_DevelopmentWebhookStatsRedelivery(t *testing.T) {
	f := newRepairM1042Fixture(t)
	// A recorded sell with no tracked holder needs no balance mutation. This
	// exercises the currently wired handler's stats path; buy/holder wiring is
	// explicitly requested in repair_m1042.kvx, outside this task's scope.
	ev := repairM1042Events(t)[1]
	const secret = "repair-m1042-recorded-development-webhook"
	body, err := json.Marshal(httpapi.WebhookBody{Events: []httpapi.WebhookEvent{{EventName: "Swap", BlockTimestamp: ev.BlockTimestamp, TxHash: ev.TxHash, LogIndex: ev.LogIndex, Args: map[string]any{"poolAddress": ev.PoolAddress, "sender": ev.Sender, "isBuy": ev.IsBuy, "amountIn": ev.AmountIn, "amountOut": ev.AmountOut, "price": ev.Price, "fee": ev.Fee}}}})
	if err != nil {
		t.Fatal(err)
	}
	for delivery := 0; delivery < 2; delivery++ {
		router := chi.NewRouter()
		httpapi.RegisterWebhook(router, httpapi.WebhookDeps{Swap: f.swap, Holder: f.holder, Logger: discardLogger(), Secret: secret})
		server := httptest.NewServer(router)
		req, err := http.NewRequest(http.MethodPost, server.URL+"/webhooks/events", bytes.NewReader(body))
		if err != nil {
			server.Close()
			t.Fatal(err)
		}
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		req.Header.Set("X-Sidiora-Timestamp", ts)
		req.Header.Set("X-Sidiora-Signature", auth.SignWebhook(secret, ts, string(body)))
		resp, err := server.Client().Do(req)
		if err != nil {
			server.Close()
			t.Fatal(err)
		}
		var result struct {
			Processed int `json:"processed"`
			Errors    int `json:"errors"`
		}
		err = json.NewDecoder(resp.Body).Decode(&result)
		_ = resp.Body.Close()
		server.Close()
		if err != nil || resp.StatusCode != http.StatusOK || result.Processed != 1 || result.Errors != 0 {
			t.Fatalf("webhook response: %+v status=%d err=%v", result, resp.StatusCode, err)
		}
		var volume string
		var buys, sells, count int
		if err := f.db.db.QueryRow("SELECT volume_usdl,buy_count,sell_count FROM volume_buckets").Scan(&volume, &buys, &sells); err != nil {
			t.Fatal(err)
		}
		if volume != ev.AmountOut || buys != 0 || sells != 1 {
			t.Fatalf("webhook replay multiplied stats: %s/%d/%d", volume, buys, sells)
		}
		if err := f.db.db.QueryRow("SELECT COUNT(*) FROM pool_transactions").Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("inbox rows=%d", count)
		}
		f.restart(t)
	}
}

func TestRepairM_1042_HolderFailureRollsBackInbox(t *testing.T) {
	ctx := context.Background()
	f := newRepairM1042Fixture(t)
	ev := repairM1042Events(t)[0]
	broken := ev
	broken.AmountOut = "invalid-token-amount"
	if err := f.swap.ProcessWebhookEvent(ctx, broken, f.holder); err == nil || !strings.Contains(err.Error(), "store: holder pct:") {
		t.Fatalf("want real holder arithmetic failure, got %v", err)
	}
	f.restart(t)
	f.assertEmpty(t)
	if err := f.swap.ProcessWebhookEvent(ctx, ev, f.holder); err != nil {
		t.Fatal(err)
	}
	f.assertBuy(t, ev, true)
}

func TestRepairM_1042_ConcurrentRedelivery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	f := newRepairM1042Fixture(t)
	ev := repairM1042Events(t)[0]
	const deliveries = 8
	results := make(chan error, deliveries)
	for i := 0; i < deliveries; i++ {
		go func() { results <- f.swap.ProcessWebhookEvent(ctx, ev, f.holder) }()
	}
	for i := 0; i < deliveries; i++ {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	f.restart(t)
	f.assertBuy(t, ev, true)
}
