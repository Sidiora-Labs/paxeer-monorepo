package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/engine"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

// The peer is a committed PostgreSQL protocol transcript, not a replacement for
// the handler, builder, store or pgx. Unexpected SQL fails closed.
type repairM1050SQLStep struct {
	Contains []string `json:"contains"`
	Tag      string   `json:"tag"`
	Status   string   `json:"status"`
	Error    string   `json:"error"`
}
type repairM1050SQLTranscript struct {
	mu      sync.Mutex
	steps   []repairM1050SQLStep
	next    int
	commits int
	failure string
}

func repairM1050Postgres(t *testing.T) (*pgxpool.Pool, *repairM1050SQLTranscript) {
	t.Helper()
	raw, err := os.ReadFile("testdata/repair_m1050/postgres.json")
	if err != nil {
		t.Fatal(err)
	}
	script := &repairM1050SQLTranscript{}
	if err = json.Unmarshal(raw, &script.steps); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	var workers sync.WaitGroup
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			workers.Add(1)
			go func() { defer workers.Done(); script.serve(conn) }()
		}
	}()
	cfg, err := pgxpool.ParseConfig("postgres://fixture@" + listener.Addr().String() + "/candles?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 3
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	cfg.ConnConfig.ConnectTimeout = time.Second
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Close(); _ = listener.Close(); workers.Wait() })
	return pool, script
}
func (s *repairM1050SQLTranscript) serve(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(20 * time.Second))
	backend := pgproto3.NewBackend(conn, conn)
	if _, err := backend.ReceiveStartupMessage(); err != nil {
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
	for {
		msg, err := backend.Receive()
		if err != nil {
			return
		}
		if _, ok := msg.(*pgproto3.Terminate); ok {
			return
		}
		query, ok := msg.(*pgproto3.Query)
		if !ok {
			return
		}
		s.mu.Lock()
		if s.next >= len(s.steps) {
			s.failure = "unexpected SQL after transcript: " + query.String
			s.mu.Unlock()
			return
		}
		step := s.steps[s.next]
		for _, part := range step.Contains {
			if !strings.Contains(query.String, part) {
				s.failure = fmt.Sprintf("step %d requires %q, received %s", s.next, part, query.String)
				s.mu.Unlock()
				return
			}
		}
		s.next++
		if step.Tag == "COMMIT" && step.Error == "" {
			s.commits++
		}
		s.mu.Unlock()
		if step.Error != "" {
			backend.Send(&pgproto3.ErrorResponse{Severity: "ERROR", Code: step.Error, Message: "recorded serialization failure"})
		} else {
			backend.Send(&pgproto3.CommandComplete{CommandTag: []byte(step.Tag)})
		}
		backend.Send(&pgproto3.ReadyForQuery{TxStatus: step.Status[0]})
		if err := backend.Flush(); err != nil {
			return
		}
	}
}
func (s *repairM1050SQLTranscript) assertProgress(t *testing.T, commits int, finished bool) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failure != "" {
		t.Fatal(s.failure)
	}
	if s.commits != commits || (finished && s.next != len(s.steps)) {
		t.Fatalf("commits=%d want=%d; consumed=%d total=%d", s.commits, commits, s.next, len(s.steps))
	}
}
func repairM1050Builder(t *testing.T, pool *pgxpool.Pool) *engine.Builder {
	t.Helper()
	// Redis is an explicitly unavailable loopback peer. Durable SQL commits, not
	// best-effort pub/sub, determine the webhook acknowledgement.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			c, e := listener.Accept()
			if e != nil {
				return
			}
			_ = c.Close()
		}
	}()
	t.Cleanup(func() { _ = listener.Close() })
	rdb := goredis.NewClient(&goredis.Options{Addr: listener.Addr().String(), Protocol: 2, MaxRetries: -1, DialTimeout: time.Second, ReadTimeout: time.Second, WriteTimeout: time.Second})
	t.Cleanup(func() { _ = rdb.Close() })
	return engine.New(pool, rdb, store.New(pool), slog.New(slog.NewTextHandler(io.Discard, nil)))
}
func repairM1050Router(builder *engine.Builder) *chi.Mux {
	router := chi.NewRouter()
	RegisterWebhook(router, WebhookDeps{Builder: builder, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Secret: whSecret})
	return router
}
func repairM1050Send(t *testing.T, endpoint string, body []byte) (int, http.Header, []byte) {
	t.Helper()
	req := signedRequest(body)
	req.URL.Scheme = "http"
	req.URL.Host = strings.TrimPrefix(endpoint, "http://")
	req.RequestURI = ""
	client := &http.Client{Timeout: 15 * time.Second}
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, response.Header, raw
}
func TestRepairM_1050_PartialFailureRetry(t *testing.T) {
	pool, script := repairM1050Postgres(t)
	var batch WebhookBody
	if err := json.Unmarshal(swapBody(1704067260), &batch); err != nil {
		t.Fatal(err)
	}
	second := batch.Events[0]
	second.TxHash = "0xbeef"
	second.BlockTimestamp += 3600
	batch.Events = append(batch.Events, second)
	body, err := json.Marshal(batch)
	if err != nil {
		t.Fatal(err)
	}
	first := httptest.NewServer(repairM1050Router(repairM1050Builder(t, pool)))
	status, headers, raw := repairM1050Send(t, first.URL, body)
	first.Close()
	var result struct {
		OK        bool `json:"ok"`
		Processed int  `json:"processed"`
		Errors    int  `json:"errors"`
		Retryable bool `json:"retryable"`
	}
	if err = json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if status != 503 || result.OK || !result.Retryable || result.Processed != 1 || result.Errors != 1 || headers.Get("Retry-After") != "1" {
		t.Fatalf("partial delivery acknowledged: %d %s", status, raw)
	}
	script.assertProgress(t, 1, false)
	// A new builder clears the hot dedup cache. The transcript requires the first
	// swap's durable claim to refuse a second fold, while the rolled-back swap folds.
	reopened := httptest.NewServer(repairM1050Router(repairM1050Builder(t, pool)))
	defer reopened.Close()
	status, _, raw = repairM1050Send(t, reopened.URL, body)
	if err = json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if status != 200 || !result.OK || result.Retryable || result.Processed != 2 || result.Errors != 0 {
		t.Fatalf("retry: %d %s", status, raw)
	}
	script.assertProgress(t, 2, true)
	status, _, raw = repairM1050Send(t, reopened.URL, body)
	if status != 200 {
		t.Fatalf("duplicate: %d %s", status, raw)
	}
	script.assertProgress(t, 2, true)
}

