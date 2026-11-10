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
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/engine"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/constants"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

// The retry fixture injects a real PostgreSQL serialization failure into the
// second swap's transaction; sequence increments survive the rollback.
type repairM1050State struct {
	pool *pgxpool.Pool
}

func repairM1050Postgres(t *testing.T) (*pgxpool.Pool, *repairM1050State) {
	t.Helper()
	_, pool := internaltest.NewPostgres(t)
	_, err := pool.Exec(context.Background(), `
CREATE SEQUENCE candles.retry_attempt;
CREATE FUNCTION candles.interrupt_second_swap() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.tx_hash='0xbeef' AND nextval('candles.retry_attempt')=1 THEN
        RAISE EXCEPTION 'second swap interrupted' USING ERRCODE='40001';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER interrupt_second_swap BEFORE INSERT ON candles.processed_swaps
FOR EACH ROW EXECUTE FUNCTION candles.interrupt_second_swap();`)
	if err != nil {
		t.Fatal(err)
	}
	return pool, &repairM1050State{pool: pool}
}

func (s *repairM1050State) assertProgress(t *testing.T, commits int, finished bool) {
	t.Helper()
	ctx := context.Background()
	var claims, sourceStates, pending int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM candles.processed_swaps`).Scan(&claims); err != nil {
		t.Fatal(err)
	}
	if claims != commits {
		t.Fatalf("commits=%d want=%d", claims, commits)
	}
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE key LIKE 'claim/%'),
count(*) FILTER (WHERE key LIKE 'outbox/%' AND convert_from(payload,'UTF8')::jsonb <> '[]'::jsonb)
FROM candles.development_state`).Scan(&sourceStates, &pending); err != nil {
		t.Fatal(err)
	}
	if sourceStates != commits || pending != 0 || (finished && claims != 2) {
		t.Fatalf("source states=%d commits=%d pending=%d finished=%t", sourceStates, claims, pending, finished)
	}
	for _, timeframe := range constants.TimeframeKeys {
		var trades int
		var volume string
		if err := s.pool.QueryRow(ctx, `SELECT coalesce(sum(trade_count),0), coalesce(sum(volume_usdl::numeric),0)::text
FROM candles.candles WHERE timeframe=$1`, timeframe).Scan(&trades, &volume); err != nil {
			t.Fatal(err)
		}
		if trades != commits || volume != strconv.Itoa(100*commits) {
			t.Fatalf("%s: trades=%d volume=%s commits=%d", timeframe, trades, volume, commits)
		}
	}
}

func repairM1050Builder(t *testing.T, pool *pgxpool.Pool) *engine.Builder {
	t.Helper()
	options, err := goredis.ParseURL(internaltest.NewRedisURL(t))
	if err != nil {
		t.Fatal(err)
	}
	rdb := goredis.NewClient(options)
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
	second.BlockNumber++
	second.Position = &engine.ChainOrder{Height: uint64(second.BlockNumber)}
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
	// A new builder must retain the first swap's durable claim while retrying
	// the rolled-back second swap against the same database and Redis.
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

// Relay failure must remain visible after the source fold commits; a healthy
// retry drains that exact durable outbox without folding the candle twice.
func TestRepairM_1050_BrokerFailurePreservesCommittedRetry(t *testing.T) {
	pool, state := repairM1050Postgres(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err = listener.Close(); err != nil {
		t.Fatal(err)
	}
	unavailable := goredis.NewClient(&goredis.Options{Addr: address, MaxRetries: -1, DialTimeout: time.Second, ReadTimeout: time.Second, WriteTimeout: time.Second})
	defer unavailable.Close()
	builder := engine.New(pool, unavailable, store.New(pool), slog.New(slog.NewTextHandler(io.Discard, nil)))
	first := httptest.NewServer(repairM1050Router(builder))
	body := swapBody(1704067260)
	status, headers, raw := repairM1050Send(t, first.URL, body)
	first.Close()
	var failure struct {
		OK        bool `json:"ok"`
		Processed int  `json:"processed"`
		Errors    int  `json:"errors"`
		Retryable bool `json:"retryable"`
	}
	if err = json.Unmarshal(raw, &failure); err != nil {
		t.Fatal(err)
	}
	if status != 503 || failure.OK || failure.Processed != 0 || failure.Errors != 1 || !failure.Retryable || headers.Get("Retry-After") != "1" {
		t.Fatalf("relay outage acknowledged: %d %s", status, raw)
	}
	var claims, pending int
	if err = pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM candles.processed_swaps),(SELECT count(*) FROM candles.development_state WHERE key LIKE 'outbox/%' AND convert_from(payload,'UTF8')::jsonb <> '[]'::jsonb)`).Scan(&claims, &pending); err != nil {
		t.Fatal(err)
	}
	if claims != 1 || pending != 1 {
		t.Fatalf("committed retry lost: claims=%d pending=%d", claims, pending)
	}
	recovered := httptest.NewServer(repairM1050Router(repairM1050Builder(t, pool)))
	defer recovered.Close()
	status, _, raw = repairM1050Send(t, recovered.URL, body)
	var success struct {
		OK        bool `json:"ok"`
		Processed int  `json:"processed"`
		Errors    int  `json:"errors"`
	}
	if err = json.Unmarshal(raw, &success); err != nil {
		t.Fatal(err)
	}
	if status != 200 || !success.OK || success.Processed != 1 || success.Errors != 0 {
		t.Fatalf("durable retry failed: %d %s", status, raw)
	}
	state.assertProgress(t, 1, false)
}
