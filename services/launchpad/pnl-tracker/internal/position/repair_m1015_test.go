package position_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/constants"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/httpapi"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/pnlcache"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/position"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/store"
)

const (
	repairM1015User   = "0x1dae78b53c9f39930a2ff2b6b7aac12ec75fb154"
	repairM1015Pool   = "0xce20da50f92d21d8bee497910c782e9aa25b6263"
	repairM1015Token  = "0x0f92fdd087c96dd3413ef98646b94d0d986192fa"
	repairM1015Secret = "repair-m1015-loopback-only"
)

func TestRepairM_1015_ProducerWebhookRestart(t *testing.T) {
	repairM1015Restart(t)
}

func TestRepairM_1015_ProducerFixtureContract(t *testing.T) {
	var provenance struct {
		Producer, Transport, Revision string
		SHA256                        map[string]string `json:"sha256"`
	}
	if err := json.Unmarshal(repairM1015Fixture(t, "provenance.json"), &provenance); err != nil {
		t.Fatal(err)
	}
	if provenance.Revision != "caa8b41ee668274ee1e5c1df6691288455c46278" ||
		provenance.Producer != "core/indexer: decode.Decoder -> processor.Processor.ProcessBatch -> store.Store -> publisher.Publisher.PublishBatch" ||
		provenance.Transport != "HMAC-signed HTTP webhook, raw request bodies captured on loopback" {
		t.Fatalf("missing real legacy producer provenance: %+v", provenance)
	}
	for _, name := range []string{"buy.json", "sell.json"} {
		body := repairM1015Fixture(t, name)
		digest := sha256.Sum256(body)
		if got := hex.EncodeToString(digest[:]); got != provenance.SHA256[name] {
			t.Fatalf("captured producer bytes changed for %s: got %s, want %s", name, got, provenance.SHA256[name])
		}
		var envelope struct {
			Events []struct {
				EventName, TxHash, TraceID string
				LogIndex                   int
				Args                       map[string]json.RawMessage
			}
		}
		if err := json.Unmarshal(body, &envelope); err != nil {
			t.Fatal(err)
		}
		if len(envelope.Events) != 1 {
			t.Fatalf("%s must be one producer emission", name)
		}
		event := envelope.Events[0]
		if event.EventName != "Swap" || event.TraceID != fmt.Sprintf("%s:%d", event.TxHash, event.LogIndex) {
			t.Fatalf("invalid producer envelope: %+v", event)
		}
		for _, field := range []string{"amountIn", "amountOut", "fee", "price"} {
			var amount string
			if err := json.Unmarshal(event.Args[field], &amount); err != nil || amount == "" {
				t.Fatalf("producer %s lost decimal-string field %s: %s", name, field, event.Args[field])
			}
		}
		var side bool
		if err := json.Unmarshal(event.Args["isBuy"], &side); err != nil || side != (name == "buy.json") {
			t.Fatalf("producer %s direction = %s, %v", name, event.Args["isBuy"], err)
		}
	}
}

func TestRepairM_1015_WebhookAuthentication(t *testing.T) {
	h := repairM1015NewHarness(t)
	body := repairM1015Fixture(t, "buy.json")
	h.seedCache(t)
	for _, tc := range []struct {
		name      string
		body      []byte
		timestamp string
		signature string
	}{
		{name: "missing_signature", body: body},
		{name: "modified_serialized_bytes", body: append(append([]byte(nil), body...), ' '), timestamp: strconv.FormatInt(time.Now().Unix(), 10), signature: "original"},
		{name: "expired_signed_delivery", body: body, timestamp: "1", signature: "expired"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sig := tc.signature
			if sig != "" {
				sig = auth.SignWebhook(repairM1015Secret, tc.timestamp, string(body))
			}
			h.post(t, tc.body, tc.timestamp, sig, http.StatusUnauthorized)
			rows, err := h.store.ListTrades(h.ctx, repairM1015User, store.TradeFilter{})
			if err != nil || len(rows) != 0 {
				t.Fatalf("unauthenticated delivery changed trades: %+v, %v", rows, err)
			}
			pos, err := h.store.GetPosition(h.ctx, repairM1015User, repairM1015Pool)
			if err != nil || pos != nil {
				t.Fatalf("unauthenticated delivery changed position: %+v, %v", pos, err)
			}
			h.assertCache(t, "stale")
			if n := h.redisPeer.publishCount(); n != 0 {
				t.Fatalf("unauthenticated delivery published %d notifications", n)
			}
		})
	}
}

