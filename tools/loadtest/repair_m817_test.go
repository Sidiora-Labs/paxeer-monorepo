package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
	"io"
	"math"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/paxeer-network/pax-load/config"
	"github.com/paxeer-network/pax-load/config/cli"
	"github.com/paxeer-network/pax-load/generator"
	"github.com/paxeer-network/pax-load/internal/scaffold"
	fixture "github.com/paxeer-network/pax-load/internal/scaffoldfixture"
	"github.com/paxeer-network/pax-load/profiles"
	"github.com/paxeer-network/pax-load/sender"
	"github.com/paxeer-network/pax-load/stats"
	"github.com/paxeer-network/pax-load/types"
	"github.com/paxeer-network/pax-load/utils/service"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func repair817Read(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}

func TestRepairM_817_LoadtestToolchainImage(t *testing.T) {
	source := string(repair817Read(t, "Dockerfile"))
	stages := regexp.MustCompile(`(?m)^FROM (\S+)`).FindAllStringSubmatch(source, -1)
	require.Len(t, stages, 2)
	require.Equal(t, "docker.io/golang:1.25.8-bookworm@sha256:4557cf171e3cdf5053a298d5171b1a5f5734d920260c25f22c79e94760eb2084", stages[0][1])
	require.Regexp(t, `@sha256:[0-9a-f]{64}$`, stages[1][1])
	for _, fragment := range []string{"GOTOOLCHAIN=local", "CGO_ENABLED=0", `test "$(go env GOVERSION)" = "go1.25.8"`, "-mod=readonly -trimpath -buildvcs=false", "-buildid="} {
		require.Contains(t, source, fragment)
	}
	require.Contains(t, string(repair817Read(t, "go.mod")), "\ngo 1.25.0\n")
	require.Less(t, strings.Index(source, "go env GOVERSION"), strings.Index(source, "go mod download"))
}

func TestRepairM_817_BindingToolProvenance(t *testing.T) {
	source := string(repair817Read(t, "Makefile"))
	module := string(repair817Read(t, "go.mod"))
	version := regexp.MustCompile(`github.com/ethereum/go-ethereum (v\S+)`).FindStringSubmatch(module)
	require.Len(t, version, 2)
	require.Contains(t, source, "ABIGEN_VERSION := "+version[1])
	for _, fragment := range []string{"TOOLS_DIR := $(CURDIR)/.tools", "SOLC := $(TOOLS_DIR)/solc", "ABIGEN := $(TOOLS_DIR)/abigen", "SOLC_SHA256 := 7a5c1d3dc9a8eba62bb2ec37192c9178ae5fe8a54a56e5573fd3c9c17cd9eb48", "sha256sum --check --strict", "mktemp", "GOBIN=", "go version -m", "$@.provenance", "$(SOLC) --version; $(ABIGEN) --version; go version; sha256sum"} {
		require.Contains(t, source, fragment)
	}
	require.NotContains(t, source, "@latest")
	require.NotContains(t, source, "| bash")
	require.NotContains(t, source, "/tmp/solc")
	build := source[strings.Index(source, "build-cli: |"):strings.Index(source, "# Install the paxload CLI")]
	require.Contains(t, build, "go build -mod=readonly")
	require.NotContains(t, build, "go mod tidy")
	require.NotContains(t, build, "install-tools")
	require.Contains(t, source, "tidy:\n\tgo mod tidy")
	// Provenance covers both the ABI/bin producer and binding consumer bytes.
	require.Contains(t, source, `"$<" "$(BUILD_DIR)/$*.bin" "$@"`)
}

// This committed transcript uses a real loopback HTTP transport. It validates
// signatures and chain identity from production-generated signed transactions.
type repair817Peer struct {
	mu       sync.Mutex
	requests int
	invalid  []string
	reject   bool
	chain    string
	delay    time.Duration
	server   *httptest.Server
}

