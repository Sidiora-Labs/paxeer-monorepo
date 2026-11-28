package quoter

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
	"github.com/stretchr/testify/require"
)

type localQuoterProcess struct {
	command   *exec.Cmd
	done      chan error
	output    *os.File
	healthURL string
	stopped   bool
}

func startLocalQuoter(t *testing.T, ctx context.Context, cfg *config.Config) *localQuoterProcess {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "perps-quoter")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "../../cmd/perps-quoter")
	output, err := build.CombinedOutput()
	require.NoError(t, err, "compile quoter: %s", output)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	logs, err := os.CreateTemp(t.TempDir(), "quoter-output-*")
	require.NoError(t, err)
	command := exec.Command(binary)
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "PERPS_QUOTER_") || name == "LOCAL_NODE_KEY" || name == "PORT" || name == "PERPS_CHAIN_ID" || name == "PERPS_RPC_URLS" {
			continue
		}
		command.Env = append(command.Env, entry)
	}
	command.Env = append(command.Env,
		"PERPS_CHAIN_ID="+strconv.FormatUint(cfg.ChainID, 10),
		"PERPS_RPC_URLS="+strings.Join(cfg.RPCURLs, ","),
		"PERPS_QUOTER_KEY_FILE="+cfg.Quoting.KeyFile,
		"PERPS_QUOTER_MAX_TRANSACTIONS_PER_ROUND="+strconv.Itoa(cfg.Quoting.MaxTransactionsPerRound),
		"PERPS_QUOTER_MARKETS=0", "PERPS_QUOTER_MODE=live", "PORT="+strconv.Itoa(port))
	command.Stdout, command.Stderr = logs, logs
	require.NoError(t, command.Start())
	p := &localQuoterProcess{command: command, done: make(chan error, 1), output: logs, healthURL: "http://127.0.0.1:" + strconv.Itoa(port) + "/healthz"}
	go func() { p.done <- command.Wait() }()
	t.Cleanup(func() { p.stop(t); _ = logs.Close() })
	return p
}

func (p *localQuoterProcess) logs() string {
	data, _ := os.ReadFile(p.output.Name())
	return string(data)
}

func (p *localQuoterProcess) waitOrders(t *testing.T, ctx context.Context, chain *RPC, priorRound uint64) []Order {
	t.Helper()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	deadline, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	client := &http.Client{Timeout: time.Second}
	for {
		select {
		case err := <-p.done:
			p.stopped = true
			t.Fatalf("quoter exited before resting its quotes: %v; %s", err, p.logs())
		default:
		}
		request, err := http.NewRequestWithContext(deadline, http.MethodGet, p.healthURL, nil)
		require.NoError(t, err)
		response, err := client.Do(request)
		if err == nil {
			var health Health
			decodeErr := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&health)
			_ = response.Body.Close()
			require.NoError(t, decodeErr)
			require.Equal(t, "live", health.Mode)
			for _, market := range health.Markets {
				require.Zero(t, market.Refusals, "quoter refusal: %s; %s", market.LastRefusal, p.logs())
				if market.Market != 0 || len(market.Orders) != 4 {
					continue
				}
				open, err := chain.Orders(deadline)
				require.NoError(t, err, "quoter: %s", p.logs())
				complete := len(open) == 4
				for _, order := range open {
					if priorRound != 0 && order.Round == priorRound {
						complete = false
					}
				}
				if complete {
					return open
				}
			}
		}
		select {
		case <-deadline.Done():
			t.Fatalf("quoter did not rest four current orders: %s", p.logs())
		case <-ticker.C:
		}
	}
}

func (p *localQuoterProcess) transactions(t *testing.T, bound int) ([]int, uint64) {
	t.Helper()
	var rounds []int
	var total uint64
	decoder := json.NewDecoder(strings.NewReader(p.logs()))
	for {
		var entry struct {
			Message      string `json:"msg"`
			Transactions int    `json:"transactions"`
			Order        string `json:"order"`
		}
		err := decoder.Decode(&entry)
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		if entry.Message == "quoter round" {
			require.GreaterOrEqual(t, entry.Transactions, 0)
			require.LessOrEqual(t, entry.Transactions, bound)
			require.Less(t, entry.Transactions, 16)
			rounds = append(rounds, entry.Transactions)
			total += uint64(entry.Transactions)
		}
		if entry.Message == "quoter stop cancel" {
			total++
		}
	}
	require.NotEmpty(t, rounds)
	require.Contains(t, rounds, 4)
	return rounds, total
}

func (p *localQuoterProcess) stop(t *testing.T) {
	t.Helper()
	if p.stopped {
		return
	}
	require.NoError(t, p.command.Process.Signal(syscall.SIGTERM))
	select {
	case err := <-p.done:
		p.stopped = true
		require.NoError(t, err, "quoter shutdown: %s", p.logs())
	case <-time.After(2*time.Minute + 10*time.Second):
		_ = p.command.Process.Kill()
		<-p.done
		p.stopped = true
		t.Fatal("quoter shutdown timed out")
	}
	require.Contains(t, p.logs(), `"msg":"perps-quoter shutdown","mode":"live","ordersLeft":0`)
}
