package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestDevelopmentRunServesHealthAndDrainsOnCancellation(t *testing.T) {
	dsn := os.Getenv("L40_TEST_APP_POSTGRES_DSN")
	redisURL := os.Getenv("L40_TEST_REDIS_URL")
	if dsn == "" || redisURL == "" {
		t.Fatal("dedicated Postgres and Redis fixtures are required")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{
		"NODE_ENV":               "development",
		"DATABASE_URL":           dsn,
		"REDIS_URL":              redisURL,
		"LIVEPEER_PROVIDER_MODE": "unavailable",
		"LIVEPEER_API_URL":       "https://livepeer.studio/api",
		"LOG_LEVEL":              "info",
		"PORT":                   strconv.Itoa(port),
		"BIND_ADDRESS":           "127.0.0.1",
		"MAX_STREAMS_PER_WALLET": "3",
	} {
		t.Setenv(name, value)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	finished := make(chan struct{})
	go func() {
		done <- Run(ctx)
		close(finished)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-finished:
		case <-time.After(10 * time.Second):
			t.Error("service did not drain")
		}
	})
	url := fmt.Sprintf("http://127.0.0.1:%d/health/live", port)
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		response, err := client.Get(url)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode != http.StatusOK {
				t.Fatalf("live health status = %d", response.StatusCode)
			}
			cancel()
			select {
			case err := <-done:
				if err != nil && !errors.Is(err, context.Canceled) {
					t.Fatalf("drain = %v", err)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("service did not drain")
			}
			return
		}
		select {
		case err := <-done:
			t.Fatalf("service stopped before health was ready: %v", err)
		case <-time.After(20 * time.Millisecond):
		}
	}
	t.Fatal("live health was unavailable")
}

func TestDevelopmentRunReportsOccupiedListener(t *testing.T) {
	dsn := os.Getenv("L40_TEST_APP_POSTGRES_DSN")
	redisURL := os.Getenv("L40_TEST_REDIS_URL")
	if dsn == "" || redisURL == "" {
		t.Fatal("dedicated Postgres and Redis fixtures are required")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	for name, value := range map[string]string{
		"NODE_ENV": "development", "DATABASE_URL": dsn, "REDIS_URL": redisURL,
		"LIVEPEER_PROVIDER_MODE": "unavailable", "LIVEPEER_API_URL": "https://livepeer.studio/api",
		"LOG_LEVEL": "info", "PORT": strconv.Itoa(port), "BIND_ADDRESS": "127.0.0.1", "MAX_STREAMS_PER_WALLET": "3",
	} {
		t.Setenv(name, value)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := Run(ctx); !errors.Is(err, syscall.EADDRINUSE) {
		t.Fatalf("occupied listener = %v", err)
	}
}