func repair817Endpoint(t *testing.T, reject bool) *repair817Peer {
	t.Helper()
	p := &repair817Peer{reject: reject, chain: "0x7d"}
	p.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Version string            `json:"jsonrpc"`
			ID      json.RawMessage   `json:"id"`
			Method  string            `json:"method"`
			Params  []json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		p.mu.Lock()
		response := map[string]any{"jsonrpc": "2.0", "id": req.ID}
		switch req.Method {
		case "eth_chainId":
			response["result"] = p.chain
		case "eth_blockNumber":
			response["result"] = "0x40"
		case "eth_getTransactionReceipt":
			var hash common.Hash
			if len(req.Params) != 1 || json.Unmarshal(req.Params[0], &hash) != nil {
				p.invalid = append(p.invalid, "invalid receipt request")
			}
			response["result"] = &ethtypes.Receipt{Type: ethtypes.DynamicFeeTxType, Status: ethtypes.ReceiptStatusSuccessful, TxHash: hash, BlockHash: common.HexToHash("0x41"), BlockNumber: big.NewInt(65), GasUsed: 21000, CumulativeGasUsed: 21000, Logs: []*ethtypes.Log{}, EffectiveGasPrice: big.NewInt(1)}
		case "eth_sendRawTransaction":
			var raw hexutil.Bytes
			var tx ethtypes.Transaction
			var err error
			if len(req.Params) != 1 {
				err = fmt.Errorf("invalid params")
			} else {
				err = json.Unmarshal(req.Params[0], &raw)
			}
			if err == nil {
				err = tx.UnmarshalBinary(raw)
			}
			if err == nil {
				_, err = ethtypes.Sender(ethtypes.LatestSignerForChainID(big.NewInt(125)), &tx)
			}
			if err == nil && (tx.ChainId().Int64() != 125 || tx.Gas() != 21000 || tx.To() == nil) {
				err = fmt.Errorf("unexpected signed transfer")
			}
			if err != nil {
				p.invalid = append(p.invalid, err.Error())
			}
			p.requests++
			if p.reject || err != nil {
				response["error"] = map[string]any{"code": -32000, "message": "fixture admission refused"}
			} else {
				response["result"] = tx.Hash().Hex()
			}
		default:
			p.invalid = append(p.invalid, req.Method)
			response["error"] = map[string]any{"code": -32601, "message": "unrecorded method"}
		}
		delay := p.delay
		p.mu.Unlock()
		if req.Method == "eth_sendRawTransaction" && delay > 0 {
			time.Sleep(delay)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	t.Cleanup(p.server.Close)
	return p
}

func repair817Config(endpoint string) config.LoadConfig {
	return config.LoadConfig{ChainID: 125, Endpoints: []string{endpoint}, Accounts: &config.AccountConfig{Accounts: 2}, Scenarios: []config.Scenario{{Name: "EVMTransfer", Weight: 1}}, Settings: &config.Settings{Workers: 2, BufferSize: 8, StatsInterval: config.Duration(20 * time.Millisecond), TPS: 200}}
}

func repair817WriteConfig(t *testing.T, cfg config.LoadConfig) string {
	t.Helper()
	data, err := json.Marshal(cfg)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "profile.json")
	require.NoError(t, os.WriteFile(path, data, 0600))
	return path
}

type repair817Output struct {
	sync.Mutex
	bytes.Buffer
}

func (w *repair817Output) Write(p []byte) (int, error) {
	w.Lock()
	defer w.Unlock()
	return w.Buffer.Write(p)
}
func (w *repair817Output) String() string { w.Lock(); defer w.Unlock(); return w.Buffer.String() }

type repair817Result struct {
	output string
	exit   int
	report *stats.FinalStats
}

// Execute the already-running test executable's actual main entrypoint. This
// needs neither a subprocess compiler nor an external CLI binary or environment.
func repair817Child() bool {
	for i, arg := range os.Args {
		if arg == "--repair-m817-cli" {
			rootCmd.SetArgs(os.Args[i+1:])
			main()
			return true
		}
	}
	return false
}

