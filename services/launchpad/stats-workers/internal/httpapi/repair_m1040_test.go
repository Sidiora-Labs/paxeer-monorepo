package httpapi_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/consumer"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/httpapi"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/store"
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
)

// The file-backed table peer speaks PostgreSQL to the production Store and
// consumers. Only the peer is a fixture: transaction snapshots, immutable-row
// conflicts and commit persistence are independent of the repaired handler.
type repairM1040Bucket struct {
	Pool, Volume string
	Start        int64
	Buys, Sells  int
}
type repairM1040Tables struct {
	Transactions map[string]store.TransactionRow
	Pools        map[string]store.PoolStatsRow
	Holders      map[string]store.HolderRow
	Buckets      map[string]repairM1040Bucket
	Traders      map[string]bool
	Snapshots    map[string]string
}
type repairM1040DB struct {
	t           *testing.T
	mu          sync.Mutex
	Tables      repairM1040Tables
	File        string
	Listener    net.Listener
	Connections sync.WaitGroup
	Accepted    chan struct{}
	FailSell    bool
	DropCommit  bool
	CommitDrops int
}

func (d *repairM1040DB) start() {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		d.t.Fatal(err)
	}
	d.Listener = listener
	d.Accepted = make(chan struct{})
	go func() {
		defer close(d.Accepted)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			d.Connections.Add(1)
			go d.connection(conn)
		}
	}()
}
func (d *repairM1040DB) stop() {
	d.Listener.Close()
	<-d.Accepted
	d.Connections.Wait()
}
func (d *repairM1040DB) save() {
	raw, err := json.Marshal(d.Tables)
	if err != nil {
		d.t.Fatal(err)
	}
	if err := os.WriteFile(d.File, raw, 0600); err != nil {
		d.t.Error(err)
	}
}
func (d *repairM1040DB) openPool() *pgxpool.Pool {
	cfg, err := pgxpool.ParseConfig("postgres://fixture@" + d.Listener.Addr().String() + "/fixture?sslmode=disable")
	if err != nil {
		d.t.Fatal(err)
	}
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	cfg.ConnConfig.ConnectTimeout = 2 * time.Second
	cfg.MaxConns = 4
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		d.t.Fatal(err)
	}
	return pool
}

