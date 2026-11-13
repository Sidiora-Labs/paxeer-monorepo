package app

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/internaltest"
)

func TestDevelopmentRunServesAndDrains(t *testing.T) {
	_, dsn := internaltest.NewPostgresURL(t)
	redisURL := internaltest.NewRedisURL(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{
		"NODE_ENV": "development", "DATABASE_URL": dsn, "REDIS_URL": redisURL,
		"S3_ENDPOINT": "http://127.0.0.1:9", "S3_BUCKET": "dev-assets",
		"S3_ACCESS_KEY_ID": "dev", "S3_SECRET_ACCESS_KEY": "dev", "S3_REGION": "us-east-1",
		"LOG_LEVEL": "info", "PORT": strconv.Itoa(port), "BIND_ADDRESS": "127.0.0.1",
		"MAX_LOGO_SIZE_BYTES": "2097152", "MAX_BANNER_SIZE_BYTES": "5242880",
	} {
		t.Setenv(name, value)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, 1)
	go func() { finished <- Run(ctx) }()
	endpoint := fmt.Sprintf("http://127.0.0.1:%d/health/live", port)
	client := http.Client{Timeout: time.Second}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		response, requestErr := client.Get(endpoint)
		if requestErr == nil {
			body, readErr := io.ReadAll(response.Body)
			closeErr := response.Body.Close()
			if readErr != nil || closeErr != nil || response.StatusCode != http.StatusOK || !strings.Contains(string(body), "ok") {
				t.Fatalf("live response: status %d body %s read %v close %v", response.StatusCode, body, readErr, closeErr)
			}
			cancel()
			select {
			case err := <-finished:
				if err != nil {
					t.Fatalf("drain: %v", err)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("service did not drain")
			}
			return
		}
		select {
		case err := <-finished:
			t.Fatalf("service stopped before serving: %v", err)
		case <-time.After(25 * time.Millisecond):
		}
	}
	t.Fatal("service did not become live")
}