type repairM1050CountedBody struct {
	io.Reader
	read   int
	closed bool
}

func (b *repairM1050CountedBody) Read(p []byte) (int, error) {
	n, e := b.Reader.Read(p)
	b.read += n
	return n, e
}
func (b *repairM1050CountedBody) Close() error { b.closed = true; return nil }
func TestRepairM_1050_RequestBounds(t *testing.T) {
	router := repairM1050Router(nil)
	tooMany := []byte(`{"events":[` + strings.Repeat(`{"eventName":"MarketCreated"},`, MaxWebhookEvents) + `{"eventName":"MarketCreated"}]}`)
	atCount := []byte(`{"events":[` + strings.Repeat(`{"eventName":"MarketCreated"},`, MaxWebhookEvents-1) + `{"eventName":"MarketCreated"}]}`)
	byteLimit := append([]byte(`{"events":[]}`), bytes.Repeat([]byte(" "), MaxWebhookBodyBytes-len(`{"events":[]}`))...)
	for _, tc := range []struct {
		name string
		body []byte
		want int
	}{
		{"byte_limit", byteLimit, 200},
		{"byte_over", append(append([]byte(nil), byteLimit...), ' '), 413},
		{"events_limit", atCount, 200},
		{"events_over", tooMany, 413},
	} {
		for _, signed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/signed=%t", tc.name, signed), func(t *testing.T) {
				req := signedRequest(tc.body)
				if !signed {
					req.Header.Set("X-Sidiora-Signature", "sha256=deadbeef")
				}
				body := &repairM1050CountedBody{Reader: bytes.NewReader(tc.body)}
				req.Body = body
				req.ContentLength = -1 // Also bound chunked requests and dishonest lengths.
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)
				want := tc.want
				if !signed && want == 200 {
					want = 401
				}
				if rec.Code != want {
					t.Fatalf("status=%d want=%d: %s", rec.Code, want, rec.Body.String())
				}
				if body.read > MaxWebhookBodyBytes+1 || !body.closed {
					t.Fatalf("read=%d closed=%t", body.read, body.closed)
				}
				if want == 413 && rec.Body.String() != `{"statusCode":413,"error":"Payload Too Large","message":"webhook request exceeds limits"}`+"\n" {
					t.Fatalf("unstable oversized refusal: %s", rec.Body.String())
				}
			})
		}
	}
}
func TestRepairM_1050_PermanentValidation(t *testing.T) {
	var batch WebhookBody
	if err := json.Unmarshal(swapBody(1704067260), &batch); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"timestamp", "txHash", "logIndex", "poolAddress", "sender", "isBuy", "amountIn", "amountOut", "price"} {
		t.Run(field, func(t *testing.T) {
			raw, _ := json.Marshal(batch)
			var invalid WebhookBody
			if err := json.Unmarshal(raw, &invalid); err != nil {
				t.Fatal(err)
			}
			event := &invalid.Events[0]
			switch field {
			case "timestamp":
				event.BlockTimestamp = 1
			case "txHash":
				event.TxHash = ""
			case "logIndex":
				event.LogIndex = -1
			default:
				event.Args[field] = nil
			}
			raw, err := json.Marshal(invalid)
			if err != nil {
				t.Fatal(err)
			}
			rec := httptest.NewRecorder()
			repairM1050Router(nil).ServeHTTP(rec, signedRequest(raw))
			var result struct {
				OK        bool
				Retryable bool
				Errors    int
				Processed int
			}
			if err = json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if rec.Code != 422 || result.OK || result.Retryable || result.Errors != 1 || result.Processed != 0 {
				t.Fatalf("validation: %d %s", rec.Code, rec.Body.String())
			}
		})
	}
	for _, body := range []string{`{"events":null}`, `{"events":[],"events":[]}`, `{"events":[]} {"events":[]}`, `{"other":true}`, `[]`, `{"events":[`} {
		rec := httptest.NewRecorder()
		repairM1050Router(nil).ServeHTTP(rec, signedRequest([]byte(body)))
		if rec.Code != 400 {
			t.Fatalf("invalid envelope %s: %d", body, rec.Code)
		}
	}
}