func (d *repairM1040DB) connection(connection net.Conn) {
	defer d.Connections.Done()
	defer connection.Close()
	backend := pgproto3.NewBackend(connection, connection)
	if _, err := backend.ReceiveStartupMessage(); err != nil {
		return
	}
	backend.Send(&pgproto3.AuthenticationOk{})
	backend.Send(&pgproto3.ParameterStatus{Name: "server_version", Value: "16.0"})
	backend.Send(&pgproto3.ParameterStatus{Name: "client_encoding", Value: "UTF8"})
	backend.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
	if backend.Flush() != nil {
		return
	}
	statements := map[string]*pgproto3.Parse{}
	var result repair1039Result
	locked := false
	var rollback repairM1040Tables
	defer func() {
		if locked {
			d.Tables = rollback
			d.mu.Unlock()
		}
	}()
	execute := func(sql string, args []string) repair1039Result {
		lower := strings.ToLower(strings.TrimSpace(sql))
		if strings.HasPrefix(lower, "begin") {
			d.mu.Lock()
			locked = true
			raw, _ := json.Marshal(d.Tables)
			rollback = repairM1040Tables{}
			_ = json.Unmarshal(raw, &rollback)
			return repair1039Result{tag: "BEGIN"}
		}
		if lower == "commit" {
			d.save()
			locked = false
			d.mu.Unlock()
			if d.DropCommit {
				d.DropCommit = false
				d.CommitDrops++
				return repair1039Result{err: "fixture: commit acknowledgement lost"}
			}
			return repair1039Result{tag: "COMMIT"}
		}
		if lower == "rollback" {
			d.Tables = rollback
			locked = false
			d.mu.Unlock()
			return repair1039Result{tag: "ROLLBACK"}
		}
		if !locked {
			d.mu.Lock()
			defer d.mu.Unlock()
		}
		result := d.query(sql, args)
		if !locked && result.err == "" {
			d.save()
		}
		return result
	}
	for {
		message, err := backend.Receive()
		if err != nil {
			return
		}
		switch m := message.(type) {
		case *pgproto3.Parse:
			copy := *m
			statements[m.Name] = &copy
			backend.Send(&pgproto3.ParseComplete{})
		case *pgproto3.Bind:
			statement := statements[m.PreparedStatement]
			if statement == nil {
				return
			}
			args := make([]string, len(m.Parameters))
			for i, raw := range m.Parameters {
				format := int16(0)
				if len(m.ParameterFormatCodes) == 1 {
					format = m.ParameterFormatCodes[0]
				} else if len(m.ParameterFormatCodes) > i {
					format = m.ParameterFormatCodes[i]
				}
				oid := uint32(25)
				if len(statement.ParameterOIDs) > i {
					oid = statement.ParameterOIDs[i]
				}
				var value any
				if err := pgtype.NewMap().Scan(oid, format, raw, &value); err != nil {
					d.t.Errorf("decode parameter %d OID %d: %v", i, oid, err)
					return
				}
				if b, ok := value.([]byte); ok {
					args[i] = string(b)
				} else if value != nil {
					args[i] = fmt.Sprint(value)
				}
			}
			result = execute(statement.Query, args)
			if result.err == "fixture: commit acknowledgement lost" {
				return
			}
			backend.Send(&pgproto3.BindComplete{})
		case *pgproto3.Describe:
			if fields := repair1039Fields(result); len(fields) > 0 {
				backend.Send(&pgproto3.RowDescription{Fields: fields})
			} else {
				backend.Send(&pgproto3.NoData{})
			}
		case *pgproto3.Execute:
			if result.err != "" {
				backend.Send(&pgproto3.ErrorResponse{Severity: "ERROR", Code: "XX000", Message: result.err})
			} else {
				for _, row := range result.rows {
					values := make([][]byte, len(row))
					for i, v := range row {
						values[i] = repair1039Wire(v)
					}
					backend.Send(&pgproto3.DataRow{Values: values})
				}
				tag := result.tag
				if tag == "" {
					tag = "INSERT 0 1"
				}
				backend.Send(&pgproto3.CommandComplete{CommandTag: []byte(tag)})
			}
		case *pgproto3.Query:
			result = execute(m.String, nil)
			if result.err == "fixture: commit acknowledgement lost" {
				return
			}
			if result.err != "" {
				backend.Send(&pgproto3.ErrorResponse{Severity: "ERROR", Code: "XX000", Message: result.err})
			} else {
				if fields := repair1039Fields(result); len(fields) > 0 {
					backend.Send(&pgproto3.RowDescription{Fields: fields})
				}
				for _, row := range result.rows {
					values := make([][]byte, len(row))
					for i, v := range row {
						values[i] = repair1039Wire(v)
					}
					backend.Send(&pgproto3.DataRow{Values: values})
				}
				tag := result.tag
				if tag == "" {
					tag = "SELECT 1"
				}
				backend.Send(&pgproto3.CommandComplete{CommandTag: []byte(tag)})
			}
			backend.Send(&pgproto3.ReadyForQuery{TxStatus: func() byte {
				if locked {
					return 'T'
				}
				return 'I'
			}()})
		case *pgproto3.Sync:
			status := byte('I')
			if locked {
				status = 'T'
			}
			backend.Send(&pgproto3.ReadyForQuery{TxStatus: status})
		case *pgproto3.Terminate:
			return
		default:
			d.t.Errorf("unsupported PostgreSQL transcript message %T", message)
			return
		}
		if backend.Flush() != nil {
			return
		}
	}
}

