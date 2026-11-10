package app_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/app"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/config"
)

type repairM1049Database struct {
	pool    *pgxpool.Pool
	history bool
}

func repairM1049Application(t *testing.T, history bool, proxies string) (*httptest.Server, *repairM1049Database) {
	t.Helper()
	dsn, pool := internaltest.NewPostgres(t)
	database := &repairM1049Database{pool: pool, history: history}
	if history {
		if _, err := pool.Exec(context.Background(), `INSERT INTO candles.candles(pool_address,timeframe,candle_start,open,high,low,close,volume_usdl,last_trade_ts,sequence_num) VALUES('0x00000000000000000000000000000000000a0a01','1h',1704067200,'1250000000000000000','2500000000000000000','1000000000000000000','2000000000000000000','4500000',1704067200,1)`); err != nil {
			t.Fatal(err)
		}
	}
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
	cfg.DatabaseURL = dsn
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
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	server := httptest.NewServer(a.Router)
	t.Cleanup(server.Close)
	t.Cleanup(func() { database.assertConsumed(t) })
	return server, database
}

func (d *repairM1049Database) assertConsumed(t *testing.T) {
	t.Helper()
	var version, count int
	if err := d.pool.QueryRow(context.Background(), `SELECT max(version_id) FROM public.goose_db_version WHERE is_applied`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 3 {
		t.Fatalf("migration version=%d, want=3", version)
	}
	if err := d.pool.QueryRow(context.Background(), `SELECT count(*) FROM candles.candles`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	want := 0
	if d.history {
		want = 1
	}
	if count != want {
		t.Fatalf("history rows=%d want=%d", count, want)
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