func repairM1015Fixture(t *testing.T, name string) []byte {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("fixture source location unavailable")
	}
	body, err := os.ReadFile(filepath.Join(filepath.Dir(file), "testdata", "repair_m1015", name))
	if err != nil {
		t.Fatal(err)
	}
	return body
}

type repairM1015Harness struct {
	ctx       context.Context
	db        *repairM1015Database
	redisPeer *repairM1015Redis
	pool      *pgxpool.Pool
	store     *store.Store
	redis     *goredis.Client
	sub       *goredis.PubSub
	server    *httptest.Server
}

func repairM1015NewHarness(t *testing.T) *repairM1015Harness {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	h := &repairM1015Harness{ctx: ctx, db: repairM1015NewDatabase(t), redisPeer: repairM1015NewRedis(t)}
	h.redis = goredis.NewClient(&goredis.Options{
		Addr: h.redisPeer.listener.Addr().String(), Protocol: 2,
		DisableIdentity: true, ContextTimeoutEnabled: true,
		ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second,
	})
	t.Cleanup(func() { _ = h.redis.Close() })
	h.sub = h.redis.Subscribe(ctx, constants.ChannelPnlUpdate)
	if _, err := h.sub.Receive(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.sub.Close() })
	h.open(t)
	t.Cleanup(func() { h.server.Close(); h.pool.Close() })
	return h
}

