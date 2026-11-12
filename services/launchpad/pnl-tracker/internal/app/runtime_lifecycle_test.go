package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/config"
	sharedprocess "github.com/Sidiora-Technologies/KindleLaunch/shared/process"
)

func assertProductionLifecycle(t *testing.T, cfg config.Config) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Port = listener.Addr().(*net.TCPAddr).Port
	cfg.RuntimeInstanceID = "pnl_lifecycle"
	cfg.BindAddress = "127.0.0.1"
	if err = listener.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	done := make(chan error, 1)
	go func() { done <- runConfigured(ctx, cfg, logger) }()
	ready := false
	client := &http.Client{Timeout: 200 * time.Millisecond}
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
		response, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health/ready", cfg.Port))
		if err == nil {
			response.Body.Close()
			if response.StatusCode == 200 {
				ready = true
				break
			}
		}
		select {
		case err := <-done:
			t.Fatal(err)
		case <-time.After(10 * time.Millisecond):
		}
	}
	if !ready {
		t.Fatal("production runtime did not become ready")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(sharedprocess.PropagationDelay + 3*time.Second):
		t.Fatal("production runtime did not shut down")
	}
	for _, action := range []string{"unsupported", "inventory_baseline"} {
		candidate := cfg
		candidate.RuntimeAction = action
		candidate.RuntimeInstanceID = "pnl_lifecycle_" + action
		if err := runConfigured(context.Background(), candidate, logger); err == nil || action == "unsupported" && !strings.Contains(err.Error(), "unsupported runtime action") {
			t.Fatal(action, err)
		}
	}
}
