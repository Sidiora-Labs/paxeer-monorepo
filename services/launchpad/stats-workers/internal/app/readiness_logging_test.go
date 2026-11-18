package app

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
)

func TestStatsReadinessLogsDependencyCause(t *testing.T) {
	pool := internaltest.NewPostgres(t)
	pool.Close()
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	check := loggedStatsCheck(logger, "database", pool.Ping)
	if err := check(context.Background()); err == nil || !strings.Contains(output.String(), "closed pool") || !strings.Contains(output.String(), `"check":"database"`) {
		t.Fatalf("readiness cause: %v %s", err, output.String())
	}
}

func TestStatsProjectionRetryLogsAndRecovers(t *testing.T) {
	redis := internaltest.NewRedis(t)
	if err := redis.Close(); err != nil {
		t.Fatal(err)
	}
	recovered := internaltest.NewRedis(t)
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	defer slog.SetDefault(previous)
	health := &statsRuntimeHealth{failures: map[string]error{}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	err := retryStatsJob(ctx, time.Millisecond, health, "holders", func(ctx context.Context) error {
		calls++
		if calls == 1 {
			return redis.Ping(ctx).Err()
		}
		if err := health.check(ctx); err == nil {
			t.Fatal("failed projection reported ready")
		}
		if err := recovered.Ping(ctx).Err(); err != nil {
			t.Fatal(err)
		}
		cancel()
		return nil
	})
	if !errors.Is(err, context.Canceled) || calls != 2 || health.check(context.Background()) != nil || !strings.Contains(output.String(), "closed") || !strings.Contains(output.String(), `"projection":"holders"`) {
		t.Fatalf("projection recovery: %d %v %s", calls, err, output.String())
	}
}