func repair817Process(t *testing.T, cfg config.LoadConfig, signal syscall.Signal, beforeStop func(), flags ...string) repair817Result {
	t.Helper()
	profile := repair817WriteConfig(t, cfg)
	report := filepath.Join(t.TempDir(), "report.json")
	args := []string{"-test.run=^TestRepairM_817_SignalExitCancellation$", "--", "--repair-m817-cli", "--config", profile, "--metricsListenAddr=127.0.0.1:0", "--report-path", report}
	args = append(args, flags...)
	executable, err := os.Executable()
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, args...)
	var output repair817Output
	cmd.Stdout, cmd.Stderr = &output, &output
	require.NoError(t, cmd.Start())
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	defer cmd.Process.Kill()
	require.Eventually(t, func() bool { return strings.Contains(output.String(), "throughput tps=") }, 10*time.Second, 5*time.Millisecond, "CLI readiness: %s", output.String())
	if beforeStop != nil {
		beforeStop()
	}
	require.NoError(t, cmd.Process.Signal(signal), output.String())
	select {
	case err = <-done:
	case <-ctx.Done():
		t.Fatalf("CLI did not drain: %s", output.String())
	}
	result := repair817Result{output: output.String(), exit: cmd.ProcessState.ExitCode()}
	if err != nil {
		var exit *exec.ExitError
		require.ErrorAs(t, err, &exit, result.output)
	}
	require.Contains(t, result.output, "Received shutdown signal", result.output)
	require.NoError(t, json.Unmarshal(repair817Read(t, report), &result.report), result.output)
	return result
}

func TestRepairM_817_SignalExitCancellation(t *testing.T) {
	if repair817Child() {
		return
	}
	for _, tc := range []struct {
		name    string
		signal  syscall.Signal
		reject  bool
		minimum uint64
		exit    int
	}{
		{name: "successful_SIGINT", signal: syscall.SIGINT},
		{name: "successful_SIGTERM", signal: syscall.SIGTERM},
		{name: "regressed_SIGTERM", signal: syscall.SIGTERM, minimum: math.MaxUint64, exit: 1},
		{name: "failed_worker_SIGINT", signal: syscall.SIGINT, reject: true, exit: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := repair817Endpoint(t, tc.reject)
			cfg := repair817Config(p.server.URL)
			cfg.Bounds = &config.BoundsConfig{MinTotalTxs: tc.minimum}
			got := repair817Process(t, cfg, tc.signal, nil)
			require.Equal(t, tc.exit, got.exit, got.output)
			require.Positive(t, got.report.OverallPerformance.TotalTransactions)
			p.mu.Lock()
			defer p.mu.Unlock()
			require.Empty(t, p.invalid)
			require.EqualValues(t, p.requests, got.report.OverallPerformance.TotalTransactions, "final report must follow the sender drain")
			if tc.reject {
				require.Contains(t, got.output, "fixture admission refused")
			}
			if tc.minimum > 0 {
				require.Contains(t, got.output, ErrPerformanceBounds.Error())
			}
			if tc.exit == 0 {
				require.NotContains(t, got.output, "Error:")
			}
		})
	}
}

