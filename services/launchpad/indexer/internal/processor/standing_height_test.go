package processor

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/source"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/telemetry"
	"github.com/ethereum/go-ethereum/rpc"
)

func TestStandingHeightGauge(t *testing.T) {
	p := New(Deps{})
	if p.StandingHeightPasses() != 0 {
		t.Fatal("initial standing passes")
	}
	previous := int64(10)
	for _, tc := range []struct {
		height int64
		want   uint64
	}{{10, 1}, {10, 2}, {9, 3}, {10, 0}, {10, 1}, {12, 0}, {12, 1}} {
		p.recordStandingHeight(previous, tc.height)
		if got := p.StandingHeightPasses(); got != tc.want {
			t.Fatalf("height %d after %d: passes=%d, want %d", tc.height, previous, got, tc.want)
		}
		previous = tc.height
	}
	metrics, err := telemetry.New("indexerd", "qualification", "zone-a")
	if err != nil {
		t.Fatal(err)
	}
	handler := p.MetricsHandler(metrics)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "# TYPE indexer_processor_standing_height_passes gauge\nindexer_processor_standing_height_passes 1\n") || !strings.Contains(response.Body.String(), "service=\"indexerd\"") {
		t.Fatalf("registered metrics: %d %s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/metrics", nil))
	if response.Code != http.StatusMethodNotAllowed || response.Body.Len() != 0 {
		t.Fatalf("metrics method: %d %s", response.Code, response.Body.String())
	}
	if New(Deps{}).StandingHeightPasses() != 0 {
		t.Fatal("processor instances share standing passes")
	}
}

func TestStandingHeightLivePasses(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := strings.Split(listener.Addr().String(), ":")[1]
	if err = listener.Close(); err != nil {
		t.Fatal(err)
	}
	ipc := filepath.Join(t.TempDir(), "node.ipc")
	cmd := exec.CommandContext(ctx, "anvil", "--host", "127.0.0.1", "--port", port, "--ipc", ipc, "--chain-id", "125", "--threads", "2", "--silent")
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	var client *rpc.Client
	for client == nil {
		client, err = rpc.DialIPC(ctx, ipc)
		if err != nil {
			if !sleep(ctx, 10*time.Millisecond) {
				t.Fatal(ctx.Err())
			}
		}
	}
	defer client.Close()
	_, pool := internaltest.NewPostgres(t)
	logPath := filepath.Join(t.TempDir(), "processor.txt")
	logFile, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()
	logger := slog.New(slog.NewTextHandler(logFile, nil))
	rpcSource, err := source.NewEVM(ctx, source.EVMOptions{RPCURLs: []string{"http://127.0.0.1:" + port}, Logger: logger})
	if err != nil {
		t.Fatal(err)
	}
	defer rpcSource.Close()
	st := store.New(pool)
	p := New(Deps{Store: st, Source: rpcSource, Head: rpcSource, Decoder: decode.NewDecoder(), ChainID: 125, EventEmitterAddress: "0x0000000000000000000000000000000000000125", PollInterval: 200 * time.Millisecond, Logger: logger})
	runCtx, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- p.Run(runCtx) }()
	defer func() {
		stop()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Errorf("processor stopped: %v", err)
		}
	}()
	wait := func(condition func() bool) {
		t.Helper()
		for !condition() {
			if !sleep(ctx, 5*time.Millisecond) {
				t.Fatal(ctx.Err())
			}
		}
	}
	wait(func() bool { return p.StandingHeightPasses() >= 2 })
	var mined any
	if err := client.CallContext(ctx, &mined, "evm_mine"); err != nil {
		t.Fatal(err)
	}
	wait(func() bool {
		cursor, err := st.GetCursor(ctx, 125)
		if err != nil {
			t.Fatal(err)
		}
		return cursor != nil && *cursor == 1 && p.StandingHeightPasses() == 0
	})
	wait(func() bool { return p.StandingHeightPasses() >= 2 })
	before := p.StandingHeightPasses()
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	wait(func() bool { return p.StandingHeightPasses() >= before+2 })
	output, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), "get head failed, retrying") {
		t.Fatalf("head refusal did not reach retry logging: %s", output)
	}
}