func (h *repairM1015Harness) open(t *testing.T) {
	t.Helper()
	cfg, err := pgxpool.ParseConfig("postgres://fixture@" + h.db.listener.Addr().String() + "/pnl?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	// Simple protocol is a supported pgx execution mode. The peer implements
	// storage and transactions only; Store.FoldTrade performs all PnL math.
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	cfg.MaxConns = 1
	h.pool, err = pgxpool.NewWithConfig(h.ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err = h.pool.Ping(h.ctx); err != nil {
		t.Fatal(err)
	}
	h.store = store.New(h.pool)
	router := chi.NewRouter()
	httpapi.RegisterWebhook(router, httpapi.WebhookDeps{
		Swap:   position.NewConsumer(h.store, h.redis, discardLogger()),
		Logger: discardLogger(), Secret: repairM1015Secret,
	})
	h.server = httptest.NewServer(router)
}

func (h *repairM1015Harness) reopen(t *testing.T) {
	t.Helper()
	h.server.Close()
	h.pool.Close()
	h.open(t)
}

func (h *repairM1015Harness) deliver(t *testing.T, body []byte) {
	t.Helper()
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	h.post(t, body, ts, auth.SignWebhook(repairM1015Secret, ts, string(body)), http.StatusOK)
}

func (h *repairM1015Harness) post(t *testing.T, body []byte, ts, signature string, status int) {
	t.Helper()
	req, err := http.NewRequestWithContext(h.ctx, http.MethodPost, h.server.URL+"/webhooks/events", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Sidiora-Timestamp", ts)
	req.Header.Set("X-Sidiora-Signature", signature)
	resp, err := h.server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != status {
		t.Fatalf("webhook status = %d, want %d: %s", resp.StatusCode, status, raw)
	}
	if status == http.StatusOK {
		var result struct {
			OK                bool `json:"ok"`
			Processed, Errors int
		}
		if err = json.Unmarshal(raw, &result); err != nil {
			t.Fatal(err)
		}
		if !result.OK || result.Processed != 1 || result.Errors != 0 {
			t.Fatalf("producer delivery did not fold successfully: %s", raw)
		}
	}
}

func (h *repairM1015Harness) seedCache(t *testing.T) {
	t.Helper()
	for _, key := range []string{pnlcache.KeyPositions(repairM1015User), pnlcache.KeyPortfolio(repairM1015User), pnlcache.KeyPositions("unrelated"), pnlcache.KeyCard("immutable")} {
		if err := h.redis.Set(h.ctx, key, "stale", 0).Err(); err != nil {
			t.Fatal(err)
		}
	}
}

func (h *repairM1015Harness) assertCache(t *testing.T, want string) {
	t.Helper()
	for _, key := range []string{pnlcache.KeyPositions(repairM1015User), pnlcache.KeyPortfolio(repairM1015User)} {
		got, err := h.redis.Get(h.ctx, key).Result()
		if want == "" {
			if !errors.Is(err, goredis.Nil) {
				t.Fatalf("cache %s survived invalidation: %q, %v", key, got, err)
			}
		} else if err != nil || got != want {
			t.Fatalf("cache %s = %q, %v; want %q", key, got, err, want)
		}
	}
	for _, key := range []string{pnlcache.KeyPositions("unrelated"), pnlcache.KeyCard("immutable")} {
		if got, err := h.redis.Get(h.ctx, key).Result(); err != nil || got != "stale" {
			t.Fatalf("unrelated cache %s changed: %q, %v", key, got, err)
		}
	}
}

func (h *repairM1015Harness) notification(t *testing.T, count int) {
	t.Helper()
	message, err := h.sub.ReceiveMessage(h.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if message.Channel != constants.ChannelPnlUpdate {
		t.Fatalf("notification channel = %s", message.Channel)
	}
	var payload map[string]string
	if err = json.Unmarshal([]byte(message.Payload), &payload); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(payload, map[string]string{"userAddress": repairM1015User}) {
		t.Fatalf("pnl:update payload = %s", message.Payload)
	}
	if n := h.redisPeer.publishCount(); n != count {
		t.Fatalf("published %d notifications, want %d", n, count)
	}
}

// The database peer stores the exact INSERT arguments emitted by the real
// pgx Store. It has no fold, unit conversion, cache or consumer implementation.
// It rejects every SQL statement outside the legacy store's declared surface.
type repairM1015Rows struct {
	trades    map[string][]*string
	positions map[string][]*string
}

func repairM1015EmptyRows() repairM1015Rows {
	return repairM1015Rows{trades: map[string][]*string{}, positions: map[string][]*string{}}
}

func (s repairM1015Rows) clone() repairM1015Rows {
	out := repairM1015EmptyRows()
	for k, row := range s.trades {
		out.trades[k] = append([]*string(nil), row...)
	}
	for k, row := range s.positions {
		out.positions[k] = append([]*string(nil), row...)
	}
	return out
}

type repairM1015Database struct {
	t           *testing.T
	listener    net.Listener
	mu          sync.Mutex
	rows        repairM1015Rows
	connections sync.WaitGroup
}

func repairM1015NewDatabase(t *testing.T) *repairM1015Database {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	db := &repairM1015Database{t: t, listener: listener, rows: repairM1015EmptyRows()}
	db.connections.Add(1)
	go func() {
		defer db.connections.Done()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			db.connections.Add(1)
			go func() { defer db.connections.Done(); defer conn.Close(); db.serve(conn) }()
		}
	}()
	t.Cleanup(func() { _ = listener.Close(); db.connections.Wait() })
	return db
}

func repairM1015Text(value string) *string { return &value }

func repairM1015Fields(names []string, types []uint32) []pgproto3.FieldDescription {
	fields := make([]pgproto3.FieldDescription, len(names))
	for i := range names {
		fields[i] = pgproto3.FieldDescription{Name: []byte(names[i]), DataTypeOID: types[i], DataTypeSize: -1, TypeModifier: -1}
	}
	return fields
}

func (db *repairM1015Database) serve(conn net.Conn) {
	_ = conn.SetDeadline(time.Now().Add(20 * time.Second))
	b := pgproto3.NewBackend(conn, conn)
	if _, err := b.ReceiveStartupMessage(); err != nil {
		db.t.Error(err)
		return
	}
	b.Send(&pgproto3.AuthenticationOk{})
	for name, value := range map[string]string{"server_version": "18.0", "client_encoding": "UTF8", "standard_conforming_strings": "on"} {
		b.Send(&pgproto3.ParameterStatus{Name: name, Value: value})
	}
	b.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
	if err := b.Flush(); err != nil {
		db.t.Error(err)
		return
	}
	var tx *repairM1015Rows
	defer func() {
		if tx != nil {
			db.mu.Unlock()
		}
	}()
	for {
		message, err := b.Receive()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				db.t.Error(err)
			}
			return
		}
		switch message := message.(type) {
		case *pgproto3.Terminate:
			return
		case *pgproto3.Query:
			sql := strings.Join(strings.Fields(message.String), " ")
			status := byte('I')
			var fields []pgproto3.FieldDescription
			var rows [][]*string
			var tag string
			var executeErr error
			switch sql {
			case "begin":
				if tx != nil {
					executeErr = errors.New("nested transaction")
				} else {
					db.mu.Lock()
					snapshot := db.rows.clone()
					tx = &snapshot
					tag = "BEGIN"
				}
			case "commit":
				if tx == nil {
					executeErr = errors.New("commit outside transaction")
				} else {
					db.rows = *tx
					tx = nil
					db.mu.Unlock()
					tag = "COMMIT"
				}
			case "rollback":
				if tx != nil {
					tx = nil
					db.mu.Unlock()
				}
				tag = "ROLLBACK"
			case "", "-- ping":
				b.Send(&pgproto3.EmptyQueryResponse{})
			default:
				if tx != nil {
					fields, rows, tag, executeErr = repairM1015Execute(tx, sql)
				} else {
					db.mu.Lock()
					fields, rows, tag, executeErr = repairM1015Execute(&db.rows, sql)
					db.mu.Unlock()
				}
			}
			if tx != nil {
				status = 'T'
			}
			if executeErr != nil {
				db.t.Errorf("database peer: %v", executeErr)
				b.Send(&pgproto3.ErrorResponse{Severity: "ERROR", Code: "XX000", Message: executeErr.Error()})
			} else {
				if fields != nil {
					b.Send(&pgproto3.RowDescription{Fields: fields})
					for _, row := range rows {
						values := make([][]byte, len(row))
						for i, value := range row {
							if value != nil {
								values[i] = []byte(*value)
							}
						}
						b.Send(&pgproto3.DataRow{Values: values})
					}
				}
				if tag != "" {
					b.Send(&pgproto3.CommandComplete{CommandTag: []byte(tag)})
				}
			}
			b.Send(&pgproto3.ReadyForQuery{TxStatus: status})
			if err = b.Flush(); err != nil {
				db.t.Error(err)
				return
			}
		default:
			db.t.Errorf("unsupported PostgreSQL message %T", message)
			return
		}
	}
}

// These column lists describe the real legacy tables, including nullable
// first_buy_ts. Reads return only arguments previously committed by Store.
var repairM1015PositionColumns = []string{"user_address", "pool_address", "token_address", "total_usdl_spent", "total_tokens_bought", "total_usdl_received", "total_tokens_sold", "avg_cost_basis", "current_holdings", "realized_pnl_usdl", "first_buy_ts", "last_trade_ts", "trade_count"}
var repairM1015PositionTypes = []uint32{25, 25, 25, 25, 25, 25, 25, 25, 25, 25, 20, 20, 23}
var repairM1015TradeColumns = []string{"id", "user_address", "pool_address", "token_address", "is_buy", "usdl_amount", "token_amount", "price", "fee", "block_number", "block_timestamp", "tx_hash"}
var repairM1015TradeTypes = []uint32{25, 25, 25, 25, 16, 25, 25, 25, 25, 20, 20, 25}

func repairM1015Execute(data *repairM1015Rows, sql string) ([]pgproto3.FieldDescription, [][]*string, string, error) {
	if strings.HasPrefix(sql, "SELECT pg_advisory_xact_lock(") {
		return nil, nil, "SELECT 1", nil
	}
	if strings.HasPrefix(sql, "INSERT INTO pnl.user_trades (") || strings.HasPrefix(sql, "INSERT INTO pnl.user_positions (") {
		start := strings.Index(sql, "VALUES (")
		if start < 0 {
			return nil, nil, "", fmt.Errorf("missing VALUES: %s", sql)
		}
		end := strings.Index(sql[start:], ")") + start
		values, err := repairM1015Literals(sql[start+len("VALUES (") : end])
		if err != nil {
			return nil, nil, "", err
		}
		columnStart := strings.IndexByte(sql, '(')
		columnEnd := strings.IndexByte(sql, ')')
		if columnStart < 0 || columnEnd < columnStart {
			return nil, nil, "", errors.New("invalid INSERT columns")
		}
		columns := strings.Split(strings.ReplaceAll(sql[columnStart+1:columnEnd], " ", ""), ",")
		if strings.HasPrefix(sql, "INSERT INTO pnl.user_trades (") {
			if len(values) != len(repairM1015TradeColumns) || !reflect.DeepEqual(columns, repairM1015TradeColumns) || !strings.HasSuffix(sql, "ON CONFLICT (id) DO NOTHING") {
				return nil, nil, "", fmt.Errorf("unexpected trade INSERT: %s", sql)
			}
			key := *values[0]
			if _, exists := data.trades[key]; exists {
				return nil, nil, "INSERT 0 0", nil
			}
			// pgx converts simple-protocol arguments through the PostgreSQL
			// text codec before quoting them: bool is exactly "t" or "f".
			if *values[4] != "t" && *values[4] != "f" {
				return nil, nil, "", errors.New("invalid stored trade direction")
			}
			data.trades[key] = values
		} else {
			if len(values) != len(repairM1015PositionColumns) || !reflect.DeepEqual(columns, repairM1015PositionColumns) || !strings.Contains(sql, "ON CONFLICT (user_address, pool_address) DO UPDATE SET") {
				return nil, nil, "", fmt.Errorf("unexpected position INSERT: %s", sql)
			}
			data.positions[*values[0]+"|"+*values[1]] = values
		}
		return nil, nil, "INSERT 0 1", nil
	}
	if !strings.HasPrefix(sql, "SELECT ") {
		return nil, nil, "", fmt.Errorf("unrecorded SQL: %s", sql)
	}
	from := strings.Index(sql, " FROM ")
	if from < 0 {
		return nil, nil, "", fmt.Errorf("unrecorded SELECT: %s", sql)
	}
	columns := strings.Split(strings.ReplaceAll(sql[len("SELECT "):from], " ", ""), ",")
	var names []string
	var types []uint32
	var candidates [][]*string
	switch {
	case strings.Contains(sql, " FROM indexer.pools WHERE "):
		names, types = []string{"pool_address", "token_address"}, []uint32{25, 25}
		candidates = [][]*string{{repairM1015Text(repairM1015Pool), repairM1015Text(repairM1015Token)}}
	case strings.Contains(sql, " FROM pnl.user_positions WHERE "):
		names, types = repairM1015PositionColumns, repairM1015PositionTypes
		for _, row := range data.positions {
			candidates = append(candidates, row)
		}
	case strings.Contains(sql, " FROM pnl.user_trades WHERE "):
		names, types = repairM1015TradeColumns, repairM1015TradeTypes
		for _, row := range data.trades {
			candidates = append(candidates, row)
		}
		sort.Slice(candidates, func(i, j int) bool { return *candidates[i][0] > *candidates[j][0] })
	default:
		return nil, nil, "", fmt.Errorf("unrecorded relation: %s", sql)
	}
	indexes := make([]int, len(columns))
	resultTypes := make([]uint32, len(columns))
	for i, column := range columns {
		indexes[i] = -1
		for j, name := range names {
			if column == name {
				indexes[i], resultTypes[i] = j, types[j]
			}
		}
		if indexes[i] < 0 {
			return nil, nil, "", fmt.Errorf("unrecorded column %s", column)
		}
	}
	where := sql[strings.Index(sql, " WHERE ")+len(" WHERE "):]
	for _, suffix := range []string{" ORDER BY ", " LIMIT "} {
		if i := strings.Index(where, suffix); i >= 0 {
			where = where[:i]
		}
	}
	predicates := strings.Split(where, " AND ")
	var result [][]*string
	for _, row := range candidates {
		matches := true
		for _, predicate := range predicates {
			parts := strings.SplitN(predicate, " = ", 2)
			if len(parts) != 2 {
				return nil, nil, "", fmt.Errorf("unrecorded predicate %s", predicate)
			}
			values, err := repairM1015Literals(parts[1])
			if err != nil || len(values) != 1 {
				return nil, nil, "", fmt.Errorf("invalid predicate %s: %v", predicate, err)
			}
			index := -1
			for j, name := range names {
				if parts[0] == name {
					index = j
				}
			}
			if index < 0 {
				return nil, nil, "", fmt.Errorf("unknown predicate %s", predicate)
			}
			if row[index] == nil || values[0] == nil || *row[index] != *values[0] {
				matches = false
			}
		}
		if matches {
			selected := make([]*string, len(indexes))
			for i, index := range indexes {
				selected[i] = row[index]
			}
			result = append(result, selected)
		}
	}
	return repairM1015Fields(columns, resultTypes), result, fmt.Sprintf("SELECT %d", len(result)), nil
}

func repairM1015Literals(sql string) ([]*string, error) {
	var result []*string
	for len(strings.TrimSpace(sql)) > 0 {
		sql = strings.TrimSpace(sql)
		if sql[0] == '\'' {
			var value strings.Builder
			i := 1
			closed := false
			for i < len(sql) {
				if sql[i] != '\'' {
					value.WriteByte(sql[i])
					i++
					continue
				}
				if i+1 < len(sql) && sql[i+1] == '\'' {
					value.WriteByte('\'')
					i += 2
					continue
				}
				i++
				closed = true
				break
			}
			if !closed {
				return nil, errors.New("unterminated SQL literal")
			}
			result = append(result, repairM1015Text(value.String()))
			sql = strings.TrimSpace(sql[i:])
		} else {
			i := strings.IndexByte(sql, ',')
			if i < 0 {
				i = len(sql)
			}
			value := strings.TrimSpace(sql[:i])
			if value == "null" {
				result = append(result, nil)
			} else {
				result = append(result, repairM1015Text(value))
			}
			sql = strings.TrimSpace(sql[i:])
		}
		if sql != "" {
			if sql[0] != ',' {
				return nil, fmt.Errorf("unexpected SQL literal suffix %s", sql)
			}
			sql = sql[1:]
		}
	}
	return result, nil
}

// RESP storage is a peer of the production Redis client. Notifications travel
// through the actual PublishJSON and PubSub client codecs over loopback TCP.
type repairM1015Redis struct {
	t           *testing.T
	listener    net.Listener
	mu          sync.Mutex
	keys        map[string]string
	subs        map[*repairM1015RESP]map[string]bool
	published   int
	connections sync.WaitGroup
}

type repairM1015RESP struct {
	sync.Mutex
	conn   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
}

type repairM1015Status string

func repairM1015NewRedis(t *testing.T) *repairM1015Redis {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	r := &repairM1015Redis{t: t, listener: listener, keys: map[string]string{}, subs: map[*repairM1015RESP]map[string]bool{}}
	r.connections.Add(1)
	go func() {
		defer r.connections.Done()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			r.connections.Add(1)
			go func() { defer r.connections.Done(); defer conn.Close(); r.serve(conn) }()
		}
	}()
	t.Cleanup(func() { _ = listener.Close(); r.connections.Wait() })
	return r
}

