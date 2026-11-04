package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"

	sharedlog "github.com/Sidiora-Technologies/KindleLaunch/shared/log"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/app"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/config"
)

func newDevApp(t *testing.T, mutate func(*config.Config)) *httptest.Server {
	t.Helper()
	cfg := testConfig(t)
	cfg.NodeEnv = "development"
	if mutate != nil {
		mutate(&cfg)
	}
	a, err := app.New(context.Background(), cfg, sharedlog.New("candles-test", "error"))
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	t.Cleanup(a.Close)
	srv := httptest.NewServer(a.Router)
	t.Cleanup(srv.Close)
	return srv
}

// Outside production the versioned UDF surface is mounted over the
// development candle store; zero pools give valid no_data documents.
func TestDevelopmentRegistersVersionedUDF(t *testing.T) {
	srv := newDevApp(t, nil)
	get := func(path string) (int, map[string]any) {
		t.Helper()
		res, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var body map[string]any
		if err = json.NewDecoder(res.Body).Decode(&body); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		return res.StatusCode, body
	}
	pool := "0x00000000000000000000000000000000000a0a01"
	for _, mode := range []string{"price", "total_supply_valuation"} {
		status, body := get("/v2/udf/history?symbol=" + pool + "&resolution=60&from=1760000000&to=1760086400&mode=" + mode)
		if status != 200 || body["s"] != "no_data" || body["mode"] != mode || body["valuationBasis"] != "total_supply_valuation" || body["snapshotId"] == "" {
			t.Fatalf("%s: %d %v", mode, status, body)
		}
	}
	if status, body := get("/v2/udf/config"); status != 200 || body["supports_time"] != true {
		t.Fatalf("config %d %v", status, body)
	}
	res, err := http.Get(srv.URL + "/v2/udf/time")
	if err != nil || res.StatusCode != 200 {
		t.Fatalf("time %v %v", err, res)
	}
	_ = res.Body.Close()
	// No RPC_URL in this configuration: symbols is an explicit dependency
	// failure, never a 404 route miss.
	if status, body := get("/v2/udf/symbols?symbol=" + pool); status != 503 || body["code"] != "DEPENDENCY_UNAVAILABLE" {
		t.Fatalf("symbols %d %v", status, body)
	}
}

func assertLegacyWSRetired(t *testing.T, srv *httptest.Server, realIP string) {
	t.Helper()
	header := http.Header{}
	header.Set("X-Real-IP", realIP)
	header.Set("Origin", "https://untrusted.example")
	conn, response, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/ws", header)
	if conn != nil {
		_ = conn.Close()
		t.Fatal("retired route accepted a WebSocket")
	}
	if response == nil {
		t.Fatalf("retirement response missing: %v", err)
	}
	defer response.Body.Close()
	var body struct {
		Code      string `json:"code"`
		Retryable bool   `json:"retryable"`
	}
	if decodeErr := json.NewDecoder(response.Body).Decode(&body); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if !errors.Is(err, websocket.ErrBadHandshake) || response.StatusCode != http.StatusGone || body.Code != "LEGACY_STREAM_RETIRED" || body.Retryable || response.Header.Get("Link") != `</v2/stream>; rel="successor-version"` {
		t.Fatalf("retirement: status=%d body=%+v err=%v", response.StatusCode, body, err)
	}
}

// Retirement precedes upgrade regardless of proxy attribution or connection limits.
func TestDevelopmentTrustedProxyPerIPLimit(t *testing.T) {
	for _, proxies := range []string{"127.0.0.0/8,::1/128", ""} {
		srv := newDevApp(t, func(c *config.Config) { c.WSMaxPerIP = 1; c.TrustedProxyCIDRs = proxies })
		for _, ip := range []string{"203.0.113.1", "203.0.113.2", "203.0.113.1"} {
			assertLegacyWSRetired(t, srv, ip)
		}
	}
}

func TestDevelopmentRejectsInvalidTrustedProxy(t *testing.T) {
	cfg := testConfig(t)
	cfg.TrustedProxyCIDRs = "not-a-cidr"
	if _, err := app.New(context.Background(), cfg, sharedlog.New("candles-test", "error")); err == nil || !strings.Contains(err.Error(), "TRUSTED_PROXY_CIDRS") {
		t.Fatalf("err = %v", err)
	}
}