// Recorded chain receipts are decoded by the production ABI binding, then
// serialized with the production webhook wire type and signed by shared auth.
func repairM1040Events(t *testing.T) []httpapi.WebhookEvent {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "projectionstats", "testdata", "real_pool_125.json"))
	if err != nil {
		t.Fatal(err)
	}
	var source struct {
		Pool, EventEmitter string
		Blocks             map[string]struct{ Timestamp int64 }
		Transactions       []struct {
			Hash        string
			BlockNumber int64
			Logs        []struct {
				Address, Data string
				Topics        []string
				LogIndex      uint
			}
		}
	}
	if err := json.Unmarshal(raw, &source); err != nil {
		t.Fatal(err)
	}
	filterer, err := bindings.NewEventEmitterFilterer(common.HexToAddress(source.EventEmitter), nil)
	if err != nil {
		t.Fatal(err)
	}
	var events []httpapi.WebhookEvent
	for _, transaction := range source.Transactions {
		for _, log := range transaction.Logs {
			if log.Address != source.EventEmitter || len(log.Topics) == 0 || log.Topics[0] != "0xa234ab85728295548823d8a26077ecc77aa5815f9ed8f4c5e6fd390ac851d4cc" {
				continue
			}
			topics := make([]common.Hash, len(log.Topics))
			for i, topic := range log.Topics {
				topics[i] = common.HexToHash(topic)
			}
			data, err := hex.DecodeString(strings.TrimPrefix(log.Data, "0x"))
			if err != nil {
				t.Fatal(err)
			}
			event, err := filterer.ParseSwap(types.Log{Address: common.HexToAddress(log.Address), Topics: topics, Data: data, TxHash: common.HexToHash(transaction.Hash), Index: log.LogIndex, BlockNumber: uint64(transaction.BlockNumber)})
			if err != nil {
				t.Fatal(err)
			}
			events = append(events, httpapi.WebhookEvent{EventName: "Swap", BlockNumber: transaction.BlockNumber, BlockTimestamp: source.Blocks[strconv.FormatInt(transaction.BlockNumber, 10)].Timestamp, TxHash: transaction.Hash, LogIndex: int(log.LogIndex), Args: map[string]any{
				"poolAddress": source.Pool, "sender": strings.ToLower(event.Sender.Hex()), "isBuy": event.IsBuy, "amountIn": event.AmountIn.String(), "amountOut": event.AmountOut.String(), "price": event.Price.String(), "fee": event.Fee.String(),
			}})
		}
	}
	if len(events) != 2 || events[0].Args["isBuy"] != true || events[1].Args["isBuy"] != false {
		t.Fatalf("recorded buy/sell: %+v", events)
	}
	return events
}

func (f *repairM1040Fixture) router(t *testing.T) http.Handler {
	t.Helper()
	market, err := consumer.NewMarketConsumer(f.store, quietLogger())
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	httpapi.RegisterWebhook(router, httpapi.WebhookDeps{Swap: f.swap, Holder: f.holder, Market: market, State: consumer.NewStateConsumer(f.store, quietLogger()), Multihop: consumer.NewMultihopConsumer(f.store, quietLogger()), Logger: quietLogger(), Secret: testSecret})
	return router
}

type repairM1040Reply struct {
	OK                bool
	Processed, Errors int
}

