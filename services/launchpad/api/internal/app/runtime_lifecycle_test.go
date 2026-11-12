package app_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/app"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
)

func TestRuntimeServesReadyAndDrainsOnCancellation(t *testing.T) {
	dsn, _ := internaltest.NewPostgresWithDSN(t)
	redisURL := internaltest.NewRedisURL(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{
		"DATABASE_URL": dsn, "REDIS_URL": redisURL, "NODE_ENV": "test", "API_ROLE": "rest",
		"PORT": strconv.Itoa(port), "BIND_ADDRESS": "127.0.0.1",
	} {
		t.Setenv(key, value)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, 1)
	go func() { finished <- app.Run(ctx) }()
	client := &http.Client{Timeout: 150 * time.Millisecond}
	endpoint := fmt.Sprintf("http://127.0.0.1:%d/health/ready", port)
	deadline := time.Now().Add(5 * time.Second)
	ready := false
	for time.Now().Before(deadline) {
		response, err := client.Get(endpoint)
		if err == nil {
			if response.StatusCode == http.StatusOK {
				ready = true
			}
			_ = response.Body.Close()
			if ready {
				break
			}
		}
		select {
		case err := <-finished:
			t.Fatalf("runtime stopped before readiness: %v", err)
		default:
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !ready {
		t.Fatal("runtime did not become ready")
	}
	cancel()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatalf("runtime shutdown: %v", err)
		}
	case <-time.After(12 * time.Second):
		t.Fatal("runtime did not drain")
	}
}
