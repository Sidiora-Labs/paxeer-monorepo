package app_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgproto3"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/app"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/config"
)

type repairM1049SQLStep struct {
	SQL    string `json:"sql"`
	Fields []struct {
		Name string `json:"name"`
		OID  uint32 `json:"oid"`
		Size int16  `json:"size"`
	} `json:"fields"`
	Rows [][]string `json:"rows"`
}

type repairM1049Transcript struct {
	mu      sync.Mutex
	steps   []repairM1049SQLStep
	next    int
	failure string
}

// Only the database peer is a transcript. App.New, migrations, pgx, the
// versioned history source, the HTTP router and socket handshake are real.
func repairM1049Application(t *testing.T, history bool, proxies string) (*httptest.Server, *repairM1049Transcript) {
	t.Helper()
	// migrations/00001_init.sql stores OHLC and volume as text. The development
	// builder persists raw integers: 18 price decimals and 6 USDL decimals;
	// the transcript carries those units through pgx into the history decoder.
	raw, err := os.ReadFile("testdata/repair_m1049/postgres.json")
	if err != nil {
		t.Fatal(err)
	}
	script := &repairM1049Transcript{}
	if err = json.Unmarshal(raw, &script.steps); err != nil {
		t.Fatal(err)
	}
	if !history {
		script.steps = script.steps[:3]
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	accepted := make(chan struct{})
	go func() {
		defer close(accepted)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			workers.Add(1)
			go func() { defer workers.Done(); script.serve(conn) }()
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		<-accepted
		workers.Wait()
		script.assertConsumed(t)
	})
	// Redis is unavailable, and neither retired route may try to contact it.
	redis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	redisDone := make(chan struct{})
	go func() {
		defer close(redisDone)
		for {
			conn, err := redis.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
			t.Error("retired surface contacted the unavailable Redis peer")
		}
	}()
	t.Cleanup(func() { _ = redis.Close(); <-redisDone })
	cfg := config.Config{}
	cfg.NodeEnv = "development"
	cfg.DatabaseURL = "postgres://fixture@" + listener.Addr().String() + "/candles?sslmode=disable&default_query_exec_mode=simple_protocol"
	cfg.RedisURL = "redis://" + redis.Addr().String()
	cfg.WebhookHMACSecret = strings.Repeat("s", 32)
	cfg.WSMaxConnections = 1
	cfg.WSMaxPerIP = 1
	cfg.TrustedProxyCIDRs = proxies
	cfg.CORSAllowedOrigins = "https://charts.example"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	a, err := app.New(ctx, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("App.New: %v", err)
	}
	t.Cleanup(a.Close)
	srv := httptest.NewServer(a.Router)
	t.Cleanup(srv.Close)
	return srv, script
}

func (s *repairM1049Transcript) serve(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
	backend := pgproto3.NewBackend(conn, conn)
	if _, err := backend.ReceiveStartupMessage(); err != nil {
		return
	}
	backend.Send(&pgproto3.AuthenticationOk{})
	for name, value := range map[string]string{"server_version": "16.0", "client_encoding": "UTF8", "standard_conforming_strings": "on"} {
		backend.Send(&pgproto3.ParameterStatus{Name: name, Value: value})
	}
	backend.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
	if err := backend.Flush(); err != nil {
		return
	}
	for {
		msg, err := backend.Receive()
		if err != nil {
			return
		}
		if _, ok := msg.(*pgproto3.Terminate); ok {
			return
		}
		query, ok := msg.(*pgproto3.Query)
		// pgconn.Ping sends a comment-only simple query. PostgreSQL answers
		// with EmptyQueryResponse; it neither reads rows nor advances the SQL
		// transcript. Keep every data query subject to the exact script below.
		if ok && query.String == "-- ping" {
			backend.Send(&pgproto3.EmptyQueryResponse{})
			backend.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
			if err := backend.Flush(); err != nil {
				return
			}
			continue
		}
		s.mu.Lock()
		if !ok || s.next >= len(s.steps) || strings.Join(strings.Fields(query.String), "") != strings.Join(strings.Fields(s.steps[s.next].SQL), "") {
			s.failure = fmt.Sprintf("unexpected database message at step %d: %#v", s.next, msg)
			s.mu.Unlock()
			backend.Send(&pgproto3.ErrorResponse{Severity: "ERROR", Code: "XX000", Message: "SQL outside committed transcript"})
			backend.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
			_ = backend.Flush()
			return
		}
		step := s.steps[s.next]
		s.next++
		s.mu.Unlock()
		fields := make([]pgproto3.FieldDescription, len(step.Fields))
		for i, field := range step.Fields {
			fields[i] = pgproto3.FieldDescription{Name: []byte(field.Name), DataTypeOID: field.OID, DataTypeSize: field.Size, TypeModifier: -1}
		}
		backend.Send(&pgproto3.RowDescription{Fields: fields})
		for _, row := range step.Rows {
			values := make([][]byte, len(row))
			for i, value := range row {
				values[i] = []byte(value)
			}
			backend.Send(&pgproto3.DataRow{Values: values})
		}
		backend.Send(&pgproto3.CommandComplete{CommandTag: []byte(fmt.Sprintf("SELECT %d", len(step.Rows)))})
		backend.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
		if err := backend.Flush(); err != nil {
			return
		}
	}
}

