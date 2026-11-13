package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
)

func TestDevelopmentRuntimeServesAndDrains(t *testing.T) {
	dsn, pool := internaltest.NewPostgresWithDSN(t)
	internaltest.EnsureMetadataSchema(t, pool)
	redisURL := internaltest.NewRedisURL(t)
	roles := deployment.Current().Roles
	for key, role := range map[string]string{
		"EVENT_EMITTER_ADDRESS": "EventEmitter", "POOL_REGISTRY_ADDRESS": "PoolRegistry",
		"ROUTER_ADDRESS": "Router", "FACTORY_ADDRESS": "SidioraFactory", "QUOTER_ADDRESS": "Quoter",
		"PROTOCOL_CONFIG_ADDRESS": "ProtocolConfig", "FEE_ACCUMULATOR_ADDRESS": "FeeAccumulator",
		"SIDIORA_NFT_ADDRESS": "SidioraNFT", "FEES_ROUTER_ADDRESS": "FeesRouter", "POOL_BEACON_ADDRESS": "PoolBeacon",
	} {
		t.Setenv(key, roles[role].Address)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{
		"DATABASE_URL": dsn, "REDIS_URL": redisURL, "REDIS_BULL_URL": redisURL,
		"RPC_URL": "http://127.0.0.1:8545", "WEBHOOK_HMAC_SECRET": strings.Repeat("s", 32),
		"NODE_ENV": "development", "HOST": "127.0.0.1", "PORT": fmt.Sprint(port),
	} {
		t.Setenv(key, value)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Run(ctx) }()
	client := &http.Client{Timeout: time.Second}
	url := fmt.Sprintf("http://127.0.0.1:%d/health/live", port)
	var response *http.Response
	for response == nil {
		select {
		case err := <-done:
			t.Fatalf("runtime exited before serving: %v", err)
		default:
		}
		response, err = client.Get(url)
		if err != nil {
			time.Sleep(20 * time.Millisecond)
		}
		if ctx.Err() != nil {
			t.Fatal(ctx.Err())
		}
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("live status: %d", response.StatusCode)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
	address := "0x0000000000000000000000000000000000000101"
	for _, tc := range []struct {
		path   string
		status int
	}{
		{"/stats/batch?pools=" + address, 200},
		{"/stats/" + address, 404},
		{"/stats/" + address + "/transactions", 200},
		{"/stats/cross-token-swaps/" + address, 200},
		{"/stats/cross-token-swaps/token/" + address, 200},
		{"/search?q=unknown", 200},
	} {
		result, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d%s", port, tc.path))
		if err != nil {
			t.Fatal(err)
		}
		var body json.RawMessage
		decodeErr := json.NewDecoder(result.Body).Decode(&body)
		closeErr := result.Body.Close()
		if result.StatusCode != tc.status || decodeErr != nil || closeErr != nil || len(body) == 0 {
			t.Fatalf("%s: status=%d body=%s decode=%v close=%v", tc.path, result.StatusCode, body, decodeErr, closeErr)
		}
	}
	cancel()
	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("runtime drain: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("runtime did not drain")
	}
}

func TestRuntimeJobLoopReportsResults(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	health := &statsRuntimeHealth{failures: map[string]error{}}
	calls := 0
	err := retryStatsJob(ctx, time.Millisecond, health, "refresh", func(context.Context) error {
		calls++
		if calls == 1 {
			return errors.New("temporary")
		}
		cancel()
		return nil
	})
	if !errors.Is(err, context.Canceled) || calls != 2 || health.check(context.Background()) != nil {
		t.Fatalf("retry: calls=%d, result=%v, health=%v", calls, err, health.check(context.Background()))
	}
}