func repairM1040Post(t *testing.T, client *http.Client, url string, body []byte) (int, http.Header, repairM1040Reply) {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, url+"/webhooks/events", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range hmacHeaders(testSecret, body) {
		request.Header.Set(key, value)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var reply repairM1040Reply
	if err := json.NewDecoder(response.Body).Decode(&reply); err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, response.Header, reply
}

func (f *repairM1040Fixture) assertState(t *testing.T, events []httpapi.WebhookEvent, count int, balance, volume string, buys, sells int) {
	t.Helper()
	f.db.mu.Lock()
	snapshotRaw, err := json.Marshal(f.db.Tables)
	f.db.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	var snapshot repairM1040Tables
	if err := json.Unmarshal(snapshotRaw, &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Transactions) != count {
		t.Fatalf("durable inbox=%d want %d", len(snapshot.Transactions), count)
	}

	gotBalance, ok, err := f.store.GetHolderBalance(context.Background(), events[0].Args["poolAddress"].(string), events[0].Args["sender"].(string))
	if err != nil || !ok || gotBalance != balance {
		t.Fatalf("holder=%s exists=%v err=%v want %s", gotBalance, ok, err, balance)
	}
	if len(snapshot.Buckets) != 1 {
		t.Fatalf("bucket count=%d want 1", len(snapshot.Buckets))
	}
	for _, bucket := range snapshot.Buckets {
		if bucket.Volume != volume || bucket.Buys != buys || bucket.Sells != sells {
			t.Fatalf("volume/counts=%s/%d/%d want %s/%d/%d", bucket.Volume, bucket.Buys, bucket.Sells, volume, buys, sells)
		}
	}
	for table, count := range map[string]int{"price_snapshots": len(snapshot.Snapshots), "bucket_traders": len(snapshot.Traders), "pool_holders": len(snapshot.Holders)} {
		if count != 1 {
			t.Fatalf("%s rows=%d want 1", table, count)
		}
	}

}

func TestRepairM_1040_WebhookFailedAck(t *testing.T) {
	f := newRepairM1040Fixture(t)
	events := repairM1040Events(t)
	body := mustJSON(t, httpapi.WebhookBody{Events: events})
	// The committed database transcript refuses the sell's holder write after
	// its inbox/volume/snapshot writes. The production outer transaction must roll back.
	f.db.FailSell = true

	server := httptest.NewServer(f.router(t))
	defer func() { server.Close() }()
	for attempt := 0; attempt < 2; attempt++ {
		status, headers, reply := repairM1040Post(t, server.Client(), server.URL, body)
		if status != http.StatusServiceUnavailable || reply.OK || reply.Processed != 1 || reply.Errors != 1 || headers.Get("Retry-After") != "1" {
			t.Fatalf("failed batch acknowledged: status=%d reply=%+v headers=%v", status, reply, headers)
		}
		f.assertState(t, events, 1, "9606813905120", "100000000", 1, 0)
		// The sender retains the exact original payload on non-2xx and retries it
		// after stopping the listener, consumers, connection pool and database.
		server.Close()
		f.restart(t)
		f.assertState(t, events, 1, "9606813905120", "100000000", 1, 0)
		server = httptest.NewServer(f.router(t))
	}
	f.db.FailSell = false
	for attempt := 0; attempt < 2; attempt++ {
		status, _, reply := repairM1040Post(t, server.Client(), server.URL, body)
		if status != http.StatusOK || !reply.OK || reply.Processed != 2 || reply.Errors != 0 {
			t.Fatalf("retry did not recover: status=%d reply=%+v", status, reply)
		}
		f.assertState(t, events, 2, "4803406952560", "147278912", 1, 1)
		server.Close()
		f.restart(t)
		f.assertState(t, events, 2, "4803406952560", "147278912", 1, 1)
		server = httptest.NewServer(f.router(t))
	}
}

func TestRepairM_1040_CommitAckLost(t *testing.T) {
	f := newRepairM1040Fixture(t)
	events := repairM1040Events(t)[:1]
	body := mustJSON(t, httpapi.WebhookBody{Events: events})
	f.db.DropCommit = true
	server := httptest.NewServer(f.router(t))
	status, _, reply := repairM1040Post(t, server.Client(), server.URL, body)
	server.Close()
	if status != http.StatusServiceUnavailable || reply.OK || reply.Errors != 1 || f.db.CommitDrops != 1 {
		t.Fatalf("ambiguous commit acknowledged: %d %+v", status, reply)
	}
	f.restart(t)
	f.assertState(t, events, 1, "9606813905120", "100000000", 1, 0)
	server = httptest.NewServer(f.router(t))
	defer server.Close()
	status, _, reply = repairM1040Post(t, server.Client(), server.URL, body)
	if status != http.StatusOK || !reply.OK || reply.Processed != 1 || reply.Errors != 0 {
		t.Fatalf("committed retry: %d %+v", status, reply)
	}
	f.assertState(t, events, 1, "9606813905120", "100000000", 1, 0)
}

func TestRepairM_1040_PostCommitCacheFailure(t *testing.T) {
	f := newRepairM1040Fixture(t)
	events := repairM1040Events(t)[:1]
	market, err := consumer.NewMarketConsumer(f.store, quietLogger())
	if err != nil {
		t.Fatal(err)
	}
	if err := market.ProcessEvent(context.Background(), consumer.MarketEvent{Pool: events[0].Args["poolAddress"].(string), Token: "recorded-token"}); err != nil {
		t.Fatal(err)
	}
	body := mustJSON(t, httpapi.WebhookBody{Events: events})
	for attempt := 0; attempt < 2; attempt++ {
		server := httptest.NewServer(f.router(t))
		status, _, reply := repairM1040Post(t, server.Client(), server.URL, body)
		server.Close()
		if status != http.StatusServiceUnavailable || reply.OK || reply.Errors != 1 {
			t.Fatalf("cache failure acknowledged: %d %+v", status, reply)
		}
		f.assertState(t, events, 1, "9606813905120", "100000000", 1, 0)
		f.restart(t)
	}
}

// This body peer records exactly how much input the registered handler reads.
// It can stream past the cap without allocating the advertised payload.
type repairM1040Body struct {
	remaining, read int64
	reads           int
}

func (b *repairM1040Body) Read(p []byte) (int, error) {
	b.reads++
	if b.remaining == 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > b.remaining {
		p = p[:b.remaining]
	}
	for i := range p {
		p[i] = ' '
	}
	b.remaining -= int64(len(p))
	b.read += int64(len(p))
	return len(p), nil
}
func (*repairM1040Body) Close() error { return nil }

func repairM1040BoundRouter() http.Handler {
	router := chi.NewRouter()
	httpapi.RegisterWebhook(router, httpapi.WebhookDeps{Logger: quietLogger(), Secret: testSecret})
	return router
}

func TestRepairM_1040_UnsignedStreamingBody(t *testing.T) {
	for _, missing := range []string{"both", "signature", "timestamp"} {
		t.Run(missing, func(t *testing.T) {
			reader, writer := io.Pipe()
			defer reader.Close()
			defer writer.Close()
			req := httptest.NewRequest(http.MethodPost, "/webhooks/events", reader)
			req.ContentLength = -1
			if missing == "signature" {
				req.Header.Set("X-Sidiora-Timestamp", strconv.FormatInt(time.Now().Unix(), 10))
			}
			if missing == "timestamp" {
				req.Header.Set("X-Hub-Signature-256", "sha256=unused")
			}
			rec := httptest.NewRecorder()
			done := make(chan struct{})
			go func() { repairM1040BoundRouter().ServeHTTP(rec, req); close(done) }()
			select {
			case <-done:
			case <-time.After(time.Second):
				reader.Close()
				writer.Close()
				<-done
				t.Fatal("unsigned streaming body was read before rejecting missing headers")
			}
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("missing %s: status=%d", missing, rec.Code)
			}
		})
	}
}

