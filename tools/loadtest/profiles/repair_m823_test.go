package profiles

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/paxeer-network/pax-load/config"
	"github.com/paxeer-network/pax-load/config/cli"
	"github.com/paxeer-network/pax-load/stats"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// The peer implements JSON-RPC over a real loopback HTTP transport. It decodes
// and verifies transactions produced by the production generator and sender.
type repair823RPC struct {
	mu           sync.Mutex
	chainCalls   int
	transactions int
	unexpected   []string
	invalid      []string
	server       *httptest.Server
}

func repair823Endpoint(t *testing.T, chainResult any) *repair823RPC {
	t.Helper()
	peer := &repair823RPC{}
	peer.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peer.mu.Lock()
		defer peer.mu.Unlock()
		var request struct {
			Version string            `json:"jsonrpc"`
			ID      json.RawMessage   `json:"id"`
			Method  string            `json:"method"`
			Params  []json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			peer.invalid = append(peer.invalid, err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if r.Method != http.MethodPost || request.Version != "2.0" || r.URL.Path != "/" {
			peer.invalid = append(peer.invalid, "invalid JSON-RPC transport")
		}
		response := map[string]any{"jsonrpc": "2.0", "id": request.ID}
		switch request.Method {
		case "eth_chainId":
			peer.chainCalls++
			response["result"] = chainResult
		case "eth_sendRawTransaction":
			peer.transactions++
			if peer.chainCalls != 1 {
				peer.invalid = append(peer.invalid, "submission preceded chain identity validation")
			}
			var raw string
			var tx ethtypes.Transaction
			var err error
			if len(request.Params) != 1 {
				err = fmt.Errorf("expected one raw transaction, got %d params", len(request.Params))
			} else if err = json.Unmarshal(request.Params[0], &raw); err == nil {
				var data []byte
				data, err = hexutil.Decode(raw)
				if err == nil {
					err = tx.UnmarshalBinary(data)
				}
			}
			if err == nil {
				_, err = ethtypes.Sender(ethtypes.LatestSignerForChainID(tx.ChainId()), &tx)
			}
			if err == nil && (tx.ChainId().Int64() != 125 || tx.To() == nil || tx.Gas() != 21000) {
				err = fmt.Errorf("unexpected signed transfer: chain=%s to=%v gas=%d", tx.ChainId(), tx.To(), tx.Gas())
			}
			if err != nil {
				peer.invalid = append(peer.invalid, err.Error())
				response["error"] = map[string]any{"code": -32602, "message": err.Error()}
			} else {
				response["result"] = tx.Hash().Hex()
			}
		default:
			peer.unexpected = append(peer.unexpected, request.Method)
			response["error"] = map[string]any{"code": -32601, "message": "unsupported method"}
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			peer.invalid = append(peer.invalid, err.Error())
		}
	}))
	t.Cleanup(peer.server.Close)
	return peer
}

func (p *repair823RPC) assert(t *testing.T, chainCalls int, sent bool) {
	t.Helper()
	p.mu.Lock()
	defer p.mu.Unlock()
	require.Equal(t, chainCalls, p.chainCalls)
	require.Empty(t, p.unexpected, "preflight must precede deployment and file-writer RPC")
	require.Empty(t, p.invalid)
	if sent {
		require.Positive(t, p.transactions, "real signed transfers must reach every endpoint")
	} else {
		require.Zero(t, p.transactions, "failed preflight must prevent transaction submission")
	}
}

func repair823Command(t *testing.T, cfg *config.LoadConfig, flags ...string) *cobra.Command {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)
	data, err := json.Marshal(cfg)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "profile.json")
	require.NoError(t, os.WriteFile(path, data, 0o600))
	cmd, err := NewCommand()
	require.NoError(t, err)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SilenceErrors, cmd.SilenceUsage = true, true
	cmd.SetArgs(append([]string{"--config", path, "--metricsListenAddr=127.0.0.1:0"}, flags...))
	return cmd
}

func repair823Profile(t *testing.T, name string) *config.LoadConfig {
	t.Helper()
	cfg, err := cli.LoadConfig(name + ".json")
	require.NoError(t, err)
	require.EqualValues(t, 125, cfg.ChainID)
	require.Equal(t, "qualification-chain-125-"+name, cfg.PaxChainID)
	for _, endpoint := range cfg.Endpoints {
		require.Regexp(t, `^http://127\.0\.0\.1:[0-9]+$`, endpoint, "shipped profiles target an operator's local root RPC")
	}
	return cfg
}

