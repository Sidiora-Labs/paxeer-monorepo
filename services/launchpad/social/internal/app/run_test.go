package app

import (
	"context"
	"net"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/internaltest"
)

func TestRunServesHealthAndDrainsOnCancellation(t *testing.T) {
	admin := os.Getenv("SOCIAL_FIXTURE_POSTGRES_DSN")
	if admin == "" {
		admin = "postgres://kindle_fixture:kindle_fixture_password@127.0.0.1:25432/kindle_fixture?sslmode=disable"
	}
	dsn := internaltest.DevelopmentPostgresDSN(t, admin)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	redisURL := os.Getenv("SOCIAL_FIXTURE_REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://:kindle_fixture_password@127.0.0.1:26379/9"
	}
	t.Setenv("NODE_ENV", "development")
	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("REDIS_URL", redisURL)
	t.Setenv("BIND_ADDRESS", "127.0.0.1")
	t.Setenv("PORT", strconv.Itoa(port))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Run(ctx) }()
	address := "http://127.0.0.1:" + strconv.Itoa(port) + "/health/live"
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(10 * time.Second)
	ready := false
	for time.Now().Before(deadline) {
		response, err := client.Get(address)
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				ready = true
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		cancel()
		t.Fatalf("social health listener did not become ready: %v", <-done)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("social shutdown: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("social shutdown did not drain")
	}
}
