package app

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/internaltest"
)

func TestDevelopmentStartupRejectsUnavailableDependencies(t *testing.T) {
	pool := internaltest.NewPostgres(t)
	redisURL := internaltest.NewRedisURL(t)
	base := config.Config{
		NodeEnv: "development", DatabaseURL: pool.Config().ConnString(), RedisURL: redisURL,
		S3Endpoint: "http://127.0.0.1:9", S3Bucket: "user-assets",
		S3AccessKeyID: "development", S3SecretAccessKey: "development", S3Region: "us-east-1",
		SecretsRefreshSeconds: 30,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, tc := range []struct {
		name string
		edit func(*config.Config)
		want string
	}{
		{"database", func(c *config.Config) { c.DatabaseURL = "postgres://user:password@127.0.0.1:1/user" }, "migrate:"},
		{"redis", func(c *config.Config) { c.RedisURL = "invalid redis URL" }, "redis:"},
		{"object storage", func(c *config.Config) { c.S3Bucket = "" }, "storage:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base
			tc.edit(&cfg)
			a, err := New(context.Background(), cfg, logger)
			if a != nil || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("startup with unavailable %s = %v, %v", tc.name, a, err)
			}
		})
	}
}

func TestProductionStartupRejectsIncompleteConfiguration(t *testing.T) {
	pool := internaltest.NewPostgres(t)
	redisURL := internaltest.NewRedisURL(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	base := config.Config{
		NodeEnv: "production", DatabaseURL: pool.Config().ConnString(), RedisURL: redisURL,
		S3Endpoint: "https://example.org", S3Bucket: "user-assets", S3AccessKeyID: "user", S3SecretAccessKey: "secret",
		SecretsRefreshSeconds: 30, SecretsProvider: "env", RuntimeMetricsEnvironment: "test", RuntimeZone: "test",
	}
	for _, tc := range []struct {
		name string
		edit func(*config.Config)
		want string
	}{
		{"invalid refresh", func(c *config.Config) { c.SecretsRefreshSeconds = 0 }, "secret_refresh_bound_invalid"},
		{"missing secret provider", func(c *config.Config) { c.SecretsProvider = "unsupported" }, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base
			tc.edit(&cfg)
			a, err := New(context.Background(), cfg, logger)
			if a != nil || err == nil || (tc.want != "" && !strings.Contains(err.Error(), tc.want)) {
				t.Fatalf("production startup with %s = %v, %v", tc.name, a, err)
			}
		})
	}
}