func repair823ProveProfile(t *testing.T, name string) {
	t.Helper()
	shipped := repair823Profile(t, name)
	for badAt := range shipped.Endpoints {
		t.Run(fmt.Sprintf("mismatch_at_endpoint_%d", badAt+1), func(t *testing.T) {
			cfg := *shipped
			cfg.Endpoints = nil
			var peers []*repair823RPC
			for i := range shipped.Endpoints {
				chain := "0x7d"
				if i == badAt {
					chain = "0xae3f2" // The former fixture chain 713714.
				}
				peer := repair823Endpoint(t, chain)
				peers = append(peers, peer)
				cfg.Endpoints = append(cfg.Endpoints, peer.server.URL)
			}
			cmd := repair823Command(t, &cfg, "--nodes=1")
			err := cmd.ExecuteContext(context.Background())
			require.ErrorContains(t, err, "eth_chainId mismatch: configured 125, actual 713714")
			require.ErrorContains(t, err, fmt.Sprintf("endpoint %d (%s)", badAt+1, cfg.Endpoints[badAt]))
			for i, peer := range peers {
				calls := 0
				if i <= badAt {
					calls = 1
				}
				peer.assert(t, calls, false)
			}
		})
	}
	t.Run("all_endpoints_match", func(t *testing.T) {
		cfg := *shipped
		cfg.Endpoints = nil
		var peers []*repair823RPC
		for range shipped.Endpoints {
			peer := repair823Endpoint(t, "0x7d")
			peers = append(peers, peer)
			cfg.Endpoints = append(cfg.Endpoints, peer.server.URL)
		}
		// Exercise the production validator without deploying contract workloads.
		// The executable wiring is exercised above; the full accepted CLI runs below.
		require.NoError(t, validateEndpoints(context.Background(), &cfg))
		for _, peer := range peers {
			peer.assert(t, 1, false)
		}
	})
}

func TestRepairM_823_ProfileChainAutobakeEVMTransfer(t *testing.T) {
	repair823ProveProfile(t, "autobake_evm_transfer")
}

func TestRepairM_823_ProfileChainConflict(t *testing.T) {
	repair823ProveProfile(t, "conflict")
}

func TestRepairM_823_ProfileChainEVMTransfer(t *testing.T) {
	repair823ProveProfile(t, "evm_transfer")
}

func TestRepairM_823_ProfileChainEVMTransferWriteFile(t *testing.T) {
	repair823ProveProfile(t, "evm_transfer_write_file")
}

func TestRepairM_823_ProfileChainLoadtestDefault(t *testing.T) {
	repair823ProveProfile(t, "loadtest_default")
}

func TestRepairM_823_ProfileChainLocal(t *testing.T) {
	repair823ProveProfile(t, "local")
}

func TestRepairM_823_ProfileChainLocalDocker(t *testing.T) {
	repair823ProveProfile(t, "local_docker")
}