func TestRepairM_817_SenderDrain(t *testing.T) {
	for _, reject := range []bool{false, true} {
		t.Run(fmt.Sprintf("refused_%t", reject), func(t *testing.T) {
			p := repair817Endpoint(t, reject)
			cfg := repair817Config(p.server.URL)
			cfg.MockDeploy = true
			gen, err := generator.NewConfigBasedGenerator(&cfg)
			require.NoError(t, err)
			collector := stats.NewCollector()
			snd, err := sender.NewShardedSender(&cfg, 32, 2, nil)
			require.NoError(t, err)
			snd.SetTrackReceipts(true)
			snd.SetStatsCollector(collector, nil)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- snd.Run(ctx) }()
			for _, tx := range gen.GenerateN(20) {
				require.NoError(t, snd.Send(ctx, tx))
			}
			snd.CloseInput()
			err = <-done
			if reject {
				require.ErrorContains(t, err, "fixture admission refused")
			} else {
				require.NoError(t, err)
			}
			require.EqualValues(t, 20, collector.GetStats().TotalTxs)
			p.mu.Lock()
			require.Equal(t, 20, p.requests)
			require.Empty(t, p.invalid)
			p.mu.Unlock()
			workerStats := snd.GetWorkerStats()
			require.Len(t, workerStats, 1)
			require.Zero(t, workerStats[0].ChannelLength)
			receipts := workerStats[0].Receipts
			require.Zero(t, receipts.Pending)
			require.Zero(t, receipts.Unresolved)
			if reject {
				require.Zero(t, receipts.Accepted)
			} else {
				require.EqualValues(t, 20, receipts.Accepted)
				require.EqualValues(t, 20, receipts.Succeeded)
			}
		})
	}
}

func TestRepairM_817_WriterShutdownTail(t *testing.T) {
	p := repair817Endpoint(t, false)
	cfg := repair817Config(p.server.URL)
	dir := t.TempDir()
	cfg.Settings.TxsDir = dir
	cfg.Settings.TargetGas = 1 << 63
	cfg.Settings.NumBlocksToWrite = 4
	got := repair817Process(t, cfg, syscall.SIGTERM, nil)
	require.Zero(t, got.exit, got.output)
	firstPath := filepath.Join(dir, "74_txs.json")
	first := repair817Read(t, firstPath)
	var batch sender.TxWriteData
	require.NoError(t, json.Unmarshal(first, &batch))
	require.NotEmpty(t, batch.TxPayloads)
	for _, payload := range batch.TxPayloads {
		var tx ethtypes.Transaction
		require.NoError(t, tx.UnmarshalBinary(payload))
		_, err := ethtypes.Sender(ethtypes.LatestSignerForChainID(big.NewInt(125)), &tx)
		require.NoError(t, err)
	}
	// Stop the CLI writer and reopen its real persisted state with a fresh owner.
	cfg.MockDeploy = true
	gen, err := generator.NewConfigBasedGenerator(&cfg)
	require.NoError(t, err)
	tx, ok := gen.Generate()
	require.True(t, ok)
	reopened := sender.NewTxsWriter(1<<63, dir, 74, 4)
	require.NoError(t, reopened.Send(t.Context(), tx))
	require.NoError(t, reopened.Close())
	require.NoError(t, reopened.Close())
	require.ErrorContains(t, reopened.Send(t.Context(), tx), "closed")
	require.Equal(t, first, repair817Read(t, firstPath))
	var tail sender.TxWriteData
	require.NoError(t, json.Unmarshal(repair817Read(t, filepath.Join(dir, "75_txs.json")), &tail))
	require.Equal(t, [][]byte{tx.Payload}, tail.TxPayloads)
	again := sender.NewTxsWriter(1<<63, dir, 74, 4)
	require.NoError(t, again.Send(t.Context(), tx))
	require.NoError(t, again.Close())
	require.FileExists(t, filepath.Join(dir, "76_txs.json"))
	pending, err := filepath.Glob(filepath.Join(dir, "*.pending"))
	require.NoError(t, err)
	require.Empty(t, pending)
	t.Run("finalization_failure_survives_cancellation", func(t *testing.T) {
		cfg.Settings.TxsDir = t.TempDir()
		badPath := filepath.Join(cfg.Settings.TxsDir, "74_txs.json")
		got := repair817Process(t, cfg, syscall.SIGINT, func() { require.NoError(t, os.Mkdir(badPath, 0700)) })
		require.Equal(t, 1, got.exit, got.output)
		require.Contains(t, got.output, "74_txs.json")
		require.Contains(t, got.output, "Error:")
	})
}

