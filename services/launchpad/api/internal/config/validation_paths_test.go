package config

import (
	"strings"
	"testing"
	"time"
)

func TestGatewayEnvironmentValidatesResourceLimits(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://fixture:fixture@127.0.0.1:5432/api?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://127.0.0.1:6379/0")
	t.Setenv("NODE_ENV", "development")
	t.Setenv("LOG_LEVEL", "info")
	t.Setenv("PORT", "3000")
	cfg, err := Load()
	if err != nil || cfg.Port != 3000 || cfg.RateLimitMax != 100 || cfg.NodeEnv != "development" {
		t.Fatalf("gateway configuration: %+v, %v", cfg, err)
	}
	if cfg.RateLimitWindow() != 60*time.Second || cfg.CoalesceFlush() != 100*time.Millisecond {
		t.Fatalf("runtime intervals: rate=%s coalesce=%s", cfg.RateLimitWindow(), cfg.CoalesceFlush())
	}
	for _, check := range []struct {
		name   string
		change func(*Config)
		want   string
	}{
		{"database URL", func(c *Config) { c.DatabaseURL = "not-a-url" }, "DATABASE_URL"},
		{"Redis URL", func(c *Config) { c.RedisURL = "https://127.0.0.1/cache" }, "REDIS_URL"},
		{"log level", func(c *Config) { c.LogLevel = "trace" }, "LOG_LEVEL"},
		{"environment", func(c *Config) { c.NodeEnv = "staging" }, "NODE_ENV"},
		{"port", func(c *Config) { c.Port = 65536 }, "PORT"},
		{"rate limit", func(c *Config) { c.RateLimitMax = 0 }, "API_RATE_LIMIT_MAX"},
		{"rate window", func(c *Config) { c.RateLimitWindowSec = 0 }, "API_RATE_LIMIT_WINDOW_SEC"},
		{"inflight", func(c *Config) { c.MaxInFlightRequests = 0 }, "API_MAX_INFLIGHT_REQUESTS"},
		{"WebSocket cap", func(c *Config) { c.WSMaxConnections = 0 }, "WS_MAX_CONNECTIONS"},
		{"WebSocket IP cap", func(c *Config) { c.WSMaxPerIP = 0 }, "WS_MAX_PER_IP"},
		{"SSE cap", func(c *Config) { c.SSEMaxConnections = 0 }, "SSE_MAX_CONNECTIONS"},
		{"SSE IP cap", func(c *Config) { c.SSEMaxPerIP = 0 }, "SSE_MAX_PER_IP"},
		{"send buffer", func(c *Config) { c.ClientSendBuffer = 0 }, "CLIENT_SEND_BUFFER"},
		{"flush interval", func(c *Config) { c.CoalesceFlushMS = 0 }, "COALESCE_FLUSH_MS"},
	} {
		t.Run(check.name, func(t *testing.T) {
			candidate := cfg
			check.change(&candidate)
			if err := candidate.Validate(); err == nil || !strings.Contains(err.Error(), check.want) {
				t.Fatalf("validation of %s: %v", check.name, err)
			}
		})
	}
}
