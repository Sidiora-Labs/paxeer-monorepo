package app_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	sharedlog "github.com/Sidiora-Technologies/KindleLaunch/shared/log"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/app"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
)

func testConfig(t *testing.T) config.Config {
	t.Helper()
	dsn, _ := internaltest.NewPostgres(t)
	redisURL := internaltest.NewRedisURL(t)

	var cfg config.Config
	cfg.DatabaseURL = dsn
	cfg.RedisURL = redisURL
	cfg.LogLevel = "error"
	cfg.Port = 0
	cfg.WebhookHMACSecret = strings.Repeat("s", 32)
	cfg.WSMaxConnections = 10
	cfg.WSMaxPerIP = 5
	cfg.CORSAllowedOrigins = "*"
	return cfg
}

func TestNewServesHealthAndCloses(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig(t)
	logger := sharedlog.New("candles-test", cfg.LogLevel)

	a, err := app.New(ctx, cfg, logger)
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}

	// Readiness probe should report healthy DB + Redis.
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()
	a.Router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("/health/ready = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}

	// The UDF surface is mounted.
	req = httptest.NewRequest(http.MethodGet, "/config", nil)
	rec = httptest.NewRecorder()
	a.Router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("/config = %d, want 200", rec.Code)
	}

	a.Close()
	// Close must be safe to call structurally; a second call must not panic.
	a.Close()
}

func TestNewBadDatabase(t *testing.T) {
	var cfg config.Config
	cfg.DatabaseURL = "postgres://bad:bad@127.0.0.1:1/none?sslmode=disable&connect_timeout=1"
	cfg.RedisURL = "redis://127.0.0.1:6379"
	cfg.LogLevel = "error"
	cfg.WebhookHMACSecret = strings.Repeat("s", 32)

	logger := sharedlog.New("candles-test", "error")
	if _, err := app.New(context.Background(), cfg, logger); err == nil {
		t.Fatal("app.New with unreachable DB should error")
	}
}

func TestNewRejectsMalformedRedisProvider(t *testing.T) {
	cfg := testConfig(t)
	cfg.RedisURL = "redis://invalid:port"
	opened, err := app.New(context.Background(), cfg, sharedlog.New("candles-test", "error"))
	if opened != nil || err == nil || !strings.Contains(err.Error(), "redis") {
		t.Fatalf("malformed Redis provider: app=%v err=%v", opened, err)
	}
}

// TestRunShutsDownOnContextCancel exercises the full Run() lifecycle: it loads
// config from the environment, starts the HTTP server (ephemeral port), the swap
// consumer and the gap-fill timers, then returns cleanly when the parent context
// is cancelled.
func TestRunShutsDownOnContextCancel(t *testing.T) {
	cfg := testConfig(t)
	roles := deployment.Current().Roles
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{
		"DATABASE_URL":            cfg.DatabaseURL,
		"REDIS_URL":               cfg.RedisURL,
		"REDIS_BULL_URL":          cfg.RedisURL,
		"RPC_URL":                 "https://rpc.example.com/rpc",
		"EVENT_EMITTER_ADDRESS":   roles["EventEmitter"].Address,
		"POOL_REGISTRY_ADDRESS":   roles["PoolRegistry"].Address,
		"ROUTER_ADDRESS":          roles["Router"].Address,
		"FACTORY_ADDRESS":         roles["SidioraFactory"].Address,
		"QUOTER_ADDRESS":          roles["Quoter"].Address,
		"PROTOCOL_CONFIG_ADDRESS": roles["ProtocolConfig"].Address,
		"FEE_ACCUMULATOR_ADDRESS": roles["FeeAccumulator"].Address,
		"SIDIORA_NFT_ADDRESS":     roles["SidioraNFT"].Address,
		"FEES_ROUTER_ADDRESS":     roles["FeesRouter"].Address,
		"POOL_BEACON_ADDRESS":     roles["PoolBeacon"].Address,
		"WEBHOOK_HMAC_SECRET":     strings.Repeat("s", 32),
		"LOG_LEVEL":               "error",
		"PORT":                    strconv.Itoa(port),
		"BIND_ADDRESS":            "127.0.0.1",
		"NODE_ENV":                "development",
	}
	for k, v := range env {
		t.Setenv(k, v)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx) }()

	// Give the service a moment to come up, then trigger graceful shutdown.
	time.Sleep(500 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error on shutdown: %v", err)
		}
	case <-time.After(40 * time.Second):
		t.Fatal("Run did not return after context cancel")
	}
	t.Setenv("REDIS_URL", "redis://127.0.0.1:1")
	t.Setenv("REDIS_BULL_URL", "redis://127.0.0.1:1/1")
	unavailable, stop := context.WithTimeout(context.Background(), 3*time.Second)
	defer stop()
	if err := app.Run(unavailable); err == nil || !strings.Contains(err.Error(), "start consumer") {
		t.Fatalf("consumer subscription admission: %v", err)
	}
	t.Setenv("WEBHOOK_HMAC_SECRET", "short")
	if err := app.Run(context.Background()); err == nil || !strings.Contains(err.Error(), "at least 32") {
		t.Fatalf("runtime configuration admission: %v", err)
	}
}