func (s *repairM1049Transcript) assertConsumed(t *testing.T) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failure != "" || s.next != len(s.steps) {
		t.Errorf("database transcript consumed %d/%d: %s", s.next, len(s.steps), s.failure)
	}
}

func repairM1049Get(t *testing.T, srv *httptest.Server, path string) (int, http.Header, map[string]any) {
	t.Helper()
	client := &http.Client{Timeout: 5 * time.Second}
	res, err := client.Get(srv.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var body map[string]any
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return res.StatusCode, res.Header, body
}

func TestRepairM_1049_HistoryRetirementAndVersionedMigration(t *testing.T) {
	srv, script := repairM1049Application(t, true, "")
	query := "?symbol=0x00000000000000000000000000000000000a0a01&resolution=60&from=1704067200&to=1704070800"
	for _, suffix := range []string{"", query, "?cursor=legacy&countback=5000", "?symbol=invalid"} {
		status, headers, body := repairM1049Get(t, srv, "/history"+suffix)
		if status != http.StatusGone || body["code"] != "LEGACY_HISTORY_RETIRED" || body["retryable"] != false || body["message"] != "Use /v2/udf/history with the versioned UDF contract" || headers.Get("Link") != `</v2/udf/history>; rel="successor-version"` || headers.Get("Cache-Control") != "no-store" {
			t.Fatalf("retired history: %d %v %v", status, headers, body)
		}
		if _, ok := body["s"]; ok {
			t.Fatal("retired history advertised a UDF result")
		}
	}
	status, _, body := repairM1049Get(t, srv, "/v2/udf/config")
	if status != http.StatusOK || body["supports_time"] != true {
		t.Fatalf("versioned config: %d %v", status, body)
	}
	status, headers, body := repairM1049Get(t, srv, "/v2/udf/history"+query)
	if status != http.StatusOK || body["s"] != "ok" || body["mode"] != "price" || body["algorithmVersion"] != "development-legacy-candles-v1" || body["snapshotId"] == "" || body["snapshotId"] == nil || headers.Get("X-Schema-Version") != "2" || headers.Get("ETag") == "" {
		t.Fatalf("versioned history: %d %v %v", status, headers, body)
	}
	for key, want := range map[string][]any{"t": {float64(1704067200)}, "o": {1.25}, "h": {2.5}, "l": {float64(1)}, "c": {float64(2)}, "v": {4.5}} {
		if !reflect.DeepEqual(body[key], want) {
			t.Fatalf("%s = %v, want %v", key, body[key], want)
		}
	}
	status, _, body = repairM1049Get(t, srv, "/v2/udf/history"+query+"&cursor=unversioned")
	if status != http.StatusUnprocessableEntity || body["code"] != "FIELD_INVALID" {
		t.Fatalf("invalid cursor: %d %v", status, body)
	}
	script.assertConsumed(t)
}

func TestRepairM_1049_AppNewRetiresLegacyStreamBeforeUpgrade(t *testing.T) {
	for _, proxies := range []string{"", "127.0.0.0/8,::1/128"} {
		t.Run("proxies="+proxies, func(t *testing.T) {
			srv, script := repairM1049Application(t, false, proxies)
			for _, ip := range []string{"203.0.113.1", "203.0.113.2", "203.0.113.1"} {
				assertLegacyWSRetired(t, srv, ip)
			}
			status, headers, body := repairM1049Get(t, srv, "/ws")
			if status != http.StatusGone || body["code"] != "LEGACY_STREAM_RETIRED" || body["retryable"] != false || body["message"] != "Use /v2/snapshots and /v2/stream for durable candle delivery" || headers.Get("Link") != `</v2/stream>; rel="successor-version"` {
				t.Fatalf("plain HTTP legacy stream: %d %v %v", status, headers, body)
			}
			script.assertConsumed(t)
		})
	}
}