func TestRepairM_1040_WebhookBodyBound(t *testing.T) {
	for _, length := range []int64{httpapi.WebhookMaxBodyBytes + 1, -1, 1} {
		t.Run(strconv.FormatInt(length, 10), func(t *testing.T) {
			body := &repairM1040Body{remaining: 10 * httpapi.WebhookMaxBodyBytes}
			req := httptest.NewRequest(http.MethodPost, "/webhooks/events", body)
			req.ContentLength = length
			req.Header.Set("X-Sidiora-Timestamp", strconv.FormatInt(time.Now().Unix(), 10))
			req.Header.Set("X-Sidiora-Signature", "sha256=not-needed-for-oversize")
			rec := httptest.NewRecorder()
			repairM1040BoundRouter().ServeHTTP(rec, req)
			if rec.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("oversize status=%d body=%s", rec.Code, rec.Body.String())
			}
			want := int64(httpapi.WebhookMaxBodyBytes + 1)
			if length > httpapi.WebhookMaxBodyBytes {
				want = 0
			}
			if body.read != want {
				t.Fatalf("body read=%d want %d", body.read, want)
			}
		})
	}
	// Exact maximum size, noncanonical whitespace, Unicode and the legacy header
	// alias all retain the original timestamp + '.' + raw-body HMAC encoding.
	prefix := []byte("{\"note\":\"雪\",\"events\":[]}")
	body := append(prefix, bytes.Repeat([]byte(" "), httpapi.WebhookMaxBodyBytes-len(prefix))...)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(testSecret))
	mac.Write([]byte(ts + "."))
	mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if sig != auth.SignWebhook(testSecret, ts, string(body)) {
		t.Fatal("legitimate HMAC encoding changed")
	}
	for _, header := range []string{"X-Sidiora-Signature", "X-Hub-Signature-256"} {
		rec := serve(t, repairM1040BoundRouter(), http.MethodPost, "/webhooks/events", body, map[string]string{"X-Sidiora-Timestamp": ts, header: sig})
		if rec.Code != http.StatusOK {
			t.Fatalf("exact maximum signed %s: %d %s", header, rec.Code, rec.Body.String())
		}
	}
	tampered := append([]byte(nil), body...)
	tampered[len(tampered)-1] = '\t'
	rec := serve(t, repairM1040BoundRouter(), http.MethodPost, "/webhooks/events", tampered, map[string]string{"X-Sidiora-Timestamp": ts, "X-Sidiora-Signature": sig})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("raw-byte tampering: status=%d", rec.Code)
	}
}