func TestRepairM_823_PreflightErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result any
	}{
		{"missing_result", nil},
		{"decimal_string", "125"},
		{"numeric_result", 125},
		{"malformed_hex", "0xzz"},
		{"zero_chain", "0x0"},
		{"oversized_chain", "0x100000000000000000000000000000000000000000000007d"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := repair823Profile(t, "evm_transfer")
			peer := repair823Endpoint(t, tc.result)
			cfg.Endpoints = []string{peer.server.URL}
			cmd := repair823Command(t, cfg, "--dry-run")
			require.ErrorContains(t, cmd.ExecuteContext(context.Background()), "eth_chainId")
			peer.assert(t, 1, false)
		})
	}
	for _, chainID := range []int64{0, -1} {
		t.Run(fmt.Sprintf("invalid_configured_chain_%d", chainID), func(t *testing.T) {
			cfg := repair823Profile(t, "evm_transfer")
			peer := repair823Endpoint(t, "0x7d")
			cfg.ChainID, cfg.Endpoints = chainID, []string{peer.server.URL}
			require.ErrorContains(t, repair823Command(t, cfg).Execute(), "chainId must be positive")
			peer.assert(t, 0, false)
		})
	}
	for _, mode := range []string{"http_error", "rpc_error", "canceled", "unreachable"} {
		t.Run(mode, func(t *testing.T) {
			endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					ID     json.RawMessage `json:"id"`
					Method string          `json:"method"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Error(err)
					return
				}
				if req.Method != "eth_chainId" {
					t.Errorf("preflight allowed workload RPC %s", req.Method)
				}
				if mode == "http_error" {
					http.Error(w, "unavailable", http.StatusServiceUnavailable)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID,
					"error": map[string]any{"code": -32601, "message": "chain identity unavailable"}})
			}))
			defer endpoint.Close()
			cfg := repair823Profile(t, "evm_transfer")
			cfg.Endpoints = []string{endpoint.URL}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "canceled" {
				cancel()
			}
			if mode == "unreachable" {
				endpoint.Close()
			}
			err := repair823Command(t, cfg).ExecuteContext(ctx)
			require.ErrorContains(t, err, "chain preflight: endpoint 1")
			if mode == "canceled" {
				require.ErrorIs(t, err, context.Canceled)
			}
		})
	}
}

var repair823Completed = regexp.MustCompile(`throughput tps=[^,]+, txs=([0-9]+),`)

type repair823Output struct {
	mu        sync.Mutex
	cancel    context.CancelFunc
	ready     func() bool
	completed bool
}

func (o *repair823Output) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if match := repair823Completed.FindSubmatch(p); match != nil {
		count, err := strconv.Atoi(string(match[1]))
		if err == nil && count >= 10 && o.ready() {
			o.completed = true
			o.cancel()
		}
	}
	return len(p), nil
}

func repair823RunTransfers(t *testing.T, cfg *config.LoadConfig, flags ...string) error {
	t.Helper()
	registerer, gatherer := prometheus.DefaultRegisterer, prometheus.DefaultGatherer
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer, prometheus.DefaultGatherer = registry, registry
	defer func() {
		prometheus.DefaultRegisterer, prometheus.DefaultGatherer = registerer, gatherer
	}()
	first, second := repair823Endpoint(t, "0x7d"), repair823Endpoint(t, "0x7d")
	cfg.Endpoints = []string{first.server.URL, second.server.URL}
	// Each transfer advances the round-robin pool twice (sender and receiver).
	// An odd pool visits every account as a sender instead of pinning one sender.
	cfg.Accounts = &config.AccountConfig{Accounts: 129}
	cfg.Settings.Workers, cfg.Settings.BufferSize = 1, 16
	cmd := repair823Command(t, cfg, append([]string{"--tps=1000", "--stats-interval=20ms"}, flags...)...)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	output := &repair823Output{cancel: cancel, ready: func() bool {
		for _, peer := range []*repair823RPC{first, second} {
			peer.mu.Lock()
			count := peer.transactions
			peer.mu.Unlock()
			// Wait for a prior request to have reached collection on each shard.
			if count < 2 {
				return false
			}
		}
		return true
	}}
	previous := log.Writer()
	log.SetOutput(output)
	defer log.SetOutput(previous)
	err := cmd.ExecuteContext(ctx)
	output.mu.Lock()
	completed := output.completed
	output.mu.Unlock()
	require.True(t, completed, "real CLI did not collect ten transactions")
	require.ErrorIs(t, ctx.Err(), context.Canceled, "real CLI timed out")
	first.assert(t, 1, true)
	second.assert(t, 1, true)
	return err
}

func TestRepairM_823_ProfileOutputPath(t *testing.T) {
	profile, err := filepath.Abs("autobake_evm_transfer.json")
	require.NoError(t, err)
	for _, layout := range []string{"local-workdir", "container-mounted-workdir"} {
		t.Run(layout, func(t *testing.T) {
			cfg, err := cli.LoadConfig(profile)
			require.NoError(t, err)
			require.Equal(t, "evm_transfer_report.json", cfg.Settings.ReportPath)
			require.False(t, filepath.IsAbs(cfg.Settings.ReportPath))
			cwd := filepath.Join(t.TempDir(), layout)
			require.NoError(t, os.Mkdir(cwd, 0o700))
			t.Chdir(cwd)
			err = repair823RunTransfers(t, cfg)
			require.ErrorIs(t, err, context.Canceled)
			require.False(t, errors.Is(err, cli.ErrPerformanceBounds), "%v", err)
			data, err := os.ReadFile(cfg.Settings.ReportPath)
			require.NoError(t, err)
			var report stats.FinalStats
			require.NoError(t, json.Unmarshal(data, &report))
			require.GreaterOrEqual(t, report.LoadTestStatistics.TotalTxs, uint64(10))
			require.Equal(t, report.LoadTestStatistics.TotalTxs, report.OverallPerformance.TotalTransactions)
			require.GreaterOrEqual(t, report.OverallPerformance.AverageTPS, cfg.Bounds.MinAverageTPS)
			require.Equal(t, report.LoadTestStatistics.TotalTxs, report.ScenarioDistribution["evmtransfer"])
			require.Len(t, report.EndpointPerformance, 2)
			for _, endpoint := range report.EndpointPerformance {
				require.Positive(t, endpoint.SampleCount)
			}
			expected, err := json.MarshalIndent(&report, "", "  ")
			require.NoError(t, err)
			require.Equal(t, expected, data)
		})
	}
}

func TestRepairM_823_ReportParentError(t *testing.T) {
	cfg := repair823Profile(t, "autobake_evm_transfer")
	t.Chdir(t.TempDir())
	missing := filepath.Join("missing-parent", "report.json")
	err := repair823RunTransfers(t, cfg, "--report-path="+missing)
	require.ErrorIs(t, err, os.ErrNotExist)
	require.ErrorContains(t, err, "write report "+missing)
	require.False(t, strings.Contains(err.Error(), "/home/ubuntu"))
	_, err = os.Stat("evm_transfer_report.json")
	require.ErrorIs(t, err, os.ErrNotExist, "report errors must not fall back to the profile path")
}