func TestRepairM_817_MetricsServerLifecycle(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := occupied.Addr().String()
	_, err = cli.NewMetricsServer(addr)
	require.ErrorContains(t, err, "bind metrics")
	require.NoError(t, occupied.Close())
	for range 2 {
		server, err := cli.NewMetricsServer(addr)
		require.NoError(t, err)
		require.Equal(t, addr, server.Addr())
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() { done <- server.Serve(ctx) }()
		response, err := http.Get("http://" + addr + "/metrics")
		require.NoError(t, err)
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode, string(body))
		cancel()
		require.NoError(t, <-done)
		require.NoError(t, server.Close())
		listener, err := net.Listen("tcp", addr)
		require.NoError(t, err)
		require.NoError(t, listener.Close())
	}
	// A listener stopped independently of the context is a real serve failure.
	server, err := cli.NewMetricsServer(addr)
	require.NoError(t, err)
	require.NoError(t, server.Close())
	require.Error(t, server.Serve(t.Context()))
	// The executable must observe an occupied listener before generating traffic.
	occupied, err = net.Listen("tcp", addr)
	require.NoError(t, err)
	defer occupied.Close()
	p := repair817Endpoint(t, false)
	viper.Reset()
	defer viper.Reset()
	cmd, err := profiles.NewCommand()
	require.NoError(t, err)
	cmd.SetArgs([]string{"--config", repair817WriteConfig(t, repair817Config(p.server.URL)), "--metricsListenAddr", addr})
	require.ErrorContains(t, cmd.ExecuteContext(t.Context()), "bind metrics")
	p.mu.Lock()
	defer p.mu.Unlock()
	require.Zero(t, p.requests)
}

func TestRepairM_817_ServiceCancellation(t *testing.T) {
	require.NoError(t, service.Run(t.Context(), func(ctx context.Context, s service.Scope) error {
		s.SpawnBg(func() error { <-ctx.Done(); return fmt.Errorf("stopped: %w", context.Canceled) })
		return nil
	}))
	sentinel := errors.New("tail publication failed")
	err := service.Run(t.Context(), func(ctx context.Context, s service.Scope) error {
		s.SpawnBg(func() error { <-ctx.Done(); return errors.Join(context.Canceled, sentinel) })
		return nil
	})
	require.ErrorIs(t, err, sentinel)
	require.False(t, service.IsCancellation(err))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.ErrorIs(t, service.Run(ctx, func(ctx context.Context, _ service.Scope) error { return ctx.Err() }), context.Canceled)
	require.ErrorIs(t, service.Run(t.Context(), func(_ context.Context, _ service.Scope) error { return context.Canceled }), context.Canceled)
}