func TestRepairM_1040_WebhookEventBound(t *testing.T) {
	f := newRepairM1040Fixture(t)
	events := make([]httpapi.WebhookEvent, httpapi.WebhookMaxEvents)
	for i := range events {
		events[i] = httpapi.WebhookEvent{EventName: "Unknown", TxHash: fmt.Sprintf("event-%d", i)}
	}
	body := mustJSON(t, httpapi.WebhookBody{Events: events})
	rec := serve(t, f.router(t), http.MethodPost, "/webhooks/events", body, hmacHeaders(testSecret, body))
	var reply repairM1040Reply
	decode(t, rec, &reply)
	if rec.Code != http.StatusOK || !reply.OK || reply.Processed != httpapi.WebhookMaxEvents || reply.Errors != 0 {
		t.Fatalf("maximum event count: %d %+v", rec.Code, reply)
	}
	events[0] = httpapi.WebhookEvent{EventName: "MarketCreated", Args: map[string]any{"pool": "must-not-apply", "token": "token"}}
	events = append(events, httpapi.WebhookEvent{EventName: "Unknown"})
	body = mustJSON(t, httpapi.WebhookBody{Events: events})
	rec = serve(t, f.router(t), http.MethodPost, "/webhooks/events", body, hmacHeaders(testSecret, body))
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("event overflow: %d %s", rec.Code, rec.Body.String())
	}
	f.db.mu.Lock()
	count := len(f.db.Tables.Pools)
	f.db.mu.Unlock()
	if count != 0 {
		t.Fatalf("partially applied rejected batch: %d pools", count)
	}

	for _, invalid := range []string{`{}`, `{"events":null}`, `{"events":{}}`, `{"events":[]} {}`, `{"events":[],"events":[]}`, `{"events":[{"eventName":"Unknown"}]`, `{"events":["invalid"]}`} {
		body := []byte(invalid)
		rec := serve(t, f.router(t), http.MethodPost, "/webhooks/events", body, hmacHeaders(testSecret, body))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("invalid batch %s: %d", invalid, rec.Code)
		}
	}
}

type repairM1040Fixture struct {
	db     *repairM1040DB
	pool   *pgxpool.Pool
	store  *store.Store
	swap   *consumer.SwapConsumer
	holder *consumer.HolderTracker
	redis  *goredis.Client
}

func newRepairM1040Fixture(t *testing.T) *repairM1040Fixture {
	t.Helper()
	db := &repairM1040DB{t: t, File: filepath.Join(t.TempDir(), "webhook-tables.json"), Tables: repairM1040Tables{
		Transactions: map[string]store.TransactionRow{}, Pools: map[string]store.PoolStatsRow{}, Holders: map[string]store.HolderRow{}, Buckets: map[string]repairM1040Bucket{}, Traders: map[string]bool{}, Snapshots: map[string]string{},
	}}
	db.save()
	f := &repairM1040Fixture{db: db}
	f.open()
	t.Cleanup(f.close)
	return f
}
func (f *repairM1040Fixture) open() {
	f.db.start()
	f.pool = f.db.openPool()
	f.store = store.New(f.pool)
	f.redis = goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:1"})
	if err := f.redis.Close(); err != nil {
		f.db.t.Fatal(err)
	}
	f.swap = consumer.NewSwapConsumer(f.store, f.redis, quietLogger())
	f.holder = consumer.NewHolderTracker(f.store, f.redis, quietLogger(), time.Hour)
}
func (f *repairM1040Fixture) close() {
	f.holder.Close()
	f.pool.Close()
	f.db.stop()
}
func (f *repairM1040Fixture) restart(t *testing.T) {
	t.Helper()
	f.close()
	raw, err := os.ReadFile(f.db.File)
	if err != nil {
		t.Fatal(err)
	}
	f.db.Tables = repairM1040Tables{}
	if err := json.Unmarshal(raw, &f.db.Tables); err != nil {
		t.Fatal(err)
	}
	f.open()
}