func (r *repairM1015Redis) publishCount() int { r.mu.Lock(); defer r.mu.Unlock(); return r.published }

func (p *repairM1015RESP) send(value any) error {
	p.Lock()
	defer p.Unlock()
	var write func(any) error
	write = func(value any) error {
		switch value := value.(type) {
		case repairM1015Status:
			_, err := fmt.Fprintf(p.writer, "+%s\r\n", value)
			return err
		case nil:
			_, err := fmt.Fprint(p.writer, "$-1\r\n")
			return err
		case int:
			_, err := fmt.Fprintf(p.writer, ":%d\r\n", value)
			return err
		case string:
			_, err := fmt.Fprintf(p.writer, "$%d\r\n%s\r\n", len(value), value)
			return err
		case []any:
			if _, err := fmt.Fprintf(p.writer, "*%d\r\n", len(value)); err != nil {
				return err
			}
			for _, entry := range value {
				if err := write(entry); err != nil {
					return err
				}
			}
			return nil
		default:
			return fmt.Errorf("unknown RESP value %T", value)
		}
	}
	if err := write(value); err != nil {
		return err
	}
	return p.writer.Flush()
}

func (p *repairM1015RESP) receive() ([]string, error) {
	line, err := p.reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(line, "*") {
		return nil, fmt.Errorf("unexpected RESP request %q", line)
	}
	n, err := strconv.Atoi(strings.TrimSpace(line[1:]))
	if err != nil || n <= 0 || n > 32 {
		return nil, errors.New("invalid RESP argument count")
	}
	args := make([]string, n)
	for i := range args {
		line, err = p.reader.ReadString('\n')
		if err != nil || !strings.HasPrefix(line, "$") {
			return nil, fmt.Errorf("invalid RESP bulk header: %q, %v", line, err)
		}
		length, err := strconv.Atoi(strings.TrimSpace(line[1:]))
		if err != nil || length < 0 || length > 65536 {
			return nil, errors.New("invalid RESP bulk length")
		}
		buf := make([]byte, length+2)
		if _, err = io.ReadFull(p.reader, buf); err != nil {
			return nil, err
		}
		if string(buf[length:]) != "\r\n" {
			return nil, errors.New("invalid RESP terminator")
		}
		args[i] = string(buf[:length])
	}
	return args, nil
}