func TestRepairM_817_ConfigurationAdmission(t *testing.T) {
	cases := []struct {
		name, field string
		mutate      func(*config.LoadConfig)
	}{
		{"chain", "chainId", func(c *config.LoadConfig) { c.ChainID = 0 }},
		{"endpoint", "endpoints[0]", func(c *config.LoadConfig) { c.Endpoints = []string{"file:///tmp/rpc"} }},
		{"too_many_endpoints", "endpoints", func(c *config.LoadConfig) { c.Endpoints = make([]string, config.MaxEndpoints+1) }},
		{"too_many_scenarios", "scenarios", func(c *config.LoadConfig) { c.Scenarios = make([]config.Scenario, config.MaxScenarios+1) }},
		{"combined_accounts", "combined pools", func(c *config.LoadConfig) {
			c.Scenarios[0].Accounts = &config.AccountConfig{Accounts: config.MaxAccounts}
		}},
		{"negative_count", "accounts.count", func(c *config.LoadConfig) { c.Accounts.Accounts = -1 }},
		{"bounded_count", "accounts.count", func(c *config.LoadConfig) { c.Accounts.Accounts = config.MaxAccounts + 1 }},
		{"empty_fixed_pool", "accounts.count", func(c *config.LoadConfig) { c.Accounts.Accounts = 0 }},
		{"low_rate", "newAccountRate", func(c *config.LoadConfig) { c.Accounts.NewAccountRate = -.01 }},
		{"high_rate", "newAccountRate", func(c *config.LoadConfig) { c.Accounts.NewAccountRate = 1.01 }},
		{"scenario_pool", "scenarios[0].accounts.count", func(c *config.LoadConfig) { c.Scenarios[0].Accounts = &config.AccountConfig{Accounts: -1} }},
		{"negative_weight", "weight", func(c *config.LoadConfig) { c.Scenarios[0].Weight = -1 }},
		{"zero_weight_total", "weight", func(c *config.LoadConfig) { c.Scenarios[0].Weight = 0 }},
		{"weight_overflow", "weight", func(c *config.LoadConfig) { c.Scenarios[0].Weight = int(config.MaxScenarioWeight) + 1 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := repair817Config("http://127.0.0.1:8545")
			tc.mutate(&cfg)
			_, err := cli.LoadConfig(repair817WriteConfig(t, cfg))
			require.ErrorContains(t, err, tc.field)
		})
	}
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		cfg := repair817Config("http://127.0.0.1:8545")
		cfg.Accounts.NewAccountRate = value
		require.ErrorContains(t, cfg.Validate(), "newAccountRate")
	}
	for _, tc := range []struct {
		name, field string
		mutate      func(*config.Settings)
	}{
		{"negative_workers", "workers", func(s *config.Settings) { s.Workers = -1 }},
		{"zero_workers", "workers", func(s *config.Settings) { s.Workers = 0 }},
		{"bounded_workers", "workers", func(s *config.Settings) { s.Workers = config.MaxWorkers + 1 }},
		{"bounded_buffer", "bufferSize", func(s *config.Settings) { s.BufferSize = config.MaxBufferSize + 1 }},
		{"zero_buffer", "bufferSize", func(s *config.Settings) { s.BufferSize = 0 }},
		{"negative_tps", "tps", func(s *config.Settings) { s.TPS = -1 }},
		{"nan_tps", "tps", func(s *config.Settings) { s.TPS = math.NaN() }},
		{"zero_duration", "statsInterval", func(s *config.Settings) { s.StatsInterval = 0 }},
		{"negative_duration", "statsInterval", func(s *config.Settings) { s.StatsInterval = -1 }},
		{"zero_writer_gas", "targetGas", func(s *config.Settings) { s.TxsDir = "txs"; s.TargetGas = 0 }},
		{"negative_blocks", "numBlocksToWrite", func(s *config.Settings) { s.TxsDir = "txs"; s.NumBlocksToWrite = -1 }},
		{"zero_blocks", "numBlocksToWrite", func(s *config.Settings) { s.TxsDir = "txs"; s.NumBlocksToWrite = 0 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := config.DefaultSettings()
			tc.mutate(&s)
			require.ErrorContains(t, s.Validate(1), tc.field)
		})
	}
	s := config.DefaultSettings()
	s.Workers = config.MaxWorkers
	require.ErrorContains(t, s.Validate(5), "participants")
	s = config.DefaultSettings()
	s.BufferSize = config.MaxBufferSize
	require.ErrorContains(t, s.Validate(11), "queued transactions")
	for _, tc := range []struct{ name, field, pickers string }{
		{"zero_gas", "gasPicker", `"gasPicker":{"Name":"fixed","Gas":0}`},
		{"inverted_gas", "gasPicker", `"gasPicker":{"Name":"random","Min":10,"Max":9}`},
		{"empty_picker", "gasPicker", `"gasPicker":{}`},
		{"fee_tip", "gasFeeCapPicker", `"gasFeeCapPicker":{"Name":"fixed","Gas":1},"gasTipCapPicker":{"Name":"fixed","Gas":2}`},
		{"range_fee_tip", "gasFeeCapPicker", `"gasFeeCapPicker":{"Name":"random","Min":10,"Max":20},"gasTipCapPicker":{"Name":"random","Min":5,"Max":11}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := `{"chainId":125,"endpoints":["http://127.0.0.1:8545"],"accounts":{"count":2},"scenarios":[{"name":"EVMTransfer","weight":1,` + tc.pickers + `}]}`
			path := filepath.Join(t.TempDir(), "bad.json")
			require.NoError(t, os.WriteFile(path, []byte(raw), 0600))
			_, err := cli.LoadConfig(path)
			require.ErrorContains(t, err, tc.field)
		})
	}
	t.Run("flags_refused_before_startup", func(t *testing.T) {
		p := repair817Endpoint(t, false)
		viper.Reset()
		defer viper.Reset()
		cmd, err := profiles.NewCommand()
		require.NoError(t, err)
		cmd.SetArgs([]string{"--config", repair817WriteConfig(t, repair817Config(p.server.URL)), "--workers=-1", "--metricsListenAddr=invalid"})
		require.ErrorContains(t, cmd.ExecuteContext(t.Context()), "workers")
		p.mu.Lock()
		defer p.mu.Unlock()
		require.Zero(t, p.requests)
	})
	t.Run("chain_identity_before_generation", func(t *testing.T) {
		p := repair817Endpoint(t, false)
		p.chain = "0x1"
		viper.Reset()
		defer viper.Reset()
		cmd, err := profiles.NewCommand()
		require.NoError(t, err)
		cmd.SetArgs([]string{"--config", repair817WriteConfig(t, repair817Config(p.server.URL)), "--metricsListenAddr=127.0.0.1:0"})
		require.ErrorContains(t, cmd.ExecuteContext(t.Context()), "eth_chainId mismatch")
		p.mu.Lock()
		defer p.mu.Unlock()
		require.Zero(t, p.requests)
	})
	// A fresh-only population is valid, and goes through the real generator.
	cfg := repair817Config("http://127.0.0.1:8545")
	cfg.Accounts = &config.AccountConfig{NewAccountRate: 1}
	cfg.MockDeploy = true
	require.NoError(t, cfg.Validate())
	gen, err := generator.NewConfigBasedGenerator(&cfg)
	require.NoError(t, err)
	tx, ok := gen.Generate()
	require.True(t, ok)
	require.EqualValues(t, 125, tx.EthTx.ChainId().Int64())
}

func TestRepairM_817_ScenarioTemplateSignature(t *testing.T) {
	var rendered bytes.Buffer
	require.NoError(t, scaffold.RenderScenario(&rendered, "ERC20Noop"))
	source := rendered.String()
	require.Contains(t, source, "NewERC20NoopScenario(cfg config.Scenario) TxGenerator")
	require.Contains(t, source, "NewContractScenarioBase[bindings.ERC20Noop](scenario, cfg)")
	require.Contains(t, source, "TODO(deploy)")
	require.Contains(t, source, "TODO(transaction)")
	// Fill in only the explicit contract operations, then compare the complete
	// rendered source with the compiled, committed factory fixture.
	source = strings.Replace(source, "import (", "import (\n \"math/big\"", 1)
	source = strings.Replace(source, "client /* TODO: constructor arguments */", `client, "Repair817Token", "R817"`, 1)
	source = strings.Replace(source, `panic("unimplemented - add contract interaction logic here")`, `return s.contract.Transfer(auth, scenario.Receiver, big.NewInt(1))`, 1)
	expected, err := format.Source([]byte(source))
	require.NoError(t, err)
	require.Equal(t, expected, repair817Read(t, "internal/scaffoldfixture/ERC20Noop.go"))
	factorySource, err := scaffold.RenderFactory(repair817Read(t, "internal/scaffoldfixture/factory.go.input"), []string{"ERC20Noop"})
	require.NoError(t, err)
	factorySource, err = format.Source(factorySource)
	require.NoError(t, err)
	require.Equal(t, factorySource, repair817Read(t, "internal/scaffoldfixture/factory.go"))
	// Drive the renderer through the same Cobra command called by the shell.
	viper.Reset()
	defer viper.Reset()
	cmd, err := profiles.NewCommand()
	require.NoError(t, err)
	output := filepath.Join(t.TempDir(), "ERC20Noop.go")
	cmd.SetArgs([]string{"scaffold", "scenario", "ERC20Noop", output})
	require.NoError(t, cmd.Execute())
	require.Equal(t, rendered.Bytes(), repair817Read(t, output))
	require.NoError(t, os.WriteFile(output, []byte("implemented custom operations"), 0600))
	require.NoError(t, cmd.Execute())
	require.Equal(t, "implemented custom operations", string(repair817Read(t, output)))
	var scenarioCfg config.Scenario
	require.NoError(t, json.Unmarshal([]byte(`{"name":"ERC20Noop","weight":1,"gasPicker":{"Name":"fixed","Gas":31337},"gasFeeCapPicker":{"Name":"fixed","Gas":4000000000},"gasTipCapPicker":{"Name":"fixed","Gas":1000000000}}`), &scenarioCfg))
	scenario := fixture.CreateScenario(scenarioCfg)
	require.Equal(t, "erc20noop", scenario.Name())
	address := common.HexToAddress("0x817")
	require.NoError(t, scenario.Attach(&config.LoadConfig{ChainID: 125, MockDeploy: true}, address))
	account := types.GenerateAccounts(1)[0]
	tx := scenario.Generate(&types.TxScenario{Name: scenario.Name(), Sender: account, Receiver: account.Address})
	require.EqualValues(t, 31337, tx.EthTx.Gas())
	require.EqualValues(t, 4000000000, tx.EthTx.GasFeeCap().Uint64())
	require.EqualValues(t, 1000000000, tx.EthTx.GasTipCap().Uint64())
	require.Equal(t, address, *tx.EthTx.To())
	owner, err := ethtypes.Sender(ethtypes.LatestSignerForChainID(big.NewInt(125)), tx.EthTx)
	require.NoError(t, err)
	require.Equal(t, account.Address, owner)
	transferABI, err := abi.JSON(strings.NewReader(`[{"type":"function","name":"transfer","inputs":[{"name":"to","type":"address"},{"name":"amount","type":"uint256"}],"outputs":[{"type":"bool"}]}]`))
	require.NoError(t, err)
	calldata, err := transferABI.Pack("transfer", account.Address, big.NewInt(1))
	require.NoError(t, err)
	require.Equal(t, calldata, tx.EthTx.Data())
	require.Error(t, scaffold.RenderScenario(io.Discard, "../../bad"))
}

func TestRepairM_817_LoadtestReadme(t *testing.T) {
	readme := string(repair817Read(t, "README.md"))
	for _, text := range []string{"build/paxload", "chain ID **125**", "eth_chainId", "--min-tps", "--max-p99-latency", "--min-total-txs", "--report-path", "settings.reportPath", "top-level", "exit status 0", "exit status 1", "profiles/evm_transfer.json"} {
		require.Contains(t, readme, text)
	}
	require.NotContains(t, readme, "./paxload ")
	// Run the documented shipped profile and its explicit flag/report precedence
	// through the real executable, using an isolated loopback chain-125 transcript.
	p := repair817Endpoint(t, false)
	cfg, err := cli.LoadConfig("profiles/evm_transfer.json")
	require.NoError(t, err)
	cfg.Endpoints = []string{p.server.URL}
	cfg.ReportPath = filepath.Join(t.TempDir(), "top.json")
	cfg.Settings.ReportPath = filepath.Join(t.TempDir(), "nested.json")
	got := repair817Process(t, *cfg, syscall.SIGINT, nil, "--dry-run", "--workers=2", "--buffer-size=8", "--stats-interval=20ms", "--tps=100", "--min-tps=0", "--max-p99-latency=0s", "--min-total-txs=1")
	require.Zero(t, got.exit, got.output)
	require.Positive(t, got.report.OverallPerformance.TotalTransactions)
	require.NoFileExists(t, cfg.ReportPath)
	require.NoFileExists(t, cfg.Settings.ReportPath)
}