// Every supported statement has a committed table operation. Unrecognized SQL
// fails explicitly; the fixture never returns fabricated generic successes.
func (d *repairM1040DB) query(sql string, a []string) repair1039Result {
	q := strings.Join(strings.Fields(sql), " ")
	if q == "-- ping" {
		return repair1039Result{tag: "SELECT 0"}
	}
	upper := strings.ToUpper(q)
	if strings.HasPrefix(upper, "SAVEPOINT ") || strings.HasPrefix(upper, "RELEASE SAVEPOINT ") {
		return repair1039Result{tag: strings.Fields(upper)[0]}
	}
	if strings.Contains(q, "pg_advisory_xact_lock") {
		return repair1039Rows([]any{int64(1)})
	}
	if strings.HasPrefix(q, "INSERT INTO stats.pool_transactions") {
		if _, ok := d.Tables.Transactions[a[0]]; ok {
			return repair1039Result{tag: "INSERT 0 0"}
		}
		d.Tables.Transactions[a[0]] = store.TransactionRow{ID: a[0], PoolAddress: a[1], Sender: a[2], IsBuy: repair1039Bool(a[3]), AmountIn: a[4], AmountOut: a[5], Price: a[6], Fee: a[7], BlockTimestamp: repair1039Int(a[8]), TxHash: a[9]}
		return repair1039Result{tag: "INSERT 0 1"}
	}
	if strings.HasPrefix(q, "SELECT volume_usdl, buy_count, sell_count FROM stats.volume_buckets") {
		var rows [][]any
		for _, bucket := range d.Tables.Buckets {
			if bucket.Pool != a[0] {
				continue
			}
			timestamp := repair1039Int(a[1])
			if strings.Contains(q, "bucket_start >=") && bucket.Start < timestamp {
				continue
			}
			if strings.Contains(q, "bucket_start =") && bucket.Start != timestamp {
				continue
			}
			rows = append(rows, []any{bucket.Volume, int64(bucket.Buys), int64(bucket.Sells)})
		}
		return repair1039Rows(rows...)
	}
	if strings.HasPrefix(q, "INSERT INTO stats.volume_buckets") {
		key := a[0] + "|" + a[1]
		if _, ok := d.Tables.Buckets[key]; ok {
			return repair1039Result{tag: "INSERT 0 0"}
		}
		d.Tables.Buckets[key] = repairM1040Bucket{Pool: a[0], Start: repair1039Int(a[1]), Volume: a[2], Buys: int(repair1039Int(a[3])), Sells: int(repair1039Int(a[4]))}
		return repair1039Result{tag: "INSERT 0 1"}
	}
	if strings.HasPrefix(q, "UPDATE stats.volume_buckets SET") {
		key := a[0] + "|" + a[1]
		bucket := d.Tables.Buckets[key]
		bucket.Volume = a[2]
		bucket.Buys = int(repair1039Int(a[3]))
		bucket.Sells = int(repair1039Int(a[4]))
		d.Tables.Buckets[key] = bucket
		return repair1039Result{tag: "UPDATE 1"}
	}
	if strings.HasPrefix(q, "INSERT INTO stats.bucket_traders") {
		d.Tables.Traders[a[0]+"|"+a[1]+"|"+a[2]] = true
		return repair1039Result{tag: "INSERT 0 1"}
	}
	if strings.HasPrefix(q, "SELECT COUNT(DISTINCT trader)") {
		traders := map[string]bool{}
		for key := range d.Tables.Traders {
			parts := strings.Split(key, "|")
			if parts[0] == a[0] && repair1039Int(parts[1]) >= repair1039Int(a[1]) {
				traders[parts[2]] = true
			}
		}
		return repair1039Rows([]any{int64(len(traders))})
	}
	if strings.HasPrefix(q, "UPDATE stats.pool_stats SET volume_5m") {
		row, ok := d.Tables.Pools[a[0]]
		if !ok {
			return repair1039Result{tag: "UPDATE 0"}
		}
		row.Volume5m = a[1]
		row.Volume1h = a[2]
		row.Volume24h = a[3]
		row.BuyCount24h = int(repair1039Int(a[4]))
		row.SellCount24h = int(repair1039Int(a[5]))
		row.UniqueTraders24h = int(repair1039Int(a[6]))
		row.UpdatedAt = repair1039Int(a[7])
		d.Tables.Pools[a[0]] = row
		return repair1039Result{tag: "UPDATE 1"}
	}
	if strings.HasPrefix(q, "INSERT INTO stats.price_snapshots") {
		d.Tables.Snapshots[a[0]+"|"+a[1]] = a[2]
		return repair1039Result{tag: "INSERT 0 1"}
	}
	if strings.HasPrefix(q, "SELECT price FROM stats.price_snapshots") {
		var selected int64
		var price string
		for key, p := range d.Tables.Snapshots {
			parts := strings.Split(key, "|")
			ts := repair1039Int(parts[1])
			if parts[0] == a[0] && ts >= repair1039Int(a[1]) && (price == "" || ts < selected) {
				selected = ts
				price = p
			}
		}
		if price == "" {
			return repair1039Rows()
		}
		return repair1039Rows([]any{price})
	}
	if strings.HasPrefix(q, "SELECT high_24h, low_24h FROM stats.pool_stats") {
		row, ok := d.Tables.Pools[a[0]]
		if !ok {
			return repair1039Rows()
		}
		return repair1039Rows([]any{row.High24h, row.Low24h})
	}
	if strings.HasPrefix(q, "UPDATE stats.pool_stats SET price =") {
		row, ok := d.Tables.Pools[a[0]]
		if !ok {
			return repair1039Result{tag: "UPDATE 0"}
		}
		row.Price = a[1]
		row.MarketCap = a[2]
		row.High24h = a[3]
		row.Low24h = a[4]
		row.UpdatedAt = repair1039Int(a[5])
		d.Tables.Pools[a[0]] = row
		return repair1039Result{tag: "UPDATE 1"}
	}
	if strings.HasPrefix(q, "SELECT balance FROM stats.pool_holders") {
		row, ok := d.Tables.Holders[a[0]+"|"+a[1]]
		if !ok {
			return repair1039Rows()
		}
		return repair1039Rows([]any{row.Balance})
	}
	if strings.HasPrefix(q, "INSERT INTO stats.pool_holders") {
		if d.FailSell && a[2] == "4803406952560" {
			return repair1039Result{err: "holder temporarily unavailable"}
		}
		d.Tables.Holders[a[0]+"|"+a[1]] = store.HolderRow{PoolAddress: a[0], HolderAddress: a[1], Balance: a[2], PctOfSupply: a[3], LastUpdated: repair1039Int(a[4])}
		return repair1039Result{tag: "INSERT 0 1"}
	}
	if strings.HasPrefix(q, "INSERT INTO stats.pool_stats") {
		if _, ok := d.Tables.Pools[a[0]]; ok {
			return repair1039Result{tag: "INSERT 0 0"}
		}
		row := store.PoolStatsRow{PoolAddress: a[0], TokenAddress: a[1], Price: a[3], MarketCap: a[4], High24h: a[5], Low24h: a[6], CreatedAt: repair1039Int(a[7]), UpdatedAt: repair1039Int(a[8])}
		if a[2] != "" {
			creator := a[2]
			row.CreatorAddress = &creator
		}
		d.Tables.Pools[a[0]] = row
		return repair1039Result{tag: "INSERT 0 1"}
	}
	if strings.HasPrefix(q, "SELECT pool_address, token_address, price,") && strings.Contains(q, "FROM stats.pool_stats WHERE pool_address") {
		row, ok := d.Tables.Pools[a[0]]
		if !ok {
			return repair1039Rows()
		}
		var creator any
		if row.CreatorAddress != nil {
			creator = *row.CreatorAddress
		}
		return repair1039Rows([]any{row.PoolAddress, row.TokenAddress, row.Price, "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", row.High24h, row.Low24h, row.Volume24h, row.Volume1h, row.Volume5m, row.MarketCap, int64(row.BuyCount24h), int64(row.SellCount24h), int64(row.UniqueTraders24h), int64(row.HolderCount), "0", "0", int64(row.RiskRating), nil, creator, row.CreatedAt, row.UpdatedAt})
	}
	return repair1039Result{err: "unsupported webhook peer SQL: " + q}
}