func (r *repairM1015Redis) serve(conn net.Conn) {
	_ = conn.SetDeadline(time.Now().Add(20 * time.Second))
	p := &repairM1015RESP{conn: conn, reader: bufio.NewReader(conn), writer: bufio.NewWriter(conn)}
	defer func() { r.mu.Lock(); delete(r.subs, p); r.mu.Unlock() }()
	for {
		args, err := p.receive()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				r.t.Error(err)
			}
			return
		}
		var response any
		r.mu.Lock()
		switch strings.ToLower(args[0]) {
		case "hello":
			response = []any{"server", "redis", "version", "8.0.0", "proto", 2, "id", 1, "mode", "standalone", "role", "master", "modules", []any{}}
		case "client":
			if len(args) < 2 || strings.ToLower(args[1]) != "setinfo" {
				err = fmt.Errorf("unrecorded Redis command %v", args)
			} else {
				response = repairM1015Status("OK")
			}
		case "set":
			if len(args) != 3 {
				err = fmt.Errorf("unexpected SET arguments %v", args)
			} else {
				r.keys[args[1]] = args[2]
				response = repairM1015Status("OK")
			}
		case "get":
			if value, exists := r.keys[args[1]]; exists {
				response = value
			}
		case "del":
			n := 0
			for _, key := range args[1:] {
				if _, exists := r.keys[key]; exists {
					delete(r.keys, key)
					n++
				}
			}
			response = n
		case "subscribe":
			channels := r.subs[p]
			if channels == nil {
				channels = map[string]bool{}
				r.subs[p] = channels
			}
			if len(args) != 2 {
				err = errors.New("expected one subscription channel")
			} else {
				channels[args[1]] = true
				response = []any{"subscribe", args[1], len(channels)}
			}
		case "publish":
			if len(args) != 3 {
				err = errors.New("invalid PUBLISH")
			} else {
				r.published++
				n := 0
				for subscriber, channels := range r.subs {
					if channels[args[1]] {
						if e := subscriber.send([]any{"message", args[1], args[2]}); e != nil {
							err = e
						}
						n++
					}
				}
				response = n
			}
		default:
			err = fmt.Errorf("unrecorded Redis command %v", args)
		}
		r.mu.Unlock()
		if err != nil {
			r.t.Error(err)
			return
		}
		if err = p.send(response); err != nil {
			r.t.Error(err)
			return
		}
	}
}