func repairM1050CandleFrames(t *testing.T) [][]byte {
	t.Helper()
	manifestBytes, err := os.ReadFile("../engine/testdata/repair74/manifests.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifests []store.ArchiveManifest
	if err = json.Unmarshal(manifestBytes, &manifests); err != nil || len(manifests) != 1 {
		t.Fatal("archive manifest", err)
	}
	archive, err := os.ReadFile("../engine/testdata/repair74/segment.json")
	if err != nil {
		t.Fatal(err)
	}
	fills, err := store.ValidateArchive(archive, manifests[0])
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string]accounting.FillV1{}
	var bar *engine.QualifiedCandle
	frames := [][]byte{}
	for i, fill := range fills[:2] {
		sources[fill.SourceFillID] = fill
		next, err := engine.FoldQualified(bar, fill, "1m", i == 0)
		if err != nil {
			t.Fatal(err)
		}
		bar = &next
		state, err := engine.DescribeSeries(next, engine.SeriesCheckpoint{Height: 103, Timestamp: 1577923300}, time.Unix(1577923400, 0))
		if err != nil {
			t.Fatal(err)
		}
		state.DisplayRevision = uint64(i + 1)
		canonical, err := engine.CanonicalCandle(state, sources)
		if err != nil {
			t.Fatal(err)
		}
		// The real canonical producer supplies every monetary/provenance field. The
		// gateway peer owns only the delivery envelope and opaque cursor protocol.
		frame := interfaces.CandleEvent{EventId: fmt.Sprintf("candle:%s:%d", fill.SourceFillID, i+1), Topic: "market.candles." + fill.PoolAddress + ".1m", Sequence: interfaces.UInt63(fmt.Sprint(i + 1)), Revision: canonical.Revision, OccurredAt: "2020-01-02T00:03:20.000Z", Payload: canonical.Candle}
		raw, err := json.Marshal(frame)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = interfaces.DecodeCandleEvent(raw); err != nil {
			t.Fatal("producer event", err)
		}
		frames = append(frames, raw)
	}
	return frames
}
func TestRepairM_1050_ProducerToSocketRestartResync(t *testing.T) {
	testAdmittedCandleApplicationStream(t, repairM1050CandleFrames(t), filepath.Join(t.TempDir(), "retained-frames.json"))
}
