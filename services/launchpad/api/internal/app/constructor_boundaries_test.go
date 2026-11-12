package app

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
)

func TestGatewayConstructorValidatesRuntimeDependencies(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if gateway, err := New(ctx, nil, logger); gateway != nil || err == nil || !strings.Contains(err.Error(), "configuration_missing") {
		t.Fatalf("missing configuration: %+v %v", gateway, err)
	}
	dsn, _ := internaltest.NewPostgresWithDSN(t)
	redisURL := internaltest.NewRedisURL(t)
	baseline := config.Config{DatabaseURL: dsn, RedisURL: redisURL, NodeEnv: "test", APIRole: "rest", Port: 3000, RateLimitMax: 100, RateLimitWindowSec: 60, MaxInFlightRequests: 100, WSMaxConnections: 10, WSMaxPerIP: 10, SSEMaxConnections: 10, SSEMaxPerIP: 10, ClientSendBuffer: 16, CoalesceFlushMS: 20}
	for _, tc := range []struct {
		name string
		change func(*config.Config)
	}{
		{"environment", func(c *config.Config) { c.NodeEnv = "invalid" }},
		{"role", func(c *config.Config) { c.APIRole = "invalid" }},
		{"database", func(c *config.Config) { c.DatabaseURL = "not-a-dsn" }},
		{"redis", func(c *config.Config) { c.RedisURL = "not-a-redis-url" }},
		{"admission", func(c *config.Config) { c.AdmissionCapacity = -1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := baseline
			tc.change(&candidate)
			gateway, err := New(ctx, &candidate, logger)
			if gateway != nil {
				gateway.Close()
			}
			if err == nil || gateway != nil {
				t.Fatalf("invalid %s admitted: %+v %v", tc.name, gateway, err)
			}
		})
	}
	baseline.APIRole = "sockets"
	baseline.CORSAllowedOrigins = "https://web.example"
	baseline.TrustedProxyCIDRs = "127.0.0.0/8"
	gateway, err := New(ctx, &baseline, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer gateway.Close()
	response := httptest.NewRecorder()
	gateway.Router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/status", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("sockets role mounted REST status: %d %s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	gateway.Router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v2/stream", nil))
	if response.Code == http.StatusNotFound {
		t.Fatal("sockets role did not mount the stream")
	}
}
